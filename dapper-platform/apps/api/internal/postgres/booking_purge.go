package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrBookingPurgeNumber: the typed booking number does not match the booking.
	ErrBookingPurgeNumber = errors.New("the typed booking number does not match")
)

// PurgeBooking removes a test booking for good with everything attached to it: its invoices (issued ones too), their items, events and
// pay links, payments recorded by hand and their refund history, requests, email log and links (owner 2026-11-14, error404 #82 #84 #86).
// Only the owner and administrators (organization access.manage) may, and only after typing the booking's number. Square card records
// are removed too (owner 2026-11-14, #126); refunds in Square are the owner's to handle. Recorded in the activity log.
func (s *Store) PurgeBooking(ctx context.Context, scope Scope, id, typedNumber string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ok, err := actorHasOrgCapability(ctx, tx, scope.ActorID, scope.OrganizationID, "access.manage")
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	var number int64
	err = tx.QueryRow(ctx, `SELECT coalesce(number,0) FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id).Scan(&number)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBookingNotFound
	}
	if err != nil {
		return err
	}
	if typed, e := strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(typedNumber), "#"), 10, 64); e != nil || typed != number || number == 0 {
		return ErrBookingPurgeNumber
	}
	inv := `(SELECT id FROM invoices WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3)`
	// Lets the "must be retained" protections allow these deletes, for this transaction only (booking_purge.sql).
	if _, err = tx.Exec(ctx, `SELECT set_config('dapper.purge_booking','on',true)`); err != nil {
		return err
	}
	pay := `(SELECT id FROM payments WHERE invoice_id IN ` + inv + `)`
	intents := `(SELECT id FROM square_payment_intents WHERE invoice_id IN ` + inv + `)`
	steps := []string{
		// Square card attempts and their history go too (owner 2026-11-14, error404 #126); Square itself is not touched.
		`DELETE FROM square_payment_allocations WHERE intent_id IN ` + intents,
		`DELETE FROM square_payment_capture_observations WHERE intent_id IN ` + intents,
		`DELETE FROM square_payment_intent_events WHERE intent_id IN ` + intents,
		`DELETE FROM square_payment_intents WHERE invoice_id IN ` + inv,
		`DELETE FROM payment_refund_adjustments WHERE payment_id IN ` + pay,
		`DELETE FROM payments WHERE invoice_id IN ` + inv,
		`DELETE FROM coupon_redemptions WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_pay_links WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_ninja_exports WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_draft_revisions WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_events WHERE invoice_id IN ` + inv,
		`DELETE FROM invoice_items WHERE invoice_id IN ` + inv,
		`DELETE FROM invoices WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3`,
		`DELETE FROM booking_email_log WHERE booking_id::text=$3 AND $1::uuid IS NOT NULL AND $2::uuid IS NOT NULL`,
		`DELETE FROM booking_capacity_requests WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3`,
		`DELETE FROM booking_reschedule_requests WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3`,
		`DELETE FROM booking_job_requests WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3`,
		`DELETE FROM employee_booking_worksites WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3`,
		`DELETE FROM legacy_booking_billing WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3`,
		`UPDATE jobs SET booking_id=NULL WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3`,
		`UPDATE employee_upload_posts SET booking_id=NULL WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3`,
		`DELETE FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id::text=$3`,
	}
	for _, q := range steps {
		if _, err = tx.Exec(ctx, q, scope.OrganizationID, scope.LocationID, id); err != nil {
			// Something still refers to the booking; nothing is removed. The message names the step for the owner to report.
			return fmt.Errorf("permanent delete stopped at %q: %w", strings.Fields(q)[2], err)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4,$5)`, scope.OrganizationID, scope.LocationID, scope.ActorID, "booking.purged."+strconv.FormatInt(number, 10), id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
