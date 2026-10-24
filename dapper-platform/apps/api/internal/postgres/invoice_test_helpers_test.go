package postgres

import (
	"context"
	"testing"
)

// Explicit configuration belongs only to disposable test schemas.
func issueInvoiceForTest(t *testing.T, ctx context.Context, s *Store, scope Scope, invoice Invoice) Invoice {
	t.Helper()
	if _, err := s.pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, scope.OrganizationID, scope.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'invoices.configure') ON CONFLICT DO NOTHING`, scope.ActorID, scope.OrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.issue') ON CONFLICT DO NOTHING`, scope.ActorID, scope.OrganizationID, scope.LocationID); err != nil {
		t.Fatal(err)
	}
	configuration, err := s.InvoiceConfiguration(ctx, scope.ActorID, scope.OrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	var issuer InvoiceIssuer
	for _, candidate := range configuration.Issuers {
		if candidate.NumberPrefix == "TEST-" {
			issuer = candidate
		}
	}
	if issuer.ID == "" {
		issuer, err = s.SaveInvoiceIssuer(ctx, scope.ActorID, scope.OrganizationID, InvoiceIssuerInput{LegalName: "Isolated test issuer", Address: "Isolated test address", TaxRegistration: "TEST-REGISTRATION", Currency: "CAD", NumberPrefix: "TEST-", StartNumber: 1})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = s.BindInvoiceIssuer(ctx, scope.ActorID, scope.OrganizationID, scope.LocationID, issuer.ID); err != nil {
		t.Fatal(err)
	}
	issued, err := s.IssueInvoice(ctx, scope, invoice.ID)
	if err != nil {
		t.Fatal("explicit test invoice issuance failed", err)
	}
	full, err := s.GetInvoice(ctx, scope, issued.ID)
	if err != nil {
		t.Fatal(err)
	}
	return full
}
