package postgres

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrAttendanceActiveShift       = errors.New("employee already has an active shift in this location")
	ErrAttendanceNoActiveShift     = errors.New("no active shift in this location")
	ErrAttendanceInvalidTransition = errors.New("invalid attendance state transition")
	ErrAttendanceInput             = errors.New("valid notes and parameters required")
)

type AttendanceRecord struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organizationId"`
	LocationID        string     `json:"locationId"`
	PrincipalID       string     `json:"principalId"`
	Status            string     `json:"status"` // 'clocked_in', 'on_break', 'clocked_out'
	ClockInAt         time.Time  `json:"clockInAt"`
	ClockOutAt        *time.Time `json:"clockOutAt,omitempty"`
	CurrentBreakStart *time.Time `json:"currentBreakStart,omitempty"`
	TotalBreakMinutes int        `json:"totalBreakMinutes"`
	TotalWorkMinutes  int        `json:"totalWorkMinutes"`
	Notes             string     `json:"notes"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type AttendanceEvent struct {
	ID             string    `json:"id"`
	AttendanceID   string    `json:"attendanceId"`
	OrganizationID string    `json:"organizationId"`
	LocationID     string    `json:"locationId"`
	PrincipalID    string    `json:"principalId"`
	ActorID        string    `json:"actorId"`
	EventType      string    `json:"eventType"`
	RecordedAt     time.Time `json:"recordedAt"`
	Notes          string    `json:"notes"`
}

const attendanceColumns = `id::text,organization_id::text,location_id::text,principal_id::text,status,clock_in_at,clock_out_at,current_break_start,total_break_minutes,notes,created_at,updated_at`

func computeWorkMinutes(clockIn time.Time, clockOut *time.Time, currentBreak *time.Time, totalBreakMinutes int) int {
	end := time.Now()
	if clockOut != nil {
		end = *clockOut
	}
	elapsed := int(end.Sub(clockIn).Minutes())
	breakMinutes := totalBreakMinutes
	if currentBreak != nil && clockOut == nil {
		breakMinutes += int(end.Sub(*currentBreak).Minutes())
	}
	work := elapsed - breakMinutes
	if work < 0 {
		return 0
	}
	return work
}

func scanAttendance(row pgx.Row) (AttendanceRecord, error) {
	var a AttendanceRecord
	err := row.Scan(
		&a.ID,
		&a.OrganizationID,
		&a.LocationID,
		&a.PrincipalID,
		&a.Status,
		&a.ClockInAt,
		&a.ClockOutAt,
		&a.CurrentBreakStart,
		&a.TotalBreakMinutes,
		&a.Notes,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		return a, err
	}
	a.TotalWorkMinutes = computeWorkMinutes(a.ClockInAt, a.ClockOutAt, a.CurrentBreakStart, a.TotalBreakMinutes)
	return a, nil
}

// ClockIn records a shift start without a location requirement (internal callers and tests). Anything a person
// can reach over HTTP uses ClockInWithLocation so a franchise that requires location is always enforced.
func (s *Store) ClockIn(ctx context.Context, scope Scope, notes string) (AttendanceRecord, error) {
	return s.clockIn(ctx, scope, notes, nil, false)
}

// ClockInWithLocation starts a shift. When on-shift location is enabled for the franchise it REQUIRES a current fix:
// without one nothing is written and the person is not clocked in. The fix becomes the shift's first location sample.
func (s *Store) ClockInWithLocation(ctx context.Context, scope Scope, notes string, fix *GPSFix) (AttendanceRecord, error) {
	return s.clockIn(ctx, scope, notes, fix, true)
}

func (s *Store) clockIn(ctx context.Context, scope Scope, notes string, fix *GPSFix, enforce bool) (AttendanceRecord, error) {
	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > 1000 {
		return AttendanceRecord{}, ErrAttendanceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AttendanceRecord{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
		return AttendanceRecord{}, err
	}
	required := false
	if enforce {
		settings, e := loadGPSSettings(ctx, tx, scope.OrganizationID, scope.LocationID, true)
		if e != nil {
			return AttendanceRecord{}, e
		}
		required = settings.Enabled
		if required {
			if fix == nil {
				return AttendanceRecord{}, ErrGPSRequired
			}
			if e = validateGPSFix(*fix); e != nil {
				return AttendanceRecord{}, e
			}
		}
	}

	var activeID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM employee_attendance WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND clock_out_at IS NULL FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, scope.ActorID).Scan(&activeID)
	if err == nil {
		return AttendanceRecord{}, ErrAttendanceActiveShift
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return AttendanceRecord{}, err
	}

	row := tx.QueryRow(ctx, `INSERT INTO employee_attendance (organization_id, location_id, principal_id, status, clock_in_at, notes)
