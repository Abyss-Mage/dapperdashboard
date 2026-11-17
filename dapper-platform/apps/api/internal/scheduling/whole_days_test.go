package scheduling

import (
	"testing"
	"time"
)

// error404 #142: a job set in days books whole opening days, skipping closed days, and starts only at opening.
func TestWholeOpeningDays(t *testing.T) {
	zone, _ := time.LoadLocation("America/Toronto")
	day := func(d int, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, zone) }
	plan := Plan{Capacity: 1}
	for _, d := range []int{19, 20, 23, 24} { // Thu, Fri, Mon, Tue (closed on the weekend)
		plan.Open = append(plan.Open, Interval{day(d, 8), day(d, 12)}, Interval{day(d, 13), day(d, 18)}) // a lunch break stays in the day
	}
	two := Request{Start: day(20, 8), Duration: 48 * time.Hour, Units: 1}
	got, err := Evaluate(plan, two)
	if err != nil || !got.Available || !got.Occupied.End.Equal(day(23, 18)) {
		t.Fatalf("Friday start should run to Monday closing: %+v %v", got, err)
	}
	if late, _ := Evaluate(plan, Request{Start: day(20, 9), Duration: 24 * time.Hour, Units: 1}); late.Available {
		t.Fatal("a whole-day job was offered after opening")
	}
	if over, _ := Evaluate(plan, Request{Start: day(24, 8), Duration: 48 * time.Hour, Units: 1}); over.Available {
		t.Fatal("a job running past the known opening days was accepted")
	}
	plan.Existing = []Allocation{{Interval: got.Occupied, Units: 1}}
	if busy, _ := Evaluate(plan, Request{Start: day(23, 8), Duration: 24 * time.Hour, Units: 1}); busy.Available {
		t.Fatal("Monday was offered while the two-day job holds it")
	}
	if WholeDays(Request{Duration: 90 * time.Minute}) != 0 || WholeDays(Request{Duration: 24 * time.Hour, WorkingTime: true}) != 0 {
		t.Fatal("ordinary or working-time jobs were treated as whole days")
	}
}
