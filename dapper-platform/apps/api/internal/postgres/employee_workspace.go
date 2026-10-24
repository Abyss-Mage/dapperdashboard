package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrEmployeeAssignment = errors.New("an administrator must assign an active dealership worksite")
var ErrEmployeeVersion = errors.New("this employee record changed; refresh before saving")
var ErrEmployeeInput = errors.New("valid worksite, timezone and availability intervals required")
var ErrEmployeeActiveShift = errors.New("clock out before changing the employee worksite")

// Assigned shop-floor users use the dealership-scoped API. A location grant must
// not expose other dealerships through older management endpoints.
func (s *Store) EmployeeUsesRestrictedAPI(ctx context.Context, actor string) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var org string
	err = tx.QueryRow(ctx, `SELECT organization_id::text FROM employee_worksite_assignments WHERE principal_id=$1`, actor).Scan(&org)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = accessManagementGrant(ctx, tx, actor, org)
	if errors.Is(err, ErrForbidden) {
		return true, nil
	}
	return false, err
}

type EmployeeWorksite struct {
	PrincipalID    string `json:"principalId"`
	OrganizationID string `json:"organizationId"`
	LocationID     string `json:"locationId"`
	LocationName   string `json:"locationName"`
	DealershipID   string `json:"dealershipId"`
	DealershipName string `json:"dealershipName"`
	Timezone       string `json:"timezone"`
	Version        int    `json:"version"`
}
type AvailabilityInterval struct {
	Day   int    `json:"day"` // Monday=0, Sunday=6, in the assigned worksite timezone.
	Start string `json:"start"`
	End   string `json:"end"`
}
type EmployeeAvailability struct {
	WeekStart   string                 `json:"weekStart"`
	Timezone    string                 `json:"timezone"`
	Intervals   []AvailabilityInterval `json:"intervals"`
	Notes       string                 `json:"notes"`
	Version     int                    `json:"version"`
	SubmittedAt *time.Time             `json:"submittedAt"`
}
type EmployeeDesk struct {
	Employee     string                `json:"employee"`
	Assignment   *EmployeeWorksite     `json:"assignment"`
	Actions      []string              `json:"actions"`
	Bookings     []Booking             `json:"bookings"`
	Attendance   []AttendanceRecord    `json:"attendance"`
	Availability *EmployeeAvailability `json:"availability"`
	GeneratedAt  time.Time             `json:"generatedAt"`
}

type EmployeeWorksiteChoice struct {
	OrganizationID string `json:"organizationId"`
	LocationID     string `json:"locationId"`
	LocationName   string `json:"locationName"`
	DealershipID   string `json:"dealershipId"`
	DealershipName string `json:"dealershipName"`
}
type EmployeeLocationChoice struct {
	OrganizationID string `json:"organizationId"`
	LocationID     string `json:"locationId"`
	LocationName   string `json:"locationName"`
	Code           string `json:"code"`
}
type EmployeeAvailabilityReview struct {
	LocationName   string          `json:"locationName"`
	PrincipalID    string          `json:"principalId"`
	DealershipName string          `json:"dealershipName"`
	WeekStart      string          `json:"weekStart"`
	Timezone       string          `json:"timezone"`
	Intervals      json.RawMessage `json:"intervals"`
	Notes          string          `json:"notes"`
	Version        int             `json:"version"`
	SubmittedAt    time.Time       `json:"submittedAt"`
}
type EmployeeAdministration struct {
	Assignments []EmployeeWorksite           `json:"assignments"`
	Locations   []EmployeeLocationChoice     `json:"locations"`
	Dealerships []EmployeeWorksiteChoice     `json:"dealerships"`
	Submissions []EmployeeAvailabilityReview `json:"submissions"`
}

