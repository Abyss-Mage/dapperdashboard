package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrMFARequired       = errors.New("an authentication code is required")
	ErrMFAInvalid        = errors.New("that authentication code is incorrect")
	ErrMFAUnavailable    = errors.New("two-factor authentication is not configured on the server")
	ErrMFAState          = errors.New("two-factor authentication is already set up, or setup has not been started")
	ErrMFAAdminRequired  = errors.New("administrators must keep two-factor authentication on")
	ErrProfileInput      = errors.New("enter your full name and an international phone number such as +14165550123")
	mfaSecretLen         = 20
	mfaRecoveryCodeCount = 10
	mfaIssuer            = "Dapper Detailing"
)

// SetMFAKey installs the 32-byte server key that encrypts authenticator secrets. Without it MFA cannot be set up,
// and (to avoid locking people out) it also cannot be enforced.
func (s *Store) SetMFAKey(key []byte) error {
	if len(key) != 32 {
		return ErrMFAUnavailable
	}
	s.mfaKey = append([]byte(nil), key...)
	return nil
}
func (s *Store) MFAConfigured() bool { return len(s.mfaKey) == 32 }

func (s *Store) sealSecret(principal string, secret []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.mfaKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, secret, []byte(principal)), nil
}
func (s *Store) openSecret(principal string, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.mfaKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(sealed) <= gcm.NonceSize() {
		return nil, ErrMFAUnavailable
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], []byte(principal))
}

// totpAt is RFC 6238 (SHA-1, 30-second step, 6 digits), the profile every authenticator app supports.
func totpAt(secret []byte, step int64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000)
}

// matchTOTP accepts the current step and one either side (clock drift) and returns the matched step.
func matchTOTP(secret []byte, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return 0, false
	}
	current := now.Unix() / 30
	matched, found := int64(0), false
	for _, step := range []int64{current - 1, current, current + 1} {
		if subtle.ConstantTimeCompare([]byte(totpAt(secret, step)), []byte(code)) == 1 {
			matched, found = step, true
		}
	}
	return matched, found
}

func recoveryHash(code string) [32]byte {
	return sha256.Sum256([]byte(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))))
}

type MFAStatus struct {
	Configured        bool `json:"configured"` // the server can do MFA at all
	Enabled           bool `json:"enabled"`
	SetupStarted      bool `json:"setupStarted"`
	Required          bool `json:"required"` // administrators must have it
	RecoveryRemaining int  `json:"recoveryRemaining"`
}

