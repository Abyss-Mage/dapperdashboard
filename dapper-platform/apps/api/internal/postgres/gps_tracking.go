package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrGPSDisabled     = errors.New("on-shift location is not enabled for this location")
	ErrGPSNotTracking  = errors.New("location is only accepted while clocked in and not on break")
	ErrGPSInput        = errors.New("valid location sample required")
	ErrGPSRate         = errors.New("location sample received too soon")
	ErrGPSVersion      = errors.New("location settings changed; refresh and retry")
	ErrGPSSettingRange = errors.New("sampling must be 60-3600 seconds and retention 1-365 days")
	ErrGPSRequired     = errors.New("a current location is required to clock in at this franchise")
)

const (
	DefaultGPSIntervalSeconds = 300
	DefaultGPSRetentionDays   = 30
	gpsMaxClockSkewPast       = 5 * time.Minute
	gpsMaxClockSkewFuture     = 30 * time.Second
)

type GPSSettings struct {
	Enabled               bool      `json:"enabled"`
	SampleIntervalSeconds int       `json:"sampleIntervalSeconds"`
	RetentionDays         int       `json:"retentionDays"`
	Version               int       `json:"version"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

func defaultGPSSettings() GPSSettings {
	return GPSSettings{SampleIntervalSeconds: DefaultGPSIntervalSeconds, RetentionDays: DefaultGPSRetentionDays}
}

func loadGPSSettings(ctx context.Context, tx pgx.Tx, org, loc string, share bool) (GPSSettings, error) {
	lock := ""
	if share {
		lock = " FOR SHARE"
	}
	g := defaultGPSSettings()
	err := tx.QueryRow(ctx, `SELECT enabled,sample_interval_seconds,retention_days,version,updated_at FROM gps_settings WHERE organization_id=$1 AND location_id=$2`+lock, org, loc).
		Scan(&g.Enabled, &g.SampleIntervalSeconds, &g.RetentionDays, &g.Version, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultGPSSettings(), nil
	}
	return g, err
}

// GetGPSSettings returns the configured values, or disabled defaults when none were saved.
func (s *Store) GetGPSSettings(ctx context.Context, scope Scope) (GPSSettings, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GPSSettings{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.read"); err != nil {
		return GPSSettings{}, err
	}
	return loadGPSSettings(ctx, tx, scope.OrganizationID, scope.LocationID, false)
}

// SaveGPSSettings needs attendance.manage. Version 0 creates; otherwise the exact current version is required.
func (s *Store) SaveGPSSettings(ctx context.Context, scope Scope, in GPSSettings) (GPSSettings, error) {
	if in.SampleIntervalSeconds < 60 || in.SampleIntervalSeconds > 3600 || in.RetentionDays < 1 || in.RetentionDays > 365 || in.Version < 0 {
		return GPSSettings{}, ErrGPSSettingRange
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GPSSettings{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.manage"); err != nil {
		return GPSSettings{}, err
	}
	var out GPSSettings
	if in.Version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO gps_settings(organization_id,location_id,enabled,sample_interval_seconds,retention_days,updated_by)
VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING RETURNING enabled,sample_interval_seconds,retention_days,version,updated_at`,
			scope.OrganizationID, scope.LocationID, in.Enabled, in.SampleIntervalSeconds, in.RetentionDays, scope.ActorID).
			Scan(&out.Enabled, &out.SampleIntervalSeconds, &out.RetentionDays, &out.Version, &out.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `UPDATE gps_settings SET enabled=$3,sample_interval_seconds=$4,retention_days=$5,version=version+1,updated_by=$6,updated_at=clock_timestamp()
WHERE organization_id=$1 AND location_id=$2 AND version=$7 RETURNING enabled,sample_interval_seconds,retention_days,version,updated_at`,
			scope.OrganizationID, scope.LocationID, in.Enabled, in.SampleIntervalSeconds, in.RetentionDays, scope.ActorID, in.Version).
			Scan(&out.Enabled, &out.SampleIntervalSeconds, &out.RetentionDays, &out.Version, &out.UpdatedAt)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return GPSSettings{}, ErrGPSVersion
	}
	if err != nil {
		return GPSSettings{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gps_settings_events(organization_id,location_id,actor_id,version,enabled,sample_interval_seconds,retention_days) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		scope.OrganizationID, scope.LocationID, scope.ActorID, out.Version, out.Enabled, out.SampleIntervalSeconds, out.RetentionDays); err != nil {
		return GPSSettings{}, err
	}
	return out, tx.Commit(ctx)
}

