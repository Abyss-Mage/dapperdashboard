package scheduling

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func instant(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return t
}
func TestAvailabilityCapacityAndBoundaries(t *testing.T) {
	start := instant("2026-10-22T09:00:00Z")
	window := func(a, b time.Duration) Interval { return Interval{start.Add(a), start.Add(b)} }
	request := Request{Start: start.Add(time.Hour), Duration: time.Hour, Units: 1}
	for _, test := range []struct {
		name      string
		plan      Plan
		request   Request
		available bool
		reason    string
	}{
		{"empty capacity", Plan{Capacity: 1, Open: []Interval{window(0, 4*time.Hour)}}, request, true, "available"},
		{"adjacent appointment", Plan{Capacity: 1, Open: []Interval{window(0, 4*time.Hour)}, Existing: []Allocation{{window(0, time.Hour), 1}}}, request, true, "available"},
		{"overlapping appointment", Plan{Capacity: 1, Open: []Interval{window(0, 4*time.Hour)}, Existing: []Allocation{{window(90*time.Minute, 3*time.Hour), 1}}}, request, false, "capacity"},
		{"sequential overlaps share lane", Plan{Capacity: 2, Open: []Interval{window(0, 4*time.Hour)}, Existing: []Allocation{{window(0, 90*time.Minute), 1}, {window(90*time.Minute, 3*time.Hour), 1}}}, request, true, "available"},
		{"peak overlaps exceed lanes", Plan{Capacity: 2, Open: []Interval{window(0, 4*time.Hour)}, Existing: []Allocation{{window(0, 100*time.Minute), 1}, {window(90*time.Minute, 3*time.Hour), 1}}}, request, false, "capacity"},
		{"closed holiday", Plan{Capacity: 2, Open: []Interval{window(0, 4*time.Hour)}, Closed: []Interval{window(0, 24*time.Hour)}}, request, false, "blocked_interval"},
		{"buffer collides", Plan{Capacity: 1, Open: []Interval{window(0, 4*time.Hour)}, Existing: []Allocation{{window(0, time.Hour), 1}}}, Request{Start: request.Start, Duration: time.Hour, Before: time.Minute, Units: 1}, false, "capacity"},
		{"buffer outside hours", Plan{Capacity: 1, Open: []Interval{window(time.Hour, 4*time.Hour)}}, Request{Start: request.Start, Duration: time.Hour, Before: time.Minute, Units: 1}, false, "outside_open_hours"},
		{"adjacent shifts", Plan{Capacity: 1, Open: []Interval{window(0, 90*time.Minute), window(90*time.Minute, 4*time.Hour)}}, request, true, "available"},
		{"closed gap between shifts", Plan{Capacity: 1, Open: []Interval{window(0, 89*time.Minute), window(90*time.Minute, 4*time.Hour)}}, request, false, "outside_open_hours"},
		{"no configured hours", Plan{Capacity: 1}, request, false, "outside_open_hours"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Evaluate(test.plan, test.request)
			if err != nil || got.Available != test.available || got.Reason != test.reason {
				t.Fatalf("decision=%+v err=%v", got, err)
			}
		})
	}
}

func TestInvalidRulesFailClosed(t *testing.T) {
	start := instant("2026-10-22T09:00:00Z")
	plan := Plan{Capacity: 1, Open: []Interval{{start, start.Add(8 * time.Hour)}}}
	request := Request{Start: start, Duration: time.Hour, Units: 1}
	for _, change := range []func(*Plan, *Request){
		func(p *Plan, r *Request) { p.Capacity = 0 }, func(p *Plan, r *Request) { r.Duration = 0 }, func(p *Plan, r *Request) { r.Before = -time.Minute }, func(p *Plan, r *Request) { r.Units = 0 }, func(p *Plan, r *Request) { p.Closed = []Interval{{start, start}} }, func(p *Plan, r *Request) { p.Existing = []Allocation{{Interval{start, start.Add(time.Hour)}, 0}} },
	} {
		p, r := plan, request
		change(&p, &r)
		if result, err := Evaluate(p, r); err != ErrInput || result.Available {
			t.Fatal("invalid rules accepted", result, err)
		}
	}
}

func TestDSTUsesDistinctInstants(t *testing.T) {
	zone, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	// Toronto repeats 01:30 when daylight saving ends: same wall clock,
	// separate adjacent one-hour allocations, rather than a false conflict.
	first := instant("2026-11-01T01:30:00-04:00").In(zone)
	second := instant("2026-11-01T01:30:00-05:00").In(zone)
	if first.Format("15:04") != second.Format("15:04") || second.Sub(first) != time.Hour {
		t.Fatal("invalid DST fixture")
	}
	plan := Plan{Capacity: 1, Open: []Interval{{first.Add(-time.Hour), second.Add(2 * time.Hour)}}, Existing: []Allocation{{Interval{first, second}, 1}}}
	got, err := Evaluate(plan, Request{Start: second, Duration: time.Hour, Units: 1})
	if err != nil || !got.Available {
		t.Fatal("distinct fall-back instant collided", got, err)
	}
	// An elapsed hour across spring-forward ends at03:30, not nonexistent02:30.
	spring := instant("2026-03-08T01:30:00-05:00").In(zone)
	got, err = Evaluate(Plan{Capacity: 1, Open: []Interval{{spring, spring.Add(2 * time.Hour)}}}, Request{Start: spring, Duration: time.Hour, Units: 1})
	if err != nil || !got.Available || got.Occupied.End.In(zone).Format("15:04") != "03:30" {
		t.Fatal("spring elapsed duration incorrect", got, err)
	}
}
