package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The shop iPad (owner decisions 2026-11-11, error404 #15 and #20): one iPad per franchise, signed in with the franchise's own shop
// username and password. It lists the franchise's workers; a worker taps their name and types the 6-digit PIN the owner gave them, which
// opens their own employee dashboard on the iPad (a short session) with Clock in / Break / Clock out. Punches go through the same shift
// rules as before (one open shift, breaks only while clocked in), recorded as that person. Wrong PINs are limited by the workspace handler.

var (
	ErrClockPinInput      = errors.New("a 6-digit PIN for someone who records time at this franchise is required")
	ErrClockPinUnknown    = errors.New("that PIN does not match")
	ErrShopLoginInput     = errors.New("shop username or password not acceptable")
	ErrShopUsernameTaken  = errors.New("that shop username is already used")
	ErrShopSession        = errors.New("the shop iPad is not signed in")
	ErrShopWorkerSession  = errors.New("not a worker session opened on the shop iPad")
	ErrShopManagerPin     = errors.New("owners and administrators sign in with their own login, not a shop PIN")
	sixDigits             = regexp.MustCompile(`^[0-9]{6}$`)
	shopUsernamePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,39}$`)
	shopHashSlots         = make(chan struct{}, 2) // argon2 is memory-heavy; two at a time
	shopDummyHash         string
)

const (
	shopSessionLifetime   = 90 * 24 * time.Hour // the iPad stays signed in
	workerSessionLifetime = 30 * time.Minute    // the iPad page also signs the worker out after a short idle time
)

// clockPinHash ties a PIN to its franchise and person.
func clockPinHash(org, loc, principal, pin string) []byte {
	h := sha256.Sum256([]byte("dapper-shop-clock-v2\x00" + org + "\x00" + loc + "\x00" + principal + "\x00" + pin))
	return h[:]
}

func shopHash(password string) (string, error) {
	shopHashSlots <- struct{}{}
	defer func() { <-shopHashSlots }()
	return argon2id.CreateHash(password, passwordParams())
}

func shopCompare(password, hash string) bool {
	shopHashSlots <- struct{}{}
	defer func() { <-shopHashSlots }()
	ok, err := argon2id.ComparePasswordAndHash(password, hash)
	return err == nil && ok
}

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	h := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), h[:], nil
}

type ClockStaff struct {
	PrincipalID string `json:"principalId"`
	Name        string `json:"name"`
	HasPin      bool   `json:"hasPin"`
}

// The franchise's team as the Employees page shows it (error404 #48): a role that records time, assigned to this location or to its
// franchise. Organization-wide roles (owners, admins) and old direct grants do not put someone on the shop iPad.
const clockStaffQuery = `WITH staff AS (
 SELECT a.principal_id FROM member_role_assignments a JOIN role_capabilities rc ON rc.role_id=a.role_id AND rc.action='attendance.record'
 JOIN locations l ON l.organization_id=$1 AND l.id=$2
 WHERE a.organization_id=$1 AND ((a.scope_type='location' AND a.location_id=$2) OR (a.scope_type='franchise' AND a.franchise_id=l.franchise_id)))
SELECT p.id::text, COALESCE(NULLIF(btrim(mp.full_name),''), NULLIF(split_part(c.email,'@',1),''), 'Team member'), k.principal_id IS NOT NULL,
 COALESCE((SELECT r.status FROM employee_attendance r WHERE r.organization_id=$1 AND r.location_id=$2 AND r.principal_id=p.id AND r.clock_out_at IS NULL ORDER BY r.clock_in_at DESC LIMIT 1),'clocked_out')
FROM staff JOIN principals p ON p.id=staff.principal_id AND p.active AND p.external_subject NOT LIKE 'system:%'
LEFT JOIN member_profiles mp ON mp.principal_id=p.id LEFT JOIN password_credentials c ON c.principal_id=p.id
LEFT JOIN employee_clock_pins k ON k.organization_id=$1 AND k.location_id=$2 AND k.principal_id=p.id
ORDER BY 2 LIMIT 300`

// ClockStaffList is everyone who records time at the franchise, and whether they have a PIN yet (never the PIN itself).
func (s *Store) ClockStaffList(ctx context.Context, scope Scope) ([]ClockStaff, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.manage"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, clockStaffQuery, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ClockStaff{}
	for rows.Next() {
		var c ClockStaff
		var state string
		if err = rows.Scan(&c.PrincipalID, &c.Name, &c.HasPin, &state); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetClockPin gives a person their 6-digit PIN (replacing any old one). The person must be able to record time at this franchise.
func (s *Store) SetClockPin(ctx context.Context, scope Scope, principal, pin string) error {
	if !sixDigits.MatchString(pin) || principal == "" {
		return ErrClockPinInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.manage"); err != nil {
		return err
	}
	staff := Scope{ActorID: principal, OrganizationID: scope.OrganizationID, LocationID: scope.LocationID}
	if err = authorizeTransaction(ctx, tx, staff, "attendance.record"); err != nil {
		if errors.Is(err, ErrForbidden) || errors.Is(err, ErrScope) {
			return ErrClockPinInput
		}
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO employee_clock_pins(organization_id,location_id,principal_id,pin_hash,updated_by) VALUES($1,$2,$3,$4,$5)
ON CONFLICT (organization_id,location_id,principal_id) DO UPDATE SET pin_hash=EXCLUDED.pin_hash,updated_by=EXCLUDED.updated_by,updated_at=now()`,
		scope.OrganizationID, scope.LocationID, principal, clockPinHash(scope.OrganizationID, scope.LocationID, principal, pin), scope.ActorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearClockPin takes a person's PIN away (they can no longer open their dashboard on the shop iPad).
func (s *Store) ClearClockPin(ctx context.Context, scope Scope, principal string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.manage"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM employee_clock_pins WHERE organization_id=$1 AND location_id=$2 AND principal_id=$3`, scope.OrganizationID, scope.LocationID, principal); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ShopLogin is the franchise's shop username, if one is set (never the password).
func (s *Store) ShopLogin(ctx context.Context, scope Scope) (string, error) {
	if err := s.authorize(ctx, scope, "attendance.manage"); err != nil {
		return "", err
	}
	var username string
	err := s.pool.QueryRow(ctx, `SELECT username FROM shop_logins WHERE organization_id=$1 AND location_id=$2`, scope.OrganizationID, scope.LocationID).Scan(&username)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return username, err
}

// SaveShopLogin sets the franchise's shop username and password. Every iPad signed in with the old one is signed out.
func (s *Store) SaveShopLogin(ctx context.Context, scope Scope, username, password string) error {
	username = strings.ToLower(strings.TrimSpace(username))
	if !shopUsernamePattern.MatchString(username) || len(password) < 10 || len(password) > 200 {
		return ErrShopLoginInput
	}
	if err := s.authorize(ctx, scope, "attendance.manage"); err != nil {
		return err
	}
	hash, err := shopHash(password)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.manage"); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO shop_logins(organization_id,location_id,username,password_hash,updated_by) VALUES($1,$2,$3,$4,$5)
ON CONFLICT (organization_id,location_id) DO UPDATE SET username=EXCLUDED.username,password_hash=EXCLUDED.password_hash,updated_by=EXCLUDED.updated_by,updated_at=now()`,
		scope.OrganizationID, scope.LocationID, username, hash, scope.ActorID)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrShopUsernameTaken
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE shop_sessions SET revoked_at=now() WHERE organization_id=$1 AND location_id=$2 AND revoked_at IS NULL`, scope.OrganizationID, scope.LocationID); err != nil {
		return err
	}
	principal, err := ensureShopAccount(ctx, tx, scope)
	if err != nil {
		return err
	}
	// A new password signs the shop's workspace out everywhere too.
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE principal_id=$1 AND revoked_at IS NULL`, principal); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// shopSubject names a franchise's Shop account. "system:" keeps it out of staff lists (PINs, hours) like the website's own account.
func shopSubject(loc string) string { return "system:shop:" + loc }

// ensureShopAccount makes (or switches back on) the franchise's Shop account: the workspace account the shop login opens (owner,
// 2026-11-12: the shop sees every workspace page). It is a member of the business, works at this franchise, and has what the
// business's Employee role may do here (the built-in Employee list when there is no such role).
func ensureShopAccount(ctx context.Context, tx pgx.Tx, scope Scope) (string, error) {
	var principal string
	if err := tx.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) ON CONFLICT (external_subject) DO UPDATE SET active=true RETURNING id::text`, shopSubject(scope.LocationID)).Scan(&principal); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, scope.OrganizationID, principal); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO employee_worksite_assignments(principal_id,organization_id,location_id,dealership_id,timezone,assigned_by) VALUES($1,$2,$3,NULL,'America/Toronto',$4) ON CONFLICT (principal_id) DO NOTHING`, principal, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action)
 SELECT $1,$2,$3,a FROM unnest(coalesce((SELECT array_agg(rc.action) FROM roles r JOIN role_capabilities rc ON rc.role_id=r.id WHERE r.organization_id=$2 AND r.name='Employee'),
 ARRAY['attendance.read','bookings.read','bookings.create','bookings.manage','customers.read','customers.create','vehicles.read','vehicles.create','invoices.read','invoices.create','invoices.issue','payments.record','payments.read','scheduling.read','scheduling.reserve','catalogue.read','jobs.read','jobs.contribute','sop.read','sop.acknowledge'])) a
 WHERE a NOT IN ('attendance.record','attendance.manage','access.manage') ON CONFLICT DO NOTHING`, principal, scope.OrganizationID, scope.LocationID); err != nil {
		return "", err
	}
	return principal, nil
}

