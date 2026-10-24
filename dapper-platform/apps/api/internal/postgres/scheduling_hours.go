package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// HourWindow is one opening window on a weekday. Day 0 is Monday. Start/End are "HH:MM" wall-clock times in the
// resource timezone; End "24:00" means midnight at the end of that day.
type HourWindow struct {
	Day   int    `json:"day"`
	Start string `json:"start"`
	End   string `json:"end"`
}

const (
	hoursHorizonDays = 84  // 12 weeks of generated opening windows
	keepPastDays     = 400 // older generated windows are pruned
)

func clockMinutes(s string) (int, bool) {
	if s == "24:00" {
		return 1440, true
	}
	t, err := time.Parse("15:04", s)
	if err != nil || len(s) != 5 {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// ValidateWeeklyHours rejects impossible or overlapping windows. The limits are computational safeguards, not business policy.
func ValidateWeeklyHours(w []HourWindow) error {
	if len(w) > 28 {
		return ErrReservationInput
	}
	byDay := map[int][][2]int{}
	for _, x := range w {
		a, okA := clockMinutes(x.Start)
		b, okB := clockMinutes(x.End)
		if x.Day < 0 || x.Day > 6 || !okA || !okB || a >= b || a >= 1440 {
			return ErrReservationInput
		}
		byDay[x.Day] = append(byDay[x.Day], [2]int{a, b})
	}
	for _, spans := range byDay {
		if len(spans) > 4 {
			return ErrReservationInput
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
		for i := 1; i < len(spans); i++ {
			if spans[i][0] < spans[i-1][1] {
				return ErrReservationInput
			}
		}
	}
	return nil
}

// expandWeekly turns the template into dated windows for calendar days [today, today+days) in loc, keeping only windows that
// have not ended by now. Wall times are resolved per date, so daylight-saving changes keep the local opening time.
func expandWeekly(loc *time.Location, weekly []HourWindow, now time.Time, days int) []scheduling.Interval {
	out := []scheduling.Interval{}
	local := now.In(loc)
	for i := 0; i < days; i++ {
		d := time.Date(local.Year(), local.Month(), local.Day()+i, 0, 0, 0, 0, loc)
		day := (int(d.Weekday()) + 6) % 7
		for _, w := range weekly {
			if w.Day != day {
				continue
			}
			a, _ := clockMinutes(w.Start)
			b, _ := clockMinutes(w.End)
			start := time.Date(d.Year(), d.Month(), d.Day(), a/60, a%60, 0, 0, loc)
			end := time.Date(d.Year(), d.Month(), d.Day(), b/60, b%60, 0, 0, loc)
			if b == 1440 {
				end = time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, loc)
			}
			if end.After(now) && start.Before(end) {
				out = append(out, scheduling.Interval{Start: start.UTC(), End: end.UTC()})
			}
		}
	}
	return out
}

func keepRecent(in []scheduling.Interval, now time.Time) []scheduling.Interval {
	cut := now.AddDate(0, 0, -keepPastDays)
	out := []scheduling.Interval{}
	for _, i := range in {
		if i.End.After(cut) {
			out = append(out, i)
		}
	}
	return out
}

// applyWeekly replaces every opening window that has not ended with windows generated from the template; ended windows are kept
// as history. Closures (plan.Closed) are untouched.
func applyWeekly(plan *scheduling.Plan, tz string, weekly []HourWindow, now time.Time) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return ErrReservationInput
	}
	kept := []scheduling.Interval{}
	for _, i := range keepRecent(plan.Open, now) {
		if !i.End.After(now) {
			kept = append(kept, i)
		}
	}
	plan.Open = append(kept, expandWeekly(loc, weekly, now, hoursHorizonDays)...)
	plan.Closed = keepRecent(plan.Closed, now)
	return scheduling.ValidatePlan(*plan)
}