// isAdministrator means holding any organization-level grant (structure, access, finance configuration...).
func isAdministrator(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, principal string) (bool, error) {
	var admin bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_grants WHERE principal_id=$1)`, principal).Scan(&admin)
	return admin, err
}

func (s *Store) MFAStatus(ctx context.Context, principal string) (MFAStatus, error) {
	out := MFAStatus{Configured: s.MFAConfigured()}
	var enabled *time.Time
	err := s.pool.QueryRow(ctx, `SELECT enabled_at FROM member_mfa WHERE principal_id=$1`, principal).Scan(&enabled)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	out.Enabled = err == nil && enabled != nil
	out.SetupStarted = err == nil && enabled == nil
	// Two-factor authentication is optional for everyone, administrators included (out.Required stays false).
	err = nil
	if out.Enabled {
		err = s.pool.QueryRow(ctx, `SELECT count(*) FROM member_mfa_recovery_codes WHERE principal_id=$1 AND used_at IS NULL`, principal).Scan(&out.RecoveryRemaining)
	}
	return out, err
}

// MFASetupBlocks reports whether this person must set up MFA before doing anything else: an administrator without it.
// It is false when the server has no MFA key, so a missing key can never lock administrators out.
func (s *Store) MFASetupBlocks(ctx context.Context, principal string) (bool, error) {
	if !s.MFAConfigured() {
		return false, nil
	}
	st, err := s.MFAStatus(ctx, principal)
	return st.Required && !st.Enabled, err
}

// BeginMFASetup creates (or replaces) a pending authenticator secret and returns it for the person to scan or type.
func (s *Store) BeginMFASetup(ctx context.Context, principal string) (secret, uri string, err error) {
	if !s.MFAConfigured() {
		return "", "", ErrMFAUnavailable
	}
	var email string
	if err = s.pool.QueryRow(ctx, `SELECT COALESCE(NULLIF(c.email,''),p.external_subject) FROM principals p LEFT JOIN password_credentials c ON c.principal_id=p.id WHERE p.id=$1 AND p.active`, principal).Scan(&email); err != nil {
		return "", "", err
	}
	raw := make([]byte, mfaSecretLen)
	if _, err = rand.Read(raw); err != nil {
		return "", "", err
	}
	sealed, err := s.sealSecret(principal, raw)
	if err != nil {
		return "", "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO member_mfa(principal_id,secret_ciphertext) VALUES($1,$2) ON CONFLICT(principal_id) DO UPDATE SET secret_ciphertext=excluded.secret_ciphertext,created_at=clock_timestamp() WHERE member_mfa.enabled_at IS NULL`, principal, sealed)
	if err != nil {
		return "", "", err
	}
	if tag.RowsAffected() == 0 {
		return "", "", ErrMFAState
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_security_events(principal_id,action) VALUES($1,'mfa_setup_started')`, principal); err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	label := url.PathEscape(mfaIssuer + ":" + email)
	uri = fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30", label, secret, url.QueryEscape(mfaIssuer))
	return secret, uri, nil
}

func newRecoveryCode() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	text := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)[:10]
	return text[:5] + "-" + text[5:], nil
}

// ConfirmMFASetup checks the first code from the app, turns MFA on, and returns the recovery codes ONCE.
func (s *Store) ConfirmMFASetup(ctx context.Context, principal, code string) ([]string, error) {
	if !s.MFAConfigured() {
		return nil, ErrMFAUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var sealed []byte
	var enabled *time.Time
	err = tx.QueryRow(ctx, `SELECT secret_ciphertext,enabled_at FROM member_mfa WHERE principal_id=$1 FOR UPDATE`, principal).Scan(&sealed, &enabled)
	if errors.Is(err, pgx.ErrNoRows) || enabled != nil {
		return nil, ErrMFAState
	}
	if err != nil {
		return nil, err
	}
	secret, err := s.openSecret(principal, sealed)
	if err != nil {
		return nil, err
	}
	step, ok := matchTOTP(secret, code, time.Now())
	if !ok {
		return nil, ErrMFAInvalid
	}
	if _, err = tx.Exec(ctx, `UPDATE member_mfa SET enabled_at=clock_timestamp(),last_used_step=$2 WHERE principal_id=$1`, principal, step); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM member_mfa_recovery_codes WHERE principal_id=$1`, principal); err != nil {
		return nil, err
	}
	codes := make([]string, 0, mfaRecoveryCodeCount)
	for i := 0; i < mfaRecoveryCodeCount; i++ {
		c, e := newRecoveryCode()
		if e != nil {
			return nil, e
		}
		h := recoveryHash(c)
		if _, err = tx.Exec(ctx, `INSERT INTO member_mfa_recovery_codes(principal_id,code_hash) VALUES($1,$2)`, principal, h[:]); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_security_events(principal_id,action) VALUES($1,'mfa_enabled')`, principal); err != nil {
		return nil, err
	}
	return codes, tx.Commit(ctx)
}

// VerifyMFALogin checks an app code (with replay protection) or a one-time recovery code for a person with MFA on.
// It returns (false, nil) when the person has no MFA, so callers can skip the second step.
func (s *Store) VerifyMFALogin(ctx context.Context, principal, code string) (required bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var sealed []byte
	var enabled *time.Time
	var last int64
	err = tx.QueryRow(ctx, `SELECT secret_ciphertext,enabled_at,last_used_step FROM member_mfa WHERE principal_id=$1 FOR UPDATE`, principal).Scan(&sealed, &enabled, &last)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && enabled == nil) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	if !s.MFAConfigured() {
		return true, ErrMFAUnavailable
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return true, ErrMFARequired
	}
	secret, err := s.openSecret(principal, sealed)
	if err != nil {
		return true, err
	}
	if step, ok := matchTOTP(secret, code, time.Now()); ok {
		if step <= last {
			return true, ErrMFAInvalid // the same code cannot be used twice
		}
		if _, err = tx.Exec(ctx, `UPDATE member_mfa SET last_used_step=$2 WHERE principal_id=$1`, principal, step); err != nil {
			return true, err
		}
		return true, tx.Commit(ctx)
	}
	h := recoveryHash(code)
	tag, err := tx.Exec(ctx, `UPDATE member_mfa_recovery_codes SET used_at=clock_timestamp() WHERE principal_id=$1 AND code_hash=$2 AND used_at IS NULL`, principal, h[:])
	if err != nil {
		return true, err
	}
	if tag.RowsAffected() != 1 {
		return true, ErrMFAInvalid
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_security_events(principal_id,action) VALUES($1,'mfa_recovery_used')`, principal); err != nil {
		return true, err
	}
	return true, tx.Commit(ctx)
}

