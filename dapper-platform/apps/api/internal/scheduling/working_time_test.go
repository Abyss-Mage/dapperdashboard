package scheduling

import (
	"testing"
	"time"
)

func openDays(first time.Time, days int) []Interval {
	out := []Interval{}
	for i := 0; i < days; i++ {
		d := first.AddDate(0, 0, i)
		out = append(out, Interval{d.Add(8 * time.Hour), d.Add(21 * time.Hour)})
	}
	return out
}

func TestWorkingTimeJobContinuesAcrossClosedHours(t *testing.T) {
	mon := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	plan := Plan{Capacity: 1, Open: openDays(mon, 5)}
	start := mon.Add(8 * time.Hour)
	// 15 hours of work: 13 on Monday, 2 on Tuesday.
	got, err := Evaluate(plan, Request{Start: start, Duration: 15 * time.Hour, Units: 1, WorkingTime: true})
	if err != nil || !got.Available {
		t.Fatalf("a working-time job across two days must be available: %+v %v", got, err)
	}
	if want := mon.AddDate(0, 0, 1).Add(10 * time.Hour); !got.Occupied.End.Equal(want) || !got.Occupied.Start.Equal(start) {
		t.Fatalf("occupied span wrong: %v - %v, want end %v", got.Occupied.Start, got.Occupied.End, want)
	}
	// The same duration as continuous clock time can never fit an opening day.
	if got, _ = Evaluate(plan, Request{Start: start, Duration: 15 * time.Hour, Units: 1}); got.Available {
		t.Fatal("a continuous 15 hour job must not fit a 13 hour day")
	}
	// A start outside open hours is refused.
	if got, _ = Evaluate(plan, Request{Start: mon.Add(22 * time.Hour), Duration: time.Hour, Units: 1, WorkingTime: true}); got.Available || got.Reason != "outside_open_hours" {
		t.Fatalf("a start after closing was accepted: %+v", got)
	}
	// The job cannot finish beyond the known opening windows.
	short := Plan{Capacity: 1, Open: openDays(mon, 1)}
	if got, _ = Evaluate(short, Request{Start: start, Duration: 15 * time.Hour, Units: 1, WorkingTime: true}); got.Available {
		t.Fatal("a job that runs out of opening windows was accepted")
	}
}

func TestWorkingTimeSkipsClosureDaysAndHonoursCapacity(t *testing.T) {
	mon := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	tue := mon.AddDate(0, 0, 1)
	plan := Plan{Capacity: 1, Open: openDays(mon, 5), Closed: []Interval{{tue, tue.AddDate(0, 0, 1)}}}
	start := mon.Add(8 * time.Hour)
	got, err := Evaluate(plan, Request{Start: start, Duration: 15 * time.Hour, Units: 1, WorkingTime: true})
	if err != nil || !got.Available {
		t.Fatalf("closure day should be skipped, not block the job: %+v %v", got, err)
	}
	if want := mon.AddDate(0, 0, 2).Add(10 * time.Hour); !got.Occupied.End.Equal(want) {
		t.Fatalf("closure day was counted as working time: end %v want %v", got.Occupied.End, want)
	}
	// Starting on the closure day itself is refused.
	if got, _ = Evaluate(plan, Request{Start: tue.Add(9 * time.Hour), Duration: time.Hour, Units: 1, WorkingTime: true}); got.Available {
		t.Fatal("a start inside a closure was accepted")
	}
	// An existing one-at-a-time job overlapping the span blocks a second one.
	plan.Existing = []Allocation{{Interval: Interval{mon.AddDate(0, 0, 3).Add(8 * time.Hour), mon.AddDate(0, 0, 3).Add(12 * time.Hour)}, Units: 1}}
	if got, _ = Evaluate(plan, Request{Start: mon.AddDate(0, 0, 2).Add(8 * time.Hour), Duration: 15 * time.Hour, Units: 1, WorkingTime: true}); got.Available || got.Reason != "capacity" {
		t.Fatalf("overlapping capacity-1 job was accepted: %+v", got)
	}
}
