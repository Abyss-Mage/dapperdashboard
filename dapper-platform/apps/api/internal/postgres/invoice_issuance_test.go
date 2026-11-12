package postgres

import (
	"context"
	"errors"
	"testing"
)

func verifyInvoiceIssuance(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	input := InvoiceIssuerInput{LegalName: "Isolated original issuer", Address: "Isolated original address", TaxRegistration: "TEST-ONLY", Currency: "CAD", NumberPrefix: "FINAL-", StartNumber: 100}
	if _, err := s.SaveInvoiceIssuer(ctx, b.ActorID, a.OrganizationID, input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign issuer configuration accepted: %v", err)
	}
	issuer, err := s.SaveInvoiceIssuer(ctx, a.ActorID, a.OrganizationID, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.BindInvoiceIssuer(ctx, a.ActorID, a.OrganizationID, b.LocationID, issuer.ID); !errors.Is(err, ErrInvoiceIssuer) {
		t.Fatal("foreign issuer binding accepted")
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_invoice_issuers WHERE organization_id=$1 AND location_id=$2`, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	draft, err := s.CreateInvoice(ctx, a, CreateInvoiceInput{CustomerID: customer, Items: []CreateInvoiceItemInput{{Description: "Isolated draft review", Quantity: 1, UnitPriceCents: 1000}}})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != "draft" || draft.IssuedAt != nil || draft.IssuerSnapshot != nil {
		t.Fatal("invoice skipped draft state")
	}
	if _, _, err = s.RecordPayment(ctx, a, RecordPaymentInput{InvoiceID: draft.ID, AmountCents: 100, Method: "cash", IdempotencyKey: "draft-cannot-be-paid"}); !errors.Is(err, ErrInvoiceStatusConflict) {
		t.Fatal("draft accepted cash settlement")
	}
	if _, err = s.IssueInvoice(ctx, a, draft.ID); !errors.Is(err, ErrInvoiceIssuer) {
		t.Fatal("invoice issued without location issuer")
	}
	if err = s.BindInvoiceIssuer(ctx, a.ActorID, a.OrganizationID, a.LocationID, issuer.ID); err != nil {
		t.Fatal(err)
	}
	type result struct {
		invoice Invoice
		err     error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() { v, e := s.IssueInvoice(ctx, a, draft.ID); results <- result{v, e} }()
	}
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil || r.invoice.InvoiceNumber != "FINAL-100" || r.invoice.IssuerSnapshot == nil || r.invoice.CustomerSnapshot == nil {
			t.Fatal("concurrent issue retry failed", r.err)
		}
	}
	var next int64
	if err = s.pool.QueryRow(ctx, `SELECT next_number FROM invoice_issuers WHERE id=$1`, issuer.ID).Scan(&next); err != nil || next != 101 {
		t.Fatal("duplicate issuance consumed multiple numbers", err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE invoices SET tax_cents=tax_cents+1 WHERE id=$1`, draft.ID); err == nil {
		t.Fatal("issued invoice monetary details changed")
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM invoices WHERE id=$1`, draft.ID); err == nil {
		t.Fatal("issued invoice deleted")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE invoice_items SET description='Changed after issuance' WHERE invoice_id=$1`, draft.ID); err == nil {
		t.Fatal("issued invoice items changed")
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO invoice_items(invoice_id,organization_id,location_id,description,quantity,unit_price_cents,total_price_cents) VALUES($1,$2,$3,'Late issued item',1,100,100)`, draft.ID, a.OrganizationID, a.LocationID); err == nil {
		t.Fatal("issued invoice accepted new items")
	}
	input.ID = issuer.ID
	input.Version = 1
	input.StartNumber = 0
	input.LegalName = "Isolated revised issuer"
	input.Address = "Isolated revised address"
	revised, err := s.SaveInvoiceIssuer(ctx, a.ActorID, a.OrganizationID, input)
	if err != nil || revised.Version != 2 || revised.NextNumber != 101 {
		t.Fatal("issuer revision reset sequence", err)
	}
	if _, err = s.SaveInvoiceIssuer(ctx, a.ActorID, a.OrganizationID, input); !errors.Is(err, ErrInvoiceIssuerConflict) {
		t.Fatal("stale issuer version overwritten")
	}
	historical, err := s.GetInvoice(ctx, a, draft.ID)
	if err != nil || historical.IssuerSnapshot.LegalName != "Isolated original issuer" {
		t.Fatal("issuer edit rewrote history", err)
	}
	failing, err := s.CreateInvoice(ctx, a, CreateInvoiceInput{CustomerID: customer, Items: []CreateInvoiceItemInput{{Description: "Isolated issuance rollback", Quantity: 1, UnitPriceCents: 1000}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.IssueInvoice(ctx, a, failing.ID, InvoiceIssuanceReview{IssuerID: issuer.ID, IssuerVersion: 1}); !errors.Is(err, ErrInvoiceIssuerConflict) {
		t.Fatal("stale reviewed issuer was accepted")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE invoice_events ADD CONSTRAINT fail_issuance_audit CHECK(to_status<>'issued') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.IssueInvoice(ctx, a, failing.ID); err == nil {
		t.Fatal("issuance audit failure committed")
	}
	if err = s.pool.QueryRow(ctx, `SELECT next_number FROM invoice_issuers WHERE id=$1`, issuer.ID).Scan(&next); err != nil || next != 101 {
		t.Fatal("failed issuance consumed a number", err)
	}
	check, err := s.GetInvoice(ctx, a, failing.ID)
	if err != nil || check.Status != "draft" || check.IssuerSnapshot != nil {
		t.Fatal("failed issuance left partial snapshot", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE invoice_events DROP CONSTRAINT fail_issuance_audit`); err != nil {
		t.Fatal(err)
	}
	issued, err := s.IssueInvoice(ctx, a, failing.ID)
	if err != nil || issued.InvoiceNumber != "FINAL-101" || issued.IssuerSnapshot.Version != 2 {
		t.Fatal("issuance after rollback failed", err)
	}
	// A running number set back onto one already used is stepped over: the next draft below still becomes FINAL-102.
	if _, err = s.pool.Exec(ctx, `UPDATE invoice_issuers SET next_number=101 WHERE id=$1`, issuer.ID); err != nil {
		t.Fatal(err)
	}
	var otherLocation, otherCustomer string
	if err = s.pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Isolated shared issuer location') RETURNING id::text`, a.OrganizationID).Scan(&otherLocation); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Original isolated customer','original@example.invalid') RETURNING id::text`, a.OrganizationID, otherLocation).Scan(&otherCustomer); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.create'),($1,$2,$3,'invoices.read'),($1,$2,$3,'invoices.issue')`, a.ActorID, a.OrganizationID, otherLocation); err != nil {
		t.Fatal(err)
	}
	otherScope := Scope{ActorID: a.ActorID, OrganizationID: a.OrganizationID, LocationID: otherLocation}
	if err = s.BindInvoiceIssuer(ctx, a.ActorID, a.OrganizationID, otherLocation, issuer.ID); err != nil {
		t.Fatal(err)
	}
	secondDraft, err := s.CreateInvoice(ctx, a, CreateInvoiceInput{CustomerID: customer, Items: []CreateInvoiceItemInput{{Description: "Shared issuer first location", Quantity: 1, UnitPriceCents: 1000}}})
	if err != nil {
		t.Fatal(err)
	}
	thirdDraft, err := s.CreateInvoice(ctx, otherScope, CreateInvoiceInput{CustomerID: otherCustomer, Items: []CreateInvoiceItemInput{{Description: "Shared issuer second location", Quantity: 1, UnitPriceCents: 1000}}})
	if err != nil {
		t.Fatal(err)
	}
	go func() { v, e := s.IssueInvoice(ctx, a, secondDraft.ID); results <- result{v, e} }()
	go func() { v, e := s.IssueInvoice(ctx, otherScope, thirdDraft.ID); results <- result{v, e} }()
	numbers := map[string]bool{}
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		numbers[r.invoice.InvoiceNumber] = true
	}
	if !numbers["FINAL-102"] || !numbers["FINAL-103"] || len(numbers) != 2 {
		t.Fatal("shared issuer locations duplicated or lost sequence numbers")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE customers SET name='Changed isolated customer' WHERE id=$1`, otherCustomer); err != nil {
		t.Fatal(err)
	}
	historicalCustomer, err := s.GetInvoice(ctx, otherScope, thirdDraft.ID)
	if err != nil || historicalCustomer.CustomerName != "Original isolated customer" {
		t.Fatal("customer update rewrote issued invoice recipient", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO invoice_items(invoice_id,organization_id,location_id,description,quantity,unit_price_cents,total_price_cents) VALUES($1,$2,$3,'Foreign location item',1,100,100)`, secondDraft.ID, a.OrganizationID, otherLocation); err == nil {
		t.Fatal("cross-location invoice item accepted")
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='invoices.issue'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.IssueInvoice(ctx, a, draft.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("issue replay bypassed revoked grant")
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.issue')`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
}
