package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"testing"
	"time"
)

func verifyBookingReschedule(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	for _, action := range []string{"bookings.create", "bookings.manage", "scheduling.reserve", "scheduling.release", "scheduling.manage", "catalogue.manage"} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID, action); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Date(2027, 2, 1, 9, 0, 0, 0, time.UTC)
	resources := make([]SchedulingResource, 2)
	rules := make([]SchedulingRule, 2)
	service, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic reschedule service", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range resources {
		resources[i], err = s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic reschedule bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(10 * time.Hour)}}}})
		if err != nil {
			t.Fatal(err)
		}
		rules[i], err = s.SaveSchedulingRule(ctx, a, SchedulingRule{ResourceID: resources[i].ID, ServiceID: service.ID, DurationMinutes: 60, Units: 1})
		if err != nil {
			t.Fatal(err)
		}
	}
	create := func(key string, r int, at time.Time) Booking {
		t.Helper()
		v, e := s.ReserveBooking(ctx, a, ReserveBookingInput{RequestKey: key, CustomerID: customer, ResourceID: resources[r].ID, RuleID: rules[r].ID, RuleVersion: 1, Start: at, Channel: "internal"})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	one := create("synthetic-reschedule-source-001", 0, start)
	input := RescheduleBookingInput{RequestKey: "synthetic-reschedule-request-001", ResourceID: resources[1].ID, RuleID: rules[1].ID, RuleVersion: 1, ReservationVersion: 1, Start: start.Add(2 * time.Hour), Notes: "Synthetic move"}
	if _, err = s.RescheduleBooking(ctx, b, one.ID, input); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign move allowed", err)
	}
	stale := input
	stale.ReservationVersion = 2
	if _, err = s.RescheduleBooking(ctx, a, one.ID, stale); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("stale move allowed", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT synthetic_move_failure CHECK(action<>'bookings.rescheduled') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RescheduleBooking(ctx, a, one.ID, input); err == nil {
		t.Fatal("audit failure committed move")
	}
	var old time.Time
	var resource string
	var version int
	if err = s.pool.QueryRow(ctx, `SELECT b.booking_time,r.resource_id::text,r.version FROM bookings b JOIN capacity_reservations r ON r.id=b.reservation_id WHERE b.id=$1`, one.ID).Scan(&old, &resource, &version); err != nil || !old.Equal(start) || resource != resources[0].ID || version != 1 {
		t.Fatal("failed move lost original allocation", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT synthetic_move_failure`); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		booking Booking
		err     error
	}
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() { v, e := s.RescheduleBooking(ctx, a, one.ID, input); results <- outcome{v, e} }()
	}
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil || r.booking.ID != one.ID || r.booking.ReservationVersion == nil || *r.booking.ReservationVersion != 2 || !r.booking.BookingTime.Equal(input.Start) || r.booking.Status != "confirmed" || r.booking.Notes != "" || r.booking.CustomerEmail != "" {
			t.Fatal("duplicate move failed/leaked", r.err)
		}
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM booking_reschedule_requests WHERE booking_id=$1`, one.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate move receipt", err)
	}
	changed := input
	changed.Notes = "Different"
	if _, err = s.RescheduleBooking(ctx, a, one.ID, changed); !errors.Is(err, ErrBookingCapacityRequest) {
		t.Fatal("changed key accepted", err)
	}
	create("synthetic-reschedule-freed-slot", 0, start)
	blocker := create("synthetic-reschedule-blocker-001", 0, start.Add(4*time.Hour))
	_ = blocker
	failed := input
	failed.RequestKey = "synthetic-reschedule-blocked-001"
	failed.ResourceID = resources[0].ID
	failed.RuleID = rules[0].ID
	failed.ReservationVersion = 2
	failed.Start = start.Add(4 * time.Hour)
	if _, err = s.RescheduleBooking(ctx, a, one.ID, failed); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("occupied move succeeded", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT b.booking_time,r.resource_id::text,r.version FROM bookings b JOIN capacity_reservations r ON r.id=b.reservation_id WHERE b.id=$1`, one.ID).Scan(&old, &resource, &version); err != nil || !old.Equal(input.Start) || resource != resources[1].ID || version != 2 {
		t.Fatal("occupied move lost original", err)
	}
	// Opposite resource transfers must use the same lock order.
	left := create("synthetic-cross-move-left-001", 0, start.Add(6*time.Hour))
	right := create("synthetic-cross-move-right-001", 1, start.Add(7*time.Hour))
	go func() {
		v, e := s.RescheduleBooking(ctx, a, left.ID, RescheduleBookingInput{RequestKey: "synthetic-cross-request-left", ResourceID: resources[1].ID, RuleID: rules[1].ID, RuleVersion: 1, ReservationVersion: 1, Start: start.Add(6 * time.Hour)})
		results <- outcome{v, e}
	}()
	go func() {
		v, e := s.RescheduleBooking(ctx, a, right.ID, RescheduleBookingInput{RequestKey: "synthetic-cross-request-right", ResourceID: resources[0].ID, RuleID: rules[0].ID, RuleVersion: 1, ReservationVersion: 1, Start: start.Add(7 * time.Hour)})
		results <- outcome{v, e}
	}()
	for i := 0; i < 2; i++ {
		if r := <-results; r.err != nil {
			t.Fatal("opposite resource move deadlocked/failed", r.err)
		}
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND organization_id=$2 AND location_id=$3 AND action='scheduling.release'`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RescheduleBooking(ctx, a, one.ID, input); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked grant replay accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'scheduling.release')`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
}