// ShopWorkspaceSession opens the workspace as the franchise's Shop account for a signed-in shop iPad (the longest a workspace session
// may last is 24 hours; the iPad asks again while its shop login lasts).
func (s *Store) ShopWorkspaceSession(ctx context.Context, d ShopDevice) (string, error) {
	var principal string
	err := s.pool.QueryRow(ctx, `SELECT id::text FROM principals WHERE external_subject=$1 AND active`, shopSubject(d.LocationID)).Scan(&principal)
	if errors.Is(err, pgx.ErrNoRows) {
		// A shop login saved before Shop accounts existed: make it now.
		tx, e := s.pool.Begin(ctx)
		if e != nil {
			return "", e
		}
		defer tx.Rollback(ctx)
		var owner string
		if e = tx.QueryRow(ctx, `SELECT updated_by::text FROM shop_logins WHERE organization_id=$1 AND location_id=$2`, d.OrganizationID, d.LocationID).Scan(&owner); e != nil {
			return "", ErrShopSession
		}
		if principal, e = ensureShopAccount(ctx, tx, Scope{ActorID: owner, OrganizationID: d.OrganizationID, LocationID: d.LocationID}); e != nil {
			return "", e
		}
		if e = tx.Commit(ctx); e != nil {
			return "", e
		}
	} else if err != nil {
		return "", err
	}
	return s.IssueSession(ctx, principal, 24*time.Hour)
}