// DisableMFA turns MFA off after a valid code.
func (s *Store) DisableMFA(ctx context.Context, principal, code string) error {
	st, err := s.MFAStatus(ctx, principal)
	if err != nil {
		return err
	}
	if !st.Enabled {
		return ErrMFAState
	}
	if _, err = s.VerifyMFALogin(ctx, principal, code); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{`DELETE FROM member_mfa_recovery_codes WHERE principal_id=$1`, `DELETE FROM member_mfa WHERE principal_id=$1`} {
		if _, err = tx.Exec(ctx, q, principal); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_security_events(principal_id,action) VALUES($1,'mfa_disabled')`, principal); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Profile struct {
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Complete bool   `json:"complete"`
}

func (s *Store) GetProfile(ctx context.Context, principal string) (Profile, error) {
	var p Profile
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(NULLIF(c.email,''),''),COALESCE(m.full_name,''),COALESCE(m.phone,'') FROM principals p LEFT JOIN password_credentials c ON c.principal_id=p.id LEFT JOIN member_profiles m ON m.principal_id=p.id WHERE p.id=$1 AND p.active`, principal).Scan(&p.Email, &p.FullName, &p.Phone)
	p.Complete = p.FullName != "" && p.Phone != ""
	return p, err
}

// SaveProfile stores the person's own name and phone. The phone is checked for format only (international form);
// no contact verification is performed.
func (s *Store) SaveProfile(ctx context.Context, principal, fullName, phone string) (Profile, error) {
	fullName = strings.Join(strings.Fields(fullName), " ")
	phone = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(strings.TrimSpace(phone))
	if n := utf8.RuneCountInString(fullName); n < 2 || n > 120 || !employeePhone.MatchString(phone) {
		return Profile{}, ErrProfileInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO member_profiles(principal_id,full_name,phone) SELECT id,$2,$3 FROM principals WHERE id=$1 AND active ON CONFLICT(principal_id) DO UPDATE SET full_name=excluded.full_name,phone=excluded.phone,updated_at=clock_timestamp()`, principal, fullName, phone)
	if err != nil {
		return Profile{}, err
	}
	if tag.RowsAffected() != 1 {
		return Profile{}, ErrForbidden
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_security_events(principal_id,action) VALUES($1,'profile_saved')`, principal); err != nil {
		return Profile{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Profile{}, err
	}
	return s.GetProfile(ctx, principal)
}

type OnboardingStatus struct {
	Profile          Profile   `json:"profile"`
	MFA              MFAStatus `json:"mfa"`
	Assigned         bool      `json:"assigned"`
	DocumentsTotal   int       `json:"documentsTotal"`
	DocumentsPending int       `json:"documentsPending"`
	DocumentsMissing int       `json:"documentsUnavailable"`
	Complete         bool      `json:"complete"`
}

// OnboardingStatus says what is left before someone can use the workspace: their details, MFA for administrators,
// and every published document read and acknowledged (documents need a franchise assignment first).
func (s *Store) OnboardingStatus(ctx context.Context, principal string) (OnboardingStatus, error) {
	var out OnboardingStatus
	var err error
	if out.Profile, err = s.GetProfile(ctx, principal); err != nil {
		return out, err
	}
	if out.MFA, err = s.MFAStatus(ctx, principal); err != nil {
		return out, err
	}
	docs, err := s.EmployeeDocuments(ctx, principal)
	switch {
	case errors.Is(err, ErrEmployeeAssignment):
		out.Assigned = false
	case err != nil:
		return out, err
	default:
		out.Assigned = true
		for _, d := range docs {
			out.DocumentsTotal++
			if !d.FileAvailable {
				out.DocumentsMissing++
			} else if d.AcknowledgedAt == nil {
				out.DocumentsPending++
			}
		}
	}
	mfaOK := !out.MFA.Required || out.MFA.Enabled || !out.MFA.Configured
	out.Complete = out.Profile.Complete && mfaOK && out.DocumentsPending == 0
	return out, nil
}
