package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"testing"
	"time"
)

func TestWeeklyHoursValidationAndDaylightSaving(t *testing.T) {
	for _, bad := range [][]HourWindow{
		{{Day: 7, Start: "09:00", End: "17:00"}},
		{{Day: 0, Start: "17:00", End: "09:00"}},
		{{Day: 0, Start: "09:00", End: "12:00"}, {Day: 0, Start: "11:00", End: "13:00"}},
		{{Day: 0, Start: "9:00", End: "17:00"}},
		{{Day: 0, Start: "09:00", End: "25:00"}},
	} {
		if ValidateWeeklyHours(bad) == nil {
			t.Fatalf("accepted invalid hours %+v", bad)
		}
	}
	if err := ValidateWeeklyHours([]HourWindow{{Day: 0, Start: "09:00", End: "12:00"}, {Day: 0, Start: "13:00", End: "24:00"}}); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Toronto")
	// Sunday 09:00-10:00 before and after the 2026-11-01 fall-back keeps the local time but changes the UTC offset.
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, loc)
	got := expandWeekly(loc, []HourWindow{{Day: 6, Start: "09:00", End: "10:00"}}, now, 14)
	if len(got) != 2 {
		t.Fatalf("expected two Sundays, got %d", len(got))
	}
	if got[0].Start.UTC().Hour() != 13 || got[1].Start.UTC().Hour() != 14 {
		t.Fatalf("daylight saving not honoured: %v %v", got[0].Start.UTC(), got[1].Start.UTC())
	}
	// A window that already ended today is not generated.
	late := time.Date(2026, 10, 26, 18, 0, 0, 0, loc) // Monday evening
	if n := len(expandWeekly(loc, []HourWindow{{Day: 0, Start: "09:00", End: "17:00"}}, late, 1)); n != 0 {
		t.Fatalf("an ended window was generated (%d)", n)
	}
}

func verifySchedulingWeeklyHours(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	res, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic hours resource", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	week := []HourWindow{{Day: 0, Start: "09:00", End: "17:00"}, {Day: 2, Start: "10:00", End: "14:00"}}
	if err = s.SaveWeeklyHours(ctx, a, res.ID, week); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveWeeklyHours(ctx, b, res.ID, week); err == nil {
		t.Fatal("another franchise changed this resource's hours")
	}
	read := func() SchedulingResource {
		items, err := s.SchedulingResources(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		for _, i := range items {
			if i.ID == res.ID {
				return i
			}
		}
		t.Fatal("resource missing")
		return SchedulingResource{}
	}
	got := read()
	if len(got.WeeklyHours) != 2 || len(got.Plan.Open) < 20 || len(got.Plan.Open) > 26 {
		t.Fatalf("expected two weekdays over 12 weeks of windows, got %d weekly, %d open", len(got.WeeklyHours), len(got.Plan.Open))
	}
	day := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	if err = s.AddClosure(ctx, a, res.ID, day); err != nil {
		t.Fatal(err)
	}
	if err = s.AddClosure(ctx, a, res.ID, day); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("duplicate closure accepted", err)
	}
	got = read()
	if len(got.Plan.Closed) != 1 {
		t.Fatalf("closure not stored: %+v", got.Plan.Closed)
	}
	if err = s.RemoveClosure(ctx, a, res.ID, got.Plan.Closed[0].Start); err != nil {
		t.Fatal(err)
	}
	if len(read().Plan.Closed) != 0 {
		t.Fatal("closure not removed")
	}
	if err = s.SaveWeeklyHours(ctx, a, res.ID, nil); err != nil {
		t.Fatal(err)
	}
	for _, i := range read().Plan.Open {
		if i.End.After(time.Now()) {
			t.Fatal("clearing the weekly hours left future opening windows")
		}
	}
	if err = s.SaveWeeklyHours(ctx, a, res.ID, week); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ExtendWeeklyHours(ctx); err != nil || n < 1 {
		t.Fatal("roll-forward failed", n, err)
	}
}

