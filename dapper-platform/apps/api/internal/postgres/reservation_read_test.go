package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"testing"
	"time"
)

func verifyReservationRead(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	from := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	if _, err := s.Reservations(ctx, a, from, to, "", 2); err != ErrForbidden {
		t.Fatal("manage/write grants implied read", err)
	}
	for _, scope := range []Scope{a, b} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'scheduling.read')`, scope.ActorID, scope.OrganizationID, scope.LocationID); err != nil {
			t.Fatal(err)
		}
	}
	resource, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic paged resource", Timezone: "UTC", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: from, End: to}}}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err = s.ReserveCapacity(ctx, a, resource.ID, scheduling.Request{Start: from.Add(time.Duration(i) * time.Hour), Duration: time.Hour, Units: 1}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		page, err := s.Reservations(ctx, a, from, to, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatal("duplicate across pages")
			}
			seen[item.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > 3 {
			t.Fatal("cursor did not advance")
		}
	}
	if len(seen) != 5 || pages != 3 {
		t.Fatal("pagination lost reservations", len(seen), pages)
	}
	if page, err := s.Reservations(ctx, b, from, to, "", 100); err != nil || len(page.Items) != 0 {
		t.Fatal("foreign read leaked", err)
	}
	if _, err = s.Reservations(ctx, a, from.Add(time.Hour), to, cursor, 2); err != ErrReservationInput {
		t.Fatal("cursor reused with changed range", err)
	}
	if _, err = s.Reservations(ctx, a, from, to, "invalid!", 2); err != ErrReservationInput {
		t.Fatal("invalid cursor accepted", err)
	}
	if page, err := s.Reservations(ctx, a, to, to.Add(time.Hour), "", 2); err != nil || len(page.Items) != 0 {
		t.Fatal("half-open range boundary incorrect", err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.read'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reservations(ctx, a, from, to, "", 2); err != ErrForbidden {
		t.Fatal("revoked read accepted", err)
	}
}
