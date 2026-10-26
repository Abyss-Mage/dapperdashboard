package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"sync"
	"testing"
	"time"
)

func verifyCapacityReservations(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	for _, scope := range []Scope{a, b} {
		for _, action := range []string{"scheduling.manage", "scheduling.reserve"} {
			if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, scope.ActorID, scope.OrganizationID, scope.LocationID, action); err != nil {
				t.Fatal(err)
			}
		}
	}
	start := time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)
	resource, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic detailing bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(12 * time.Hour)}}}})
	if err != nil {
		t.Fatal(err)
	}
	request := scheduling.Request{Start: start.Add(time.Hour), Duration: time.Hour, Units: 1}
	if _, err = s.ReserveCapacity(ctx, b, resource.ID, request); err != ErrReservationConflict {
		t.Fatal("foreign resource reserved", err)
	}
	var wg sync.WaitGroup
	type result struct {
		reservation CapacityReservation
		err         error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.ReserveCapacity(ctx, a, resource.ID, request)
			results <- result{r, e}
		}()
	}
	wg.Wait()
	close(results)
	var winner CapacityReservation
	success, conflict := 0, 0
	for result := range results {
		if result.err == nil {
			winner = result.reservation
			success++
		} else if result.err == ErrReservationConflict {
			conflict++
		} else {
			t.Fatal(result.err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("overlap capacity not serialized", success, conflict)
	}
	// DST fall-back repeats01:00. These actual adjacent one-hour intervals fit.
	adjacent, err := s.ReserveCapacity(ctx, a, resource.ID, scheduling.Request{Start: request.Start.Add(time.Hour), Duration: time.Hour, Units: 1})
	if err != nil {
		t.Fatal("adjacent repeated-clock slot rejected", err)
	}
	if _, err = s.RescheduleCapacity(ctx, a, resource.ID, winner.ID, winner.Version, adjacent.Request); err != ErrReservationConflict {
		t.Fatal("conflicting reschedule accepted", err)
	}
	var oldStart time.Time
	var oldVersion int
	if err = s.pool.QueryRow(ctx, `SELECT occupied_start,version FROM capacity_reservations WHERE id=$1`, winner.ID).Scan(&oldStart, &oldVersion); err != nil || !oldStart.Equal(winner.Occupied.Start) || oldVersion != 1 {
		t.Fatal("failed move lost original interval", err)
	}
	moved, err := s.RescheduleCapacity(ctx, a, resource.ID, winner.ID, 1, scheduling.Request{Start: start.Add(5 * time.Hour), Duration: time.Hour, Units: 1})
	if err != nil || moved.Version != 2 {
		t.Fatal("valid move failed", err)
	}
	if _, err = s.RescheduleCapacity(ctx, a, resource.ID, winner.ID, 1, request); err != ErrReservationConflict {
		t.Fatal("stale move accepted", err)
	}
	if _, err = s.ReserveCapacity(ctx, a, resource.ID, request); err != nil {
		t.Fatal("move did not release old capacity", err)
	}
	if _, err = s.RescheduleCapacity(ctx, b, resource.ID, winner.ID, 2, request); err != ErrReservationConflict {
		t.Fatal("cross-scope reschedule accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_reservation_audit_failure CHECK(action<>'scheduling.reserved') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	rollbackRequest := scheduling.Request{Start: start.Add(8 * time.Hour), Duration: time.Hour, Units: 1}
	if _, err = s.ReserveCapacity(ctx, a, resource.ID, rollbackRequest); err == nil {
		t.Fatal("reservation committed without audit")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_reservation_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReserveCapacity(ctx, a, resource.ID, rollbackRequest); err != nil {
		t.Fatal("failed audit leaked occupied capacity", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_reschedule_audit_failure CHECK(action<>'scheduling.rescheduled') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RescheduleCapacity(ctx, a, resource.ID, winner.ID, 2, scheduling.Request{Start: start.Add(10 * time.Hour), Duration: time.Hour, Units: 1}); err == nil {
		t.Fatal("reschedule committed without audit")
	}
	if err = s.pool.QueryRow(ctx, `SELECT occupied_start,version FROM capacity_reservations WHERE id=$1`, winner.ID).Scan(&oldStart, &oldVersion); err != nil || !oldStart.Equal(moved.Occupied.Start) || oldVersion != 2 {
		t.Fatal("reschedule audit failure lost old slot", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_reschedule_audit_failure`); err != nil {
		t.Fatal(err)
	}
	precise := request
	precise.Start = precise.Start.Add(time.Nanosecond)
	if _, err = s.ReserveCapacity(ctx, a, resource.ID, precise); err != ErrReservationInput {
		t.Fatal("sub-microsecond instant silently rounded", err)
	}
	if _, err = s.ReleaseCapacity(ctx, a, resource.ID, winner.ID, 2); err != ErrForbidden {
		t.Fatal("reserve implicitly granted release", err)
	}
	for _, scope := range []Scope{a, b} {
		if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'scheduling.release')`, scope.ActorID, scope.OrganizationID, scope.LocationID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.ReleaseCapacity(ctx, b, resource.ID, winner.ID, 2); err != ErrReservationConflict {
		t.Fatal("foreign release accepted", err)
	}
	if _, err = s.ReleaseCapacity(ctx, a, resource.ID, winner.ID, 1); err != ErrReservationConflict {
		t.Fatal("stale release accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_release_audit_failure CHECK(action<>'scheduling.released') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReleaseCapacity(ctx, a, resource.ID, winner.ID, 2); err == nil {
		t.Fatal("release committed without audit")
	}
	if _, err = s.ReserveCapacity(ctx, a, resource.ID, moved.Request); err != ErrReservationConflict {
		t.Fatal("audit failure prematurely freed capacity", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_release_audit_failure`); err != nil {
		t.Fatal(err)
	}
	releaseResults := make(chan result, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.ReleaseCapacity(ctx, a, resource.ID, winner.ID, 2)
			releaseResults <- result{r, e}
		}()
	}
	wg.Wait()
	close(releaseResults)
	success, conflict = 0, 0
	for released := range releaseResults {
		if released.err == nil {
			success++
			if released.reservation.CancelledAt == nil || released.reservation.CancelledBy == nil || *released.reservation.CancelledBy != a.ActorID || released.reservation.Version != 3 || !released.reservation.Occupied.Start.Equal(moved.Occupied.Start) {
				t.Fatal("release receipt incomplete")
			}
		} else if released.err == ErrReservationConflict {
			conflict++
		} else {
			t.Fatal(released.err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("concurrent release not exclusive", success, conflict)
	}
	if _, err = s.RescheduleCapacity(ctx, a, resource.ID, winner.ID, 3, request); err != ErrReservationConflict {
		t.Fatal("cancelled reservation revived", err)
	}
	if _, err = s.ReleaseCapacity(ctx, a, resource.ID, winner.ID, 3); err != ErrReservationConflict {
		t.Fatal("duplicate cancellation accepted", err)
	}
	if _, err = s.ReserveCapacity(ctx, a, resource.ID, moved.Request); err != nil {
		t.Fatal("released capacity not reusable", err)
	}
	var releaseAudits int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='scheduling.released' AND entity_id=$1`, winner.ID).Scan(&releaseAudits); err != nil || releaseAudits != 1 {
		t.Fatal("duplicate/missing release audit", err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.release'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReleaseCapacity(ctx, a, resource.ID, adjacent.ID, 1); err != ErrForbidden {
		t.Fatal("revoked release grant accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.reserve'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReserveCapacity(ctx, a, resource.ID, request); err != ErrForbidden {
		t.Fatal("revoked reservation grant accepted", err)
	}
}