func verifyWorkingTimeRule(t *testing.T, ctx context.Context, s *Store, a Scope) {
	t.Helper()
	var service string
	if err := s.pool.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind) VALUES($1,$2,'Synthetic multi-day service','main') RETURNING id::text`, a.OrganizationID, a.LocationID).Scan(&service); err != nil {
		t.Fatal(err)
	}
	res, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic long jobs", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	week := make([]HourWindow, 0, 7)
	for d := 0; d < 7; d++ {
		week = append(week, HourWindow{Day: d, Start: "08:00", End: "21:00"})
	}
	if err = s.SaveWeeklyHours(ctx, a, res.ID, week); err != nil {
		t.Fatal(err)
	}
	// 15 hours is longer than the 13 hour opening day: refused as clock time, accepted as working time.
	clock, err := s.SaveSchedulingRule(ctx, a, SchedulingRule{ResourceID: res.ID, ServiceID: service, DurationMinutes: 900, Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Toronto")
	day := time.Now().In(loc).AddDate(0, 0, 3)
	start := time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, loc)
	if _, err = s.ReserveServiceCapacity(ctx, a, res.ID, clock.ID, clock.Version, start); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("a 15 hour clock-time job fitted a 13 hour day", err)
	}
	edited := clock
	edited.WorkingTime = true
	rule, err := s.SaveSchedulingRule(ctx, a, edited)
	if err != nil || !rule.WorkingTime || rule.Version != 2 {
		t.Fatal("working-time flag not saved", err)
	}
	got, err := s.ReserveServiceCapacity(ctx, a, res.ID, rule.ID, rule.Version, start)
	if err != nil {
		t.Fatal("working-time reservation failed", err)
	}
	wantEnd := time.Date(day.Year(), day.Month(), day.Day()+1, 10, 0, 0, 0, loc)
	if !got.Occupied.Start.Equal(start) || !got.Occupied.End.Equal(wantEnd) {
		t.Fatalf("occupied span %v - %v, want %v - %v", got.Occupied.Start, got.Occupied.End, start, wantEnd)
	}
	// One at a time: a second job overlapping the next morning is refused.
	if _, err = s.ReserveServiceCapacity(ctx, a, res.ID, rule.ID, rule.Version, start.AddDate(0, 0, 1)); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("overlapping long job accepted on a one-at-a-time resource", err)
	}
}

// A working-time booking spans closed hours and a base-rule booking may carry add-ons; the booking/reservation consistency
// trigger must accept both (migration 59) while still rejecting real mismatches.
func verifyWorkingTimeAndAddonBookings(t *testing.T, ctx context.Context, s *Store, a Scope, customer string) {
	t.Helper()
	main, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic long booking main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	addon, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic long booking add-on", Kind: "addon", ParentID: &main.ID})
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic long booking resource", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	week := make([]HourWindow, 0, 7)
	for d := 0; d < 7; d++ {
		week = append(week, HourWindow{Day: d, Start: "08:00", End: "21:00"})
	}
	if err = s.SaveWeeklyHours(ctx, a, res.ID, week); err != nil {
		t.Fatal(err)
	}
	rule, err := s.SaveSchedulingRule(ctx, a, SchedulingRule{ResourceID: res.ID, ServiceID: main.ID, DurationMinutes: 900, Units: 1, WorkingTime: true})
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Toronto")
	day := time.Now().In(loc).AddDate(0, 0, 5)
	start := time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, loc)
	booking, err := s.ReserveBooking(ctx, a, ReserveBookingInput{RequestKey: "working-time-booking-0001", CustomerID: customer, ResourceID: res.ID, RuleID: rule.ID, RuleVersion: rule.Version, Start: start.UTC(), Channel: "phone"})
	if err != nil {
		t.Fatal("a working-time booking was rejected by the consistency check", err)
	}
	var span time.Duration
	if err = s.pool.QueryRow(ctx, `SELECT r.occupied_end-r.occupied_start FROM capacity_reservations r JOIN bookings b ON b.reservation_id=r.id WHERE b.id=$1`, booking.ID).Scan(&span); err != nil || span < 24*time.Hour {
		t.Fatalf("working-time booking should occupy more than a day of clock time, got %v %v", span, err)
	}
	// Add-ons on a base rule: the booking may carry its service's add-ons without changing the reservation.
	base, err := s.SaveSchedulingRule(ctx, a, SchedulingRule{ResourceID: res.ID, ServiceID: main.ID, AddonIDs: nil, DurationMinutes: 60, Units: 1})
	if err == nil || !errors.Is(err, ErrSchedulingRuleConflict) {
		_ = base // a second base rule for the same service is a duplicate; the rule above is the base rule (working time)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE bookings SET addons=$2::jsonb WHERE id=$1`, booking.ID, `[{"serviceId":"`+addon.ID+`","name":"Synthetic long booking add-on"}]`); err != nil {
		t.Fatal("a base-rule booking with an add-on was rejected", err)
	}
	// A real mismatch is still refused: a different service on the booking.
	if _, err = s.pool.Exec(ctx, `UPDATE bookings SET service_id=$2 WHERE id=$1`, booking.ID, addon.ID); err == nil {
		t.Fatal("a booking whose service differs from its reservation was accepted")
	}
}
