package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrPurgeConfirm: the typed invoice number or dealership code does not match.
	ErrPurgeConfirm = errors.New("the typed confirmation does not match")
	// ErrPurgeSquare: a Square card payment was attempted or taken; that history is never removed.
	ErrPurgeSquare = errors.New("square card payment records exist")
	// ErrDealershipInUse: the dealership still has invoices, assigned staff or staff photos, which must go first.
	ErrDealershipInUse = errors.New("the dealership still has invoices, staff or photos")
)

func purgeOwner(ctx context.Context, tx pgx.Tx, scope Scope) error {
	ok, err := actorHasOrgCapability(ctx, tx, scope.ActorID, scope.OrganizationID, "access.manage")
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	// Lets the "must be retained" protections allow these deletes, for this transaction only (booking_purge.sql).
	_, err = tx.Exec(ctx, `SELECT set_config('dapper.purge_booking','on',true)`)
	return err
}

// PurgeDealerInvoice removes a dealership invoice for good with its items, events, pay links and hand-recorded payments (owner
// 2026-11-14, error404 #124). Owner and administrators only (organization access.manage), after typing the invoice number. An invoice with
// any Square card record is refused, because card history must match Square. Recorded in the activity log.
func (s *Store) PurgeDealerInvoice(ctx context.Context, scope Scope, id, typedNumber string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = purgeOwner(ctx, tx, scope); err != nil {
		return err
	}
	var number string
	var square bool
	// A draft is confirmed by the number typed on it, or DELETE when it has none (owner 2026-11-14).
	err = tx.QueryRow(ctx, `SELECT CASE WHEN status='draft' THEN coalesce(nullif(btrim(manual_number),''),'DELETE') ELSE invoice_number END, EXISTS(SELECT 1 FROM square_payment_intents p WHERE p.invoice_id=i.id) FROM invoices i
	 WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND dealership_account_id IS NOT NULL FOR UPDATE`, scope.OrganizationID, scope.LocationID, id).Scan(&number, &square)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvoiceNotFound
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(typedNumber), number) {
		return ErrPurgeConfirm
	}
	if square {
		return ErrPurgeSquare
	}
	if err = purgeInvoiceRows(ctx, tx, `SELECT id FROM invoices WHERE id::text=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4,$5)`, scope.OrganizationID, scope.LocationID, scope.ActorID, "invoice.purged."+number, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// purgeInvoiceRows deletes the invoices the query selects (one argument, $1) and everything recorded against them.
func purgeInvoiceRows(ctx context.Context, tx pgx.Tx, invoices string, arg any) error {
	inv := `(` + invoices + `)`
	steps := []string{
		`DELETE FROM payment_refund_adjustments WHERE payment_id IN (SELECT id FROM payments WHERE invoice_id IN ` + inv + `)`,
		`DELETE FROM payments WHERE invoice_id IN ` + inv,
		`DELETE FROM coupon_redemptions WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_pay_links WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_ninja_exports WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_draft_revisions WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_events WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_items WHERE invoice_id IN ` + inv,
		`DELETE FROM invoices WHERE id IN ` + inv,
	}
	for _, q := range steps {
		if _, err := tx.Exec(ctx, q, arg); err != nil {
			return fmt.Errorf("permanent delete stopped at %q: %w", strings.Fields(q)[2], err)
		}
	}
	return nil
}

// PurgeDealership removes a dealership for good (owner 2026-11-14, error404 #116): its contacts, tags, portal logins and sessions, the
// saved-card references (the cards stay in Square), fleet intake, staff availability for it and the links of bookings to it. The bookings
// and the customer record stay. Refused while it still has invoices (delete those first), staff assigned to it, or staff photos taken there.
// Owner and administrators only, after typing the dealership's code. Recorded in the activity log.
func (s *Store) PurgeDealership(ctx context.Context, scope Scope, id, typedCode string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = purgeOwner(ctx, tx, scope); err != nil {
		return err
	}
	var code string
	var inUse bool
	err = tx.QueryRow(ctx, `SELECT account_code,
	 EXISTS(SELECT 1 FROM invoices WHERE dealership_account_id=d.id) OR EXISTS(SELECT 1 FROM employee_worksite_assignments WHERE dealership_id=d.id)
	 OR EXISTS(SELECT 1 FROM employee_upload_posts WHERE dealership_id=d.id)
	 FROM dealership_accounts d WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id).Scan(&code, &inUse)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDealershipNotFound
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(typedCode), code) {
		return ErrPurgeConfirm
	}
	if inUse {
		return ErrDealershipInUse
	}
	steps := []string{
		`DELETE FROM dealer_portal_sessions WHERE dealership_account_id::text=$1`,
		`DELETE FROM dealer_portal_users WHERE dealership_account_id::text=$1`,
		`DELETE FROM dealer_saved_cards WHERE dealership_account_id::text=$1`,
		`DELETE FROM dealer_square_customers WHERE dealership_account_id::text=$1`,
		`DELETE FROM dealership_fleet_intake WHERE dealership_id::text=$1`,
		`DELETE FROM employee_availability_submissions WHERE dealership_id::text=$1`,
		`DELETE FROM employee_booking_worksites WHERE dealership_id::text=$1`,
		`DELETE FROM dealership_accounts WHERE id::text=$1`, // contacts and tags go with it (ON DELETE CASCADE)
	}
	for _, q := range steps {
		if _, err = tx.Exec(ctx, q, id); err != nil {
			return fmt.Errorf("permanent delete stopped at %q: %w", strings.Fields(q)[2], err)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4,$5)`, scope.OrganizationID, scope.LocationID, scope.ActorID, "dealership.purged."+code, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
