package postgres

import (
	"context"
	"net"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

func clip(v string, n int) string {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > n {
		r := []rune(v)
		return string(r[:n])
	}
	return v
}

// RecordActivity stores one line of the activity log. It never fails the request that triggered it: callers ignore the error.
func (s *Store) RecordActivity(ctx context.Context, principal, email, kind, method, path string, status int, ip, userAgent string) error {
	var p any
	if principal != "" {
		p = principal
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO activity_log(principal_id,email,kind,method,path,status,ip,user_agent) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		p, clip(email, 254), kind, clip(method, 10), clip(path, 300), status, clip(ip, 64), clip(userAgent, 300))
	return err
}

// RecordSignIn logs a sign-in or a failed attempt. A wrong password leaves the person unknown, so only the email typed is kept.
func (a *PasswordAuthenticator) RecordSignIn(ctx context.Context, email string, success bool, ip, userAgent string) {
	email, _ = normalizedEmail(email)
	var principal string
	_ = a.store.pool.QueryRow(ctx, `SELECT principal_id::text FROM password_credentials WHERE email=$1`, email).Scan(&principal)
	kind := "sign_in"
	if !success {
		kind = "sign_in_failed"
	}
	_ = a.store.RecordActivity(ctx, principal, email, kind, "POST", "/api/v1/auth/login", map[bool]int{true: 204, false: 401}[success], ip, userAgent)
}

// ClientIP is the visitor's address: the first X-Forwarded-For entry when the request reached us through our own proxy on this
// machine, else the connection address.
func ClientIP(remoteAddr, forwardedFor string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() && forwardedFor != "" {
		first := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		if net.ParseIP(first) != nil {
			return first
		}
	}
	return host
}

type ActivityEntry struct {
	ID        int64     `json:"id"`
	At        time.Time `json:"at"`
	Person    string    `json:"person"`
	Email     string    `json:"email"`
	Kind      string    `json:"kind"`
	What      string    `json:"what"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"userAgent"`
}

var uuidInPath = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

type activityRule struct {
	method string
	re     *regexp.Regexp
	label  string
}

func rule(method, pattern, label string) activityRule {
	return activityRule{method, regexp.MustCompile(pattern), label}
}

// What a change was, in words. Anything not listed shows as a generic change with its address.
var activityRules = []activityRule{
	rule("POST", `/members/[^/]+/terminate$`, "Terminated a team member"),
	rule("POST", `/members/[^/]+/reinstate$`, "Reinstated a team member"),
	rule("POST", `/bookings/[^/]+/status$`, "Changed a booking's status"),
	rule("POST", `/bookings/[^/]+/move$`, "Rescheduled a booking"),
	rule("POST", `/bookings/[^/]+/reschedule$`, "Rescheduled a booking"),
	rule("POST", `/bookings/reserve$`, "Created a booking"),
	rule("POST", `/bookings$`, "Created a booking"),
	rule("POST", `/invoices/[^/]+/payments$`, "Recorded a payment"),
	rule("POST", `/invoices/[^/]+/issue$`, "Issued an invoice"),
	rule("POST", `/invoices/[^/]+/void$`, "Voided an invoice"),
	rule("POST", `/invoices$`, "Created an invoice"),
	rule("PUT", `/invoices/[^/]+/draft$`, "Edited an invoice draft"),
	rule("POST", `/customers$`, "Added a customer"),
	rule("PUT", `/customers/[^/]+$`, "Edited a customer"),
	rule("DELETE", `/customers/[^/]+$`, "Deleted a customer"),
	rule("POST", `/quotes`, "Worked on a quotation"),
	rule("PUT", `/quotes`, "Edited a quotation"),
	rule("POST", `/central-services`, "Changed the central catalogue"),
	rule("PUT", `/central-services`, "Changed the central catalogue"),
	rule("DELETE", `/central-services`, "Changed the central catalogue"),
	rule("POST", `/services`, "Changed a service"),
	rule("PUT", `/services`, "Changed a service"),
	rule("DELETE", `/services`, "Removed a service"),
	rule("PUT", `/scheduling/`, "Changed scheduling"),
	rule("POST", `/scheduling/`, "Changed scheduling"),
	rule("PUT", `/invoice-template`, "Changed the invoice template"),
	rule("DELETE", `/invoice-template`, "Removed a franchise invoice template"),
	rule("PUT", `/logo$`, "Changed a franchise logo"),
	rule("PUT", `/website-tracking$`, "Changed website tracking"),
	rule("POST", `/website-enquiries/[^/]+/status$`, "Sorted a website enquiry"),
	rule("POST", `/calendar-feeds`, "Made a calendar link"),
	rule("DELETE", `/calendar-feeds`, "Turned off a calendar link"),
	rule("PUT", `/api/v1/account/password$`, "Changed their password"),
	rule("PUT", `/api/v1/account/email$`, "Changed their sign-in email"),
	rule("PUT", `/api/v1/account/profile$`, "Edited their details"),
	rule("POST", `/api/v1/account/mfa/`, "Changed two-factor sign-in"),
	rule("POST", `/api/v1/auth/logout$`, "Signed out"),
	rule("POST", `/roles`, "Changed roles"),
	rule("PUT", `/roles`, "Changed roles"),
	rule("DELETE", `/roles`, "Changed roles"),
	rule("POST", `/members`, "Changed a team member"),
	rule("PUT", `/members`, "Changed a team member"),
	rule("DELETE", `/members`, "Changed a team member"),
	rule("POST", `/invitations`, "Sent or changed an invitation"),
	rule("POST", `/employee/attendance/`, "Recorded attendance"),
	rule("PUT", `/employee/availability$`, "Submitted availability"),
	rule("POST", `/employee/walk-in$`, "Booked a walk-in"),
	rule("POST", `/employee/bookings/[^/]+/status$`, "Changed a walk-in booking's status"),
	rule("POST", `/employee/uploads`, "Posted photos"),
	rule("POST", `/sops$`, "Added a policy document"),
	rule("POST", `/custom-scripts`, "Added a website script"),
	rule("PUT", `/custom-scripts`, "Changed a website script"),
	rule("DELETE", `/custom-scripts`, "Deleted a website script"),
	rule("DELETE", `/sops/`, "Removed a policy document"),
}

func describeActivity(method, path string) string {
	for _, r := range activityRules {
		if r.method == method && r.re.MatchString(path) {
			return r.label
		}
	}
	return "Changed data"
}

// ActivityFilter narrows the log. Empty fields mean no limit.
type ActivityFilter struct {
	Kind, Query string
	From, To    time.Time
	Limit       int
}

// ActivityLog returns the newest entries first for administrators with access.manage. It covers sign-ins of members of the
// organization, failed attempts, and every change members made.
func (s *Store) ActivityLog(ctx context.Context, actor, org string, f ActivityFilter) ([]ActivityEntry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ok, err := actorHasOrgCapability(ctx, tx, actor, org, "access.manage")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 200
	}
	args := []any{org}
	where := `(a.principal_id IN (SELECT principal_id FROM organization_memberships WHERE organization_id=$1) OR a.principal_id IS NULL)`
	if f.Kind == "sign_in" || f.Kind == "sign_in_failed" || f.Kind == "write" {
		args = append(args, f.Kind)
		where += ` AND a.kind=$` + itoa(len(args))
	}
	if !f.From.IsZero() {
		args = append(args, f.From)
		where += ` AND a.at>=$` + itoa(len(args))
	}
	if !f.To.IsZero() {
		args = append(args, f.To)
		where += ` AND a.at<$` + itoa(len(args))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		n := itoa(len(args))
		where += ` AND (lower(a.email) LIKE $` + n + ` OR lower(COALESCE(mp.full_name,'')) LIKE $` + n + ` OR lower(c.email) LIKE $` + n + ` OR a.ip LIKE $` + n + ` OR lower(a.path) LIKE $` + n + `)`
	}
	rows, err := tx.Query(ctx, `SELECT a.id,a.at,COALESCE(mp.full_name,''),COALESCE(NULLIF(a.email,''),c.email,''),a.kind,a.method,a.path,a.status,a.ip,a.user_agent
 FROM activity_log a LEFT JOIN member_profiles mp ON mp.principal_id=a.principal_id LEFT JOIN password_credentials c ON c.principal_id=a.principal_id
 WHERE `+where+` ORDER BY a.at DESC, a.id DESC LIMIT `+itoa(f.Limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ActivityEntry{}
	for rows.Next() {
		var e ActivityEntry
		if err = rows.Scan(&e.ID, &e.At, &e.Person, &e.Email, &e.Kind, &e.Method, &e.Path, &e.Status, &e.IP, &e.UserAgent); err != nil {
			return nil, err
		}
		switch e.Kind {
		case "sign_in":
			e.What = "Signed in"
		case "sign_in_failed":
			e.What = "Failed sign-in attempt"
		default:
			e.What = describeActivity(e.Method, e.Path)
		}
		e.Path = uuidInPath.ReplaceAllString(e.Path, ":id")
		out = append(out, e)
	}
	return out, rows.Err()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

var _ = pgx.ErrNoRows
