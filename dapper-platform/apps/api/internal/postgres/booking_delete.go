package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrBookingKept: the booking was cancelled, but it carries records the database keeps for the books (an issued invoice, a payment, a
// Square payment, an employee's work log), so it could not be erased.
var ErrBookingKept = errors.New("the booking has money or work records that are kept, so it was cancelled instead of erased")

// DeleteBooking erases a booking for good (owner decision, tester report #79). It is cancelled first, quietly (no email), which takes back
// any cash, e-transfer or cheque payment and voids its invoices. Then the booking, its draft invoices and its links are removed in one go.
// Records the database protects for the books (issued invoices, payments, Square history, employee logs) are never removed: if any exist,
// nothing is erased and ErrBookingKept is returned, with the booking left cancelled.
func (s *Store) DeleteBooking(ctx context.Context, scope Scope, id string) error {
	var status string
	if err := s.pool.QueryRow(ctx, `SELECT status FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id::text=$3`, scope.OrganizationID, scope.LocationID, id).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return ErrBookingNotFound
	} else if err != nil {
		return err
	}
	if status != "cancelled" {
		if _, err := s.UpdateBookingStatus(context.WithValue(QuietBookingEvents(ctx), keepDraftsKey{}, true), scope, id, "cancelled", "Deleted by staff", ""); err != nil {
			return err
		}
	} else {
		_ = s.settleCancelledBooking(context.WithValue(ctx, keepDraftsKey{}, true), scope, id)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.manage"); err != nil {
		return err
	}
	inv := `(SELECT id FROM invoices WHERE organization_id=$1 AND location_id=$2 AND booking_id::text=$3)`
	steps := []string{
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
			// A protected record (issued invoice, payment, Square or employee history) is in the way: keep everything, cancelled.
			return ErrBookingKept
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return ErrBookingKept
	}
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'booking.erased',$4)`, scope.OrganizationID, scope.LocationID, scope.ActorID, id)
	return nil
}
