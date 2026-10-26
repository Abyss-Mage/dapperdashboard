package postgres

import (
	"context"
	"errors"
	"testing"
)

func verifyPaymentSafety(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	inv, err := s.CreateInvoice(ctx, a, CreateInvoiceInput{CustomerID: customer, InvoiceNumber: "PAYMENT-RETRY-SAFETY", Notes: "INTERNAL-INVOICE-NOTE", Items: []CreateInvoiceItemInput{{Description: "Synthetic receipt safety", Quantity: 1, UnitPriceCents: 2000}}})
	if err != nil {
		t.Fatal(err)
	}
	inv = issueInvoiceForTest(t, ctx, s, a, inv)
	var writer string
	if err = s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-cash-receipt-writer') RETURNING id::text`).Scan(&writer); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'payments.record')`, writer, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	scope := Scope{ActorID: writer, OrganizationID: a.OrganizationID, LocationID: a.LocationID}
	if _, err = s.GetInvoice(ctx, scope, inv.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("payment authority implicitly granted invoice read")
	}
	input := RecordPaymentInput{InvoiceID: inv.ID, AmountCents: 300, Method: "cash", IdempotencyKey: "isolated-retry-cash-001", Notes: "Synthetic cash receipt"}
	type outcome struct {
		p Payment
		i Invoice
		e error
	}
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() { p, invoice, e := s.RecordPayment(ctx, scope, input); results <- outcome{p, invoice, e} }()
	}
	first, second := <-results, <-results
	if first.e != nil || second.e != nil || first.p.ID == "" || first.p.ID != second.p.ID {
		t.Fatalf("concurrent cash retry failed: %v %v", first.e, second.e)
	}
	if first.i.PaidCents != 300 || first.i.Notes != "" || first.i.CustomerEmail != "" || len(first.i.Items) > 0 {
		t.Fatal("cash response had wrong totals or disclosed invoice details")
	}
	for _, candidate := range []RecordPaymentInput{
		{InvoiceID: inv.ID, AmountCents: 301, Method: "cash", IdempotencyKey: input.IdempotencyKey, Notes: input.Notes},
		{InvoiceID: inv.ID, AmountCents: 300, Method: "cash", IdempotencyKey: input.IdempotencyKey, Notes: "Changed note"},
	} {
		if _, _, err := s.RecordPayment(ctx, scope, candidate); !errors.Is(err, ErrPaymentRequest) {
			t.Fatalf("changed cash request accepted: %v", err)
		}
	}
	if _, _, err := s.RecordPayment(ctx, a, input); !errors.Is(err, ErrPaymentRequest) {
		t.Fatalf("different actor replay exposed receipt: %v", err)
	}
	if _, _, err := s.RecordPayment(ctx, b, input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign scope receipt read: %v", err)
	}
	noKey := input
	noKey.IdempotencyKey = ""
	if _, _, err := s.RecordPayment(ctx, scope, noKey); !errors.Is(err, ErrPaymentInput) {
		t.Fatal("cash request without stable key accepted")
	}
	for index, amount := range []int64{400, 1500} {
		go func(index int, amount int64) {
			candidate := input
			candidate.AmountCents = amount
			candidate.IdempotencyKey = []string{"isolated-distinct-cash-001", "isolated-distinct-cash-002"}[index]
			p, i, e := s.RecordPayment(ctx, scope, candidate)
			results <- outcome{p, i, e}
		}(index, amount)
	}
	successes, overpayments := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.e == nil {
			successes++
		} else if errors.Is(r.e, ErrPaymentOverpay) {
			overpayments++
		} else {
			t.Fatal(r.e)
		}
	}
	if successes != 1 || overpayments != 1 {
		t.Fatal("concurrent cash receipts overpaid invoice or both failed")
	}
	before, err := s.GetInvoice(ctx, a, inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Payments) != 2 || before.PaidCents > before.TotalCents {
		t.Fatal("duplicate cash receipt or overpayment committed")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE invoice_events ADD CONSTRAINT fail_cash_receipt_audit CHECK(notes NOT LIKE 'Payment of%') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	failing := input
	failing.AmountCents = 100
	failing.IdempotencyKey = "isolated-audit-cash-001"
	if _, _, err := s.RecordPayment(ctx, scope, failing); err == nil {
		t.Fatal("cash audit failure committed receipt")
	}
	after, err := s.GetInvoice(ctx, a, inv.ID)
	if err != nil || after.PaidCents != before.PaidCents || len(after.Payments) != len(before.Payments) {
		t.Fatal("cash audit failure did not roll back", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE invoice_events DROP CONSTRAINT fail_cash_receipt_audit`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='payments.record'`, writer); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.RecordPayment(ctx, scope, input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("retry bypassed revoked cash grant: %v", err)
	}
}
