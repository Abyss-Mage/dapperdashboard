package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUploadInput        = errors.New("valid photos and post details required")
	ErrUploadsUnavailable = errors.New("photo storage is not configured")
	ErrUploadNotFound     = errors.New("upload not found")
)

const (
	MaxUploadPhotos     = 10
	MaxUploadPhotoBytes = 5 << 20
)

// SetUploadDirectory installs the server-side photo directory (outside the web root, never served directly).
func (s *Store) SetUploadDirectory(dir string) { s.uploadDir = dir }

type UploadPostInput struct {
	Kind           string
	CustomerName   string
	InspectionType string
	BookingID      string // customer_car: the dealership-linked booking being inspected
	DealershipID   string // dealership_tag: required, a partner dealership account (never the worksite)
	Notes          string
	RequestKey     string
	Latitude       *float64
	Longitude      *float64
	Accuracy       *float64
	CapturedAt     time.Time
	Photos         [][]byte
}

type UploadPhoto struct {
	ID        string `json:"id"`
	Ordinal   int    `json:"ordinal"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	SizeBytes int    `json:"sizeBytes"`
}

type UploadPost struct {
	ID             string        `json:"id"`
	Kind           string        `json:"kind"`
	DealershipName string        `json:"dealershipName"`
	CustomerName   string        `json:"customerName,omitempty"`
	InspectionType string        `json:"inspectionType,omitempty"`
	BookingID      string        `json:"bookingId,omitempty"`
	ServiceName    string        `json:"serviceName,omitempty"`
	BookingTime    *time.Time    `json:"bookingTime,omitempty"`
	Notes          string        `json:"notes"`
	Latitude       *float64      `json:"latitude,omitempty"`
	Longitude      *float64      `json:"longitude,omitempty"`
	AccuracyMeters *float32      `json:"accuracyMeters,omitempty"`
	CapturedAt     time.Time     `json:"capturedAt"`
	CreatedAt      time.Time     `json:"createdAt"`
	RetentionUntil *time.Time    `json:"retentionUntil,omitempty"`
	Photos         []UploadPhoto `json:"photos"`
}

func validUploadKey(v string) bool {
	if len(v) < 16 || len(v) > 128 {
		return false
	}
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}

// CreateEmployeeUploadPost stores one post (1-10 JPEG photos). The dealership, organisation and location come
// from the administrator's assignment, never from the request. The server records its own time and the
// authenticated identity; the burned-in stamp on the pixels is a convenience, this record is the audit trail.
// A repeated request key returns the original post without writing anything again.
func (s *Store) CreateEmployeeUploadPost(ctx context.Context, actor string, in UploadPostInput) (UploadPost, error) {
	in.CustomerName, in.Notes = strings.TrimSpace(in.CustomerName), strings.TrimSpace(in.Notes)
	if s.uploadDir == "" {
		return UploadPost{}, ErrUploadsUnavailable
	}
	now := time.Now()
	if !validUploadKey(in.RequestKey) || utf8.RuneCountInString(in.Notes) > 2000 || len(in.Photos) < 1 || len(in.Photos) > MaxUploadPhotos ||
		in.CapturedAt.IsZero() || in.CapturedAt.Before(now.Add(-24*time.Hour)) || in.CapturedAt.After(now.Add(5*time.Minute)) ||
		(in.Latitude == nil) != (in.Longitude == nil) {
		return UploadPost{}, ErrUploadInput
	}
	if in.Latitude != nil && (math.IsNaN(*in.Latitude) || math.IsNaN(*in.Longitude) || *in.Latitude < -90 || *in.Latitude > 90 || *in.Longitude < -180 || *in.Longitude > 180) {
		return UploadPost{}, ErrUploadInput
	}
	if in.Accuracy != nil && (math.IsNaN(*in.Accuracy) || *in.Accuracy < 0 || *in.Accuracy > 100000) {
		return UploadPost{}, ErrUploadInput
	}
	switch in.Kind {
	case "dealership_tag":
		if in.CustomerName != "" || in.InspectionType != "" || in.BookingID != "" || len(in.DealershipID) != 36 {
			return UploadPost{}, ErrUploadInput
		}
	case "customer_car":
		// The customer comes from the booking; typed names and free dealership choices are not accepted.
		if in.CustomerName != "" || in.DealershipID != "" || len(in.BookingID) != 36 || (in.InspectionType != "pre" && in.InspectionType != "post") {
			return UploadPost{}, ErrUploadInput
		}
	default:
		return UploadPost{}, ErrUploadInput
	}
	type decoded struct {
		data          []byte
		width, height int
		digest        [32]byte
		key           string
	}
	photos := make([]decoded, len(in.Photos))
	for i, raw := range in.Photos {
		if len(raw) == 0 || len(raw) > MaxUploadPhotoBytes {
			return UploadPost{}, ErrUploadInput
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 12000 || cfg.Height > 12000 || len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
			return UploadPost{}, ErrUploadInput
		}
		name := make([]byte, 32)
		if _, err = rand.Read(name); err != nil {
			return UploadPost{}, err
		}
		photos[i] = decoded{data: raw, width: cfg.Width, height: cfg.Height, digest: sha256.Sum256(raw), key: hex.EncodeToString(name) + ".jpg"}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return UploadPost{}, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return UploadPost{}, err
	}
	scope := employeeScope(a)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.record"); err != nil {
		return UploadPost{}, err
	}
	var existing string
	err = tx.QueryRow(ctx, `SELECT id::text FROM employee_upload_posts WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3 AND request_key=$4`, a.OrganizationID, a.LocationID, actor, in.RequestKey).Scan(&existing)
	if err == nil {
		tx.Rollback(ctx)
		return s.employeeUploadPost(ctx, actor, existing)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return UploadPost{}, err
	}
	var id string
	var customer, inspection, booking *string
	var dealership any
	switch in.Kind {
	case "dealership_tag":
		// Dealerships are partner car dealers added in Master, not the employee's location or internal worksite account.
		var ok bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM dealership_accounts WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND active AND account_type='dealership')`, a.OrganizationID, a.LocationID, in.DealershipID).Scan(&ok); err != nil {
			return UploadPost{}, err
		}
		if !ok {
			return UploadPost{}, ErrUploadInput
		}
		dealership = in.DealershipID
	case "customer_car":
		if err = employeeBookingLock(ctx, tx, a, in.BookingID); err != nil {
			return UploadPost{}, err
		}
		var name string
		if err = tx.QueryRow(ctx, `SELECT c.name FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.id=$1 AND b.organization_id=$2 AND b.location_id=$3`, in.BookingID, a.OrganizationID, a.LocationID).Scan(&name); err != nil {
			return UploadPost{}, err
		}
		var linked *string
		if err = tx.QueryRow(ctx, `SELECT dealership_id::text FROM employee_booking_worksites WHERE booking_id=$1`, in.BookingID).Scan(&linked); err != nil {
			return UploadPost{}, err
		}
		dealership = linked
		customer, inspection, booking = &name, &in.InspectionType, &in.BookingID
	}
	if err = tx.QueryRow(ctx, `INSERT INTO employee_upload_posts(organization_id,location_id,dealership_id,principal_id,kind,customer_name,inspection_type,booking_id,notes,latitude,longitude,accuracy_meters,captured_at,request_key,retention_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,CASE WHEN $5='dealership_tag' THEN clock_timestamp()+interval '30 days' END) RETURNING id::text`,
		a.OrganizationID, a.LocationID, dealership, actor, in.Kind, customer, inspection, booking, in.Notes, in.Latitude, in.Longitude, in.Accuracy, in.CapturedAt, in.RequestKey).Scan(&id); err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return UploadPost{}, ErrUploadInput
		}
		return UploadPost{}, err
	}
	if err = os.MkdirAll(s.uploadDir, 0o700); err != nil {
		return UploadPost{}, err
	}
	written := []string{}
	cleanup := func() {
		for _, name := range written {
			_ = os.Remove(filepath.Join(s.uploadDir, name))
		}
	}
	for i, p := range photos {
		if err = os.WriteFile(filepath.Join(s.uploadDir, p.key), p.data, 0o600); err != nil {
			cleanup()
			return UploadPost{}, err
		}
		written = append(written, p.key)
		if _, err = tx.Exec(ctx, `INSERT INTO employee_upload_photos(post_id,ordinal,storage_key,sha256,size_bytes,width,height) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, i+1, p.key, p.digest[:], len(p.data), p.width, p.height); err != nil {
			cleanup()
			return UploadPost{}, err
		}
	}
	if err = employeeEvent(ctx, tx, actor, a.OrganizationID, id, "upload.posted", map[string]any{"kind": in.Kind, "photos": len(photos)}); err != nil {
		cleanup()
		return UploadPost{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		cleanup()
		return UploadPost{}, err
	}
	return s.employeeUploadPost(ctx, actor, id)
}

func (s *Store) employeeUploadPost(ctx context.Context, actor, id string) (UploadPost, error) {
	posts, err := s.ListEmployeeUploads(ctx, actor, "", id)
	if err != nil {
		return UploadPost{}, err
	}
	if len(posts) != 1 {
		return UploadPost{}, ErrUploadNotFound
	}
	return posts[0], nil
}

// ListEmployeeUploads returns the caller's own posts, newest first (up to 100). Other staff are never included.
func (s *Store) ListEmployeeUploads(ctx context.Context, actor, kind, only string) ([]UploadPost, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id::text,p.kind,COALESCE(d.account_name,''),COALESCE(p.customer_name,''),COALESCE(p.inspection_type,''),p.notes,p.latitude,p.longitude,p.accuracy_meters,p.captured_at,p.created_at,COALESCE(p.booking_id::text,''),COALESCE(b.service_name,''),b.booking_time,p.retention_until
FROM employee_upload_posts p LEFT JOIN dealership_accounts d ON (d.organization_id,d.location_id,d.id)=(p.organization_id,p.location_id,p.dealership_id)
LEFT JOIN bookings b ON b.id=p.booking_id
WHERE p.principal_id=$1 AND ($2='' OR p.kind=$2) AND ($3='' OR p.id::text=$3) ORDER BY p.created_at DESC LIMIT 100`, actor, kind, only)
	if err != nil {
		return nil, err
	}
	posts := []UploadPost{}
	index := map[string]int{}
	for rows.Next() {
		var p UploadPost
		if err = rows.Scan(&p.ID, &p.Kind, &p.DealershipName, &p.CustomerName, &p.InspectionType, &p.Notes, &p.Latitude, &p.Longitude, &p.AccuracyMeters, &p.CapturedAt, &p.CreatedAt, &p.BookingID, &p.ServiceName, &p.BookingTime, &p.RetentionUntil); err != nil {
			rows.Close()
			return nil, err
		}
		p.Photos = []UploadPhoto{}
		index[p.ID] = len(posts)
		posts = append(posts, p)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(posts) == 0 {
		return posts, err
	}
	ids := make([]string, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.ID)
	}
	photoRows, err := s.pool.Query(ctx, `SELECT id::text,post_id::text,ordinal,width,height,size_bytes FROM employee_upload_photos WHERE post_id::text = ANY($1) ORDER BY post_id,ordinal`, ids)
	if err != nil {
		return nil, err
	}
	defer photoRows.Close()
	for photoRows.Next() {
		var ph UploadPhoto
		var post string
		if err = photoRows.Scan(&ph.ID, &post, &ph.Ordinal, &ph.Width, &ph.Height, &ph.SizeBytes); err != nil {
			return nil, err
		}
		posts[index[post]].Photos = append(posts[index[post]].Photos, ph)
	}
	return posts, photoRows.Err()
}

