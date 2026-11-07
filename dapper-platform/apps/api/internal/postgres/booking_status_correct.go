package postgres

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// CorrectBookingStatus is the staff status control on the appointments page. Normal forward moves behave exactly as
// UpdateBookingStatus. On top of that staff can change a completed or cancelled booking, because records get marked
// wrongly: completed to cancelled releases its time slot, and reopening a cancelled booking detaches its released slot
// (the booking stays on the books without holding capacity; move it to hold a new time). Every change is recorded in
// the booking history.
func (s *Store) correctBookingStatus(ctx context.Context, scope Scope, id, toStatus, reason, notes string, versions ...int) (Booking, error) {
	toStatus = strings.TrimSpace(toStatus)
	reason = strings.TrimSpace(reason)
	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > 2000 || utf8.RuneCountInString(reason) > 2000 {
		return Booking{}, ErrBookingInput
	}
	var current string
	err := s.pool.QueryRow(ctx, `SELECT status FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	terminal := current == "completed" || current == "cancelled"
	if !terminal {
		b, err := s.UpdateBookingStatus(ctx, scope, id, toStatus, reason, notes, versions...)
		if errors.Is(err, ErrBookingTransition) && toStatus == "confirmed" {
			return s.setBookingStatusDirect(ctx, scope, id, toStatus, notes)
		}
		return b, err
	}
	switch {
	case toStatus == current && current == "cancelled":
		// a repeated cancellation is answered by the cancellation rules (idempotent retry)
		return s.cancelBookingFrom(ctx, scope, id, reason, notes, versions, false)
	case toStatus == current:
		return s.setBookingStatusDirect(ctx, scope, id, toStatus, notes)
	case current == "completed" && toStatus == "cancelled":
		return s.cancelBookingFrom(ctx, scope, id, reason, notes, versions, true)
	case toStatus == "confirmed" || toStatus == "completed":
		return s.setBookingStatusDirect(ctx, scope, id, toStatus, notes)
	}
	return Booking{}, ErrBookingTransition
}

// setBookingStatusDirect changes the status without the forward-only rule. A booking leaving "cancelled" gives up its
// released reservation so it stays consistent with the capacity records.
func (s *Store) setBookingStatusDirect(ctx context.Context, scope Scope, id, toStatus, notes string) (Booking, error) {
	if toStatus != "confirmed" && toStatus != "completed" {
		return Booking{}, ErrBookingTransition
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.manage"); err != nil {
		return Booking{}, err
	}
	if _, err = lockBookingAllocation(ctx, tx, scope, id); err != nil {
		return Booking{}, err
	}
	var current string
	if err = tx.QueryRow(ctx, `SELECT status FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	} else if err != nil {
		return Booking{}, err
	}
	if current != toStatus {
		if _, err = tx.Exec(ctx, `UPDATE bookings SET status=$1,
reservation_id=CASE WHEN status='cancelled' THEN NULL ELSE reservation_id END,
cancellation_reason=CASE WHEN status='cancelled' THEN '' ELSE cancellation_reason END,
updated_at=now() WHERE organization_id=$2 AND location_id=$3 AND id=$4`, toStatus, scope.OrganizationID, scope.LocationID, id); err != nil {
			return Booking{}, bookingWriteError(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO booking_events(booking_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,$5,$6,$7)`,
			id, scope.OrganizationID, scope.LocationID, scope.ActorID, current, toStatus, "Status corrected by staff"); err != nil {
			return Booking{}, err
		}
	}
	notice := ""
	if current == "completed" && toStatus != "completed" {
		if notice, err = reverseCompletionPayments(ctx, tx, scope, id); err != nil {
			return Booking{}, err
		}
		if notice != "" {
			if _, err = tx.Exec(ctx, `INSERT INTO booking_events(booking_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,$5,$6,$7)`,
				id, scope.OrganizationID, scope.LocationID, scope.ActorID, toStatus, toStatus, notice); err != nil {
				return Booking{}, err
			}
		}
	}
	updated, err := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.id=$1`, id))
	if err != nil {
		return Booking{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, bookingWriteError(err)
	}
	updated.PaymentNotice = notice
	return updated, nil
}

// CorrectBookingStatus is the staff status change; a cancellation is announced so the customer can be emailed.
func (s *Store) CorrectBookingStatus(ctx context.Context, scope Scope, id, toStatus, reason, notes string, versions ...int) (Booking, error) {
	b, err := s.correctBookingStatus(ctx, scope, id, toStatus, reason, notes, versions...)
	if err == nil && toStatus == "cancelled" {
		s.emitBooking(ctx, "booking_cancelled", id)
	}
	return b, err
}