// DeleteShopLogin removes the shop login and signs every iPad of the franchise out.
func (s *Store) DeleteShopLogin(ctx context.Context, scope Scope) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.manage"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM shop_logins WHERE organization_id=$1 AND location_id=$2`, scope.OrganizationID, scope.LocationID); err != nil {
		return err
	}
	// The Shop account stops working with its login (its sessions end; it is kept for the records it made).
	if _, err = tx.Exec(ctx, `UPDATE principals SET active=false WHERE external_subject=$1`, shopSubject(scope.LocationID)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE shop_sessions SET revoked_at=now() WHERE organization_id=$1 AND location_id=$2 AND revoked_at IS NULL`, scope.OrganizationID, scope.LocationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ShopSignIn checks a shop username and password and returns a new iPad session token.
func (s *Store) ShopSignIn(ctx context.Context, username, password string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(password) > 200 {
		return "", ErrShopSession
	}
	var org, loc, hash string
	err := s.pool.QueryRow(ctx, `SELECT organization_id::text,location_id::text,password_hash FROM shop_logins WHERE username=$1`, username).Scan(&org, &loc, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	found := err == nil
	if !found {
		if shopDummyHash == "" {
			shopDummyHash, _ = shopHash("dapper-shop-dummy-password")
		}
		hash = shopDummyHash // the same work either way, so timing does not reveal which usernames exist
	}
	if !shopCompare(password, hash) || !found {
		return "", ErrShopSession
	}
	token, digest, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO shop_sessions(token_hash,organization_id,location_id,expires_at) VALUES($1,$2,$3,now()+make_interval(secs=>$4))`, digest, org, loc, shopSessionLifetime.Seconds())
	return token, err
}

