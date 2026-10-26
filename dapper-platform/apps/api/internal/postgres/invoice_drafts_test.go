package postgres

import (
	"context"
	"errors"
	"math"
	"testing"
)

func verifyInvoiceDrafts(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	create := func() Invoice {
		v, e := s.CreateInvoice(ctx, a, CreateInvoiceInput{CustomerID: customer, Notes: "Isolated original note", Items: []CreateInvoiceItemInput{{Description: "Original isolated service", Quantity: 1, UnitPriceCents: 1000}}})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	draft := create()
	input := UpdateInvoiceDraftInput{Version: 1, TaxCents: 60, Notes: "Revised isolated note", Items: []CreateInvoiceItemInput{{Description: "Revised isolated service", Quantity: 2, UnitPriceCents: 600}}}
	if _, e := s.UpdateInvoiceDraft(ctx, b, draft.ID, input); !errors.Is(e, ErrForbidden) {
		t.Fatal("foreign draft update accepted", e)
	}
	if _, e := s.UpdateInvoiceDraft(ctx, a, draft.ID, UpdateInvoiceDraftInput{Version: 1, Items: []CreateInvoiceItemInput{{Description: "Overflow", Quantity: 2, UnitPriceCents: math.MaxInt64}}}); !errors.Is(e, ErrInvoiceInput) {
		t.Fatal("draft overflow accepted", e)
	}
	if _, e := s.pool.Exec(ctx, `ALTER TABLE invoice_draft_revisions ADD CONSTRAINT isolated_revision_failure CHECK(false) NOT VALID`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.UpdateInvoiceDraft(ctx, a, draft.ID, input); e == nil {
		t.Fatal("draft audit failure committed")
	}
	unchanged, e := s.GetInvoice(ctx, a, draft.ID)
	if e != nil || unchanged.Version != 1 || unchanged.TotalCents != 1000 || unchanged.Items[0].Description != "Original isolated service" {
		t.Fatal("draft audit failure changed header/items", e)
	}
	if _, e = s.pool.Exec(ctx, `ALTER TABLE invoice_draft_revisions DROP CONSTRAINT isolated_revision_failure`); e != nil {
		t.Fatal(e)
	}
	type result struct {
		invoice Invoice
		err     error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() { v, e := s.UpdateInvoiceDraft(ctx, a, draft.ID, input); results <- result{v, e} }()
	}
	wins := 0
	conflicts := 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err == nil && r.invoice.Version == 2 && r.invoice.TotalCents == 1260 {
			wins++
		} else if errors.Is(r.err, ErrInvoiceVersion) {
			conflicts++
		} else {
			t.Fatal("unexpected draft race outcome", r.err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("stale edits did not have exactly one winner")
	}
	history, e := s.InvoiceDraftHistory(ctx, a, draft.ID)
	if e != nil || len(history) != 1 || history[0].Before.Version != 1 || history[0].After.Version != 2 || history[0].Before.Items[0].UnitPriceCents != 1000 || history[0].After.Items[0].Quantity != 2 {
		t.Fatal("draft revision snapshots missing", e)
	}
	if _, e = s.InvoiceDraftHistory(ctx, b, draft.ID); !errors.Is(e, ErrForbidden) {
		t.Fatal("foreign draft history accepted", e)
	}
	var actor string
	if _, e = s.pool.Exec(ctx, `ALTER TABLE invoice_events ADD CONSTRAINT isolated_draft_event_failure CHECK(notes NOT LIKE 'Invoice draft revised%') NOT VALID`); e != nil {
		t.Fatal(e)
	}
	input.Version = 2
	if _, e = s.UpdateInvoiceDraft(ctx, a, draft.ID, input); e == nil {
		t.Fatal("draft event failure committed")
	}
	if _, e = s.pool.Exec(ctx, `ALTER TABLE invoice_events DROP CONSTRAINT isolated_draft_event_failure`); e != nil {
		t.Fatal(e)
	}
	afterRollback, e := s.GetInvoice(ctx, a, draft.ID)
	history, e = s.InvoiceDraftHistory(ctx, a, draft.ID)
	if e != nil || afterRollback.Version != 2 || len(history) != 1 {
		t.Fatal("event failure left a revision/header change", e)
	}
	if e = s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-draft-only-manager') RETURNING id::text`).Scan(&actor); e != nil {
		t.Fatal(e)
	}
	if _, e = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.manage')`, actor, a.OrganizationID, a.LocationID); e != nil {
		t.Fatal(e)
	}
	manager := Scope{ActorID: actor, OrganizationID: a.OrganizationID, LocationID: a.LocationID}
	input.Version = 2
	edited, e := s.UpdateInvoiceDraft(ctx, manager, draft.ID, input)
	if e != nil || edited.Version != 3 || edited.Notes != "" || edited.CustomerEmail != "" || len(edited.Items) > 0 {
		t.Fatal("manage-only edit failed or exposed invoice details", e)
	}
	if _, e = s.InvoiceDraftHistory(ctx, manager, draft.ID); !errors.Is(e, ErrForbidden) {
		t.Fatal("draft edit implied history authority", e)
	}
	issued := issueInvoiceForTest(t, ctx, s, a, draft)
	if _, e = s.UpdateInvoiceDraft(ctx, a, draft.ID, UpdateInvoiceDraftInput{Version: issued.Version, Items: input.Items}); !errors.Is(e, ErrInvoiceStatusConflict) {
		t.Fatal("issued invoice editable", e)
	}
	racing := create()
	issuer, e := s.InvoiceIssuancePreview(ctx, a, racing.ID)
	if e != nil {
		t.Fatal(e)
	}
	updateResult := make(chan error, 1)
	issueResult := make(chan error, 1)
	input.Version = 1
	go func() { _, e := s.UpdateInvoiceDraft(ctx, a, racing.ID, input); updateResult <- e }()
	go func() {
		_, e := s.IssueInvoice(ctx, a, racing.ID, InvoiceIssuanceReview{IssuerID: issuer.ID, IssuerVersion: issuer.Version, InvoiceVersion: 1})
		issueResult <- e
	}()
	updateErr, issueErr := <-updateResult, <-issueResult
	if !((updateErr == nil && errors.Is(issueErr, ErrInvoiceVersion)) || (issueErr == nil && errors.Is(updateErr, ErrInvoiceStatusConflict))) {
		t.Fatal("issuance failed to enforce reviewed draft version", updateErr, issueErr)
	}
	final, e := s.GetInvoice(ctx, a, racing.ID)
	if e != nil {
		t.Fatal(e)
	}
	var subtotal int64
	for _, it := range final.Items {
		subtotal += it.TotalPriceCents
	}
	if subtotal != final.SubtotalCents || final.TotalCents != subtotal+final.TaxCents {
		t.Fatal("draft race produced mixed line/header snapshot")
	}
	voidDraft := create()
	if _, e = s.VoidInvoice(ctx, a, voidDraft.ID, "Cancelled isolated draft"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.UpdateInvoiceDraft(ctx, a, voidDraft.ID, input); !errors.Is(e, ErrInvoiceStatusConflict) {
		t.Fatal("void draft editable", e)
	}
	if _, e = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1`, actor); e != nil {
		t.Fatal(e)
	}
	input.Version = 3
	if _, e = s.UpdateInvoiceDraft(ctx, manager, racing.ID, input); !errors.Is(e, ErrForbidden) {
		t.Fatal("revoked draft manager retained write access", e)
	}
}
