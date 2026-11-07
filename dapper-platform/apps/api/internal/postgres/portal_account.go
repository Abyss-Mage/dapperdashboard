package postgres

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// portalLink checks that this signed-in person is linked to the customer record and holds the link for the rest of the transaction.
func portalLink(ctx context.Context, tx pgx.Tx, actor, customer string) (org, loc string, err error) {
	if actor == "" || customer == "" {
		return "", "", ErrForbidden
	}
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text FROM customer_account_links a
 JOIN principals p ON p.id=a.principal_id AND p.active
 WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`, actor, customer).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrForbidden
	}
	return org, loc, err
}

// UpdatePortalDetails lets a customer correct their own name, phone and address. Email is their sign-in and changes elsewhere.
func (s *Store) UpdatePortalDetails(ctx context.Context, actor, customer, name, phone, address string) error {
	name, phone, address = strings.TrimSpace(name), strings.TrimSpace(phone), strings.TrimSpace(address)
	if n := utf8.RuneCountInString(name); n < 2 || n > 100 || utf8.RuneCountInString(phone) > 30 || utf8.RuneCountInString(address) > 300 {
		return ErrCustomerInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE customers SET name=$4,phone=$5,address=$6 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND removed_at IS NULL`, org, loc, customer, name, phone, address)
	if err != nil {
		return customerUniqueness(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrCustomerNotFound
	}
	return tx.Commit(ctx)
}

// PortalInvoice returns one issued, partly paid or paid invoice of the customer's own, with its lines, for viewing and printing.
func (s *Store) PortalInvoice(ctx context.Context, actor, customer, invoiceID string) (Invoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return Invoice{}, err
	}
	inv, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i
 JOIN customers c ON c.organization_id=i.organization_id AND c.location_id=i.location_id AND c.id=i.customer_id
 WHERE i.id=$1 AND i.organization_id=$2 AND i.location_id=$3 AND i.customer_id=$4 AND i.status IN ('issued','partially_paid','paid')`, invoiceID, org, loc, customer))
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text, invoice_id::text, description, quantity, unit_price_cents, total_price_cents, created_at FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at ASC`, inv.ID)
	if err != nil {
		return Invoice{}, err
	}
	defer rows.Close()
	inv.Items = []InvoiceItem{}
	for rows.Next() {
		var it InvoiceItem
		if err = rows.Scan(&it.ID, &it.InvoiceID, &it.Description, &it.Quantity, &it.UnitPriceCents, &it.TotalPriceCents, &it.CreatedAt); err != nil {
			return Invoice{}, err
		}
		inv.Items = append(inv.Items, it)
	}
	if err = rows.Err(); err != nil {
		return Invoice{}, err
	}
	// Staff-only detail stays out of the customer's copy.
	inv.IssuedBy, inv.Notes = nil, ""
	return inv, nil
}

// PortalLegacyInvoice is the invoice of one of the customer's own bookings imported from the old site, built from its recorded figures.
func (s *Store) PortalLegacyInvoice(ctx context.Context, actor, customer, bookingID string) (Invoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return Invoice{}, err
	}
	var own bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND customer_id=$4)`, org, loc, bookingID, customer).Scan(&own); err != nil {
		return Invoice{}, err
	}
	if !own {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	inv, err := s.legacyInvoice(ctx, org, loc, bookingID)
	if err != nil {
		return Invoice{}, err
	}
	inv.IssuedBy, inv.Notes = nil, ""
	return inv, nil
}
