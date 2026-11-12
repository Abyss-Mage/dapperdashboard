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
// The local days are worked out here in Go: the database is only given instants, because its time zone list can lack names a browser
// sends (old aliases such as Asia/Calcutta made the page fail with 503, error404 #40/#47).
func (s *Store) AttendanceTotals(ctx context.Context, scope Scope, from, to, zone string) ([]StaffWorkTotal, error) {
	out := []StaffWorkTotal{}
	start, e1 := time.Parse("2006-01-02", from)
	end, e2 := time.Parse("2006-01-02", to)
	loc, e3 := time.LoadLocation(zone)
	if e1 != nil || e2 != nil || e3 != nil || zone == "" || end.Before(start) || end.Sub(start) > 400*24*time.Hour {
		return out, ErrAttendanceTotalsInput
	}
	if err := s.authorize(ctx, scope, "attendance.read"); err != nil {
		return out, err
	}
	first := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	after := time.Date(end.Year(), end.Month(), end.Day()+1, 0, 0, 0, 0, loc)
	rows, err := s.pool.Query(ctx, `SELECT a.principal_id::text, COALESCE(NULLIF(mp.full_name,''), NULLIF(pc.email,''), p.external_subject), COALESCE(pc.email,''),
 a.clock_in_at, a.clock_out_at,
 GREATEST(0, floor(extract(epoch FROM (COALESCE(a.clock_out_at, now()) - a.clock_in_at)) / 60)::int - a.total_break_minutes
  - CASE WHEN a.clock_out_at IS NULL AND a.current_break_start IS NOT NULL THEN floor(extract(epoch FROM (now() - a.current_break_start)) / 60)::int ELSE 0 END)
FROM employee_attendance a JOIN principals p ON p.id=a.principal_id
LEFT JOIN member_profiles mp ON mp.principal_id=p.id LEFT JOIN password_credentials pc ON pc.principal_id=p.id
WHERE a.organization_id=$1 AND a.location_id=$2 AND a.clock_in_at >= $3 AND a.clock_in_at < $4
ORDER BY 2, a.clock_in_at`, scope.OrganizationID, scope.LocationID, first, after)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	index := map[string]int{}
	for rows.Next() {
		var id, name, email string
		var in time.Time
		var outAt *time.Time
		var minutes int
		if err = rows.Scan(&id, &name, &email, &in, &outAt, &minutes); err != nil {
			return out, err
		}
		i, ok := index[id]
		if !ok {
			i = len(out)
			index[id] = i
			out = append(out, StaffWorkTotal{PrincipalID: id, Name: name, Email: email, Days: []StaffWorkDay{}})
		}
		t := &out[i]
		date := in.In(loc).Format("2006-01-02")
		n := len(t.Days)
		if n == 0 || t.Days[n-1].Date != date {
			clockIn := in
			t.Days = append(t.Days, StaffWorkDay{Date: date, ClockIn: &clockIn})
			n++
		}
		d := &t.Days[n-1]
		d.Minutes += minutes
		d.Shifts++
		// The day's last clock-out, or none while one of its shifts is still open.
		if outAt == nil {
			d.Open, d.ClockOut = true, nil
		} else if !d.Open && (d.ClockOut == nil || outAt.After(*d.ClockOut)) {
			clockOut := *outAt
			d.ClockOut = &clockOut
		}
		t.TotalMinutes += minutes
		t.ClockedIn = t.ClockedIn || outAt == nil
	}
	return out, rows.Err()
}