// ShopSignOut ends an iPad session.
func (s *Store) ShopSignOut(ctx context.Context, token string) error {
	digest, err := tokenDigest(token)
	if err != nil {
		return nil
	}
	_, err = s.pool.Exec(ctx, `UPDATE shop_sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, digest)
	return err
}

type ShopDevice struct {
	OrganizationID string `json:"-"`
	LocationID     string `json:"-"`
	Name           string `json:"name"`
}

// ShopDeviceOf is the franchise an iPad session belongs to; the shop login must still exist.
func (s *Store) ShopDeviceOf(ctx context.Context, token string) (ShopDevice, error) {
	digest, err := tokenDigest(token)
	if err != nil {
		return ShopDevice{}, ErrShopSession
	}
	var d ShopDevice
	err = s.pool.QueryRow(ctx, `SELECT s.organization_id::text,s.location_id::text,l.name FROM shop_sessions s JOIN shop_logins sl ON (sl.organization_id,sl.location_id)=(s.organization_id,s.location_id)
 JOIN locations l ON l.organization_id=s.organization_id AND l.id=s.location_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now()`, digest).Scan(&d.OrganizationID, &d.LocationID, &d.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShopDevice{}, ErrShopSession
	}
	return d, err
}

type ShopWorker struct {
	PrincipalID string `json:"principalId"`
	Name        string `json:"name"`
	Status      string `json:"status"` // clocked_in, on_break, clocked_out
}

// ShopWorkers is the iPad's name list: the franchise's workers who have a PIN.
func (s *Store) ShopWorkers(ctx context.Context, d ShopDevice) ([]ShopWorker, error) {
	rows, err := s.pool.Query(ctx, clockStaffQuery, d.OrganizationID, d.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShopWorker{}
	for rows.Next() {
		var w ShopWorker
		var hasPin bool
		if err = rows.Scan(&w.PrincipalID, &w.Name, &hasPin, &w.Status); err != nil {
			return nil, err
		}
		if hasPin {
			out = append(out, w)
		}
	}
	return out, rows.Err()
}

// ShopUnlock checks a worker's PIN on the iPad and opens a short session as them, marked as opened on this franchise's iPad.
// checkPin says whether a person's PIN at this franchise matches (ErrClockPinUnknown when it does not, or they have none).
func (s *Store) checkPin(ctx context.Context, d ShopDevice, principal, pin string) error {
	if !sixDigits.MatchString(pin) {
		return ErrClockPinUnknown
	}
	var stored []byte
	err := s.pool.QueryRow(ctx, `SELECT k.pin_hash FROM employee_clock_pins k JOIN principals p ON p.id=k.principal_id AND p.active WHERE k.organization_id=$1 AND k.location_id=$2 AND k.principal_id::text=$3`,
		d.OrganizationID, d.LocationID, principal).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrClockPinUnknown
	}
	if err != nil {
		return err
	}
	if string(stored) != string(clockPinHash(d.OrganizationID, d.LocationID, principal, pin)) {
		return ErrClockPinUnknown
	}
	return nil
}

func (s *Store) ShopUnlock(ctx context.Context, d ShopDevice, principal, pin string) (string, error) {
	err := s.checkPin(ctx, d, principal, pin)
	if err != nil {
		return "", err
	}
	// A 6-digit PIN on a shared iPad must never open an owner's or administrator's dashboard (or skip someone's two-factor sign-in).
	var manager bool
	if err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_grants WHERE principal_id::text=$1 AND organization_id=$2 AND action='access.manage')
 OR EXISTS(SELECT 1 FROM member_mfa WHERE principal_id::text=$1 AND enabled_at IS NOT NULL)`, principal, d.OrganizationID).Scan(&manager); err != nil {
		return "", err
	}
	if manager {
		return "", ErrShopManagerPin
	}
	token, err := s.IssueSession(ctx, principal, workerSessionLifetime)
	if err != nil {
		return "", err
	}
	digest, _ := tokenDigest(token)
	if _, err = s.pool.Exec(ctx, `UPDATE auth_sessions SET shop_location_id=$2 WHERE token_hash=$1`, digest, d.LocationID); err != nil {
		return "", err
	}
	return token, nil
}

type ShopClockState struct {
	Name  string            `json:"name"`
	Shop  string            `json:"shop"`
	Shift *AttendanceRecord `json:"shift"`
}