func (s *Store) lockResource(ctx context.Context, tx pgx.Tx, scope Scope, resourceID string) (tz string, plan scheduling.Plan, weekly []HourWindow, err error) {
	err = tx.QueryRow(ctx, `SELECT timezone,rules,weekly_hours FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, resourceID).Scan(&tz, &plan, &weekly)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrReservationConflict
	}
	return
}

// SaveWeeklyHours sets the weekly template and regenerates the dated opening windows. An empty template closes the resource.
// Existing reservations are never cancelled or moved by this change.
func (s *Store) SaveWeeklyHours(ctx context.Context, scope Scope, resourceID string, weekly []HourWindow) error {
	if err := ValidateWeeklyHours(weekly); err != nil {
		return err
	}
	if weekly == nil {
		weekly = []HourWindow{}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.manage"); err != nil {
		return err
	}
	tz, plan, _, err := s.lockResource(ctx, tx, scope, resourceID)
	if err != nil {
		return err
	}
	if err = applyWeekly(&plan, tz, weekly, time.Now()); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE scheduling_resources SET rules=$4,weekly_hours=$5 WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, resourceID, plan, weekly); err != nil {
		return err
	}
	if err = jobAudit(ctx, tx, scope, "scheduling.hours.saved", resourceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AddClosure closes the resource for one whole calendar day (resource timezone), e.g. a holiday.
func (s *Store) AddClosure(ctx context.Context, scope Scope, resourceID, date string) error {
	day, err := time.Parse("2006-01-02", date)
	if err != nil || day.Year() < 2000 || day.After(time.Now().AddDate(2, 0, 0)) {
		return ErrReservationInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.manage"); err != nil {
		return err
	}
	tz, plan, _, err := s.lockResource(ctx, tx, scope, resourceID)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return ErrReservationInput
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, loc)
	for _, c := range plan.Closed {
		if c.Start.Equal(start) && c.End.Equal(end) {
			return ErrReservationConflict
		}
	}
	plan.Closed = append(keepRecent(plan.Closed, time.Now()), scheduling.Interval{Start: start.UTC(), End: end.UTC()})
	if err = scheduling.ValidatePlan(plan); err != nil {
		return ErrReservationInput
	}
	if _, err = tx.Exec(ctx, `UPDATE scheduling_resources SET rules=$4 WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, resourceID, plan); err != nil {
		return err
	}
	if err = jobAudit(ctx, tx, scope, "scheduling.closure.added", resourceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemoveClosure removes a closure by its exact start instant.
func (s *Store) RemoveClosure(ctx context.Context, scope Scope, resourceID string, start time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.manage"); err != nil {
		return err
	}
	_, plan, _, err := s.lockResource(ctx, tx, scope, resourceID)
	if err != nil {
		return err
	}
	kept := []scheduling.Interval{}
	found := false
	for _, c := range plan.Closed {
		if c.Start.Equal(start) {
			found = true
			continue
		}
		kept = append(kept, c)
	}
	if !found {
		return ErrReservationConflict
	}
	plan.Closed = kept
	if _, err = tx.Exec(ctx, `UPDATE scheduling_resources SET rules=$4 WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, resourceID, plan); err != nil {
		return err
	}
	if err = jobAudit(ctx, tx, scope, "scheduling.closure.removed", resourceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ExtendWeeklyHours is the system roll-forward: every resource that has a weekly template gets its generated windows topped
// up to the 12-week horizon. It changes only opening windows derived from the template. Returns how many resources changed.
func (s *Store) ExtendWeeklyHours(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM scheduling_resources WHERE weekly_hours IS NOT NULL ORDER BY id`)
	if err != nil {
		return 0, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	changed := 0
	for _, id := range ids {
		if err = s.extendOne(ctx, id); err != nil {
			return changed, fmt.Errorf("extend resource: %w", err)
		}
		changed++
	}
	return changed, nil
}

func (s *Store) extendOne(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var tz string
	var plan scheduling.Plan
	var weekly []HourWindow
	if err = tx.QueryRow(ctx, `SELECT timezone,rules,weekly_hours FROM scheduling_resources WHERE id=$1 FOR UPDATE`, id).Scan(&tz, &plan, &weekly); err != nil {
		return err
	}
	if err = applyWeekly(&plan, tz, weekly, time.Now()); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE scheduling_resources SET rules=$2 WHERE id=$1`, id, plan); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
