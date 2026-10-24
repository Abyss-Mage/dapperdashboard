package postgres

import (
	"context"
	"time"
)

type EmployeeCalendar struct {
	Items     []Booking `json:"items"`
	Truncated bool      `json:"truncated"`
}

func (s *Store) EmployeeCalendarBookings(ctx context.Context, actor, from, to string) (EmployeeCalendar, error) {
	result := EmployeeCalendar{Items: []Booking{}}
	first, e1 := time.Parse("2006-01-02", from)
	last, e2 := time.Parse("2006-01-02", to)
	if e1 != nil || e2 != nil || !last.After(first) || last.Sub(first) > 42*24*time.Hour {
		return result, ErrEmployeeInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return result, err
	}
	if err = authorizeTransaction(ctx, tx, employeeScope(a), "bookings.read"); err != nil {
		return result, err
	}
	z, err := time.LoadLocation(a.Timezone)
	if err != nil {
		return result, ErrEmployeeInput
	}
	start := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, z)
	end := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, z)
	rows, err := tx.Query(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) JOIN employee_booking_worksites w ON (w.organization_id,w.location_id,w.booking_id)=(b.organization_id,b.location_id,b.id) WHERE b.organization_id=$1 AND b.location_id=$2 AND (w.dealership_id IS NULL OR w.dealership_id=NULLIF($3,'')::uuid) AND b.booking_time >= $4 AND b.booking_time < $5 ORDER BY b.booking_time,b.id LIMIT 501`, a.OrganizationID, a.LocationID, a.DealershipID, start, end)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		b, e := scanBooking(rows)
		if e != nil {
			rows.Close()
			return result, e
		}
		result.Items = append(result.Items, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(result.Items) > 500 {
		result.Truncated = true
		result.Items = result.Items[:500]
	}
	return result, tx.Commit(ctx)
}
