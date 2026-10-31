package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrCalendarFeedInput    = errors.New("calendar link name must be 80 characters or fewer")
	ErrCalendarFeedNotFound = errors.New("calendar link not found")
)

type CalendarFeed struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	LocationID   string    `json:"locationId"`
	LocationName string    `json:"locationName"`
	Label        string    `json:"label"`
	CreatedAt    time.Time `json:"createdAt"`
}

func newFeedToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), digest[:], nil
}

func (s *Store) createFeed(ctx context.Context, tx pgx.Tx, kind string, scope Scope, label string) (CalendarFeed, string, error) {
	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > 80 {
		return CalendarFeed{}, "", ErrCalendarFeedInput
	}
	token, digest, err := newFeedToken()
	if err != nil {
		return CalendarFeed{}, "", err
	}
	var f CalendarFeed
	err = tx.QueryRow(ctx, `INSERT INTO calendar_feeds(token_hash,kind,organization_id,location_id,principal_id,label) VALUES($1,$2,$3,$4,$5,$6)
 RETURNING id::text, kind, location_id::text, label, created_at`, digest, kind, scope.OrganizationID, scope.LocationID, scope.ActorID, label).Scan(&f.ID, &f.Kind, &f.LocationID, &f.Label, &f.CreatedAt)
	if err != nil {
		return CalendarFeed{}, "", err
	}
	_ = tx.QueryRow(ctx, `SELECT name FROM locations WHERE organization_id=$1 AND id=$2`, scope.OrganizationID, scope.LocationID).Scan(&f.LocationName)
	return f, token, nil
}

// CreateFranchiseCalendarFeed makes a link for everything booked at the franchise. It needs bookings.read there.
func (s *Store) CreateFranchiseCalendarFeed(ctx context.Context, scope Scope, label string) (CalendarFeed, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CalendarFeed{}, "", err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.read"); err != nil {
		return CalendarFeed{}, "", err
	}
	f, token, err := s.createFeed(ctx, tx, "franchise", scope, label)
	if err != nil {
		return CalendarFeed{}, "", err
	}
	return f, token, tx.Commit(ctx)
}

// CreateEmployeeCalendarFeed makes a link for what the employee's own calendar shows (their franchise and dealership focus).
func (s *Store) CreateEmployeeCalendarFeed(ctx context.Context, actor, label string) (CalendarFeed, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CalendarFeed{}, "", err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return CalendarFeed{}, "", err
	}
	if err = authorizeTransaction(ctx, tx, employeeScope(a), "bookings.read"); err != nil {
		return CalendarFeed{}, "", err
	}
	f, token, err := s.createFeed(ctx, tx, "employee", employeeScope(a), label)
	if err != nil {
		return CalendarFeed{}, "", err
	}
	return f, token, tx.Commit(ctx)
}

