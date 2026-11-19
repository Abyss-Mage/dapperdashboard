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
	// Each event carries what the team needs on the phone (owner, error404 #162): customer, number, add-ons, payment, place, time, total.
	query := `SELECT b.id::text,b.service_name,b.status,b.booking_time,COALESCE(r.occupied_end,b.booking_time+interval '1 hour'),c.name,b.service_address,
 COALESCE(c.phone,''),COALESCE(c.email,''),COALESCE(b.number::text,''),COALESCE(b.notes,''),
 COALESCE((SELECT string_agg(a->>'name', ', ') FROM jsonb_array_elements(CASE WHEN jsonb_typeof(b.addons)='array' THEN b.addons ELSE '[]'::jsonb END) a),''),
 COALESCE(inv.total_cents,lb.total_cents,0),COALESCE(inv.paid_cents,lb.paid_cents,0),
 COALESCE((SELECT p.method FROM payments p WHERE p.invoice_id=inv.id AND p.status IN ('completed','refunded') ORDER BY p.recorded_at DESC LIMIT 1),lb.payment_method,'')
 FROM bookings b
 LEFT JOIN LATERAL (SELECT i.id,i.total_cents,i.paid_cents FROM invoices i WHERE i.booking_id=b.id AND i.status<>'void' ORDER BY i.created_at DESC LIMIT 1) inv ON true
 LEFT JOIN legacy_booking_billing lb ON lb.booking_id=b.id AND lb.status<>'void' JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id)
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
	line("REFRESH-INTERVAL;VALUE=DURATION:PT5M")
	line("X-PUBLISHED-TTL:PT5M")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for rows.Next() {
		var id, service, status, customer, address, phone, email, number, notes, addons, method string
		var start, end time.Time
		var total, paid int64
		if err = rows.Scan(&id, &service, &status, &start, &end, &customer, &address, &phone, &email, &number, &notes, &addons, &total, &paid, &method); err != nil {
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
		line("DESCRIPTION:" + icsEscape(feedDetails(start, number, customer, phone, email, addons, address, notes, total, paid, method, status)))
		line("END:VEVENT")
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	line("END:VCALENDAR")
	return out.String(), nil
}

// feedDetails is the text under a calendar event: one fact per line, in Toronto time.
func feedDetails(start time.Time, number, customer, phone, email, addons, address, notes string, total, paid int64, method, status string) string {
	zone, err := time.LoadLocation("America/Toronto")
	if err != nil {
		zone = time.UTC
	}
	lines := []string{"Booking #" + number, "When: " + start.In(zone).Format("Mon, Jan 2, 2006 at 3:04 PM"), "Customer: " + customer}
	if number == "" {
		lines[0] = "Booking"
	}
	if phone != "" {
		lines = append(lines, "Phone: "+phone)
	}
	if email != "" {
		lines = append(lines, "Email: "+email)
	}
	if addons == "" {
		addons = "None"
	}
	lines = append(lines, "Add-ons: "+addons, "Where: "+feedPlace(address, notes))
	pay := "Pay at the appointment"
	how := map[string]string{"square_card": "online by card", "square_terminal": "by card (reader)", "cash": "in cash", "bank_transfer": "by e-transfer", "cheque": "by cheque"}[method]
	switch {
	case total > 0 && paid >= total:
		pay = strings.TrimSpace("Paid " + how)
	case paid > 0:
		pay = strings.TrimSpace("Part paid "+how) + ", " + emailMoney(total-paid) + " due"
	}
	lines = append(lines, "Payment: "+pay)
	if total > 0 {
		lines = append(lines, "Total: "+emailMoney(total))
	}
	lines = append(lines, "Status: "+status)
	return strings.Join(lines, "\n")
}

// feedPlace is the shop, mobile or valet line, from the "Service place:" note the booking steps write, else from the address.
func feedPlace(address, notes string) string {
	for _, l := range strings.Split(notes, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "Service place:"); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if strings.TrimSpace(address) != "" {
		return "Mobile: " + address
	}
	return "At the shop"
}
