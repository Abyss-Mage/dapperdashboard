package postgres

import (
	"context"
	"errors"
	"testing"
)

// Brand-owned and franchisee outlets print with different invoice templates; an outlet is one or the other.
func verifyOutletTemplates(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	if _, err := s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'invoices.configure') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	defer s.pool.Exec(ctx, `DELETE FROM organization_grants WHERE principal_id=$1 AND organization_id=$2 AND action='invoices.configure'`, a.ActorID, a.OrganizationID)
	defer s.pool.Exec(ctx, `DELETE FROM franchise_print_templates WHERE organization_id=$1`, a.OrganizationID)
	defer s.pool.Exec(ctx, `UPDATE locations SET ownership='brand' WHERE organization_id=$1 AND id=$2`, a.OrganizationID, a.LocationID)

	loc, err := s.LocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID)
	if err != nil || loc.Outlet != "brand" {
		t.Fatal("a new outlet should be brand-owned", loc, err)
	}
	fr, err := s.OutletInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, "franchise")
	if err != nil || fr.Version != 0 || fr.Template.Title != loc.Template.Title {
		t.Fatal("the franchisee template should start from the brand-owned one", fr, err)
	}
	edited := fr.Template
	edited.Title = "Franchise invoice"
	if _, err = s.SaveOutletInvoicePrintTemplate(ctx, b.ActorID, a.OrganizationID, "franchise", edited, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("franchisee template saved without invoices.configure", err)
	}
	saved, err := s.SaveOutletInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, "franchise", edited, 0)
	if err != nil || saved.Version != 1 {
		t.Fatal("franchisee template not saved", saved, err)
	}
	brand, err := s.OutletInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, "brand")
	if err != nil || brand.Template.Title == "Franchise invoice" {
		t.Fatal("saving the franchisee template changed the brand-owned one", brand, err)
	}
	if loc, err = s.LocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil || loc.Template.Title == "Franchise invoice" {
		t.Fatal("a brand-owned outlet printed with the franchisee template", loc, err)
	}
	if err = s.SetOutletType(ctx, b.ActorID, a.OrganizationID, a.LocationID, "franchise"); !errors.Is(err, ErrForbidden) {
		t.Fatal("outlet type changed without invoices.configure", err)
	}
	if err = s.SetOutletType(ctx, a.ActorID, a.OrganizationID, a.LocationID, "sideways"); !errors.Is(err, ErrOutletType) {
		t.Fatal("an unknown outlet type was accepted", err)
	}
	if err = s.SetOutletType(ctx, a.ActorID, a.OrganizationID, a.LocationID, "franchise"); err != nil {
		t.Fatal(err)
	}
	if loc, err = s.LocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil || loc.Outlet != "franchise" || loc.Template.Title != "Franchise invoice" || loc.Scope != "organization" {
		t.Fatal("a franchisee outlet did not use the franchisee template", loc, err)
	}
	if err = s.SetOutletType(ctx, a.ActorID, a.OrganizationID, a.LocationID, "brand"); err != nil {
		t.Fatal(err)
	}
	if loc, err = s.LocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil || loc.Template.Title == "Franchise invoice" {
		t.Fatal("switching back to brand-owned did not restore its template", loc, err)
	}
}
