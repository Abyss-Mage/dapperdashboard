package postgres

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTOTPMatchesRFC6238Vectors(t *testing.T) {
	secret := []byte("12345678901234567890") // RFC 6238 appendix B, SHA-1
	for _, c := range []struct {
		unix int64
		want string
	}{{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"}, {1234567890, "005924"}, {2000000000, "279037"}} {
		if got := totpAt(secret, c.unix/30); got != c.want {
			t.Fatalf("T=%d: got %s want %s", c.unix, got, c.want)
		}
	}
	if _, ok := matchTOTP(secret, "287082", time.Unix(59, 0)); !ok {
		t.Fatal("a correct code was rejected")
	}
	if _, ok := matchTOTP(secret, "287082", time.Unix(59+300, 0)); ok {
		t.Fatal("a code from five minutes ago was accepted")
	}
	if _, ok := matchTOTP(secret, "12345", time.Unix(59, 0)); ok {
		t.Fatal("a short code was accepted")
	}
}

func TestAccountSecurityLifecycle(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_acct_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		_, _ = admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
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
	auth, err := NewPasswordAuthenticator(s)
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var org, employee, boss string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic MFA Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:mfa-employee') RETURNING id::text`).Scan(&employee))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:mfa-admin') RETURNING id::text`).Scan(&boss))
	_, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'structure.read')`, org, boss)
	must(err)
	const password = "synthetic-password-1234"
	must(auth.SetPassword(ctx, employee, "mfa-employee@example.invalid", password))
	must(auth.SetPassword(ctx, boss, "mfa-admin@example.invalid", password))

	// Without a server key MFA cannot start and is never enforced (no lock-out).
	if _, _, err = s.BeginMFASetup(ctx, employee); !errors.Is(err, ErrMFAUnavailable) {
		t.Fatal("MFA started without a server key", err)
	}
	if blocked, err := s.MFASetupBlocks(ctx, boss); err != nil || blocked {
		t.Fatal("an administrator was blocked although the server has no MFA key", err)
	}
	must(s.SetMFAKey([]byte("0123456789abcdef0123456789abcdef")))
	if blocked, err := s.MFASetupBlocks(ctx, boss); err != nil || blocked {
		t.Fatal("MFA must be optional for administrators too", err)
	}
	if blocked, _ := s.MFASetupBlocks(ctx, employee); blocked {
		t.Fatal("MFA must be optional for employees")
	}

	// Setup: wrong code refused, right code enables and returns ten recovery codes once.
	secretText, uri, err := s.BeginMFASetup(ctx, employee)
	must(err)
	if uri == "" || secretText == "" {
		t.Fatal("no provisioning data")
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secretText)
	must(err)
	if _, err = s.ConfirmMFASetup(ctx, employee, "000000"); !errors.Is(err, ErrMFAInvalid) && totpAt(secret, time.Now().Unix()/30) != "000000" {
		t.Fatal("a wrong setup code was accepted", err)
	}
	step := time.Now().Unix() / 30
	codes, err := s.ConfirmMFASetup(ctx, employee, totpAt(secret, step))
	if err != nil || len(codes) != 10 {
		t.Fatalf("setup confirmation failed: %d codes, %v", len(codes), err)
	}
	if st, _ := s.MFAStatus(ctx, employee); !st.Enabled || st.RecoveryRemaining != 10 || st.Required {
		t.Fatalf("status after setup %+v", st)
	}
	if _, _, err = s.BeginMFASetup(ctx, employee); !errors.Is(err, ErrMFAState) {
		t.Fatal("setup restarted while MFA is on", err)
	}
	var stored []byte
	must(pool.QueryRow(ctx, `SELECT secret_ciphertext FROM member_mfa WHERE principal_id=$1`, employee).Scan(&stored))
	if string(stored) == string(secret) || len(stored) <= 28 {
		t.Fatal("the authenticator secret is not encrypted at rest")
	}

	// Sign-in: password alone is not enough; wrong and replayed codes fail; a fresh code and a recovery code work once.
	if _, err = auth.SignInWithCode(ctx, "mfa-employee@example.invalid", password, "", time.Hour); !errors.Is(err, ErrMFARequired) {
		t.Fatal("sign-in without a code succeeded", err)
	}
	var sessions int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM auth_sessions WHERE principal_id=$1`, employee).Scan(&sessions))
	if sessions != 0 {
		t.Fatal("a refused sign-in left a session")
	}
	if _, err = auth.SignInWithCode(ctx, "mfa-employee@example.invalid", "wrong-password-12345", totpAt(secret, step), time.Hour); !errors.Is(err, ErrSession) {
		t.Fatal("a wrong password with a valid code signed in", err)
	}
	if _, err = auth.SignInWithCode(ctx, "mfa-employee@example.invalid", password, totpAt(secret, step), time.Hour); !errors.Is(err, ErrMFAInvalid) {
		t.Fatal("the setup code was accepted a second time", err)
	}
	if token, err := auth.SignInWithCode(ctx, "mfa-employee@example.invalid", password, totpAt(secret, step+1), time.Hour); err != nil || token == "" {
		t.Fatal("a fresh code was refused", err)
	}
	if _, err = auth.SignInWithCode(ctx, "mfa-employee@example.invalid", password, totpAt(secret, step+1), time.Hour); !errors.Is(err, ErrMFAInvalid) {
		t.Fatal("a code was accepted twice", err)
	}
	if token, err := auth.SignInWithCode(ctx, "mfa-employee@example.invalid", password, codes[0], time.Hour); err != nil || token == "" {
		t.Fatal("a recovery code was refused", err)
	}
	if _, err = auth.SignInWithCode(ctx, "mfa-employee@example.invalid", password, codes[0], time.Hour); !errors.Is(err, ErrMFAInvalid) {
		t.Fatal("a recovery code worked twice", err)
	}
	if st, _ := s.MFAStatus(ctx, employee); st.RecoveryRemaining != 9 {
		t.Fatalf("recovery codes remaining %d", st.RecoveryRemaining)
	}
	// Accounts without MFA still sign in with the password only.
	if token, err := auth.SignInWithCode(ctx, "mfa-admin@example.invalid", password, "", time.Hour); err != nil || token == "" {
		t.Fatal("a password-only account could not sign in", err)
	}

	// MFA is optional for everyone; turning it off needs a valid code. An administrator who never turned it on has nothing to turn off.
	if err = s.DisableMFA(ctx, boss, "123456"); !errors.Is(err, ErrMFAState) {
		t.Fatal("disabling MFA that is not on should be a state error", err)
	}
	if err = s.DisableMFA(ctx, employee, "not-a-code"); !errors.Is(err, ErrMFAInvalid) {
		t.Fatal("MFA disabled without a valid code", err)
	}
	if err = s.DisableMFA(ctx, employee, codes[1]); err != nil {
		t.Fatal("employee could not turn MFA off", err)
	}
	if st, _ := s.MFAStatus(ctx, employee); st.Enabled {
		t.Fatal("MFA still on")
	}

	// Profile: international phone, normalised; format only.
	p, err := s.SaveProfile(ctx, employee, "  Synthetic   Employee ", "+1 (416) 555-0123")
	if err != nil || p.FullName != "Synthetic Employee" || p.Phone != "+14165550123" || !p.Complete || p.Email != "mfa-employee@example.invalid" {
		t.Fatalf("profile %+v %v", p, err)
	}
	for _, bad := range [][2]string{{"X", "+14165550123"}, {"Synthetic Employee", "4165550123"}, {"Synthetic Employee", "+0123"}, {"", ""}} {
		if _, err = s.SaveProfile(ctx, employee, bad[0], bad[1]); !errors.Is(err, ErrProfileInput) {
			t.Fatalf("invalid profile %v accepted: %v", bad, err)
		}
	}
	status, err := s.OnboardingStatus(ctx, employee)
	if err != nil || status.Assigned || !status.Profile.Complete || !status.Complete {
		t.Fatalf("onboarding for an unassigned employee with details %+v %v", status, err)
	}
	if status, err = s.OnboardingStatus(ctx, boss); err != nil || status.Complete || status.MFA.Required {
		t.Fatalf("an administrator without details/MFA must not be complete %+v %v", status, err)
	}

	// Change password: the current one is required, the new one must be long enough, and every session ends.
	if err = auth.ChangePassword(ctx, boss, "wrong-password-0000", "a-brand-new-password-1"); !errors.Is(err, ErrPasswordWrong) {
		t.Fatal("password changed without the current one", err)
	}
	if err = auth.ChangePassword(ctx, boss, password, "short"); !errors.Is(err, ErrPasswordPolicy) {
		t.Fatal("a short password was accepted", err)
	}
	oldToken, err := auth.SignIn(ctx, "mfa-admin@example.invalid", password, time.Hour)
	must(err)
	must(auth.ChangePassword(ctx, boss, password, "a-brand-new-password-1"))
	if _, err = s.SessionPrincipal(ctx, oldToken); err == nil {
		t.Fatal("an old session survived a password change")
	}
	if _, err = auth.SignIn(ctx, "mfa-admin@example.invalid", password, time.Hour); err == nil {
		t.Fatal("the old password still works")
	}
	if _, err = auth.SignIn(ctx, "mfa-admin@example.invalid", "a-brand-new-password-1", time.Hour); err != nil {
		t.Fatal("the new password does not work", err)
	}

	// Change email: password required, address validated and unique; the new address signs in.
	if err = auth.ChangeEmail(ctx, boss, "wrong-password-0000", "new-admin@example.invalid"); !errors.Is(err, ErrPasswordWrong) {
		t.Fatal("email changed without the password", err)
	}
	if err = auth.ChangeEmail(ctx, boss, "a-brand-new-password-1", "not-an-email"); !errors.Is(err, ErrEmailInvalid) {
		t.Fatal("an invalid email was accepted", err)
	}
	if err = auth.ChangeEmail(ctx, boss, "a-brand-new-password-1", "mfa-employee@example.invalid"); !errors.Is(err, ErrEmailTaken) {
		t.Fatal("an email used by someone else was accepted", err)
	}
	must(auth.ChangeEmail(ctx, boss, "a-brand-new-password-1", "New-Admin@example.invalid"))
	if _, err = auth.SignIn(ctx, "new-admin@example.invalid", "a-brand-new-password-1", time.Hour); err != nil {
		t.Fatal("the new email does not sign in", err)
	}

	// Forgot password: unknown address gives nothing; a real one gives a one-time link token that sets a new password.
	if token, err := auth.RequestPasswordReset(ctx, "nobody@example.invalid"); err != nil || token != "" {
		t.Fatal("an unknown address produced a reset token", token, err)
	}
	resetToken, err := auth.RequestPasswordReset(ctx, "new-admin@example.invalid")
	if err != nil || resetToken == "" {
		t.Fatal("no reset token for a real account", err)
	}
	if again, _ := auth.RequestPasswordReset(ctx, "new-admin@example.invalid"); again != "" {
		t.Fatal("reset requests were not rate limited per account")
	}
	if err = auth.ConfirmPasswordReset(ctx, "not-a-token", "yet-another-password-22"); !errors.Is(err, ErrResetTokenBad) {
		t.Fatal("a garbage reset token was accepted", err)
	}
	if err = auth.ConfirmPasswordReset(ctx, resetToken, "short"); !errors.Is(err, ErrPasswordPolicy) {
		t.Fatal("a short reset password was accepted", err)
	}
	must(auth.ConfirmPasswordReset(ctx, resetToken, "yet-another-password-22"))
	if err = auth.ConfirmPasswordReset(ctx, resetToken, "one-more-password-333"); !errors.Is(err, ErrResetTokenBad) {
		t.Fatal("a reset link worked twice", err)
	}
	if _, err = auth.SignIn(ctx, "new-admin@example.invalid", "yet-another-password-22", time.Hour); err != nil {
		t.Fatal("the reset password does not work", err)
	}
}