// EmployeeUploadPhoto returns bytes for a photo the caller owns. There is no cross-employee access path here.
func (s *Store) EmployeeUploadPhoto(ctx context.Context, actor, photo string) ([]byte, error) {
	var key string
	err := s.pool.QueryRow(ctx, `SELECT f.storage_key FROM employee_upload_photos f JOIN employee_upload_posts p ON p.id=f.post_id JOIN principals pr ON pr.id=p.principal_id WHERE f.id=$1 AND p.principal_id=$2 AND pr.active`, photo, actor).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUploadNotFound
	}
	if err != nil {
		return nil, err
	}
	if s.uploadDir == "" {
		return nil, ErrUploadsUnavailable
	}
	data, err := os.ReadFile(filepath.Join(s.uploadDir, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrUploadNotFound
	}
	return data, err
}

type EmployeeHours struct {
	Timezone     string  `json:"timezone"`
	TodaySeconds int64   `json:"todaySeconds"`
	WeekSeconds  int64   `json:"weekSeconds"`
	MonthSeconds int64   `json:"monthSeconds"`
	Attribution  string  `json:"attribution"`
	TodayHours   float64 `json:"todayHours"`
	WeekHours    float64 `json:"weekHours"`
	MonthHours   float64 `json:"monthHours"`
	WeekStartsOn string  `json:"weekStartsOn"`
	CalculatedAt string  `json:"calculatedAt"`
}