type GPSSampleInput struct {
	Latitude       float64
	Longitude      float64
	AccuracyMeters *float64
	CapturedAt     time.Time
}

type GPSSampleReceipt struct {
	Recorded              bool `json:"recorded"`
	SampleIntervalSeconds int  `json:"sampleIntervalSeconds"`
}

// activeShift locks the principal's open shift. The shift, not the request, supplies the scope.
func activeShift(ctx context.Context, tx pgx.Tx, principal string) (id string, scope Scope, status string, err error) {
	scope.ActorID = principal
	err = tx.QueryRow(ctx, `SELECT id::text,organization_id::text,location_id::text,status FROM employee_attendance WHERE principal_id=$1 AND clock_out_at IS NULL FOR SHARE`, principal).
		Scan(&id, &scope.OrganizationID, &scope.LocationID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrGPSNotTracking
	}
	return
}

// RecordGPSSample stores one employee sample. It is accepted only while the shift is
// clocked_in (never on break or after clock-out), tracking is enabled, the sample time is
// plausible and samples are not arriving faster than half the configured interval.
func (s *Store) RecordGPSSample(ctx context.Context, principal string, in GPSSampleInput) (GPSSampleReceipt, error) {
	if math.IsNaN(in.Latitude) || math.IsNaN(in.Longitude) || in.Latitude < -90 || in.Latitude > 90 || in.Longitude < -180 || in.Longitude > 180 ||
		(in.AccuracyMeters != nil && (math.IsNaN(*in.AccuracyMeters) || *in.AccuracyMeters < 0 || *in.AccuracyMeters > 100000)) {
		return GPSSampleReceipt{}, ErrGPSInput
	}
	now := time.Now()
	if in.CapturedAt.IsZero() || in.CapturedAt.Before(now.Add(-gpsMaxClockSkewPast)) || in.CapturedAt.After(now.Add(gpsMaxClockSkewFuture)) {
		return GPSSampleReceipt{}, ErrGPSInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GPSSampleReceipt{}, err
	}
	defer tx.Rollback(ctx)
	id, scope, status, err := activeShift(ctx, tx, principal)
	if err != nil {
		return GPSSampleReceipt{}, err
	}
	if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
		return GPSSampleReceipt{}, err
	}
	if status != "clocked_in" {
		return GPSSampleReceipt{}, ErrGPSNotTracking
	}
	settings, err := loadGPSSettings(ctx, tx, scope.OrganizationID, scope.LocationID, true)
	if err != nil {
		return GPSSampleReceipt{}, err
	}
	if !settings.Enabled {
		return GPSSampleReceipt{}, ErrGPSDisabled
	}
	var tooSoon bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gps_samples WHERE attendance_id=$1 AND received_at > clock_timestamp() - make_interval(secs => $2))`, id, float64(settings.SampleIntervalSeconds)/2).Scan(&tooSoon); err != nil {
		return GPSSampleReceipt{}, err
	}
	if tooSoon {
		return GPSSampleReceipt{}, ErrGPSRate
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gps_samples(attendance_id,organization_id,location_id,principal_id,latitude,longitude,accuracy_meters,captured_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		id, scope.OrganizationID, scope.LocationID, principal, in.Latitude, in.Longitude, in.AccuracyMeters, in.CapturedAt); err != nil {
		return GPSSampleReceipt{}, ErrGPSInput
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gps_shift_state(attendance_id,organization_id,location_id,principal_id,state,last_sample_at) VALUES($1,$2,$3,$4,'active',$5)
ON CONFLICT(attendance_id) DO UPDATE SET state='active',last_sample_at=EXCLUDED.last_sample_at,updated_at=clock_timestamp()`, id, scope.OrganizationID, scope.LocationID, principal, in.CapturedAt); err != nil {
		return GPSSampleReceipt{}, err
	}
	// Retention is enforced on read and physically purged here, bounded per call.
	if _, err = tx.Exec(ctx, `DELETE FROM gps_samples WHERE id IN (SELECT id FROM gps_samples WHERE organization_id=$1 AND location_id=$2 AND received_at < clock_timestamp() - make_interval(days => $3) LIMIT 500)`,
		scope.OrganizationID, scope.LocationID, settings.RetentionDays); err != nil {
		return GPSSampleReceipt{}, err
	}
	return GPSSampleReceipt{Recorded: true, SampleIntervalSeconds: settings.SampleIntervalSeconds}, tx.Commit(ctx)
}

