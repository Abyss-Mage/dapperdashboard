package postgres

import (
	"context"
	"errors"
)

// ErrEmployeeStatusRule means an employee tried to change a booking status that only an administrator manages.
var ErrEmployeeStatusRule = errors.New("employees can only mark walk-in bookings completed or cancelled; an administrator manages every other status")

// CheckEmployeeBookingStatus enforces the employee rule: a principal assigned to a franchise as an employee who does not
// hold scheduling.manage (the administrative capability) may only mark a walk-in booking completed or cancelled.
// Administrators and owners are unaffected.
func (s *Store) CheckEmployeeBookingStatus(ctx context.Context, scope Scope, bookingID, toStatus string) error {
	var assigned bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM employee_worksite_assignments WHERE principal_id=$1 AND organization_id=$2)`, scope.ActorID, scope.OrganizationID).Scan(&assigned); err != nil {
		return err
	}
	if !assigned {
		return nil
	}
	if err := s.authorize(ctx, scope, "scheduling.manage"); err == nil {
		return nil
	} else if !errors.Is(err, ErrForbidden) {
		return err
	}
	var channel string
	if err := s.pool.QueryRow(ctx, `SELECT channel FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, bookingID).Scan(&channel); err != nil {
		return ErrBookingNotFound
	}
	if channel != "walk_in" || (toStatus != "completed" && toStatus != "cancelled") {
		return ErrEmployeeStatusRule
	}
	return nil
}

// EmployeeSetBookingStatus lets an employee complete or cancel a walk-in booking at their own franchise.
func (s *Store) EmployeeSetBookingStatus(ctx context.Context, actor, booking, toStatus, reason string) (Booking, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	a, err := employeeWorksite(ctx, tx, actor)
	if err == nil {
		err = employeeBookingLock(ctx, tx, a, booking)
	}
	_ = tx.Rollback(ctx)
	if err != nil {
		return Booking{}, err
	}
	scope := employeeScope(a)
	if err = s.CheckEmployeeBookingStatus(ctx, scope, booking, toStatus); err != nil {
		return Booking{}, err
	}
	if toStatus == "cancelled" {
		var version int
		if err = s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT r.version FROM capacity_reservations r WHERE r.id=b.reservation_id),0) FROM bookings b WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3`, scope.OrganizationID, scope.LocationID, booking).Scan(&version); err != nil {
			return Booking{}, ErrBookingNotFound
		}
		return s.UpdateBookingStatus(ctx, scope, booking, toStatus, reason, "Employee status update", version)
	}
	return s.UpdateBookingStatus(ctx, scope, booking, toStatus, reason, "Employee status update")
}
