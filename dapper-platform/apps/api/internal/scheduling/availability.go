// Package scheduling calculates availability from explicitly supplied rules.
// It does not approve business policy or reserve capacity. A caller must load
// authorized location/resource data and serialize reservations in a database
// transaction before using a positive decision to create a booking.
package scheduling

import (
	"errors"
	"sort"
	"time"
)

var ErrInput = errors.New("explicit valid scheduling inputs required")

// Intervals are half-open: [Start,End). All comparisons use actual instants,
// allowing adjacent appointments and distinguishing repeated DST wall times.
type Interval struct{ Start, End time.Time }
type Allocation struct {
	Interval
	Units int
}
type Plan struct {
	Capacity int
	Open     []Interval
	Closed   []Interval
	// Existing allocations must already include their own approved buffers.
	Existing []Allocation
}
type Request struct {
	Start                   time.Time
	Duration, Before, After time.Duration
	Units                   int
	// WorkingTime means Duration counts only opening hours: the job continues across closed hours (nights, closure days)
	// until that much working time has passed. The resource stays occupied from the start to the last working minute.
	WorkingTime bool
}
type Decision struct {
	Available bool
	Reason    string
	Occupied  Interval
}

func valid(i Interval) bool      { return !i.Start.IsZero() && !i.End.IsZero() && i.Start.Before(i.End) }
func overlap(a, b Interval) bool { return a.Start.Before(b.End) && b.Start.Before(a.End) }

// ValidatePlan rejects incomplete/invalid rules. Input limits are computational
// safeguards, not approved business values. Empty open hours remain closed.
func ValidatePlan(plan Plan) error {
	if plan.Capacity < 1 || plan.Capacity > 1000000 || len(plan.Open) > 10000 || len(plan.Closed) > 10000 || len(plan.Existing) > 10000 {
		return ErrInput
	}
	for _, intervals := range [][]Interval{plan.Open, plan.Closed} {
		for _, i := range intervals {
			if !valid(i) {
				return ErrInput
			}
		}
	}
	for _, a := range plan.Existing {
		if !valid(a.Interval) || a.Units < 1 || a.Units > 1000000 {
			return ErrInput
		}
	}
	return nil
}

// Evaluate checks opening coverage, closures and peak simultaneous capacity.
func Evaluate(plan Plan, request Request) (Decision, error) {
	if err := ValidatePlan(plan); err != nil {
		return Decision{}, err
	}
	if request.Units < 1 || request.Units > 1000000 || request.Start.IsZero() || request.Duration <= 0 || request.Before < 0 || request.After < 0 {
		return Decision{}, ErrInput
	}
	occupied := Interval{request.Start.Add(-request.Before), request.Start.Add(request.Duration).Add(request.After)}
	if !valid(occupied) {
		return Decision{}, ErrInput
	}
	if request.WorkingTime {
		span, ok := workingSpan(plan, request)
		decision := Decision{Occupied: occupied}
		if !ok {
			decision.Reason = "outside_open_hours"
			return decision, nil
		}
		return evaluateCapacity(plan, request, span)
	}
	decision := Decision{Occupied: occupied}
	// Merge touching/overlapping opening windows; a buffer may span adjacent
	// shifts, but never a genuine closed gap. Do not mutate the caller's plan.
	open := append([]Interval(nil), plan.Open...)
	sort.Slice(open, func(i, j int) bool { return open[i].Start.Before(open[j].Start) })
	covered := false
	for i := 0; i < len(open); {
		window := open[i]
		i++
		for i < len(open) && !open[i].Start.After(window.End) {
			if open[i].End.After(window.End) {
				window.End = open[i].End
			}
			i++
		}
		if !occupied.Start.Before(window.Start) && !occupied.End.After(window.End) {
			covered = true
			break
		}
	}
	if !covered {
		decision.Reason = "outside_open_hours"
		return decision, nil
	}
	for _, closed := range plan.Closed {
		if overlap(occupied, closed) {
			decision.Reason = "blocked_interval"
			return decision, nil
		}
	}
	return evaluateCapacity(plan, request, occupied)
}

// workingSpan walks the opening windows (closures removed) from the start, spending request.Duration of working time, and returns
// the occupied interval including buffers. It fails if the start is not inside open time or the job does not finish inside known windows.
func workingSpan(plan Plan, request Request) (Interval, bool) {
	open := append([]Interval(nil), plan.Open...)
	sort.Slice(open, func(i, j int) bool { return open[i].Start.Before(open[j].Start) })
	merged := []Interval{}
	for _, w := range open {
		if len(merged) > 0 && !w.Start.After(merged[len(merged)-1].End) {
			if w.End.After(merged[len(merged)-1].End) {
				merged[len(merged)-1].End = w.End
			}
		} else {
			merged = append(merged, w)
		}
	}
	// remove closures from the windows
	for _, c := range plan.Closed {
		next := []Interval{}
		for _, w := range merged {
			if !overlap(w, c) {
				next = append(next, w)
				continue
			}
			if w.Start.Before(c.Start) {
				next = append(next, Interval{w.Start, c.Start})
			}
			if w.End.After(c.End) {
				next = append(next, Interval{c.End, w.End})
			}
		}
		merged = next
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Start.Before(merged[j].Start) })
	first := -1
	for i, w := range merged {
		if !request.Start.Before(w.Start) && request.Start.Before(w.End) {
			first = i
			break
		}
	}
	if first < 0 || request.Start.Add(-request.Before).Before(merged[first].Start) {
		return Interval{}, false
	}
	remaining := request.Duration
	cursor := request.Start
	for i := first; i < len(merged); i++ {
		if i > first {
			cursor = merged[i].Start
		}
		room := merged[i].End.Sub(cursor)
		if remaining <= room {
			end := cursor.Add(remaining)
			if end.Add(request.After).After(merged[i].End) {
				return Interval{}, false
			}
			return Interval{request.Start.Add(-request.Before), end.Add(request.After)}, true
		}
		remaining -= room
	}
	return Interval{}, false
}

func evaluateCapacity(plan Plan, request Request, occupied Interval) (Decision, error) {
	decision := Decision{Occupied: occupied}
	if request.Units > plan.Capacity {
		decision.Reason = "capacity"
		return decision, nil
	}
	type event struct {
		at    time.Time
		delta int64
	}
	events := []event{}
	for _, a := range plan.Existing {
		if !overlap(occupied, a.Interval) {
			continue
		}
		start, end := a.Start, a.End
		if start.Before(occupied.Start) {
			start = occupied.Start
		}
		if end.After(occupied.End) {
			end = occupied.End
		}
		events = append(events, event{start, int64(a.Units)}, event{end, -int64(a.Units)})
	}
	// End events precede start events at the same instant (half-open boundary).
	sort.Slice(events, func(i, j int) bool {
		if events[i].at.Equal(events[j].at) {
			return events[i].delta < events[j].delta
		}
		return events[i].at.Before(events[j].at)
	})
	used := int64(request.Units)
	for _, event := range events {
		used += event.delta
		if used > int64(plan.Capacity) {
			decision.Reason = "capacity"
			return decision, nil
		}
	}
	decision.Available = true
	decision.Reason = "available"
	return decision, nil
}