// ReportGPSState lets the device report permission loss etc. so supervisors see why samples stopped.
func (s *Store) ReportGPSState(ctx context.Context, principal, state string) error {
	switch state {
	case "active", "permission_denied", "permission_lost", "unsupported", "stopped":
	default:
		return ErrGPSInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	id, scope, _, err := activeShift(ctx, tx, principal)
	if err != nil {
		return err
	}
	if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gps_shift_state(attendance_id,organization_id,location_id,principal_id,state) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(attendance_id) DO UPDATE SET state=EXCLUDED.state,updated_at=clock_timestamp()`, id, scope.OrganizationID, scope.LocationID, principal, state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type EmployeeGPSStatus struct {
	OnShift        bool        `json:"onShift"`
	ShiftStatus    string      `json:"shiftStatus,omitempty"`
	Settings       GPSSettings `json:"settings"`
	State          string      `json:"state,omitempty"`
	LastSampleAt   *time.Time  `json:"lastSampleAt,omitempty"`
	TrackingActive bool        `json:"trackingActive"`
}

// EmployeeGPSStatus tells the device whether it may sample now. It never exposes other staff.
func (s *Store) EmployeeGPSStatus(ctx context.Context, principal string) (EmployeeGPSStatus, error) {
	out := EmployeeGPSStatus{Settings: defaultGPSSettings()}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	id, scope, status, err := activeShift(ctx, tx, principal)
	if errors.Is(err, ErrGPSNotTracking) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
		return out, err
	}
	out.OnShift, out.ShiftStatus = true, status
	if out.Settings, err = loadGPSSettings(ctx, tx, scope.OrganizationID, scope.LocationID, false); err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `SELECT state,last_sample_at FROM gps_shift_state WHERE attendance_id=$1`, id).Scan(&out.State, &out.LastSampleAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	out.TrackingActive = out.Settings.Enabled && status == "clocked_in"
	return out, nil
}

type GPSLiveEntry struct {
	AttendanceID string     `json:"attendanceId"`
	Employee     string     `json:"employee"`
	ShiftStatus  string     `json:"shiftStatus"`
	State        string     `json:"state"`
	Stale        bool       `json:"stale"`
	LastSampleAt *time.Time `json:"lastSampleAt,omitempty"`
	Latitude     *float64   `json:"latitude,omitempty"`
	Longitude    *float64   `json:"longitude,omitempty"`
	Accuracy     *float32   `json:"accuracyMeters,omitempty"`
}

type GPSLive struct {
	Settings GPSSettings    `json:"settings"`
	Entries  []GPSLiveEntry `json:"entries"`
}

// GPSLive lists open shifts for a location. Coordinates are returned only for staff who are
// clocked in (not on break) and only if the last sample is inside the retention window.
func (s *Store) GPSLive(ctx context.Context, scope Scope) (GPSLive, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GPSLive{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.read"); err != nil {
		return GPSLive{}, err
	}
	out := GPSLive{Entries: []GPSLiveEntry{}}
	if out.Settings, err = loadGPSSettings(ctx, tx, scope.OrganizationID, scope.LocationID, false); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT a.id::text,COALESCE(NULLIF(c_p.email,''),p.external_subject),a.status,COALESCE(st.state,'none'),st.last_sample_at,ls.latitude,ls.longitude,ls.accuracy_meters
FROM employee_attendance a JOIN principals p ON p.id=a.principal_id LEFT JOIN password_credentials c_p ON c_p.principal_id=p.id
LEFT JOIN gps_shift_state st ON st.attendance_id=a.id
LEFT JOIN LATERAL (SELECT latitude,longitude,accuracy_meters FROM gps_samples g WHERE g.attendance_id=a.id AND g.received_at >= clock_timestamp() - make_interval(days => $3) ORDER BY captured_at DESC LIMIT 1) ls ON true
WHERE a.organization_id=$1 AND a.location_id=$2 AND a.clock_out_at IS NULL ORDER BY 2 LIMIT 200`, scope.OrganizationID, scope.LocationID, out.Settings.RetentionDays)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	staleAfter := time.Duration(2*out.Settings.SampleIntervalSeconds+60) * time.Second
	for rows.Next() {
		var e GPSLiveEntry
		if err = rows.Scan(&e.AttendanceID, &e.Employee, &e.ShiftStatus, &e.State, &e.LastSampleAt, &e.Latitude, &e.Longitude, &e.Accuracy); err != nil {
			return out, err
		}
		if e.ShiftStatus != "clocked_in" || !out.Settings.Enabled {
			e.Latitude, e.Longitude, e.Accuracy = nil, nil, nil
		} else {
			e.Stale = e.LastSampleAt == nil || time.Since(*e.LastSampleAt) > staleAfter
		}
		out.Entries = append(out.Entries, e)
	}
	return out, rows.Err()
}

