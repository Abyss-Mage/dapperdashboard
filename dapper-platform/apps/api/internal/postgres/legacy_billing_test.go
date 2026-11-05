package postgres

import (
	"context"
	"errors"
	"testing"
)

// An imported booking's invoice is built when it is opened from one small row of recorded figures; no invoice, payment or event rows exist for it.
func verifyLegacyBilling(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	var customer, booking string
	if err := s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Legacy Customer','legacy@example.test','') RETURNING id::text`, a.OrganizationID, a.LocationID).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	defer s.pool.Exec(ctx, `DELETE FROM customers WHERE id=$1`, customer)
	if err := s.pool.QueryRow(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,'Sedan - Express Interior Detail','online','completed','2024-06-23T12:00:00Z') RETURNING id::text`, a.OrganizationID, a.LocationID, customer).Scan(&booking); err != nil {
		t.Fatal(err)
	}
	defer s.pool.Exec(ctx, `DELETE FROM bookings WHERE id=$1`, booking)
	defer s.pool.Exec(ctx, `DELETE FROM legacy_booking_billing WHERE booking_id=$1`, booking)
	if _, err := s.pool.Exec(ctx, `INSERT INTO legacy_booking_billing(booking_id,organization_id,location_id,source_id,invoice_number,status,items,subtotal_cents,discount_cents,coupon_code,tax_cents,total_cents,paid_cents,payment_method,gateway,note)
		VALUES($1,$2,$3,'15','DDP15','paid','[{"description":"Sedan - Express Interior Detail","quantity":1,"unitPriceCents":8900},{"description":"Add-ons and extras (as recorded): Pet hairs","quantity":1,"unitPriceCents":2000}]'::jsonb,10900,1090,'DDP10',1417,11227,11227,'cash','on-site','Imported')`, booking, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}

	list, err := s.LegacyBillingList(ctx, a)
	if err != nil || len(list) != 1 || list[0].BookingID != booking || list[0].TotalCents != 11227 {
		t.Fatal("the imported billing was not listed", list, err)
	}
	inv, err := s.LegacyInvoice(ctx, a, booking)
	if err != nil || inv.InvoiceNumber != "DDP15" || inv.Status != "paid" || inv.TotalCents != 11227 || inv.PaidCents != 11227 || inv.DueCents != 0 || inv.DiscountCents != 1090 || inv.CouponCode != "DDP10" {
		t.Fatal("the imported invoice was not built from the recorded figures", inv, err)
	}
	if len(inv.Items) != 2 || inv.Items[0].TotalPriceCents != 8900 || len(inv.Payments) != 1 || inv.Payments[0].Method != "cash" || inv.CustomerSnapshot == nil || inv.CustomerSnapshot.Name != "Legacy Customer" {
		t.Fatal("the imported invoice lines, payment or customer are wrong", inv, err)
	}
	var stored int
	if err = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM invoices WHERE booking_id=$1)+(SELECT count(*) FROM payments p JOIN invoices i ON i.id=p.invoice_id WHERE i.booking_id=$1)`, booking).Scan(&stored); err != nil || stored != 0 {
		t.Fatal("opening an imported invoice stored rows", stored, err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE legacy_booking_billing SET status='void',paid_cents=0,payment_method='' WHERE booking_id=$1`, booking); err != nil {
		t.Fatal(err)
	}
	if inv, err = s.LegacyInvoice(ctx, a, booking); err != nil || inv.Status != "void" || inv.PaidCents != 0 || inv.DueCents != 0 || len(inv.Payments) != 0 {
		t.Fatal("a void imported invoice should show no payment and nothing due", inv, err)
	}
	if _, err = s.LegacyInvoice(ctx, b, booking); err == nil {
		t.Fatal("another organization opened an imported invoice")
	}
	if _, err = s.LegacyInvoice(ctx, a, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrInvoiceNotFound) {
		t.Fatal("an unknown booking should be not found", err)
	}
}
