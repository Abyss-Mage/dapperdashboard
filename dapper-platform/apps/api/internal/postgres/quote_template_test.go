package postgres

import (
	"context"
	"errors"
	"testing"
)

// Quotations have their own template: it starts from the invoice template (titled "Quotation") until saved, saves apart from
// it, can be given to one franchise, and every franchise template read carries its business details and tax.
func verifyQuotePrintTemplate(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	got, err := s.QuotePrintTemplate(ctx, a.ActorID, a.OrganizationID)
	if err != nil || got.Version != 0 || got.Template.Title != "Quotation" || got.Template.PaymentInstructions != "" {
		t.Fatal("unsaved quote template did not start from the invoice template", got, err)
	}
	if _, err = s.QuotePrintTemplate(ctx, b.ActorID, a.OrganizationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("quote template readable by a non-member", err)
	}
	edited := got.Template
	edited.Title = "Estimate"
	edited.QuotationNote = "Valid for thirty days."
	edited.ValidDays = 30
	if _, err = s.SaveQuotePrintTemplate(ctx, b.ActorID, a.OrganizationID, edited, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("quote template saved without invoices.configure", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'invoices.configure') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	defer s.pool.Exec(ctx, `DELETE FROM organization_grants WHERE principal_id=$1 AND organization_id=$2 AND action='invoices.configure'`, a.ActorID, a.OrganizationID)
	defer s.pool.Exec(ctx, `DELETE FROM quote_print_templates WHERE organization_id=$1`, a.OrganizationID)
	defer s.pool.Exec(ctx, `DELETE FROM location_quote_templates WHERE organization_id=$1`, a.OrganizationID)
	bad := edited
	bad.ValidDays = 400
	if _, err = s.SaveQuotePrintTemplate(ctx, a.ActorID, a.OrganizationID, bad, 0); !errors.Is(err, ErrInvoiceTemplateInput) {
		t.Fatal("a validity of more than a year was accepted", err)
	}
	saved, err := s.SaveQuotePrintTemplate(ctx, a.ActorID, a.OrganizationID, edited, 0)
	if err != nil || saved.Version != 1 {
		t.Fatal("quote template not saved", saved, err)
	}
	inv, err := s.InvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID)
	if err != nil || inv.Template.Title == "Estimate" {
		t.Fatal("saving the quote template changed the invoice template", inv, err)
	}
	loc, err := s.LocationQuotePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID)
	if err != nil || loc.Scope != "organization" || loc.Template.Title != "Estimate" || loc.Template.ValidDays != 30 {
		t.Fatal("franchise did not get the organization quote template", loc, err)
	}
	own := edited
	own.Title = "Franchise estimate"
	if _, err = s.SaveLocationQuotePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID, own, 0); err != nil {
		t.Fatal(err)
	}
	if loc, err = s.LocationQuotePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil || loc.Scope != "franchise" || loc.Template.Title != "Franchise estimate" {
		t.Fatal("franchise quote template not used", loc, err)
	}
	if err = s.DeleteLocationQuotePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	if loc, err = s.LocationQuotePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil || loc.Template.Title != "Estimate" {
		t.Fatal("franchise did not go back to the organization quote template", loc, err)
	}
}
