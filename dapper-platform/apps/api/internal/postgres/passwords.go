package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
)

// PasswordAuthenticator is internal. Its caller must enforce request throttling
// and same-origin HTTPS before exposing sign-in. Two concurrent hashes cap RAM.
type PasswordAuthenticator struct {
	store *Store
	dummy string
	slots chan struct{}
}

var ErrAuthenticationBusy = errors.New("authentication capacity unavailable")

func passwordParams() *argon2id.Params {
	return &argon2id.Params{Memory: 65536, Iterations: 3, Parallelism: 2, SaltLength: 16, KeyLength: 32}
}
func NewPasswordAuthenticator(s *Store) (*PasswordAuthenticator, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	dummy, err := argon2id.CreateHash(base64.RawURLEncoding.EncodeToString(raw), passwordParams())
	if err != nil {
		return nil, err
	}
	return &PasswordAuthenticator{store: s, dummy: dummy, slots: make(chan struct{}, 2)}, nil
}
func normalizedEmail(email string) (string, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	return email, err == nil && address.Address == email && len(email) <= 254
}

// SetPassword is a trusted provisioning boundary, not a public signup route.
// It neither creates principals nor grants permissions. Callers must authorize
// provisioning/recovery independently. Plaintext is never persisted or logged.
func (a *PasswordAuthenticator) SetPassword(ctx context.Context, principalID, email, password string) error {
	email, valid := normalizedEmail(email)
	if !valid || len(password) < 15 || len(password) > 1024 {
		return ErrSession
	}
	select {
	case a.slots <- struct{}{}:
		defer func() { <-a.slots }()
	default:
		return ErrAuthenticationBusy
	}
	hash, err := argon2id.CreateHash(password, passwordParams())
	if err != nil {
		return err
	}
	result, err := a.store.pool.Exec(ctx, `INSERT INTO password_credentials(principal_id,email,password_hash)
 SELECT id,$2,$3 FROM principals WHERE id=$1 AND active FOR SHARE
 ON CONFLICT(principal_id) DO UPDATE SET email=excluded.email,password_hash=excluded.password_hash,changed_at=now()`, principalID, email, hash)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrSession
	}
	return nil
}

// SignIn atomically rechecks the credential hash after expensive verification.
// A concurrent password replacement cannot issue a session from the old hash.
func (a *PasswordAuthenticator) SignIn(ctx context.Context, email, password string, lifetime time.Duration) (string, error) {
	return a.SignInWithCode(ctx, email, password, "", lifetime)
}

// SignInWithCode is SignIn plus the second step: when the person has authenticator-app MFA on, a valid app code or
// one-time recovery code is required and no session is issued without it. The code is only checked after the password
// matched, so it reveals nothing about accounts that do not exist.
func (a *PasswordAuthenticator) SignInWithCode(ctx context.Context, email, password, code string, lifetime time.Duration) (string, error) {
	if lifetime < time.Second || lifetime > 24*time.Hour || len(password) > 1024 {
		return "", ErrSession
	}
	email, valid := normalizedEmail(email)
	select {
	case a.slots <- struct{}{}:
		defer func() { <-a.slots }()
	default:
		return "", ErrAuthenticationBusy
	}
	var principalID, hash string
	err := a.store.pool.QueryRow(ctx, `SELECT p.id::text,c.password_hash FROM password_credentials c JOIN principals p ON p.id=c.principal_id WHERE c.email=$1 AND p.active`, email).Scan(&principalID, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	found := err == nil && valid
	if !found {
		hash = a.dummy
	}
	matches, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil || !matches || !found {
		return "", ErrSession
	}
	if _, err = a.store.VerifyMFALogin(ctx, principalID, code); err != nil {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256(raw)
	tx, err := a.store.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `SELECT p.id::text FROM principals p JOIN password_credentials c ON c.principal_id=p.id WHERE p.id=$1 AND p.active AND c.password_hash=$2 FOR SHARE OF p,c`, principalID, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrSession
	}
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO auth_sessions(token_hash,principal_id,expires_at) VALUES($1,$2,now()+make_interval(secs=>$3))`, digest[:], id, lifetime.Seconds())
	if err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return token, nil
}