// EmployeeHours totals recorded work time (breaks excluded) for today, this week (from Monday) and this month in the
// worksite timezone. A shift counts toward the local day it started on.
func (s *Store) EmployeeHours(ctx context.Context, actor string) (EmployeeHours, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return EmployeeHours{}, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return EmployeeHours{}, err
	}
	zone, err := time.LoadLocation(a.Timezone)
	if err != nil {
		return EmployeeHours{}, ErrEmployeeInput
	}
	now := time.Now().In(zone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone)
	week := today.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, zone)
	var d, w, m float64
	err = tx.QueryRow(ctx, `WITH s AS (SELECT clock_in_at, GREATEST(0, EXTRACT(EPOCH FROM (COALESCE(clock_out_at,clock_timestamp())-clock_in_at)) - total_break_minutes*60 - CASE WHEN current_break_start IS NOT NULL AND clock_out_at IS NULL THEN EXTRACT(EPOCH FROM (clock_timestamp()-current_break_start)) ELSE 0 END) AS secs
FROM employee_attendance WHERE principal_id=$1 AND clock_in_at >= LEAST($2::timestamptz,$3::timestamptz,$4::timestamptz))
SELECT COALESCE(sum(secs) FILTER (WHERE clock_in_at >= $2),0)::float8,COALESCE(sum(secs) FILTER (WHERE clock_in_at >= $3),0)::float8,COALESCE(sum(secs) FILTER (WHERE clock_in_at >= $4),0)::float8 FROM s`,
		actor, today, week, month).Scan(&d, &w, &m)
	if err != nil {
		return EmployeeHours{}, err
	}
	round := func(secs float64) float64 { return math.Round(secs/36) / 100 }
	return EmployeeHours{Timezone: a.Timezone, TodaySeconds: int64(d), WeekSeconds: int64(w), MonthSeconds: int64(m), TodayHours: round(d), WeekHours: round(w), MonthHours: round(m),
		Attribution: "A shift counts toward the day it started.", WeekStartsOn: "Monday", CalculatedAt: now.Format(time.RFC3339)}, nil
}

