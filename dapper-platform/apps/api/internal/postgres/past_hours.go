package postgres

import (
	"sort"
	"time"

	"dapper.local/platform/internal/scheduling"
)

// addPastWindows adds the opening windows the weekly hours imply for days that are already over. The stored opening windows only
// remember the last few days, so an administrator entering a booking that happened last month would otherwise find the shop "closed".
// A window that overlaps one already stored is left out.
func addPastWindows(plan *scheduling.Plan, tz string, weekly []HourWindow, from, to time.Time) {
	loc, err := time.LoadLocation(tz)
	if err != nil || !from.Before(to) {
		return
	}
	days := int(to.Sub(from).Hours()/24) + 2
	if days > 400 {
		days = 400
	}
	for _, w := range expandWeekly(loc, weekly, from, days) {
		if !w.Start.Before(to) {
			continue
		}
		overlaps := false
		for _, o := range plan.Open {
			if o.Start.Before(w.End) && o.End.After(w.Start) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			plan.Open = append(plan.Open, w)
		}
	}
	sort.Slice(plan.Open, func(i, j int) bool { return plan.Open[i].Start.Before(plan.Open[j].Start) })
}

func insideOpenWindow(plan scheduling.Plan, at time.Time) bool {
	for _, o := range plan.Open {
		if !at.Before(o.Start) && at.Before(o.End) {
			return true
		}
	}
	return false
}
