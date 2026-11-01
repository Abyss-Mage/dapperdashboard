package postgres

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// All booking mutations that may wait on a reservation take the resource and
// reservation locks before the booking row, matching cancellation/scheduling.
func lockBookingAllocation(ctx context.Context, tx pgx.Tx, scope Scope, id string) (*string, error) {
	var reservation, resource *string
	err := tx.QueryRow(ctx, `SELECT b.reservation_id::text,r.resource_id::text FROM bookings b LEFT JOIN capacity_reservations r ON (r.organization_id,r.location_id,r.id)=(b.organization_id,b.location_id,b.reservation_id) WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3`, scope.OrganizationID, scope.LocationID, id).Scan(&reservation, &resource)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBookingNotFound
	}
	if err != nil {
		return nil, err
	}
	if reservation == nil {
		return nil, nil
	}
	if resource == nil {
		return nil, ErrReservationConflict
	}
	var locked string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, *resource).Scan(&locked); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT resource_id::text FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, scope.OrganizationID, scope.LocationID, *reservation).Scan(&locked); err != nil {
		return nil, err
	}
	if locked != *resource {
		return nil, ErrReservationConflict
	}
	return reservation, nil
}

func (s *Store) cancelBooking(ctx context.Context, scope Scope, id, reason, notes string, versions []int) (Booking, error) {
	return s.cancelBookingFrom(ctx, scope, id, reason, notes, versions, false)
}

// cancelBookingFrom cancels and releases the held time slot. allowCompleted lets staff correct a completed booking.
func (s *Store) cancelBookingFrom(ctx context.Context, scope Scope, id, reason, notes string, versions []int, allowCompleted bool) (Booking, error) {
	reason = strings.TrimSpace(reason)
	notes = strings.TrimSpace(notes)
	if len(versions) > 1 || len(versions) == 1 && versions[0] < 0 || utf8.RuneCountInString(reason) > 2000 || utf8.RuneCountInString(notes) > 2000 {
		return Booking{}, ErrBookingInput
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.manage"); err != nil {
		return Booking{}, err
	}
	var reservationID *string
	if err = tx.QueryRow(ctx, `SELECT reservation_id::text FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id).Scan(&reservationID); errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	} else if err != nil {
		return Booking{}, err
	}
	var resourceID string
	var version int
	var cancelledAt *time.Time
	if reservationID != nil {
		if err = authorizeTransaction(ctx, tx, scope, "scheduling.release"); err != nil {
			return Booking{}, err
		}
		if err = tx.QueryRow(ctx, `SELECT resource_id::text FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, *reservationID).Scan(&resourceID); err != nil {
			return Booking{}, ErrBookingReservation
		}
		var locked string
		if err = tx.QueryRow(ctx, `SELECT id::text FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, resourceID).Scan(&locked); err != nil {
			return Booking{}, err
		}
		if err = tx.QueryRow(ctx, `SELECT version,cancelled_at FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, *reservationID).Scan(&version, &cancelledAt); err != nil {
			return Booking{}, err
		}
	} else if len(versions) == 1 && versions[0] != 0 {
		return Booking{}, ErrReservationConflict
	}
	booking, err := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3 FOR UPDATE OF b`, scope.OrganizationID, scope.LocationID, id))
	if err != nil {
		return Booking{}, err
	}
	if (booking.ReservationID == nil) != (reservationID == nil) || booking.ReservationID != nil && *booking.ReservationID != *reservationID {
		return Booking{}, ErrReservationConflict
	}
	if booking.Status == "cancelled" {
		if reservationID != nil && (cancelledAt == nil || len(versions) == 1 && versions[0] != version && versions[0] != version-1) {
			return Booking{}, ErrReservationConflict
		}
		var actor, eventNotes string
		err = tx.QueryRow(ctx, `SELECT actor_id::text,notes FROM booking_events WHERE organization_id=$1 AND location_id=$2 AND booking_id=$3 AND to_status='cancelled' ORDER BY created_at DESC,id DESC LIMIT 1`, scope.OrganizationID, scope.LocationID, id).Scan(&actor, &eventNotes)
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingTransition
		}
		if err != nil {
			return Booking{}, err
		}
		if actor != scope.ActorID || eventNotes != notes || booking.CancellationReason != reason {
			return Booking{}, ErrBookingCapacityRequest
		}
		if err = tx.Commit(ctx); err != nil {
			return Booking{}, bookingWriteError(err)
		}
		return bookingMutationSummary(booking), nil
	}
	if booking.Status == "completed" && !allowCompleted {
		return Booking{}, ErrBookingTransition
	}
	if reservationID != nil && (cancelledAt != nil || len(versions) == 1 && versions[0] != version) {
		return Booking{}, ErrReservationConflict
	}
	oldStatus := booking.Status
	if _, err = tx.Exec(ctx, `UPDATE bookings SET status='cancelled',cancellation_reason=$4,updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id, reason); err != nil {
		return Booking{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_events(organization_id,location_id,booking_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,$5,'cancelled',$6)`, scope.OrganizationID, scope.LocationID, id, scope.ActorID, oldStatus, notes); err != nil {
		return Booking{}, err
	}
	if reservationID != nil {
		if _, err = s.releaseCapacityInTransaction(ctx, tx, scope, resourceID, *reservationID, version); err != nil {
			return Booking{}, err
		}
	}
	updated, err := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.id=$1`, id))
	if err != nil {
		return Booking{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, bookingWriteError(err)
	}
	return bookingMutationSummary(updated), nil
}