// ShopPunch records a punch (or with "status" just reads the shift) for the worker whose session the shop iPad opened.
func (s *Store) ShopPunch(ctx context.Context, sessionToken, action string) (ShopClockState, error) {
	digest, err := tokenDigest(sessionToken)
	if err != nil {
		return ShopClockState{}, ErrShopWorkerSession
	}
	var principal, org, loc string
	out := ShopClockState{}
	err = s.pool.QueryRow(ctx, `SELECT a.principal_id::text,l.organization_id::text,l.id::text,l.name,COALESCE(NULLIF(btrim(mp.full_name),''),'') FROM auth_sessions a
 JOIN locations l ON l.id=a.shop_location_id JOIN principals p ON p.id=a.principal_id AND p.active LEFT JOIN member_profiles mp ON mp.principal_id=p.id
 WHERE a.token_hash=$1 AND a.revoked_at IS NULL AND a.expires_at>now() AND a.shop_location_id IS NOT NULL`, digest).Scan(&principal, &org, &loc, &out.Shop, &out.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShopClockState{}, ErrShopWorkerSession
	}
	if err != nil {
		return ShopClockState{}, err
	}
	scope := Scope{ActorID: principal, OrganizationID: org, LocationID: loc}
	var rec AttendanceRecord
	switch action {
	case "status":
		out.Shift, err = s.ActiveAttendance(ctx, scope)
		return out, err
	case "clock-in":
		rec, err = s.ClockIn(ctx, scope, "")
	case "start-break":
		rec, err = s.StartBreak(ctx, scope, "")
	case "end-break":
		rec, err = s.EndBreak(ctx, scope, "")
	case "clock-out":
		rec, err = s.ClockOut(ctx, scope, "")
	default:
		return ShopClockState{}, ErrAttendanceInput
	}
	if err != nil {
		return out, err
	}
	out.Shift = &rec
	return out, nil
}

// ShopRelease ends a worker session the shop iPad opened (Done, idle, or the next person). It never touches a session signed in another way.
func (s *Store) ShopRelease(ctx context.Context, sessionToken string) error {
	digest, err := tokenDigest(sessionToken)
	if err != nil {
		return nil
	}
	_, err = s.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1 AND shop_location_id IS NOT NULL`, digest)
	return err
}

// ShopPinPunch is the workspace's clock page on the shop iPad (owner, 2026-11-12): the worker taps their name and types their PIN, and
// the punch (or with "status", just their shift) is recorded as them, without opening their dashboard. Owners may punch this way too.
func (s *Store) ShopPinPunch(ctx context.Context, d ShopDevice, principal, pin, action string) (ShopClockState, error) {
	if err := s.checkPin(ctx, d, principal, pin); err != nil {
		return ShopClockState{}, err
	}
	out := ShopClockState{Shop: d.Name}
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(NULLIF(btrim(full_name),''),'') FROM member_profiles WHERE principal_id::text=$1`, principal).Scan(&out.Name)
	scope := Scope{ActorID: principal, OrganizationID: d.OrganizationID, LocationID: d.LocationID}
	var rec AttendanceRecord
	var err error
	switch action {
	case "status":
		out.Shift, err = s.ActiveAttendance(ctx, scope)
		return out, err
	case "clock-in":
		rec, err = s.ClockIn(ctx, scope, "")
	case "start-break":
		rec, err = s.StartBreak(ctx, scope, "")
	case "end-break":
		rec, err = s.EndBreak(ctx, scope, "")
	case "clock-out":
		rec, err = s.ClockOut(ctx, scope, "")
	default:
		return ShopClockState{}, ErrAttendanceInput
	}
	if err != nil {
		return out, err
	}
	out.Shift = &rec
	return out, nil
}

// ShopDeviceForStaff lets anyone signed in to the workspace use the clock page (owner, 2026-11-12): the device is the franchise the
// person works at (their worksite assignment). It does not open anyone's dashboard; that stays with the shop login.
func (s *Store) ShopDeviceForStaff(ctx context.Context, sessionToken string) (ShopDevice, error) {
	principal, err := s.SessionPrincipal(ctx, sessionToken)
	if err != nil {
		return ShopDevice{}, ErrShopSession
	}
	var d ShopDevice
	err = s.pool.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text,l.name FROM employee_worksite_assignments a JOIN locations l ON l.organization_id=a.organization_id AND l.id=a.location_id WHERE a.principal_id::text=$1`, principal).Scan(&d.OrganizationID, &d.LocationID, &d.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShopDevice{}, ErrShopSession
	}
	return d, err
}

// ShopAvailability reads (input nil) or submits a worker's availability on the shop iPad after their PIN (owner, 2026-11-12: "the same
// system for availability").
func (s *Store) ShopAvailability(ctx context.Context, d ShopDevice, principal, pin, week string, input *EmployeeAvailability) (EmployeeAvailability, error) {
	if err := s.checkPin(ctx, d, principal, pin); err != nil {
		return EmployeeAvailability{}, err
	}
	if input == nil {
		return s.EmployeeAvailabilityFor(ctx, principal, week)
	}
	return s.SubmitEmployeeAvailability(ctx, principal, *input)
}