func (s *Store) EmployeeWorksiteAdministration(ctx context.Context, actor, org string) (EmployeeAdministration, error) {
	r := EmployeeAdministration{Assignments: []EmployeeWorksite{}, Locations: []EmployeeLocationChoice{}, Dealerships: []EmployeeWorksiteChoice{}, Submissions: []EmployeeAvailabilityReview{}}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer tx.Rollback(ctx)
	if err = accessManagementGrant(ctx, tx, actor, org); err != nil {
		return r, err
	}
	rows, err := tx.Query(ctx, `SELECT `+employeeAssignmentColumns+` FROM employee_worksite_assignments a JOIN locations l ON l.id=a.location_id AND l.organization_id=a.organization_id LEFT JOIN dealership_accounts d ON d.id=a.dealership_id AND d.organization_id=a.organization_id AND d.location_id=a.location_id WHERE a.organization_id=$1 ORDER BY l.name,a.principal_id`, org)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var a EmployeeWorksite
		if err = rows.Scan(&a.PrincipalID, &a.OrganizationID, &a.LocationID, &a.LocationName, &a.DealershipID, &a.DealershipName, &a.Timezone, &a.Version); err != nil {
			rows.Close()
			return r, err
		}
		r.Assignments = append(r.Assignments, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return r, err
	}
	rows, err = tx.Query(ctx, `SELECT organization_id::text,id::text,name,COALESCE(code,'') FROM locations WHERE organization_id=$1 ORDER BY name`, org)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var c EmployeeLocationChoice
		if err = rows.Scan(&c.OrganizationID, &c.LocationID, &c.LocationName, &c.Code); err != nil {
			rows.Close()
			return r, err
		}
		r.Locations = append(r.Locations, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return r, err
	}
	rows, err = tx.Query(ctx, `SELECT d.organization_id::text,d.location_id::text,l.name,d.id::text,d.account_name FROM dealership_accounts d JOIN locations l ON l.id=d.location_id AND l.organization_id=d.organization_id WHERE d.organization_id=$1 AND d.active AND d.account_type='dealership' ORDER BY l.name,d.account_name`, org)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var a EmployeeWorksiteChoice
		if err = rows.Scan(&a.OrganizationID, &a.LocationID, &a.LocationName, &a.DealershipID, &a.DealershipName); err != nil {
			rows.Close()
			return r, err
		}
		r.Dealerships = append(r.Dealerships, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return r, err
	}
	rows, err = tx.Query(ctx, `SELECT s.principal_id::text,COALESCE(d.account_name,''),l.name,s.week_start::text,s.timezone,s.intervals,s.notes,s.version,s.submitted_at FROM employee_availability_submissions s JOIN locations l ON l.organization_id=s.organization_id AND l.id=s.location_id LEFT JOIN dealership_accounts d ON d.organization_id=s.organization_id AND d.location_id=s.location_id AND d.id=s.dealership_id WHERE s.organization_id=$1 AND s.week_start>=current_date-7 ORDER BY s.week_start,s.submitted_at DESC LIMIT 100`, org)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var a EmployeeAvailabilityReview
		if err = rows.Scan(&a.PrincipalID, &a.DealershipName, &a.LocationName, &a.WeekStart, &a.Timezone, &a.Intervals, &a.Notes, &a.Version, &a.SubmittedAt); err != nil {
			rows.Close()
			return r, err
		}
		r.Submissions = append(r.Submissions, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return r, err
	}
	return r, tx.Commit(ctx)
}

const employeeAssignmentColumns = `a.principal_id::text,a.organization_id::text,a.location_id::text,l.name,COALESCE(a.dealership_id::text,''),COALESCE(d.account_name,''),a.timezone,a.version`

