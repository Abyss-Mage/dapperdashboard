package postgres

import (
	"context"
	"errors"
	"time"
)

var ErrAttendanceTotalsInput = errors.New("valid date range (at most 400 days) and time zone required")

// StaffWorkDay is one staff member's worked time on one local day: shift length minus breaks.
// A shift counts on the day it was clocked in (franchise time zone); a shift still open counts up to now.
type StaffWorkDay struct {
	Date    string `json:"date"`
	Minutes int    `json:"minutes"`
	Shifts  int    `json:"shifts"`
	// ClockIn is the first clock-in of the day and ClockOut the last clock-out (empty while a shift is still open).
	ClockIn  *time.Time `json:"clockIn"`
	ClockOut *time.Time `json:"clockOut"`
	Open     bool       `json:"open"`
}
type StaffWorkTotal struct {
	PrincipalID  string         `json:"principalId"`
	Name         string         `json:"name"`
	Email        string         `json:"email"`
	TotalMinutes int            `json:"totalMinutes"`
	ClockedIn    bool           `json:"clockedIn"`
	Days         []StaffWorkDay `json:"days"`
}

// AttendanceTotals adds up worked minutes per staff member and local day between from and to (inclusive dates).
func (s *Store) AttendanceTotals(ctx context.Context, scope Scope, from, to, zone string) ([]StaffWorkTotal, error) {
	out := []StaffWorkTotal{}
	start, e1 := time.Parse("2006-01-02", from)
	end, e2 := time.Parse("2006-01-02", to)
	if _, e3 := time.LoadLocation(zone); e1 != nil || e2 != nil || e3 != nil || zone == "" || end.Before(start) || end.Sub(start) > 400*24*time.Hour {
		return out, ErrAttendanceTotalsInput
	}
	if err := s.authorize(ctx, scope, "attendance.read"); err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `WITH shifts AS (
 SELECT a.principal_id, (a.clock_in_at AT TIME ZONE $5)::date AS day, a.clock_in_at, a.clock_out_at, a.clock_out_at IS NULL AS open,
  GREATEST(0, floor(extract(epoch FROM (COALESCE(a.clock_out_at, now()) - a.clock_in_at)) / 60)::int - a.total_break_minutes
   - CASE WHEN a.clock_out_at IS NULL AND a.current_break_start IS NOT NULL THEN floor(extract(epoch FROM (now() - a.current_break_start)) / 60)::int ELSE 0 END) AS minutes
 FROM employee_attendance a
 WHERE a.organization_id=$1 AND a.location_id=$2 AND (a.clock_in_at AT TIME ZONE $5)::date BETWEEN $3::date AND $4::date)
SELECT sh.principal_id::text, COALESCE(NULLIF(mp.full_name,''), NULLIF(pc.email,''), p.external_subject), COALESCE(pc.email,''), sh.day::text, sum(sh.minutes)::int, count(*)::int, min(sh.clock_in_at), CASE WHEN bool_or(sh.open) THEN NULL ELSE max(sh.clock_out_at) END, bool_or(sh.open)
FROM shifts sh JOIN principals p ON p.id=sh.principal_id
LEFT JOIN member_profiles mp ON mp.principal_id=p.id LEFT JOIN password_credentials pc ON pc.principal_id=p.id
GROUP BY 1,2,3,4 ORDER BY 2,4`, scope.OrganizationID, scope.LocationID, from, to, zone)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	index := map[string]int{}
	for rows.Next() {
		var id, name, email string
		var d StaffWorkDay
		var open bool
		if err = rows.Scan(&id, &name, &email, &d.Date, &d.Minutes, &d.Shifts, &d.ClockIn, &d.ClockOut, &open); err != nil {
			return out, err
		}
		i, ok := index[id]
		if !ok {
			i = len(out)
			index[id] = i
			out = append(out, StaffWorkTotal{PrincipalID: id, Name: name, Email: email, Days: []StaffWorkDay{}})
		}
		d.Open = open
		out[i].Days = append(out[i].Days, d)
		out[i].TotalMinutes += d.Minutes
		out[i].ClockedIn = out[i].ClockedIn || open
	}
	return out, rows.Err()
}
