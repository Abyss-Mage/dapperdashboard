package postgres

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Moving a completed booking back to Approved reverses the cash payment the Complete dialog recorded, so the invoice is owed again,
// and completing it again can record a new payment under the same key.
func TestReopenReversesCompletionPayment(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_reopen_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		_, _ = admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
	}()
	config := admin.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var org, loc, staff, customer, booking, issuer string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Reopen Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic reopen site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:reopen-staff') RETURNING id::text`).Scan(&staff))
	for _, action := range []string{"bookings.read", "bookings.manage", "invoices.read", "invoices.create", "payments.read"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, staff, org, loc, action)
		must(err)
	}
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Ann Reopen','ann@example.invalid','') RETURNING id::text`, org, loc).Scan(&customer))
	must(pool.QueryRow(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,'Synthetic Detail','internal','completed',now()) RETURNING id::text`, org, loc, customer).Scan(&booking))
	scope := Scope{ActorID: staff, OrganizationID: org, LocationID: loc}
	inv, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: customer, BookingID: &booking, Items: []CreateInvoiceItemInput{{Description: "Synthetic Detail", Quantity: 1, UnitPriceCents: 20000}}})
	must(err)
	must(pool.QueryRow(ctx, `INSERT INTO invoice_issuers(organization_id,legal_name,address,tax_registration,currency,number_prefix,next_number,updated_by) VALUES($1,'Synthetic Issuer','1 Test Street','','CAD','SYN',1,$2) RETURNING id::text`, org, staff).Scan(&issuer))
	_, err = pool.Exec(ctx, `UPDATE invoices SET status='paid',invoice_number='SYN1',issuer_id=$2,issuer_snapshot='{}',customer_snapshot='{}',issued_by=$3,issued_at=now(),paid_cents=total_cents,due_cents=0,paid_at=now() WHERE id=$1`, inv.ID, issuer, staff)
	must(err)
	_, err = pool.Exec(ctx, `INSERT INTO payments(organization_id,location_id,invoice_id,customer_id,actor_id,amount_cents,method,status,idempotency_key) VALUES($1,$2,$3,$4,$5,20000,'cash','completed',$6)`, org, loc, inv.ID, customer, staff, "complete-"+booking)
	must(err)
	if name, pdf := s.BookingInvoicePDF(ctx, booking); name != "Invoice-SYN1.pdf" || !bytes.Contains(pdf, []byte("(Synthetic Detail)")) {
		t.Fatalf("emailed invoice PDF not built from the saved invoice: %q %d bytes", name, len(pdf))
	}

	back, err := s.CorrectBookingStatus(ctx, scope, booking, "confirmed", "", "")
	if err != nil || back.Status != "confirmed" || !strings.Contains(back.PaymentNotice, "$200.00") {
		t.Fatalf("reopen failed or said nothing about the payment: %+v %v", back.PaymentNotice, err)
	}
	var status string
	var paid, due int64
	must(pool.QueryRow(ctx, `SELECT status,paid_cents,due_cents FROM invoices WHERE id=$1`, inv.ID).Scan(&status, &paid, &due))
	if status != "issued" || paid != 0 || due != 20000 {
		t.Fatalf("invoice not owed again: %s paid %d due %d", status, paid, due)
	}
	var pstatus, key string
	var refunded int64
	must(pool.QueryRow(ctx, `SELECT status,refunded_cents,idempotency_key FROM payments WHERE invoice_id=$1`, inv.ID).Scan(&pstatus, &refunded, &key))
	if pstatus != "refunded" || refunded != 20000 || key == "complete-"+booking {
		t.Fatalf("payment not reversed: %s %d %s", pstatus, refunded, key)
	}
	var events int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM invoice_events WHERE invoice_id=$1 AND notes LIKE 'Payment reversed%'`, inv.ID).Scan(&events))
	if events != 1 {
		t.Fatal("reversal not in the invoice history", events)
	}
	// The key is free again, so completing it once more records a new payment.
	_, err = pool.Exec(ctx, `INSERT INTO payments(organization_id,location_id,invoice_id,customer_id,actor_id,amount_cents,method,status,idempotency_key) VALUES($1,$2,$3,$4,$5,20000,'cash','completed',$6)`, org, loc, inv.ID, customer, staff, "complete-"+booking)
	must(err)
	// Reopening a booking that has no completion payment says nothing.
	var other string
	must(pool.QueryRow(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,'Synthetic Detail','internal','completed',now()) RETURNING id::text`, org, loc, customer).Scan(&other))
	if b, e := s.CorrectBookingStatus(ctx, scope, other, "confirmed", "", ""); e != nil || b.PaymentNotice != "" {
		t.Fatal("reopen without a payment misbehaved", b.PaymentNotice, e)
	}

	// Cancelling a paid booking takes the hand-recorded money back and voids its invoice, so nothing is owed and nothing counts as takings.
	_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.manage')`, staff, org, loc)
	must(err)
	var paidBooking string
	must(pool.QueryRow(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,'Synthetic Detail','internal','confirmed',now()+interval '2 days') RETURNING id::text`, org, loc, customer).Scan(&paidBooking))
	inv2, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: customer, BookingID: &paidBooking, Items: []CreateInvoiceItemInput{{Description: "Synthetic Detail", Quantity: 1, UnitPriceCents: 15000}}})
	must(err)
	_, err = pool.Exec(ctx, `UPDATE invoices SET status='paid',invoice_number='SYN2',issuer_id=$2,issuer_snapshot='{}',customer_snapshot='{}',issued_by=$3,issued_at=now(),paid_cents=total_cents,due_cents=0,paid_at=now() WHERE id=$1`, inv2.ID, issuer, staff)
	must(err)
	_, err = pool.Exec(ctx, `INSERT INTO payments(organization_id,location_id,invoice_id,customer_id,actor_id,amount_cents,method,status,idempotency_key) VALUES($1,$2,$3,$4,$5,15000,'bank_transfer','completed','deposit-synthetic')`, org, loc, inv2.ID, customer, staff)
	must(err)
	gone, err := s.CorrectBookingStatus(ctx, scope, paidBooking, "cancelled", "Customer asked", "")
	if err != nil || gone.Status != "cancelled" || !strings.Contains(gone.PaymentNotice, "$150.00") {
		t.Fatalf("cancel failed or said nothing about the payment: %+v %v", gone.PaymentNotice, err)
	}
	must(pool.QueryRow(ctx, `SELECT status,paid_cents FROM invoices WHERE id=$1`, inv2.ID).Scan(&status, &paid))
	must(pool.QueryRow(ctx, `SELECT status,refunded_cents FROM payments WHERE invoice_id=$1`, inv2.ID).Scan(&pstatus, &refunded))
	if status != "void" || paid != 0 || pstatus != "refunded" || refunded != 15000 {
		t.Fatalf("cancelled booking still holds money: invoice %s paid %d, payment %s refunded %d", status, paid, pstatus, refunded)
	}
}
