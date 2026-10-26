package postgres

import (
	"context"
	"dapper.local/platform/internal/square"
	"errors"
	"math"
	"testing"
)

func verifyInvoices(t *testing.T, ctx context.Context, s *Store, a, b Scope, customerID string) {
	t.Helper()

	// Grant required actions to a.ActorID
	if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES
($1,$2,$3,'invoices.create'),
($1,$2,$3,'invoices.read'),
($1,$2,$3,'invoices.manage'),
($1,$2,$3,'payments.record'),
($1,$2,$3,'payments.read') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}

	// 1. Create Invoice with 2 items and tax
	invInput := CreateInvoiceInput{
		CustomerID:    customerID,
		InvoiceNumber: "INV-TEST-001",
		TaxCents:      1950, // $19.50 (13% on $150)
		Notes:         "Test invoice for executive detail",
		Items: []CreateInvoiceItemInput{
			{Description: "Full Exterior Paint Correction", Quantity: 1, UnitPriceCents: 10000},    // $100.00
			{Description: "Interior Steam & Leather Treatment", Quantity: 1, UnitPriceCents: 5000}, // $50.00
		},
	}

	inv, err := s.CreateInvoice(ctx, a, invInput)
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}
	if inv.ID == "" || inv.InvoiceNumber != "INV-TEST-001" || inv.Status != "draft" || inv.IssuedAt != nil {
		t.Fatalf("unexpected invoice state: %+v", inv)
	}
	inv = issueInvoiceForTest(t, ctx, s, a, inv)
	if inv.SubtotalCents != 15000 || inv.TaxCents != 1950 || inv.TotalCents != 16950 || inv.PaidCents != 0 || inv.DueCents != 16950 {
		t.Fatalf("unexpected monetary totals on created invoice: %+v", inv)
	}
	if len(inv.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(inv.Items))
	}

	// 2. List Invoices
	list, err := s.ListInvoices(ctx, a)
	if err != nil {
		t.Fatalf("failed to list invoices: %v", err)
	}
	if len(list) < 1 {
		t.Fatalf("expected at least 1 invoice, got %d", len(list))
	}

	// 3. Get Invoice with full items and events
	full, err := s.GetInvoice(ctx, a, inv.ID)
	if err != nil {
		t.Fatalf("failed to get invoice: %v", err)
	}
	if len(full.Items) != 2 || len(full.Events) < 1 {
		t.Fatalf("missing items or events in full invoice: %+v", full)
	}

	for _, method := range []string{"square_card", "square_terminal"} {
		if _, _, err := s.RecordPayment(ctx, a, RecordPaymentInput{InvoiceID: inv.ID, AmountCents: 100, Method: method, ProviderReference: "unverified-provider-reference"}); !errors.Is(err, ErrPaymentProvider) {
			t.Fatalf("manual provider payment accepted: %v", err)
		}
	}
	untouched, err := s.GetInvoice(ctx, a, inv.ID)
	if err != nil || untouched.PaidCents != 0 || len(untouched.Payments) != 0 {
		t.Fatal("rejected provider reference changed invoice", err)
	}
	for _, input := range []CreateInvoiceInput{
		{CustomerID: customerID, Items: []CreateInvoiceItemInput{{Description: "Overflow product", Quantity: 3, UnitPriceCents: math.MaxInt64}}},
		{CustomerID: customerID, TaxCents: 1, Items: []CreateInvoiceItemInput{{Description: "Overflow tax", Quantity: 1, UnitPriceCents: math.MaxInt64}}},
		{CustomerID: customerID, Items: []CreateInvoiceItemInput{{Description: "Overflow sum one", Quantity: 1, UnitPriceCents: math.MaxInt64}, {Description: "Overflow sum two", Quantity: 1, UnitPriceCents: 1}}},
	} {
		if _, err := s.CreateInvoice(ctx, a, input); !errors.Is(err, ErrInvoiceInput) {
			t.Fatalf("overflow invoice accepted: %v", err)
		}
	}
	// 4. Record partial payment of $69.50 (6950 cents) via cash
	pmt1, updated1, err := s.RecordPayment(ctx, a, RecordPaymentInput{
		InvoiceID:      inv.ID,
		AmountCents:    6950,
		Method:         "cash",
		IdempotencyKey: "idem-cash-001",
		Notes:          "Deposit received in cash",
	})
	if err != nil {
		t.Fatalf("failed to record partial payment: %v", err)
	}
	if pmt1.AmountCents != 6950 || pmt1.Method != "cash" || pmt1.Status != "completed" {
		t.Fatalf("unexpected payment record: %+v", pmt1)
	}
	if updated1.PaidCents != 6950 || updated1.DueCents != 10000 || updated1.Status != "partially_paid" {
		t.Fatalf("unexpected updated invoice after partial payment: %+v", updated1)
	}

	// 5. Test idempotency key
	pmt1Dup, _, err := s.RecordPayment(ctx, a, RecordPaymentInput{
		InvoiceID:      inv.ID,
		AmountCents:    6950,
		Method:         "cash",
		IdempotencyKey: "idem-cash-001",
		Notes:          "Deposit received in cash",
	})
	if err != nil {
		t.Fatalf("failed idempotent payment call: %v", err)
	}
	if pmt1Dup.ID != pmt1.ID {
		t.Fatalf("idempotency failed: expected payment ID %s, got %s", pmt1.ID, pmt1Dup.ID)
	}

	// 6. Test overpayment rejection: balance due is 10000 cents, attempt to pay 10001
	_, _, err = s.RecordPayment(ctx, a, RecordPaymentInput{
		InvoiceID:      inv.ID,
		AmountCents:    10001,
		Method:         "cash",
		IdempotencyKey: "idem-overpayment-001",
	})
	if !errors.Is(err, ErrPaymentOverpay) {
		t.Fatalf("expected ErrPaymentOverpay, got: %v", err)
	}

	// 7. Pay remaining balance of 10000 cents via cash
	pmt2, updated2, err := s.RecordPayment(ctx, a, RecordPaymentInput{
		InvoiceID:         inv.ID,
		AmountCents:       10000,
		Method:            "cash",
		ProviderReference: "",
		IdempotencyKey:    "idem-final-cash-001",
		Notes:             "Final cash balance received",
	})
	if err != nil {
		t.Fatalf("failed to record final payment: %v", err)
	}
	if pmt2.AmountCents != 10000 || updated2.PaidCents != 16950 || updated2.DueCents != 0 || updated2.Status != "paid" || updated2.PaidAt == nil {
		t.Fatalf("unexpected invoice state after full payment: %+v", updated2)
	}

	// 8. Attempting to void paid invoice must fail
	_, err = s.VoidInvoice(ctx, a, inv.ID, "Attempting to void paid invoice")
	if err == nil {
		t.Fatalf("expected error voiding paid invoice, got nil")
	}

	// 9. Create a second invoice and void it
	inv2, err := s.CreateInvoice(ctx, a, CreateInvoiceInput{
		CustomerID:    customerID,
		InvoiceNumber: "INV-TEST-002",
		TaxCents:      0,
		Notes:         "Invoice to be voided",
		Items: []CreateInvoiceItemInput{
			{Description: "Quick Wash", Quantity: 1, UnitPriceCents: 2500},
		},
	})
	if err != nil {
		t.Fatalf("failed to create invoice 2: %v", err)
	}

	voided, err := s.VoidInvoice(ctx, a, inv2.ID, "Customer cancelled appointment")
	if err != nil {
		t.Fatalf("failed to void invoice: %v", err)
	}
	if voided.Status != "void" {
		t.Fatalf("expected void status, got %s", voided.Status)
	}

	// 10. Attempting payment on void invoice must fail
	_, _, err = s.RecordPayment(ctx, a, RecordPaymentInput{
		InvoiceID:      inv2.ID,
		AmountCents:    2500,
		Method:         "cash",
		IdempotencyKey: "idem-void-payment-001",
	})
	if !errors.Is(err, ErrInvoiceStatusConflict) {
		t.Fatalf("expected ErrInvoiceStatusConflict for void invoice, got: %v", err)
	}

	// 11. Cross-scope isolation: scopeB cannot read or record payments on scopeA's invoice
	_, err = s.GetInvoice(ctx, b, inv.ID)
	if err != ErrForbidden && !errors.Is(err, ErrInvoiceNotFound) {
		t.Fatalf("expected ErrForbidden or ErrInvoiceNotFound across scopes, got: %v", err)
	}

	// 12. Verified Square payment processing
	inv3, err := s.CreateInvoice(ctx, a, CreateInvoiceInput{
		CustomerID:    customerID,
		InvoiceNumber: "INV-TEST-003",
		TaxCents:      0,
		Notes:         "Invoice for Square payment test",
		Items: []CreateInvoiceItemInput{
			{Description: "Ceramic Coating Stage 1", Quantity: 1, UnitPriceCents: 10000},
		},
	})
	if err != nil {
		t.Fatalf("failed to create invoice 3: %v", err)
	}
	inv3 = issueInvoiceForTest(t, ctx, s, a, inv3)

	approvedOnly := &square.MockClient{CreatePaymentFunc: func(_ context.Context, req square.PaymentRequest) (square.PaymentResponse, error) {
		return square.PaymentResponse{ID: "synthetic-authorization-only", Status: "APPROVED", AmountCents: req.AmountCents, Currency: req.Currency}, nil
	}}
	_, _, authorizationErr := s.RecordSquarePayment(ctx, a, approvedOnly, RecordSquarePaymentInput{InvoiceID: inv3.ID, AmountCents: 5000, SourceID: "synthetic-authorized-source", IdempotencyKey: "codex-authorized-only-001", Method: "square_card"})
	if !errors.Is(authorizationErr, ErrPaymentProvider) {
		t.Fatalf("authorization counted as settled: %v", authorizationErr)
	}
	var paidAfterAuthorization, paymentsAfterAuthorization int64
	if err = s.pool.QueryRow(ctx, `SELECT paid_cents FROM invoices WHERE id=$1`, inv3.ID).Scan(&paidAfterAuthorization); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM payments WHERE invoice_id=$1`, inv3.ID).Scan(&paymentsAfterAuthorization); err != nil {
		t.Fatal(err)
	}
	if paidAfterAuthorization != 0 || paymentsAfterAuthorization != 0 {
		t.Fatalf("authorization mutated balances/payment rows: %d/%d", paidAfterAuthorization, paymentsAfterAuthorization)
	}
	t.Log("VERIFIED: APPROVED authorization rejected with zero invoice paid balance and zero payment rows")

	mockSq := &square.MockClient{}

	// Successful square_card partial payment
	sqPmt1, updatedInv3, err := s.RecordSquarePayment(ctx, a, mockSq, RecordSquarePaymentInput{
		InvoiceID:      inv3.ID,
		AmountCents:    5000,
		SourceID:       "cnon:card-nonce-ok",
		IdempotencyKey: "sq-idem-card-001",
		Method:         "square_card",
		Notes:          "Online card deposit",
	})
	if err != nil {
		t.Fatalf("RecordSquarePayment failed: %v", err)
	}
	if sqPmt1.Method != "square_card" || sqPmt1.Status != "completed" || sqPmt1.AmountCents != 5000 {
		t.Fatalf("unexpected square payment 1: %+v", sqPmt1)
	}
	if sqPmt1.ProviderReference != "sq_mock_pay_sq-idem-card-001" {
		t.Fatalf("unexpected provider reference: %s", sqPmt1.ProviderReference)
	}
	if updatedInv3.Status != "partially_paid" || updatedInv3.PaidCents != 5000 || updatedInv3.DueCents != 5000 {
		t.Fatalf("unexpected invoice balance after square payment: %+v", updatedInv3)
	}

	// Idempotent retry returns existing payment
	retrySq, _, err := s.RecordSquarePayment(ctx, a, mockSq, RecordSquarePaymentInput{
		InvoiceID:      inv3.ID,
		AmountCents:    5000,
		SourceID:       "cnon:card-nonce-ok",
		IdempotencyKey: "sq-idem-card-001",
		Method:         "square_card",
		Notes:          "Online card deposit",
	})
	if err != nil || retrySq.ID != sqPmt1.ID {
		t.Fatalf("idempotent retry failed: %v, %s vs %s", err, retrySq.ID, sqPmt1.ID)
	}

	// Overpayment rejection (attempting 6000 when due is 5000)
	_, _, err = s.RecordSquarePayment(ctx, a, mockSq, RecordSquarePaymentInput{
		InvoiceID:      inv3.ID,
		AmountCents:    6000,
		SourceID:       "cnon:card-nonce-ok",
		IdempotencyKey: "sq-idem-overpay-001",
		Method:         "square_card",
	})
	if !errors.Is(err, ErrPaymentOverpay) {
		t.Fatalf("expected ErrPaymentOverpay, got %v", err)
	}

	// Declined card leaves invoice balance unchanged
	decliningMock := &square.MockClient{
		CreatePaymentFunc: func(ctx context.Context, req square.PaymentRequest) (square.PaymentResponse, error) {
			return square.PaymentResponse{}, square.ErrSquareDeclined
		},
	}
	_, _, err = s.RecordSquarePayment(ctx, a, decliningMock, RecordSquarePaymentInput{
		InvoiceID:      inv3.ID,
		AmountCents:    2000,
		SourceID:       "cnon:card-declined",
		IdempotencyKey: "sq-idem-declined-001",
		Method:         "square_card",
	})
	if !errors.Is(err, square.ErrSquareDeclined) {
		t.Fatalf("expected ErrSquareDeclined, got %v", err)
	}

	// Verify invoice balance was untouched after decline
	currentInv3, err := s.GetInvoice(ctx, a, inv3.ID)
	if err != nil || currentInv3.PaidCents != 5000 || currentInv3.DueCents != 5000 {
		t.Fatalf("invoice balance corrupted after decline: %+v, err: %v", currentInv3, err)
	}

	// Final settlement with square_terminal
	sqPmt2, finalInv3, err := s.RecordSquarePayment(ctx, a, mockSq, RecordSquarePaymentInput{
		InvoiceID:      inv3.ID,
		AmountCents:    5000,
		SourceID:       "cnon:terminal-checkout-ok",
		IdempotencyKey: "sq-idem-terminal-002",
		Method:         "square_terminal",
		Notes:          "In-person terminal settlement",
	})
	if err != nil {
		t.Fatalf("final square_terminal settlement failed: %v", err)
	}
	if sqPmt2.Method != "square_terminal" || finalInv3.Status != "paid" || finalInv3.DueCents != 0 || finalInv3.PaidCents != 10000 {
		t.Fatalf("unexpected final invoice status: %+v", finalInv3)
	}

	// 13. Asynchronous Square Webhook handling: payment refund and failure
	err = s.HandleSquareWebhookPayment(ctx, sqPmt1.ProviderReference, "REFUNDED", 5000)
	if err != nil {
		t.Fatalf("HandleSquareWebhookPayment refund failed: %v", err)
	}
	refundedInv3, err := s.GetInvoice(ctx, a, inv3.ID)
	if err != nil || refundedInv3.Status != "partially_paid" || refundedInv3.PaidCents != 5000 || refundedInv3.DueCents != 5000 {
		t.Fatalf("unexpected invoice after webhook refund: %+v, err: %v", refundedInv3, err)
	}

	// Refund ledger/invoice changes must roll back if the final audit fails.
	if _, err = s.pool.Exec(ctx, `CREATE FUNCTION codex_fail_refund_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='payment.provider_reconciled' THEN RAISE EXCEPTION 'synthetic refund audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER codex_fail_refund_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION codex_fail_refund_audit()`); err != nil {
		t.Fatal(err)
	}
	if err = s.HandleSquareWebhookPayment(ctx, sqPmt2.ProviderReference, "COMPLETED", 1000); err == nil {
		t.Fatal("refund ignored audit failure")
	}
	var rollbackPaid, rollbackRefund, rollbackHistory int64
	if err = s.pool.QueryRow(ctx, `SELECT i.paid_cents,p.refunded_cents,(SELECT count(*) FROM payment_refund_adjustments r WHERE r.payment_id=p.id) FROM payments p JOIN invoices i ON i.id=p.invoice_id WHERE p.id=$1`, sqPmt2.ID).Scan(&rollbackPaid, &rollbackRefund, &rollbackHistory); err != nil {
		t.Fatal(err)
	}
	if rollbackPaid != 5000 || rollbackRefund != 0 || rollbackHistory != 0 {
		t.Fatalf("refund audit rollback failed%d/%d/%d", rollbackPaid, rollbackRefund, rollbackHistory)
	}
	if _, err = s.pool.Exec(ctx, `DROP TRIGGER codex_fail_refund_audit ON audit_events; DROP FUNCTION codex_fail_refund_audit()`); err != nil {
		t.Fatal(err)
	}
	outcomes := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { outcomes <- s.HandleSquareWebhookPayment(ctx, sqPmt2.ProviderReference, "COMPLETED", 1000) }()
	}
	for i := 0; i < 2; i++ {
		if err = <-outcomes; err != nil {
			t.Fatalf("concurrent refund: %v", err)
		}
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM payment_refund_adjustments WHERE payment_id=$1`, sqPmt2.ID).Scan(&rollbackHistory); err != nil || rollbackHistory != 1 {
		t.Fatalf("duplicate concurrent refund history%d: %v", rollbackHistory, err)
	}
	t.Log("VERIFIED: refund audit failure rolls back payment/invoice/history; concurrent identical refunds produce one adjustment")
	// Cumulative partial refunds apply only new deltas, despite duplicate/old events.
	for _, step := range []struct {
		status       string
		refund, paid int64
	}{{"COMPLETED", 1000, 4000}, {"COMPLETED", 2000, 3000}, {"COMPLETED", 2000, 3000}, {"COMPLETED", 1000, 3000}, {"FAILED", 0, 3000}, {"REFUNDED", 5000, 0}, {"COMPLETED", 0, 0}, {"COMPLETED", 2000, 0}} {
		if err = s.HandleSquareWebhookPayment(ctx, sqPmt2.ProviderReference, step.status, step.refund); err != nil {
			t.Fatalf("refund step %+v: %v", step, err)
		}
		actual, readErr := s.GetInvoice(ctx, a, inv3.ID)
		if readErr != nil || actual.PaidCents != step.paid {
			t.Fatalf("refund step %+v paid%d err%v", step, actual.PaidCents, readErr)
		}
	}
	var cumulative, adjustments int64
	if err = s.pool.QueryRow(ctx, `SELECT refunded_cents,(SELECT count(*) FROM payment_refund_adjustments WHERE payment_id=$1) FROM payments WHERE id=$1`, sqPmt2.ID).Scan(&cumulative, &adjustments); err != nil {
		t.Fatal(err)
	}
	if cumulative != 5000 || adjustments != 3 {
		t.Fatalf("cumulative/history mismatch%d/%d", cumulative, adjustments)
	}
	t.Log("VERIFIED: partial1000→2000→full5000; duplicate/older/stale completed/failure events do not change net balance;3adjustments")
	// Unknown Square payment ID returns nil idempotently
	if err = s.HandleSquareWebhookPayment(ctx, "sq_unknown_payment_id", "COMPLETED", 0); err != nil {
		t.Fatalf("expected nil for unknown payment ID, got %v", err)
	}
}
