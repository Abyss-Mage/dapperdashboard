package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGPSTrackingLifecycle(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_gps_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if _, e := admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	config := admin.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	var org, loc, staff, viewer, manager, stranger string
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic GPS Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic GPS Location') RETURNING id::text`, org).Scan(&loc))
	for i, dst := range []*string{&staff, &viewer, &manager, &stranger} {
		must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) RETURNING id::text`, fmt.Sprintf("gps-synthetic-%d@example.invalid", i)).Scan(dst))
	}
	for principal, actions := range map[string][]string{staff: {"attendance.record"}, viewer: {"attendance.read"}, manager: {"attendance.read", "attendance.manage"}} {
		for _, action := range actions {
			_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, principal, org, loc, action)
			must(err)
		}
	}
	staffScope, viewerScope, managerScope, strangerScope := Scope{org, loc, staff}, Scope{org, loc, viewer}, Scope{org, loc, manager}, Scope{org, loc, stranger}
	sample := func() GPSSampleInput {
		return GPSSampleInput{Latitude: 44.3091, Longitude: -78.3197, CapturedAt: time.Now()}
	}
	backdate := func() {
		_, err := pool.Exec(ctx, `UPDATE gps_samples SET received_at=received_at - interval '10 minutes'`)
		must(err)
	}

	// No shift, then shift with collection still disabled by default.
	if _, err = s.RecordGPSSample(ctx, staff, sample()); !errors.Is(err, ErrGPSNotTracking) {
		t.Fatalf("sample without shift: %v", err)
	}
	if _, err = s.ClockIn(ctx, staffScope, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecordGPSSample(ctx, staff, sample()); !errors.Is(err, ErrGPSDisabled) {
		t.Fatalf("collection must be off by default: %v", err)
	}
	defaults, err := s.GetGPSSettings(ctx, viewerScope)
	if err != nil || defaults.Enabled || defaults.SampleIntervalSeconds != 300 || defaults.RetentionDays != 30 || defaults.Version != 0 {
		t.Fatalf("defaults %+v %v", defaults, err)
	}

	// Settings authority, ranges and optimistic versions.
	if _, err = s.SaveGPSSettings(ctx, viewerScope, GPSSettings{Enabled: true, SampleIntervalSeconds: 300, RetentionDays: 30}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("read-only viewer changed settings: %v", err)
	}
	if _, err = s.SaveGPSSettings(ctx, managerScope, GPSSettings{Enabled: true, SampleIntervalSeconds: 30, RetentionDays: 30}); !errors.Is(err, ErrGPSSettingRange) {
		t.Fatalf("interval below range: %v", err)
	}
	saved, err := s.SaveGPSSettings(ctx, managerScope, GPSSettings{Enabled: true, SampleIntervalSeconds: 300, RetentionDays: 30})
	if err != nil || saved.Version != 1 || !saved.Enabled {
		t.Fatalf("create settings %+v %v", saved, err)
	}
	if _, err = s.SaveGPSSettings(ctx, managerScope, GPSSettings{Enabled: true, SampleIntervalSeconds: 300, RetentionDays: 30}); !errors.Is(err, ErrGPSVersion) {
		t.Fatalf("duplicate create: %v", err)
	}
	if _, err = s.SaveGPSSettings(ctx, managerScope, GPSSettings{Enabled: true, SampleIntervalSeconds: 600, RetentionDays: 30, Version: 7}); !errors.Is(err, ErrGPSVersion) {
		t.Fatalf("stale version: %v", err)
	}

	// Sampling rules.
	bad := sample()
	bad.Latitude = 91
	if _, err = s.RecordGPSSample(ctx, staff, bad); !errors.Is(err, ErrGPSInput) {
		t.Fatalf("latitude: %v", err)
	}
	old := sample()
	old.CapturedAt = time.Now().Add(-time.Hour)
	if _, err = s.RecordGPSSample(ctx, staff, old); !errors.Is(err, ErrGPSInput) {
		t.Fatalf("stale capture time: %v", err)
	}
	receipt, err := s.RecordGPSSample(ctx, staff, sample())
	if err != nil || !receipt.Recorded || receipt.SampleIntervalSeconds != 300 {
		t.Fatalf("first sample %+v %v", receipt, err)
	}
	if _, err = s.RecordGPSSample(ctx, staff, sample()); !errors.Is(err, ErrGPSRate) {
		t.Fatalf("burst must be rate limited: %v", err)
	}
	if _, err = s.RecordGPSSample(ctx, stranger, sample()); !errors.Is(err, ErrGPSNotTracking) {
		t.Fatalf("a principal with no shift recorded: %v", err)
	}

	// Viewers: grant required, coordinates only while clocked in.
	if _, err = s.GPSLive(ctx, strangerScope); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ungranted live view: %v", err)
	}
	live, err := s.GPSLive(ctx, viewerScope)
	if err != nil || len(live.Entries) != 1 || live.Entries[0].Latitude == nil || live.Entries[0].Stale || live.Entries[0].State != "active" {
		t.Fatalf("live %+v %v", live, err)
	}
	if _, err = s.StartBreak(ctx, staffScope, ""); err != nil {
		t.Fatal(err)
	}
	backdate()
	if _, err = s.RecordGPSSample(ctx, staff, sample()); !errors.Is(err, ErrGPSNotTracking) {
		t.Fatalf("sample accepted on break: %v", err)
	}
	live, err = s.GPSLive(ctx, viewerScope)
	if err != nil || len(live.Entries) != 1 || live.Entries[0].Latitude != nil || live.Entries[0].ShiftStatus != "on_break" {
		t.Fatalf("coordinates shown on break: %+v %v", live, err)
	}
	if _, err = s.EndBreak(ctx, staffScope, ""); err != nil {
		t.Fatal(err)
	}

	// Device-reported states are visible to supervisors; unknown states are rejected.
	if err = s.ReportGPSState(ctx, staff, "permission_lost"); err != nil {
		t.Fatal(err)
	}
	if err = s.ReportGPSState(ctx, staff, "bogus"); !errors.Is(err, ErrGPSInput) {
		t.Fatalf("unknown state: %v", err)
	}
	live, _ = s.GPSLive(ctx, viewerScope)
	if live.Entries[0].State != "permission_lost" {
		t.Fatalf("state not visible: %+v", live.Entries[0])
	}
	status, err := s.EmployeeGPSStatus(ctx, staff)
	if err != nil || !status.OnShift || !status.TrackingActive || status.State != "permission_lost" {
		t.Fatalf("employee status %+v %v", status, err)
	}

	// Retention: expired samples vanish from reads and are purged on the next write.
	if _, err = s.RecordGPSSample(ctx, staff, sample()); err != nil {
		t.Fatal(err)
	}
	var shiftID string
	must(pool.QueryRow(ctx, `SELECT attendance_id::text FROM gps_samples LIMIT 1`).Scan(&shiftID))
	if got, _ := s.GPSShiftSamples(ctx, viewerScope, shiftID); len(got) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(got))
	}
	_, err = pool.Exec(ctx, `UPDATE gps_samples SET received_at=clock_timestamp() - interval '40 days' WHERE id=(SELECT id FROM gps_samples ORDER BY captured_at LIMIT 1)`)
	must(err)
	if got, _ := s.GPSShiftSamples(ctx, viewerScope, shiftID); len(got) != 1 {
		t.Fatalf("expired sample still readable: %d", len(got))
	}
	backdate()
	if _, err = s.RecordGPSSample(ctx, staff, sample()); err != nil {
		t.Fatal(err)
	}
	var rows int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM gps_samples WHERE received_at < clock_timestamp() - interval '30 days'`).Scan(&rows))
	if rows != 0 {
		t.Fatalf("expired sample not purged: %d", rows)
	}

	// Revoked permission stops recording; clock-out stops it too.
	_, err = pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='attendance.record'`, staff)
	must(err)
	backdate()
	if _, err = s.RecordGPSSample(ctx, staff, sample()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked grant: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'attendance.record')`, staff, org, loc)
	must(err)
	if _, err = s.ClockOut(ctx, staffScope, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecordGPSSample(ctx, staff, sample()); !errors.Is(err, ErrGPSNotTracking) {
		t.Fatalf("sample after clock-out: %v", err)
	}
	// Clocking in requires a current location while collection is enabled for the franchise.
	fresh := GPSFix{Latitude: 44.3091, Longitude: -78.3197, CapturedAt: time.Now()}
	if _, err = s.ClockInWithLocation(ctx, staffScope, "", nil); !errors.Is(err, ErrGPSRequired) {
		t.Fatalf("clock-in without location accepted: %v", err)
	}
	var open int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM employee_attendance WHERE principal_id=$1 AND clock_out_at IS NULL`, staff).Scan(&open))
	if open != 0 {
		t.Fatal("a refused clock-in still created a shift")
	}
	stale := fresh
	stale.CapturedAt = time.Now().Add(-time.Hour)
	if _, err = s.ClockInWithLocation(ctx, staffScope, "", &stale); !errors.Is(err, ErrGPSInput) {
		t.Fatalf("stale location accepted at clock-in: %v", err)
	}
	outOfRange := fresh
	outOfRange.Latitude = 123
	if _, err = s.ClockInWithLocation(ctx, staffScope, "", &outOfRange); !errors.Is(err, ErrGPSInput) {
		t.Fatalf("impossible location accepted at clock-in: %v", err)
	}
	shift, err := s.ClockInWithLocation(ctx, staffScope, "", &fresh)
	if err != nil || shift.Status != "clocked_in" {
		t.Fatalf("clock-in with location failed: %+v %v", shift, err)
	}
	var samples int
	var firstState string
	must(pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM gps_samples WHERE attendance_id=$1),(SELECT state FROM gps_shift_state WHERE attendance_id=$1)`, shift.ID).Scan(&samples, &firstState))
	if samples != 1 || firstState != "active" {
		t.Fatalf("clock-in location was not recorded as the first sample: %d %s", samples, firstState)
	}
	if _, err = s.ClockOut(ctx, staffScope, ""); err != nil {
		t.Fatal(err)
	}
	// With collection switched off the rule does not apply.
	cur, err := s.GetGPSSettings(ctx, managerScope)
	must(err)
	if _, err = s.SaveGPSSettings(ctx, managerScope, GPSSettings{Enabled: false, SampleIntervalSeconds: cur.SampleIntervalSeconds, RetentionDays: cur.RetentionDays, Version: cur.Version}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClockInWithLocation(ctx, staffScope, "", nil); err != nil {
		t.Fatalf("clock-in blocked although location is not enabled: %v", err)
	}
	if _, err = s.ClockOut(ctx, staffScope, ""); err != nil {
		t.Fatal(err)
	}
	var events int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM gps_settings_events`).Scan(&events))
	if events != 2 {
		t.Fatalf("settings audit events: %d", events)
	}
}
