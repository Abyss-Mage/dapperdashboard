package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrPasswordWrong   = errors.New("the current password is not correct")
	ErrPasswordPolicy  = errors.New("the new password must be at least 15 characters")
	ErrEmailInvalid    = errors.New("enter a valid email address")
	ErrEmailTaken      = errors.New("that email address is already used by another account")
	ErrResetTokenBad   = errors.New("this reset link is invalid or has expired")
	resetTokenLifetime = time.Hour
)

const passwordMinLength = 15

func (a *PasswordAuthenticator) acquire() (func(), error) {
	select {
	case a.slots <- struct{}{}:
		return func() { <-a.slots }, nil
	default:
		return nil, ErrAuthenticationBusy
	}
}

func (a *PasswordAuthenticator) event(ctx context.Context, tx pgx.Tx, principal, action string) error {
	_, err := tx.Exec(ctx, `INSERT INTO member_security_events(principal_id,action) VALUES($1,$2)`, principal, action)
	return err
}

// verifyCurrent checks the person's own password. It returns the stored hash so the caller can guard the update with it.
func (a *PasswordAuthenticator) verifyCurrent(ctx context.Context, principal, password string) (string, error) {
	var hash string
	if err := a.store.pool.QueryRow(ctx, `SELECT password_hash FROM password_credentials WHERE principal_id=$1`, principal).Scan(&hash); err != nil {
		return "", ErrPasswordWrong
	}
	if ok, err := argon2id.ComparePasswordAndHash(password, hash); err != nil || !ok {
		return "", ErrPasswordWrong
	}
	return hash, nil
}

// ChangePassword replaces a signed-in person's password after checking the current one. Every session ends, so they sign in again.
func (a *PasswordAuthenticator) ChangePassword(ctx context.Context, principal, current, next string) error {
	if len(next) < passwordMinLength || len(next) > 1024 || len(current) > 1024 {
		return ErrPasswordPolicy
	}
	release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	old, err := a.verifyCurrent(ctx, principal, current)
	if err != nil {
		return err
	}
	hash, err := argon2id.CreateHash(next, passwordParams())
	if err != nil {
		return err
	}
	tx, err := a.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE password_credentials SET password_hash=$2,changed_at=now() WHERE principal_id=$1 AND password_hash=$3`, principal, hash, old)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrPasswordWrong
	}
	if err = a.event(ctx, tx, principal, "password_changed"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ChangeEmail changes the sign-in email after checking the password. Every session ends, so they sign in again with the new address.
func (a *PasswordAuthenticator) ChangeEmail(ctx context.Context, principal, password, email string) error {
	email, valid := normalizedEmail(email)
	if !valid {
		return ErrEmailInvalid
	}
	release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	old, err := a.verifyCurrent(ctx, principal, password)
	if err != nil {
		return err
	}
	tx, err := a.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE password_credentials SET email=$2 WHERE principal_id=$1 AND password_hash=$3 AND email<>$2`, principal, email, old)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrEmailTaken
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrEmailInvalid
	}
	if err = a.event(ctx, tx, principal, "email_changed"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RequestPasswordReset creates a one-hour, single-use token for the account with this email and returns it for the caller to email.
// An unknown address, or a request made within two minutes of the last, returns "" with no error, so the answer never reveals
// whether an account exists.
func (a *PasswordAuthenticator) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	email, valid := normalizedEmail(email)
	if !valid {
		return "", nil
	}
	var principal string
	err := a.store.pool.QueryRow(ctx, `SELECT p.id::text FROM password_credentials c JOIN principals p ON p.id=c.principal_id WHERE c.email=$1 AND p.active`, email).Scan(&principal)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var recent bool
	if err = a.store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM password_reset_tokens WHERE principal_id=$1 AND created_at>clock_timestamp()-interval '2 minutes')`, principal).Scan(&recent); err != nil {
		return "", err
	}
	if recent {
		return "", nil
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	tx, err := a.store.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM password_reset_tokens WHERE principal_id=$1 AND (used_at IS NOT NULL OR expires_at<clock_timestamp())`, principal); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO password_reset_tokens(token_hash,principal_id,expires_at) VALUES($1,$2,$3)`, digest[:], principal, time.Now().Add(resetTokenLifetime)); err != nil {
		return "", err
	}
	if err = a.event(ctx, tx, principal, "password_reset_requested"); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// ConfirmPasswordReset sets a new password from a valid token. The token works once; every session ends.
func (a *PasswordAuthenticator) ConfirmPasswordReset(ctx context.Context, token, next string) error {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(raw) != 32 {
		return ErrResetTokenBad
	}
	if len(next) < passwordMinLength || len(next) > 1024 {
		return ErrPasswordPolicy
	}
	release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	hash, err := argon2id.CreateHash(next, passwordParams())
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	tx, err := a.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var principal string
	err = tx.QueryRow(ctx, `UPDATE password_reset_tokens SET used_at=clock_timestamp() WHERE token_hash=$1 AND used_at IS NULL AND expires_at>clock_timestamp() RETURNING principal_id::text`, digest[:]).Scan(&principal)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrResetTokenBad
	}
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE password_credentials SET password_hash=$2,changed_at=now() WHERE principal_id=$1`, principal, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrResetTokenBad
	}
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET used_at=COALESCE(used_at,clock_timestamp()) WHERE principal_id=$1`, principal); err != nil {
		return err
	}
	if err = a.event(ctx, tx, principal, "password_reset_completed"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