func employeeWorksite(ctx context.Context, tx pgx.Tx, actor string) (EmployeeWorksite, error) {
	return employeeWorksiteIncludingInactive(ctx, tx, actor, false)
}
func employeeWorksiteIncludingInactive(ctx context.Context, tx pgx.Tx, actor string, allowInactive bool) (EmployeeWorksite, error) {
	var a EmployeeWorksite
	err := tx.QueryRow(ctx, `SELECT `+employeeAssignmentColumns+` FROM employee_worksite_assignments a
 JOIN principals p ON p.id=a.principal_id
 JOIN organization_memberships m ON m.organization_id=a.organization_id AND m.principal_id=p.id
 JOIN locations l ON l.id=a.location_id AND l.organization_id=a.organization_id
 LEFT JOIN dealership_accounts d ON d.id=a.dealership_id AND d.location_id=a.location_id AND d.organization_id=a.organization_id
 WHERE a.principal_id=$1 AND p.active AND (d.id IS NULL OR d.active OR $2) FOR SHARE OF a,p,m,l`, actor, allowInactive).Scan(&a.PrincipalID, &a.OrganizationID, &a.LocationID, &a.LocationName, &a.DealershipID, &a.DealershipName, &a.Timezone, &a.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrEmployeeAssignment
	}
	return a, err
}
func employeeScope(a EmployeeWorksite) Scope {
	return Scope{ActorID: a.PrincipalID, OrganizationID: a.OrganizationID, LocationID: a.LocationID}
}
func employeeEvent(ctx context.Context, tx pgx.Tx, actor, org, id, action string, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO employee_workspace_events(organization_id,actor_id,entity_id,action,detail) VALUES($1,$2,$3,$4,$5)`, org, actor, id, action, data)
	return err
}
func nextEmployeeWeek(now time.Time, zone string) (string, error) {
	z, err := time.LoadLocation(zone)
	if err != nil {
		return "", ErrEmployeeInput
	}
	n := now.In(z)
	days := (8 - int(n.Weekday())) % 7
	if days == 0 {
		days = 7
	}
	return time.Date(n.Year(), n.Month(), n.Day()+days, 0, 0, 0, 0, z).Format("2006-01-02"), nil
}

// EmployeeDesk is personal and server-scoped: no client organization/location selector.
func (s *Store) EmployeeWorkspace(ctx context.Context, actor string) (EmployeeDesk, error) {
	r := EmployeeDesk{Actions: []string{}, Bookings: []Booking{}, Attendance: []AttendanceRecord{}, GeneratedAt: time.Now().UTC()}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if errors.Is(err, ErrEmployeeAssignment) {
		return r, tx.Commit(ctx)
	}
	if err != nil {
		return r, err
	}
	r.Assignment = &a
	if err = tx.QueryRow(ctx, `SELECT COALESCE(NULLIF(c.email,''),p.external_subject) FROM principals p LEFT JOIN password_credentials c ON c.principal_id=p.id WHERE p.id=$1`, actor).Scan(&r.Employee); err != nil {
		return r, err
	}
	scope := employeeScope(a)
	for _, action := range []string{"attendance.record", "attendance.read", "bookings.read", "catalogue.read", "bookings.create", "payments.record", "invoices.read"} {
		if e := authorizeTransaction(ctx, tx, scope, action); e == nil {
			r.Actions = append(r.Actions, action)
		} else if !errors.Is(e, ErrForbidden) {
			return r, e
		}
	}
	if containsEmployeeAction(r.Actions, "bookings.read") {
		rows, e := tx.Query(ctx, `SELECT `+bookingColumns+` FROM bookings b
   JOIN customers c ON c.organization_id=b.organization_id AND c.location_id=b.location_id AND c.id=b.customer_id
   JOIN employee_booking_worksites w ON w.booking_id=b.id AND w.organization_id=b.organization_id AND w.location_id=b.location_id
   WHERE b.organization_id=$1 AND b.location_id=$2 AND (w.dealership_id IS NULL OR w.dealership_id=NULLIF($3,'')::uuid)
   AND b.booking_time >= now()-interval '1 day' AND b.booking_time < now()+interval '31 days'
   ORDER BY b.booking_time,b.id LIMIT 101`, a.OrganizationID, a.LocationID, a.DealershipID)
		if e != nil {
			return r, e
		}
		for rows.Next() {
			b, e := scanBooking(rows)
			if e != nil {
				rows.Close()
				return r, e
			}
			r.Bookings = append(r.Bookings, b)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return r, e
		}
	}
	if containsEmployeeAction(r.Actions, "attendance.record") || containsEmployeeAction(r.Actions, "attendance.read") {
		rows, e := tx.Query(ctx, `SELECT `+attendanceColumns+` FROM employee_attendance WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 ORDER BY clock_in_at DESC LIMIT 14`, a.OrganizationID, a.LocationID, actor)
		if e != nil {
			return r, e
		}
		for rows.Next() {
			v, e := scanAttendance(rows)
			if e != nil {
				rows.Close()
				return r, e
			}
			r.Attendance = append(r.Attendance, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return r, e
		}
	}
	week, e := nextEmployeeWeek(r.GeneratedAt, a.Timezone)
	if e != nil {
		return r, e
	}
	av := EmployeeAvailability{WeekStart: week, Timezone: a.Timezone, Intervals: []AvailabilityInterval{}}
	var data []byte
	var stamp time.Time
	e = tx.QueryRow(ctx, `SELECT intervals,notes,version,submitted_at FROM employee_availability_submissions WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND week_start=$4::date`, a.OrganizationID, a.LocationID, actor, week).Scan(&data, &av.Notes, &av.Version, &stamp)
	if e == nil {
		if e = json.Unmarshal(data, &av.Intervals); e != nil {
			return r, e
		}
		av.SubmittedAt = &stamp
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return r, e
	}
	r.Availability = &av
	return r, tx.Commit(ctx)
}
func containsEmployeeAction(actions []string, a string) bool {
	for _, v := range actions {
		if v == a {
			return true
		}
	}
	return false
}

// Assignment authorizes a worksite association only; it never adds operational capabilities.
func (s *Store) AssignEmployeeWorksite(ctx context.Context, actor, org, principal, location, dealer, zone string, version int) (EmployeeWorksite, error) {
	var result EmployeeWorksite
	if _, err := time.LoadLocation(zone); err != nil || zone == "Local" || zone == "" || len(zone) > 100 {
		return result, ErrEmployeeInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if err = accessManagementGrant(ctx, tx, actor, org); err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, principal+"/employee-worksite"); err != nil {
		return result, err
	}
	var active bool
	err = tx.QueryRow(ctx, `SELECT p.active FROM principals p JOIN organization_memberships m ON m.principal_id=p.id WHERE p.id=$1 AND m.organization_id=$2 FOR SHARE OF p,m`, principal, org).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || !active {
		return result, ErrForbidden
	}
	if err != nil {
		return result, err
	}
	var existingOrg string
	var current int
	err = tx.QueryRow(ctx, `SELECT organization_id::text,version FROM employee_worksite_assignments WHERE principal_id=$1 FOR UPDATE`, principal).Scan(&existingOrg, &current)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	if existingOrg != "" && existingOrg != org {
		return result, ErrForbidden
	}
	if current != version {
		return result, ErrEmployeeVersion
	}
	var working bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM employee_attendance WHERE principal_id=$1 AND clock_out_at IS NULL)`, principal).Scan(&working); err != nil {
		return result, err
	}
	if working {
		return result, ErrEmployeeActiveShift
	}
	var site string
	err = tx.QueryRow(ctx, `SELECT id::text FROM locations WHERE organization_id=$1 AND id=$2 FOR SHARE`, org, location).Scan(&site)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrEmployeeInput
	}
	if err != nil {
		return result, err
	}
	if dealer != "" {
		// The dealership is optional and must be a partner dealership account at the same franchise site.
		err = tx.QueryRow(ctx, `SELECT id::text FROM dealership_accounts WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND active AND account_type='dealership' FOR SHARE`, org, location, dealer).Scan(&site)
		if errors.Is(err, pgx.ErrNoRows) {
			return result, ErrEmployeeInput
		}
		if err != nil {
			return result, err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO employee_worksite_assignments(principal_id,organization_id,location_id,dealership_id,timezone,assigned_by) VALUES($1,$2,$3,NULLIF($4,'')::uuid,$5,$6)
 ON CONFLICT(principal_id) DO UPDATE SET location_id=excluded.location_id,dealership_id=excluded.dealership_id,timezone=excluded.timezone,assigned_by=excluded.assigned_by,assigned_at=now(),version=employee_worksite_assignments.version+1`, principal, org, location, dealer, zone, actor)
	if err != nil {
		return result, err
	}
	if err = employeeEvent(ctx, tx, actor, org, principal, "employee.worksite_assigned", map[string]any{"locationId": location, "dealershipId": dealer, "timezone": zone, "version": current + 1}); err != nil {
		return result, err
	}
	result, err = employeeWorksite(ctx, tx, principal)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

func validateAvailability(intervals []AvailabilityInterval) error {
	if len(intervals) > 28 {
		return ErrEmployeeInput
	}
	type span struct{ d, start, end int }
	all := []span{}
	for _, i := range intervals {
		start, e1 := time.Parse("15:04", i.Start)
		end, e2 := time.Parse("15:04", i.End)
		if e1 != nil || e2 != nil || i.Day < 0 || i.Day > 6 || len(i.Start) != 5 || len(i.End) != 5 {
			return ErrEmployeeInput
		}
		a, b := start.Hour()*60+start.Minute(), end.Hour()*60+end.Minute()
		if a >= b {
			return ErrEmployeeInput
		}
		all = append(all, span{i.Day, a, b})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].d != all[j].d {
			return all[i].d < all[j].d
		}
		return all[i].start < all[j].start
	})
	for i := 1; i < len(all); i++ {
		if all[i].d == all[i-1].d && all[i].start < all[i-1].end {
			return ErrEmployeeInput
		}
	}
	return nil
}
func (s *Store) SubmitEmployeeAvailability(ctx context.Context, actor string, input EmployeeAvailability) (EmployeeAvailability, error) {
	if err := validateAvailability(input.Intervals); err != nil {
		return input, err
	}
	input.Notes = strings.TrimSpace(input.Notes)
	if utf8.RuneCountInString(input.Notes) > 1000 || input.Version < 0 {
		return input, ErrEmployeeInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return input, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return input, err
	}
	week, err := nextEmployeeWeek(time.Now(), a.Timezone)
	if err != nil {
		return input, err
	}
	if input.WeekStart != week {
		return input, ErrEmployeeInput
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, actor+"/availability/"+week); err != nil {
		return input, err
	}
	var id string
	var version int
	err = tx.QueryRow(ctx, `SELECT id::text,version FROM employee_availability_submissions WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND week_start=$4::date FOR UPDATE`, a.OrganizationID, a.LocationID, actor, week).Scan(&id, &version)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return input, err
	}
	if version != input.Version {
		return input, ErrEmployeeVersion
	}
	if input.Intervals == nil {
		input.Intervals = []AvailabilityInterval{}
	}
	data, err := json.Marshal(input.Intervals)
	if err != nil {
		return input, err
	}
	input.Timezone = a.Timezone
	var stamp time.Time
	if id == "" {
		err = tx.QueryRow(ctx, `INSERT INTO employee_availability_submissions(organization_id,location_id,dealership_id,principal_id,week_start,timezone,intervals,notes) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5::date,$6,$7,$8) RETURNING id::text,version,submitted_at`, a.OrganizationID, a.LocationID, a.DealershipID, actor, week, a.Timezone, data, input.Notes).Scan(&id, &input.Version, &stamp)
	} else {
		err = tx.QueryRow(ctx, `UPDATE employee_availability_submissions SET intervals=$2,notes=$3,version=version+1,submitted_at=now() WHERE id=$1 RETURNING version,submitted_at`, id, data, input.Notes).Scan(&input.Version, &stamp)
	}
	if err != nil {
		return input, err
	}
	input.SubmittedAt = &stamp
	if err = employeeEvent(ctx, tx, actor, a.OrganizationID, id, "employee.availability_submitted", input); err != nil {
		return input, err
	}
	return input, tx.Commit(ctx)
}

// Keep the assignment lock until the attendance operation ends. Reassignment cannot race a punch.
func (s *Store) EmployeeAttendanceAction(ctx context.Context, actor, action string, fix *GPSFix) (AttendanceRecord, error) {
	var r AttendanceRecord
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksiteIncludingInactive(ctx, tx, actor, action == "clock-out")
	if err != nil {
		return r, err
	}
	switch action {
	case "clock-in":
		r, err = s.ClockInWithLocation(ctx, employeeScope(a), "", fix)
	case "clock-out":
		r, err = s.ClockOut(ctx, employeeScope(a), "")
	case "start-break":
		r, err = s.StartBreak(ctx, employeeScope(a), "")
	case "end-break":
		r, err = s.EndBreak(ctx, employeeScope(a), "")
	default:
		return r, ErrEmployeeInput
	}
	if err != nil {
		return r, err
	}
	return r, tx.Commit(ctx)
}

func (s *Store) LinkEmployeeBooking(ctx context.Context, actor, org, location, dealer, booking string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = accessManagementGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, org, location, booking).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEmployeeInput
	}
	if err != nil {
		return err
	}
	if dealer != "" {
		err = tx.QueryRow(ctx, `SELECT id::text FROM dealership_accounts WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND active AND account_type='dealership' FOR SHARE`, org, location, dealer).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEmployeeInput
		}
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO employee_booking_worksites(booking_id,organization_id,location_id,dealership_id,linked_by) VALUES($1,$2,$3,NULLIF($4,'')::uuid,$5) ON CONFLICT(booking_id) DO UPDATE SET dealership_id=excluded.dealership_id,linked_by=excluded.linked_by,linked_at=now()`, booking, org, location, dealer, actor)
	if err != nil {
		return err
	}
	if err = employeeEvent(ctx, tx, actor, org, booking, "employee.booking_worksite_linked", map[string]string{"dealershipId": dealer}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