type EmployeeBreak struct {
	Start   time.Time  `json:"start"`
	End     *time.Time `json:"end,omitempty"`
	Minutes int        `json:"minutes"`
}

// EmployeeBreaks pairs the caller's recorded break start/end events (most recent 100 events).
func (s *Store) EmployeeBreaks(ctx context.Context, actor string) ([]EmployeeBreak, error) {
	rows, err := s.pool.Query(ctx, `SELECT attendance_id::text,event_type,recorded_at FROM (SELECT attendance_id,event_type,recorded_at FROM employee_attendance_events WHERE principal_id=$1 AND event_type IN ('start_break','end_break') ORDER BY recorded_at DESC LIMIT 100) e ORDER BY attendance_id,recorded_at`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EmployeeBreak{}
	var open *time.Time
	var shift string
	for rows.Next() {
		var id, kind string
		var at time.Time
		if err = rows.Scan(&id, &kind, &at); err != nil {
			return nil, err
		}
		if id != shift {
			if open != nil {
				out = append(out, EmployeeBreak{Start: *open})
			}
			open, shift = nil, id
		}
		if kind == "start_break" {
			t := at
			open = &t
		} else if open != nil {
			out = append(out, EmployeeBreak{Start: *open, End: &at, Minutes: int(at.Sub(*open).Round(time.Minute) / time.Minute)})
			open = nil
		}
	}
	if open != nil {
		out = append(out, EmployeeBreak{Start: *open})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	sortBreaksNewestFirst(out)
	return out, rows.Err()
}

func sortBreaksNewestFirst(b []EmployeeBreak) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j].Start.After(b[j-1].Start); j-- {
			b[j], b[j-1] = b[j-1], b[j]
		}
	}
}

type UploadDealership struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// EmployeeUploadDealerships lists the partner dealerships a key-tag post may name: active accounts of type
// "dealership" at the employee's location. Dealerships are accounts an administrator adds in Master; they are not
// locations, franchises or the internal worksite account the employee happens to be assigned to.
func (s *Store) EmployeeUploadDealerships(ctx context.Context, actor string) ([]UploadDealership, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,account_name FROM dealership_accounts WHERE organization_id=$1 AND location_id=$2 AND active AND account_type='dealership' ORDER BY account_name LIMIT 200`, a.OrganizationID, a.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UploadDealership{}
	for rows.Next() {
		var d UploadDealership
		if err = rows.Scan(&d.ID, &d.Name); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