type GPSSample struct {
	CapturedAt time.Time `json:"capturedAt"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	Accuracy   *float32  `json:"accuracyMeters,omitempty"`
}

// GPSShiftSamples returns samples for one shift in the scope, inside the retention window.
func (s *Store) GPSShiftSamples(ctx context.Context, scope Scope, attendanceID string) ([]GPSSample, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.read"); err != nil {
		return nil, err
	}
	settings, err := loadGPSSettings(ctx, tx, scope.OrganizationID, scope.LocationID, false)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT captured_at,latitude,longitude,accuracy_meters FROM gps_samples WHERE attendance_id=$1 AND organization_id=$2 AND location_id=$3
AND received_at >= clock_timestamp() - make_interval(days => $4) ORDER BY captured_at LIMIT 2000`, attendanceID, scope.OrganizationID, scope.LocationID, settings.RetentionDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GPSSample{}
	for rows.Next() {
		var g GPSSample
		if err = rows.Scan(&g.CapturedAt, &g.Latitude, &g.Longitude, &g.Accuracy); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// GPSFix is a location reading supplied with a clock-in.
type GPSFix struct {
	Latitude       float64
	Longitude      float64
	AccuracyMeters *float64
	CapturedAt     time.Time
}

func validateGPSFix(f GPSFix) error {
	if math.IsNaN(f.Latitude) || math.IsNaN(f.Longitude) || f.Latitude < -90 || f.Latitude > 90 || f.Longitude < -180 || f.Longitude > 180 ||
		(f.AccuracyMeters != nil && (math.IsNaN(*f.AccuracyMeters) || *f.AccuracyMeters < 0 || *f.AccuracyMeters > 100000)) {
		return ErrGPSInput
	}
	now := time.Now()
	if f.CapturedAt.IsZero() || f.CapturedAt.Before(now.Add(-gpsMaxClockSkewPast)) || f.CapturedAt.After(now.Add(gpsMaxClockSkewFuture)) {
		return ErrGPSInput
	}
	return nil
}