VALUES ($1, $2, $3, 'clocked_in', now(), $4)
RETURNING `+attendanceColumns, scope.OrganizationID, scope.LocationID, scope.ActorID, notes)

	item, err := scanAttendance(row)
	if err != nil {
		return AttendanceRecord{}, err
	}

	_, err = tx.Exec(ctx, `INSERT INTO employee_attendance_events (attendance_id, organization_id, location_id, principal_id, actor_id, event_type, notes)
VALUES ($1, $2, $3, $4, $5, 'clock_in', $6)`, item.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, scope.ActorID, notes)
	if err != nil {
		return AttendanceRecord{}, err
	}
	if required {
		if _, err = tx.Exec(ctx, `INSERT INTO gps_samples(attendance_id,organization_id,location_id,principal_id,latitude,longitude,accuracy_meters,captured_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
			item.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, fix.Latitude, fix.Longitude, fix.AccuracyMeters, fix.CapturedAt); err != nil {
			return AttendanceRecord{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO gps_shift_state(attendance_id,organization_id,location_id,principal_id,state,last_sample_at) VALUES($1,$2,$3,$4,'active',$5)`,
			item.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, fix.CapturedAt); err != nil {
			return AttendanceRecord{}, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return AttendanceRecord{}, err
	}
	return item, nil
}

func (s *Store) StartBreak(ctx context.Context, scope Scope, notes string) (AttendanceRecord, error) {
	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > 1000 {
		return AttendanceRecord{}, ErrAttendanceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AttendanceRecord{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
		return AttendanceRecord{}, err
	}

	row := tx.QueryRow(ctx, `SELECT `+attendanceColumns+` FROM employee_attendance WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND clock_out_at IS NULL FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, scope.ActorID)
	current, err := scanAttendance(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return AttendanceRecord{}, ErrAttendanceNoActiveShift
	}
	if err != nil {
		return AttendanceRecord{}, err
	}
	if current.Status != "clocked_in" {
		return AttendanceRecord{}, ErrAttendanceInvalidTransition
	}

	updatedRow := tx.QueryRow(ctx, `UPDATE employee_attendance
SET status='on_break', current_break_start=now(), updated_at=now()
WHERE id=$1
RETURNING `+attendanceColumns, current.ID)

	updated, err := scanAttendance(updatedRow)
	if err != nil {
		return AttendanceRecord{}, err
	}

	_, err = tx.Exec(ctx, `INSERT INTO employee_attendance_events (attendance_id, organization_id, location_id, principal_id, actor_id, event_type, notes)
VALUES ($1, $2, $3, $4, $5, 'start_break', $6)`, current.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, scope.ActorID, notes)
	if err != nil {
		return AttendanceRecord{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return AttendanceRecord{}, err
	}
	return updated, nil
}

func (s *Store) EndBreak(ctx context.Context, scope Scope, notes string) (AttendanceRecord, error) {
	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > 1000 {
		return AttendanceRecord{}, ErrAttendanceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AttendanceRecord{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
		return AttendanceRecord{}, err
	}

	row := tx.QueryRow(ctx, `SELECT `+attendanceColumns+` FROM employee_attendance WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND clock_out_at IS NULL FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, scope.ActorID)
	current, err := scanAttendance(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return AttendanceRecord{}, ErrAttendanceNoActiveShift
	}
	if err != nil {
		return AttendanceRecord{}, err
	}
	if current.Status != "on_break" || current.CurrentBreakStart == nil {
		return AttendanceRecord{}, ErrAttendanceInvalidTransition
	}

	breakDuration := int(time.Since(*current.CurrentBreakStart).Minutes())
	if breakDuration < 0 {
		breakDuration = 0
	}

	updatedRow := tx.QueryRow(ctx, `UPDATE employee_attendance
SET status='clocked_in', current_break_start=NULL, total_break_minutes=total_break_minutes+$2, updated_at=now()
WHERE id=$1
RETURNING `+attendanceColumns, current.ID, breakDuration)

	updated, err := scanAttendance(updatedRow)
	if err != nil {
		return AttendanceRecord{}, err
	}

	_, err = tx.Exec(ctx, `INSERT INTO employee_attendance_events (attendance_id, organization_id, location_id, principal_id, actor_id, event_type, notes)
VALUES ($1, $2, $3, $4, $5, 'end_break', $6)`, current.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, scope.ActorID, notes)
	if err != nil {
		return AttendanceRecord{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return AttendanceRecord{}, err
	}
	return updated, nil
}

func (s *Store) ClockOut(ctx context.Context, scope Scope, notes string) (AttendanceRecord, error) {
	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > 1000 {
		return AttendanceRecord{}, ErrAttendanceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AttendanceRecord{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
		return AttendanceRecord{}, err
	}

	row := tx.QueryRow(ctx, `SELECT `+attendanceColumns+` FROM employee_attendance WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND clock_out_at IS NULL FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, scope.ActorID)
	current, err := scanAttendance(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return AttendanceRecord{}, ErrAttendanceNoActiveShift
	}
	if err != nil {
		return AttendanceRecord{}, err
	}

	totalBreak := current.TotalBreakMinutes
	if current.Status == "on_break" && current.CurrentBreakStart != nil {
		breakDuration := int(time.Since(*current.CurrentBreakStart).Minutes())
		if breakDuration > 0 {
			totalBreak += breakDuration
		}
	}

	updatedRow := tx.QueryRow(ctx, `UPDATE employee_attendance
SET status='clocked_out', clock_out_at=now(), current_break_start=NULL,
    total_break_minutes=$2,
    notes=CASE WHEN $3='' THEN notes WHEN notes='' THEN $3 ELSE notes || ' | ' || $3 END,
    updated_at=now()
WHERE id=$1
RETURNING `+attendanceColumns, current.ID, totalBreak, notes)

	updated, err := scanAttendance(updatedRow)
	if err != nil {
		return AttendanceRecord{}, err
	}

	_, err = tx.Exec(ctx, `INSERT INTO employee_attendance_events (attendance_id, organization_id, location_id, principal_id, actor_id, event_type, notes)
VALUES ($1, $2, $3, $4, $5, 'clock_out', $6)`, current.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, scope.ActorID, notes)
	if err != nil {
		return AttendanceRecord{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return AttendanceRecord{}, err
	}
	return updated, nil
}

func (s *Store) ActiveAttendance(ctx context.Context, scope Scope) (*AttendanceRecord, error) {
	if err := scope.validate(); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// User can read their active status if they have attendance.record or attendance.read
	err = authorizeTransaction(ctx, tx, scope, "attendance.record")
	if err != nil {
		err = authorizeTransaction(ctx, tx, scope, "attendance.read")
	}
	if err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx, `SELECT `+attendanceColumns+` FROM employee_attendance WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND clock_out_at IS NULL`,
		scope.OrganizationID, scope.LocationID, scope.ActorID)
	item, err := scanAttendance(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListAttendance(ctx context.Context, scope Scope, from, to time.Time, limit int) ([]AttendanceRecord, error) {
	if err := scope.validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Check if user has attendance.read for entire location, or fallback to attendance.record for self only
	hasLocationRead := authorizeTransaction(ctx, tx, scope, "attendance.read") == nil
	if !hasLocationRead {
		if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
			return nil, err
		}
	}

	var rows pgx.Rows
	if hasLocationRead {
		rows, err = tx.Query(ctx, `SELECT `+attendanceColumns+` FROM employee_attendance
WHERE organization_id=$1 AND location_id=$2 AND clock_in_at >= $3 AND clock_in_at <= $4
ORDER BY clock_in_at DESC LIMIT $5`, scope.OrganizationID, scope.LocationID, from, to, limit)
	} else {
		rows, err = tx.Query(ctx, `SELECT `+attendanceColumns+` FROM employee_attendance
WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND clock_in_at >= $4 AND clock_in_at <= $5
ORDER BY clock_in_at DESC LIMIT $6`, scope.OrganizationID, scope.LocationID, scope.ActorID, from, to, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []AttendanceRecord
	for rows.Next() {
		item, err := scanAttendance(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) AttendanceEvents(ctx context.Context, scope Scope, attendanceID string) ([]AttendanceEvent, error) {
	if err := scope.validate(); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	hasLocationRead := authorizeTransaction(ctx, tx, scope, "attendance.read") == nil
	if !hasLocationRead {
		if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
			return nil, err
		}
	}

	var rows pgx.Rows
	if hasLocationRead {
		rows, err = tx.Query(ctx, `SELECT id::text, attendance_id::text, organization_id::text, location_id::text, principal_id::text, actor_id::text, event_type, recorded_at, notes
FROM employee_attendance_events WHERE organization_id=$1 AND location_id=$2 AND attendance_id=$3 ORDER BY recorded_at ASC`, scope.OrganizationID, scope.LocationID, attendanceID)
	} else {
		rows, err = tx.Query(ctx, `SELECT id::text, attendance_id::text, organization_id::text, location_id::text, principal_id::text, actor_id::text, event_type, recorded_at, notes
FROM employee_attendance_events WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND attendance_id=$4 ORDER BY recorded_at ASC`, scope.OrganizationID, scope.LocationID, scope.ActorID, attendanceID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AttendanceEvent
	for rows.Next() {
		var ev AttendanceEvent
		if err = rows.Scan(&ev.ID, &ev.AttendanceID, &ev.OrganizationID, &ev.LocationID, &ev.PrincipalID, &ev.ActorID, &ev.EventType, &ev.RecordedAt, &ev.Notes); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}
