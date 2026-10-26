package scheduling

import (
	"testing"
	"time"
)

func TestAvailableStartsPagingAndBoundaries(t *testing.T) {
	start := time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC)
	plan := Plan{Capacity: 1, Open: []Interval{{Start: start, End: start.Add(4 * time.Hour)}, {Start: start.Add(4 * time.Hour), End: start.Add(8 * time.Hour)}}}
	request := Request{Duration: time.Hour, Units: 1}
	var all []time.Time
	from := start
	to := start.Add(8 * time.Hour)
	for i := 0; i < 10; i++ {
		page, err := AvailableStarts(plan, request, from, to, time.Minute, 100, 1000)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, page.Starts...)
		if page.NextFrom == nil {
			break
		}
		if !page.NextFrom.After(from) {
			t.Fatal("cursor did not advance")
		}
		from = *page.NextFrom
	}
	if len(all) != 421 || !all[0].Equal(start) || !all[len(all)-1].Equal(start.Add(7*time.Hour)) {
		t.Fatal("paging lost starts across merged shifts", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i].Sub(all[i-1]) != time.Minute {
			t.Fatal("duplicate/gap in starts")
		}
	}
	plan.Existing = []Allocation{{Interval: Interval{Start: start.Add(time.Hour), End: start.Add(2 * time.Hour)}, Units: 1}}
	request.Before = 15 * time.Minute
	request.After = 15 * time.Minute
	page, err := AvailableStarts(plan, request, start, start.Add(4*time.Hour), 15*time.Minute, 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range page.Starts {
		request.Start = candidate
		decision, err := Evaluate(plan, request)
		if err != nil || !decision.Available {
			t.Fatal("returned invalid capacity/buffer start", err)
		}
	}
}
func TestAvailableStartsDSTAndLongWindowAnchor(t *testing.T) {
	zone, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)
	plan := Plan{Capacity: 1, Open: []Interval{{Start: start, End: start.Add(5 * time.Hour)}}}
	page, err := AvailableStarts(plan, Request{Duration: 30 * time.Minute, Units: 1}, start, start.Add(5*time.Hour), 30*time.Minute, 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	repeated := 0
	for _, at := range page.Starts {
		if at.In(zone).Format("15:04") == "01:30" {
			repeated++
		}
	}
	if repeated != 2 {
		t.Fatal("repeated DST time collapsed", repeated)
	}
	ancient := time.Date(1600, 1, 1, 0, 0, 0, 0, time.UTC)
	plan.Open = []Interval{{Start: ancient, End: time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC)}}
	page, err = AvailableStarts(plan, Request{Duration: time.Hour, Units: 1}, start, start.Add(time.Hour), 15*time.Minute, 100, 1000)
	if err != nil || len(page.Starts) != 4 || page.Starts[0].Before(start) {
		t.Fatal("long anchor overflow returned past start", err)
	}
}