func (s *Store) listFeeds(ctx context.Context, where string, args ...any) ([]CalendarFeed, error) {
	rows, err := s.pool.Query(ctx, `SELECT f.id::text,f.kind,f.location_id::text,l.name,f.label,f.created_at FROM calendar_feeds f JOIN locations l ON l.organization_id=f.organization_id AND l.id=f.location_id WHERE f.revoked_at IS NULL AND `+where+` ORDER BY f.created_at DESC LIMIT 50`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CalendarFeed{}
	for rows.Next() {
		var f CalendarFeed
		if err = rows.Scan(&f.ID, &f.Kind, &f.LocationID, &f.LocationName, &f.Label, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FranchiseCalendarFeeds lists the active franchise links at a location for someone who can read its bookings.
func (s *Store) FranchiseCalendarFeeds(ctx context.Context, scope Scope) ([]CalendarFeed, error) {
	if err := s.authorize(ctx, scope, "bookings.read"); err != nil {
		return nil, err
	}
	return s.listFeeds(ctx, `f.kind='franchise' AND f.organization_id=$1 AND f.location_id=$2`, scope.OrganizationID, scope.LocationID)
}

// EmployeeCalendarFeeds lists the employee's own active links.
func (s *Store) EmployeeCalendarFeeds(ctx context.Context, actor string) ([]CalendarFeed, error) {
	return s.listFeeds(ctx, `f.kind='employee' AND f.principal_id=$1`, actor)
}

// RevokeFranchiseCalendarFeed stops a franchise link working (anyone who can read the franchise's bookings may do this).
func (s *Store) RevokeFranchiseCalendarFeed(ctx context.Context, scope Scope, id string) error {
	if err := s.authorize(ctx, scope, "bookings.read"); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE calendar_feeds SET revoked_at=now() WHERE id=$1 AND kind='franchise' AND organization_id=$2 AND location_id=$3 AND revoked_at IS NULL`, id, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrCalendarFeedNotFound
	}
	return nil
}

// RevokeEmployeeCalendarFeed stops one of the employee's own links working.
func (s *Store) RevokeEmployeeCalendarFeed(ctx context.Context, actor, id string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE calendar_feeds SET revoked_at=now() WHERE id=$1 AND kind='employee' AND principal_id=$2 AND revoked_at IS NULL`, id, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrCalendarFeedNotFound
	}
	return nil
}

func icsEscape(v string) string {
	v = strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\r\n", "\\n", "\n", "\\n", "\r", "").Replace(v)
	return v
}

// icsFold wraps a content line at 75 octets as RFC 5545 asks, never splitting a character.
func icsFold(line string) string {
	if len(line) <= 75 {
		return line
	}
	var b strings.Builder
	n := 0
	for _, r := range line {
		size := utf8.RuneLen(r)
		if n+size > 75 {
			b.WriteString("\r\n ")
			n = 1
		}
		b.WriteRune(r)
		n += size
	}
	return b.String()
}

// CalendarFeedICS returns the calendar for a link token: bookings from 30 days ago to 180 days ahead, cancelled ones left out.
// An unknown or revoked token is ErrCalendarFeedNotFound.
func (s *Store) CalendarFeedICS(ctx context.Context, token string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(raw) != 32 {
		return "", ErrCalendarFeedNotFound
	}
	digest := sha256.Sum256(raw)
	var kind, org, loc, principal, locationName, label string
	err = s.pool.QueryRow(ctx, `SELECT f.kind,f.organization_id::text,f.location_id::text,f.principal_id::text,l.name,f.label FROM calendar_feeds f JOIN locations l ON l.organization_id=f.organization_id AND l.id=f.location_id WHERE f.token_hash=$1 AND f.revoked_at IS NULL`, digest[:]).Scan(&kind, &org, &loc, &principal, &locationName, &label)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrCalendarFeedNotFound
	}
	if err != nil {
		return "", err
	}
	from, to := time.Now().AddDate(0, 0, -30), time.Now().AddDate(0, 0, 180)
	query := `SELECT b.id::text,b.service_name,b.status,b.booking_time,COALESCE(r.occupied_end,b.booking_time+interval '1 hour'),c.name,b.service_address
 FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id)
 LEFT JOIN capacity_reservations r ON r.id=b.reservation_id AND r.cancelled_at IS NULL`
	args := []any{org, loc, from, to}
	if kind == "employee" {
		var dealership string
		if err = s.pool.QueryRow(ctx, `SELECT COALESCE(dealership_id::text,'') FROM employee_worksite_assignments WHERE principal_id=$1 AND organization_id=$2 AND location_id=$3`, principal, org, loc).Scan(&dealership); err != nil {
			return "", ErrCalendarFeedNotFound
		}
		query += ` JOIN employee_booking_worksites w ON (w.organization_id,w.location_id,w.booking_id)=(b.organization_id,b.location_id,b.id)
 WHERE b.organization_id=$1 AND b.location_id=$2 AND b.booking_time>=$3 AND b.booking_time<$4 AND b.status<>'cancelled' AND (w.dealership_id IS NULL OR w.dealership_id=NULLIF($5,'')::uuid)`
		args = append(args, dealership)
	} else {
		query += ` WHERE b.organization_id=$1 AND b.location_id=$2 AND b.booking_time>=$3 AND b.booking_time<$4 AND b.status<>'cancelled'`
	}
	rows, err := s.pool.Query(ctx, query+` ORDER BY b.booking_time,b.id LIMIT 2000`, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	name := "Dapper Detailing, " + locationName
	if label != "" {
		name = label
	}
	var out strings.Builder
	line := func(l string) { out.WriteString(icsFold(l)); out.WriteString("\r\n") }
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Dapper Detailing//Bookings//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:" + icsEscape(name))
	line("REFRESH-INTERVAL;VALUE=DURATION:PT15M")
	line("X-PUBLISHED-TTL:PT15M")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for rows.Next() {
		var id, service, status, customer, address string
		var start, end time.Time
		if err = rows.Scan(&id, &service, &status, &start, &end, &customer, &address); err != nil {
			return "", err
		}
		if service == "" {
			service = "Appointment"
		}
		line("BEGIN:VEVENT")
		line("UID:" + id + "@dapper")
		line("DTSTAMP:" + stamp)
		line("DTSTART:" + start.UTC().Format("20060102T150405Z"))
		line("DTEND:" + end.UTC().Format("20060102T150405Z"))
		line("SUMMARY:" + icsEscape(service+" · "+customer))
		if address != "" {
			line("LOCATION:" + icsEscape(address))
		}
		line("DESCRIPTION:" + icsEscape("Status: "+status))
		line("END:VEVENT")
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	line("END:VCALENDAR")
	return out.String(), nil
}
