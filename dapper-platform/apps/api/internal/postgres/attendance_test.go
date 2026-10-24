package postgres

import (
	"context"
	"testing"
	"time"
)

func verifyAttendance(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()

	// Grant attendance.record and attendance.read to a.ActorID
	if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES
($1,$2,$3,'attendance.record'),
($1,$2,$3,'attendance.read') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}

	// 1. Initial active attendance should be nil
	active, err := s.ActiveAttendance(ctx, a)
	if err != nil {
		t.Fatalf("expected nil error for initial active attendance, got: %v", err)
	}
	if active != nil {
		t.Fatalf("expected nil initial active attendance, got: %+v", active)
	}

	// 2. Clock in
	shift, err := s.ClockIn(ctx, a, "Morning shift start")
	if err != nil {
		t.Fatalf("failed to clock in: %v", err)
	}
	if shift.ID == "" || shift.Status != "clocked_in" || shift.ClockOutAt != nil || shift.CurrentBreakStart != nil || shift.Notes != "Morning shift start" {
		t.Fatalf("invalid clocked_in record: %+v", shift)
	}

	// 3. Active attendance returns the active shift
	active, err = s.ActiveAttendance(ctx, a)
	if err != nil || active == nil || active.ID != shift.ID || active.Status != "clocked_in" {
		t.Fatalf("expected active shift %s, got: %+v, err: %v", shift.ID, active, err)
	}

	// 4. Double clock-in must be rejected
	if _, err = s.ClockIn(ctx, a, "Second clock in"); err != ErrAttendanceActiveShift {
		t.Fatalf("expected ErrAttendanceActiveShift, got: %v", err)
	}

	// 5. Start break
	onBreak, err := s.StartBreak(ctx, a, "Midday lunch break")
	if err != nil {
		t.Fatalf("failed to start break: %v", err)
	}
	if onBreak.Status != "on_break" || onBreak.CurrentBreakStart == nil {
		t.Fatalf("expected status on_break with current_break_start set, got: %+v", onBreak)
	}

	// 6. Starting break again while already on break must fail
	if _, err = s.StartBreak(ctx, a, "Break again"); err != ErrAttendanceInvalidTransition {
		t.Fatalf("expected ErrAttendanceInvalidTransition, got: %v", err)
	}

	// 7. End break
	backToWork, err := s.EndBreak(ctx, a, "Return to service bay")
	if err != nil {
		t.Fatalf("failed to end break: %v", err)
	}
	if backToWork.Status != "clocked_in" || backToWork.CurrentBreakStart != nil {
		t.Fatalf("expected clocked_in without current break, got: %+v", backToWork)
	}

	// 8. Ending break again while not on break must fail
	if _, err = s.EndBreak(ctx, a, "End break again"); err != ErrAttendanceInvalidTransition {
		t.Fatalf("expected ErrAttendanceInvalidTransition, got: %v", err)
	}

	// 9. Clock out
	clockedOut, err := s.ClockOut(ctx, a, "Shift complete")
	if err != nil {
		t.Fatalf("failed to clock out: %v", err)
	}
	if clockedOut.Status != "clocked_out" || clockedOut.ClockOutAt == nil || clockedOut.CurrentBreakStart != nil {
		t.Fatalf("expected clocked_out record with clockOutAt, got: %+v", clockedOut)
	}
	if clockedOut.Notes != "Morning shift start | Shift complete" {
		t.Fatalf("expected combined notes, got: %q", clockedOut.Notes)
	}

	// 10. Active attendance is now nil
	active, err = s.ActiveAttendance(ctx, a)
	if err != nil || active != nil {
		t.Fatalf("expected nil active attendance after clock out, got: %+v, err: %v", active, err)
	}

	// 11. Clock out when no active shift must fail
	if _, err = s.ClockOut(ctx, a, "Clock out again"); err != ErrAttendanceNoActiveShift {
		t.Fatalf("expected ErrAttendanceNoActiveShift, got: %v", err)
	}

	// 12. History list
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(24 * time.Hour)
	list, err := s.ListAttendance(ctx, a, from, to, 10)
	if err != nil {
		t.Fatalf("failed to list attendance: %v", err)
	}
	if len(list) != 1 || list[0].ID != clockedOut.ID {
		t.Fatalf("expected 1 attendance record matching %s, got: %+v", clockedOut.ID, list)
	}

	// 13. Audit events
	events, err := s.AttendanceEvents(ctx, a, clockedOut.ID)
	if err != nil {
		t.Fatalf("failed to get attendance events: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("expected 4 punch events (clock_in, start_break, end_break, clock_out), got %d: %+v", len(events), events)
	}
	expectedTypes := []string{"clock_in", "start_break", "end_break", "clock_out"}
	for i, exp := range expectedTypes {
		if events[i].EventType != exp {
			t.Errorf("event %d: expected %s, got %s", i, exp, events[i].EventType)
		}
		if events[i].ActorID != a.ActorID {
			t.Errorf("event %d: expected actor %s, got %s", i, a.ActorID, events[i].ActorID)
		}
	}

	// 14. Cross-scope isolation: b cannot view a's attendance events
	bEvents, err := s.AttendanceEvents(ctx, b, clockedOut.ID)
	if err == nil && len(bEvents) > 0 {
		t.Fatalf("cross-scope data leak: b saw events from a: %+v", bEvents)
	}

	// 15. Revocation denial: delete attendance.record and verify ClockIn fails
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='attendance.record'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClockIn(ctx, a, "Unpermitted clock in"); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden after grant deletion, got: %v", err)
	}
}
