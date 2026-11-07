package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// LegacyBilling is the billing summary of one booking imported from the old site (see legacy_billing.sql).
type LegacyBilling struct {
	BookingID     string `json:"bookingId"`
	InvoiceNumber string `json:"invoiceNumber"`
	Status        string `json:"status"`
	SubtotalCents int64  `json:"subtotalCents"`
	DiscountCents int64  `json:"discountCents"`
	CouponCode    string `json:"couponCode,omitempty"`
	TaxCents      int64  `json:"taxCents"`
	TotalCents    int64  `json:"totalCents"`
	PaidCents     int64  `json:"paidCents"`
}

// LegacyBillingList is every imported booking's billing summary at one franchise, so the appointment list can show what each one paid.
func (s *Store) LegacyBillingList(ctx context.Context, scope Scope) ([]LegacyBilling, error) {
	if err := s.authorize(ctx, scope, "invoices.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT booking_id::text,invoice_number,status,subtotal_cents,discount_cents,coupon_code,tax_cents,total_cents,paid_cents FROM legacy_booking_billing WHERE organization_id=$1 AND location_id=$2 ORDER BY invoice_number`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LegacyBilling{}
	for rows.Next() {
		var l LegacyBilling
		if err = rows.Scan(&l.BookingID, &l.InvoiceNumber, &l.Status, &l.SubtotalCents, &l.DiscountCents, &l.CouponCode, &l.TaxCents, &l.TotalCents, &l.PaidCents); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

type legacyItem struct {
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unitPriceCents"`
}

// LegacyInvoice builds the invoice of an imported booking when it is opened. Nothing is stored for it: the document is made from the
// recorded figures, the customer, and the issuer the franchise has now (so correcting the business details corrects these too).
func (s *Store) LegacyInvoice(ctx context.Context, scope Scope, bookingID string) (Invoice, error) {
	if err := s.authorize(ctx, scope, "invoices.read"); err != nil {
		return Invoice{}, err
	}
	return s.legacyInvoice(ctx, scope.OrganizationID, scope.LocationID, bookingID)
}

// legacyInvoice builds the document; callers check who may see it.
func (s *Store) legacyInvoice(ctx context.Context, org, loc, bookingID string) (Invoice, error) {
	var (
		inv        Invoice
		itemsJSON  []byte
		method     string
		gateway    string
		bookedAt   time.Time
		customer   InvoiceCustomerSnapshot
		createdAt  time.Time
		customerID string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT l.invoice_number,l.status,l.items,l.subtotal_cents,l.discount_cents,l.coupon_code,l.tax_cents,l.total_cents,l.paid_cents,l.payment_method,l.gateway,l.note,
		       b.booking_time,b.created_at,b.customer_id::text,c.name,c.email,c.phone
		FROM legacy_booking_billing l
		JOIN bookings b ON b.id=l.booking_id
		JOIN customers c ON c.organization_id=b.organization_id AND c.location_id=b.location_id AND c.id=b.customer_id
		WHERE l.organization_id=$1 AND l.location_id=$2 AND l.booking_id=$3`, org, loc, bookingID).
		Scan(&inv.InvoiceNumber, &inv.Status, &itemsJSON, &inv.SubtotalCents, &inv.DiscountCents, &inv.CouponCode, &inv.TaxCents, &inv.TotalCents, &inv.PaidCents, &method, &gateway, &inv.Notes,
			&bookedAt, &createdAt, &customerID, &customer.Name, &customer.Email, &customer.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}
	var items []legacyItem
	if err = json.Unmarshal(itemsJSON, &items); err != nil {
		return Invoice{}, err
	}
	inv.ID = "legacy-" + bookingID
	inv.Version = 1
	inv.OrganizationID, inv.LocationID, inv.CustomerID = org, loc, customerID
	inv.BookingID = &bookingID
	inv.Currency = "CAD"
	if inv.Status != "void" {
		inv.DueCents = inv.TotalCents - inv.PaidCents
	}
	inv.IssuedAt, inv.CreatedAt, inv.UpdatedAt = &bookedAt, createdAt, bookedAt
	if inv.Status == "paid" {
		inv.PaidAt = &bookedAt
	}
	inv.CustomerName, inv.CustomerEmail = customer.Name, customer.Email
	inv.CustomerSnapshot = &customer
	inv.Items = make([]InvoiceItem, 0, len(items))
	for i, it := range items {
		qty := it.Quantity
		if qty < 1 {
			qty = 1
		}
		inv.Items = append(inv.Items, InvoiceItem{ID: inv.ID + "-" + string(rune('a'+i%26)), InvoiceID: inv.ID, Description: it.Description, Quantity: qty, UnitPriceCents: it.UnitPriceCents, TotalPriceCents: int64(qty) * it.UnitPriceCents, CreatedAt: bookedAt})
	}
	inv.Payments = []Payment{}
	if inv.PaidCents > 0 && inv.Status != "void" {
		inv.Payments = append(inv.Payments, Payment{ID: inv.ID + "-payment", OrganizationID: org, LocationID: loc, InvoiceID: inv.ID, CustomerID: customerID,
			AmountCents: inv.PaidCents, Currency: "CAD", Method: method, Status: "completed", Notes: "Legacy BookingPress payment (gateway: " + gateway + ")", RecordedAt: bookedAt, CreatedAt: bookedAt})
	}
	// The issuer the franchise has now; none bound means the document simply has no issuer block yet.
	issuer, err := scanInvoiceIssuer(s.pool.QueryRow(ctx, `SELECT i.id::text,i.legal_name,i.address,i.tax_registration,i.currency,i.number_prefix,i.version,i.next_number FROM invoice_issuers i JOIN location_invoice_issuers b ON b.organization_id=i.organization_id AND b.issuer_id=i.id WHERE b.organization_id=$1 AND b.location_id=$2`, org, loc))
	if err == nil {
		snap := issuer.InvoiceIssuerSnapshot
		inv.IssuerSnapshot = &snap
		inv.IssuerID = &snap.ID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, err
	}
	return inv, nil
}
