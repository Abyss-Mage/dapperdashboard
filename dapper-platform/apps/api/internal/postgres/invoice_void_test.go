package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func verifyInvoiceVoid(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	var actor string
	if err := s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-invoice-void-operator') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.manage')`, actor, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	manager := Scope{ActorID: actor, OrganizationID: a.OrganizationID, LocationID: a.LocationID}
	create := func() Invoice {
		v, e := s.CreateInvoice(ctx, a, CreateInvoiceInput{CustomerID: customer, Notes: "Isolated private invoice note", Items: []CreateInvoiceItemInput{{Description: "Isolated void test item", Quantity: 1, UnitPriceCents: 1000}}})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	draft := create()
	if _, e := s.VoidInvoice(ctx, b, draft.ID, "foreign scope"); e == nil {
		t.Fatal("foreign void accepted")
	}
	if _, e := s.VoidInvoice(ctx, manager, draft.ID, strings.Repeat("x", 2001)); !errors.Is(e, ErrInvoiceInput) {
		t.Fatal("oversized notes accepted", e)
	}
	if _, e := s.pool.Exec(ctx, `ALTER TABLE invoice_events ADD CONSTRAINT isolated_void_audit_failure CHECK(to_status<>'void') NOT VALID`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.VoidInvoice(ctx, manager, draft.ID, "audit failure"); e == nil {
		t.Fatal("void survived failed audit")
	}
	unchanged, e := s.GetInvoice(ctx, a, draft.ID)
	if e != nil || unchanged.Status != "draft" {
		t.Fatal("audit failure changed invoice", e)
	}
	if _, e = s.pool.Exec(ctx, `ALTER TABLE invoice_events DROP CONSTRAINT isolated_void_audit_failure`); e != nil {
		t.Fatal(e)
	}
	type result struct {
		v Invoice
		e error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() { v, e := s.VoidInvoice(ctx, manager, draft.ID, "Concurrent void"); results <- result{v, e} }()
	}
	for i := 0; i < 2; i++ {
		r := <-results
		if r.e != nil || r.v.Status != "void" || r.v.Notes != "" || r.v.CustomerEmail != "" || len(r.v.Items) != 0 {
			t.Fatal("manage-only void response failed or exposed private data", r.e)
		}
	}
	var count int
	if e = s.pool.QueryRow(ctx, `SELECT count(*) FROM invoice_events WHERE invoice_id=$1 AND to_status='void'`, draft.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("void retry duplicated audit", e)
	}
	paid := issueInvoiceForTest(t, ctx, s, a, create())
	racing := issueInvoiceForTest(t, ctx, s, a, create())
	voidResult := make(chan error, 1)
	cashResult := make(chan error, 1)
	go func() { _, e := s.VoidInvoice(ctx, manager, racing.ID, "Concurrent cancellation"); voidResult <- e }()
	go func() {
		_, _, e := s.RecordPayment(ctx, a, RecordPaymentInput{InvoiceID: racing.ID, AmountCents: 100, Method: "cash", IdempotencyKey: "isolated-void-racing-cash"})
		cashResult <- e
	}()
	voidErr, cashErr := <-voidResult, <-cashResult
	if !((voidErr == nil && errors.Is(cashErr, ErrInvoiceStatusConflict)) || (cashErr == nil && errors.Is(voidErr, ErrInvoiceStatusConflict))) {
		t.Fatal("cash and void did not serialize to exactly one winner", voidErr, cashErr)
	}
	final, e := s.GetInvoice(ctx, a, racing.ID)
	if e != nil || !((final.Status == "void" && final.PaidCents == 0 && len(final.Payments) == 0) || (final.Status == "partially_paid" && final.PaidCents == 100 && len(final.Payments) == 1)) {
		t.Fatal("cash/void race left inconsistent ledger", e)
	}
	if _, _, e = s.RecordPayment(ctx, a, RecordPaymentInput{InvoiceID: paid.ID, AmountCents: 100, Method: "cash", IdempotencyKey: "isolated-void-payment"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.VoidInvoice(ctx, manager, paid.ID, "Cannot erase payment"); !errors.Is(e, ErrInvoiceStatusConflict) {
		t.Fatal("paid void should be typed conflict", e)
	}
	if _, e = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1`, actor); e != nil {
		t.Fatal(e)
	}
	if _, e = s.VoidInvoice(ctx, manager, draft.ID, "revoked retry"); !errors.Is(e, ErrForbidden) {
		t.Fatal("revoked operator retried void", e)
	}
}
