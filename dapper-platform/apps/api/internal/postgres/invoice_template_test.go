package postgres

import (
	"context"
	"errors"
	"testing"
	"time"
)

func verifyInvoicePrintTemplate(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	// Nothing saved yet: the built-in layout, revision 0, readable by any member.
	got, err := s.InvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID)
	if err != nil || got.Version != 0 || got.Template.Title != "Invoice" || got.Template.PaperSize != "letter" {
		t.Fatal("default template wrong", got, err)
	}
	if _, err = s.InvoicePrintTemplate(ctx, b.ActorID, a.OrganizationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("template readable by a non-member", err)
	}
	// Saving needs the organization-level invoices.configure grant.
	edited := got.Template
	edited.Title = "Tax Invoice"
	edited.AccentColor = "#aa2200"
	edited.FooterNote = "Thank you for choosing us."
	if _, err = s.SaveInvoicePrintTemplate(ctx, b.ActorID, a.OrganizationID, edited, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("template saved without invoices.configure", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'invoices.configure') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	defer s.pool.Exec(ctx, `DELETE FROM organization_grants WHERE principal_id=$1 AND organization_id=$2 AND action='invoices.configure'`, a.ActorID, a.OrganizationID)
	defer s.pool.Exec(ctx, `DELETE FROM invoice_print_templates WHERE organization_id=$1`, a.OrganizationID)
	bad := edited
	bad.AccentColor = "red"
	if _, err = s.SaveInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, bad, 0); !errors.Is(err, ErrInvoiceTemplateInput) {
		t.Fatal("non-hex colour accepted", err)
	}
	bad = edited
	bad.Title = "  "
	if _, err = s.SaveInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, bad, 0); !errors.Is(err, ErrInvoiceTemplateInput) {
		t.Fatal("empty title accepted", err)
	}
	saved, err := s.SaveInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, edited, 0)
	if err != nil || saved.Version != 1 || saved.Template.Title != "Tax Invoice" {
		t.Fatal("template save failed", saved, err)
	}
	if _, err = s.SaveInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, edited, 0); !errors.Is(err, ErrInvoiceTemplateConflict) {
		t.Fatal("second first-save accepted", err)
	}
	edited.Title = "Invoice / Facture"
	if saved, err = s.SaveInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, edited, 1); err != nil || saved.Version != 2 {
		t.Fatal("template revision failed", saved, err)
	}
	if _, err = s.SaveInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, edited, 1); !errors.Is(err, ErrInvoiceTemplateConflict) {
		t.Fatal("stale template revision accepted", err)
	}
	if got, err = s.InvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID); err != nil || got.Template.Title != "Invoice / Facture" || got.Template.FooterNote != "Thank you for choosing us." || got.Version != 2 {
		t.Fatal("saved template not returned", got, err)
	}

	// A franchise can have its own template; without one it prints with the organization's.
	defer s.pool.Exec(ctx, `DELETE FROM location_invoice_templates WHERE organization_id=$1`, a.OrganizationID)
	effective, err := s.LocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID)
	if err != nil || effective.Scope != "organization" || effective.Template.Title != "Invoice / Facture" {
		t.Fatal("a franchise without its own template should use the organization one", effective, err)
	}
	mine := got.Template
	mine.Title = "Franchise Invoice"
	mine.FooterNote = "Franchise footer."
	if _, err = s.SaveLocationInvoicePrintTemplate(ctx, b.ActorID, a.OrganizationID, a.LocationID, mine, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("franchise template saved without invoices.configure", err)
	}
	own, err := s.SaveLocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID, mine, 0)
	if err != nil || own.Version != 1 || own.Scope != "franchise" {
		t.Fatal("franchise template save failed", own, err)
	}
	if _, err = s.SaveLocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID, mine, 0); !errors.Is(err, ErrInvoiceTemplateConflict) {
		t.Fatal("second first-save of a franchise template accepted", err)
	}
	if effective, err = s.LocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil || effective.Scope != "franchise" || effective.Template.Title != "Franchise Invoice" || effective.Template.FooterNote != "Franchise footer." {
		t.Fatal("franchise template not returned", effective, err)
	}
	if orgOnly, e := s.InvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID); e != nil || orgOnly.Template.Title != "Invoice / Facture" {
		t.Fatal("the organization template was changed by a franchise save", orgOnly, e)
	}
	if err = s.DeleteLocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	if effective, err = s.LocationInvoicePrintTemplate(ctx, a.ActorID, a.OrganizationID, a.LocationID); err != nil || effective.Scope != "organization" {
		t.Fatal("removing the franchise template did not fall back", effective, err)
	}
}

func verifyOneInvoicePerBooking(t *testing.T, ctx context.Context, s *Store, a Scope, customerID string) {
	t.Helper()
	for _, action := range []string{"invoices.create", "bookings.create"} {
		tag, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID, action)
		if err != nil {
			t.Fatal(err)
		}
		if tag.RowsAffected() == 1 { // only remove what this check added
			defer s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND organization_id=$2 AND location_id=$3 AND action=$4`, a.ActorID, a.OrganizationID, a.LocationID, action)
		}
	}
	booking, err := s.CreateBooking(ctx, a, Booking{CustomerID: customerID, ServiceName: "Synthetic single billing", Channel: "internal", Status: "pending", BookingTime: time.Now().Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { // leave no booking or invoice behind for later checks
		for _, q := range []string{
			`DELETE FROM invoice_items WHERE invoice_id IN (SELECT id FROM invoices WHERE booking_id=$1)`,
			`DELETE FROM invoices WHERE booking_id=$1`,
			`DELETE FROM employee_booking_worksites WHERE booking_id=$1`,
			`DELETE FROM bookings WHERE id=$1`,
		} {
			if _, err := s.pool.Exec(ctx, q, booking.ID); err != nil {
				t.Log("cleanup:", err)
			}
		}
	}()
	input := CreateInvoiceInput{CustomerID: customerID, BookingID: &booking.ID, Items: []CreateInvoiceItemInput{{Description: "Synthetic service", Quantity: 1, UnitPriceCents: 10000}}}
	first, err := s.CreateInvoice(ctx, a, input)
	if err != nil {
		t.Fatal("first invoice for a booking failed", err)
	}
	if _, err = s.CreateInvoice(ctx, a, input); !errors.Is(err, ErrInvoiceAlreadyRaised) {
		t.Fatal("second invoice for the same booking accepted", err)
	}
	// Voiding the first one frees the booking for a replacement.
	if _, err = s.pool.Exec(ctx, `UPDATE invoices SET status='void' WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateInvoice(ctx, a, input); err != nil {
		t.Fatal("replacement after void refused", err)
	}
}
