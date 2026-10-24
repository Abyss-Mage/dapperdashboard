package scheduling

import (
	"sort"
	"time"
)

// AvailableStarts uses the same Evaluate authority as reservation commits.
// Times are actual instants, including both occurrences of repeated DST hours.
// NextFrom resumes after the last evaluated candidate, even on an empty page.
type StartsPage struct {
	Starts   []time.Time
	NextFrom *time.Time
}

func AvailableStarts(plan Plan, request Request, from, to time.Time, step time.Duration, limit, scanLimit int) (StartsPage, error) {
	result := StartsPage{Starts: []time.Time{}}
	if ValidatePlan(plan) != nil || from.IsZero() || !from.Before(to) || to.Sub(from) > 26*time.Hour || step < time.Minute || step > 24*time.Hour || step%time.Minute != 0 || request.Duration <= 0 || request.Before < 0 || request.After < 0 || request.Units < 1 || request.Units > 1000000 || limit < 1 || limit > 100 || scanLimit < limit || scanLimit > 2000 {
		return result, ErrInput
	}
	open := append([]Interval(nil), plan.Open...)
	sort.Slice(open, func(i, j int) bool { return open[i].Start.Before(open[j].Start) })
	merged := []Interval{}
	for _, interval := range open {
		if len(merged) > 0 && !interval.Start.After(merged[len(merged)-1].End) {
			if interval.End.After(merged[len(merged)-1].End) {
				merged[len(merged)-1].End = interval.End
			}
		} else {
			merged = append(merged, interval)
		}
	}
	plan.Open = merged
	scanned := 0
	for _, window := range merged {
		if !window.End.After(from) || !window.Start.Before(to) {
			continue
		}
		anchor := window.Start.Add(request.Before)
		candidate := anchor
		if candidate.Before(from) {
			seconds := from.Unix() - anchor.Unix()
			nanos := from.Nanosecond() - anchor.Nanosecond()
			if nanos < 0 {
				seconds--
				nanos += int(time.Second)
			}
			remainder := seconds % int64(step/time.Second)
			candidate = from.Add(-time.Duration(remainder)*time.Second - time.Duration(nanos))
			if candidate.Before(from) {
				candidate = candidate.Add(step)
			}
		}
		for candidate.Before(window.End) && candidate.Before(to) {
			if scanned >= scanLimit || len(result.Starts) >= limit {
				next := candidate
				result.NextFrom = &next
				return result, nil
			}
			scanned++
			request.Start = candidate
			decision, err := Evaluate(plan, request)
			if err != nil {
				return StartsPage{}, err
			}
			if decision.Available {
				result.Starts = append(result.Starts, candidate)
			}
			candidate = candidate.Add(step)
		}
	}
	return result, nil
}
