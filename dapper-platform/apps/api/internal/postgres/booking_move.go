package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// MoveUnreservedBooking changes the time of an open booking that holds no calendar reservation (for example one an
// administrator entered by hand). Bookings with a reservation move through RescheduleBooking, which also moves the
// reserved capacity. The change is written to the booking's history. It needs bookings.manage.
func (s *Store) MoveUnreservedBooking(ctx context.Context, scope Scope, id string, to time.Time) (Booking, error) {
	if to.IsZero() {
		return Booking{}, ErrBookingInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.manage"); err != nil {
		return Booking{}, err
	}
	var status string
	var reservation *string
	var from time.Time
	err = tx.QueryRow(ctx, `SELECT status, reservation_id::text, booking_time FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id).Scan(&status, &reservation, &from)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if status == "completed" || status == "cancelled" || reservation != nil {
		return Booking{}, ErrBookingTransition
	}
	if _, err = tx.Exec(ctx, `UPDATE bookings SET booking_time=$4, updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id, to); err != nil {
		return Booking{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_events(booking_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,$5,$5,$6)`,
		id, scope.OrganizationID, scope.LocationID, scope.ActorID, status, "Rescheduled from "+from.UTC().Format("2006-01-02 15:04")+" UTC to "+to.UTC().Format("2006-01-02 15:04")+" UTC"); err != nil {
		return Booking{}, err
	}
	updated, err := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON c.organization_id=b.organization_id AND c.location_id=b.location_id AND c.id=b.customer_id WHERE b.id=$1`, id))
	if err != nil {
		return Booking{}, err
	}
	return updated, tx.Commit(ctx)
}
