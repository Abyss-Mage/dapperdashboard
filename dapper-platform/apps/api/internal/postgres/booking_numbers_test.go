package postgres

import (
	"context"
	"testing"
)

// Bookings get a short number from one sequence per organization; an explicit number (an imported booking's old ID) is kept and moves the sequence on.
func verifyBookingNumbers(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	var customer string
	if err := s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Numbered Customer','numbers@example.test','') RETURNING id::text`, a.OrganizationID, a.LocationID).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	defer s.pool.Exec(ctx, `DELETE FROM customers WHERE id=$1`, customer)
	add := func(number *int64) (n int64, err error) {
		err = s.pool.QueryRow(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time,number) VALUES($1,$2,$3,'Number test','online','confirmed','2030-01-01T12:00:00Z',$4) RETURNING number`, a.OrganizationID, a.LocationID, customer, number).Scan(&n)
		return
	}
	defer s.pool.Exec(ctx, `DELETE FROM bookings WHERE customer_id=$1`, customer)
	first, err := add(nil)
	if err != nil || first < 1 {
		t.Fatal("a booking got no number", first, err)
	}
	if second, err := add(nil); err != nil || second != first+1 {
		t.Fatal("numbers should follow one another", first, second, err)
	}
	old := first + 100
	if got, err := add(&old); err != nil || got != old {
		t.Fatal("an explicit (imported) number was not kept", got, err)
	}
	if next, err := add(nil); err != nil || next != old+1 {
		t.Fatal("the sequence did not continue after an imported number", next, err)
	}
	if _, err = add(&old); err == nil {
		t.Fatal("a booking number was used twice in one organization")
	}
}
