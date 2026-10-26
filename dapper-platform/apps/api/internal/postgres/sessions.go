package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

var ErrSession = errors.New("session unavailable or invalid")

// IssueSession is INTERNAL: call only after verified identity authentication.
// There is deliberately no public session-issuance HTTP route. Callers choose
// an explicit lifetime; the technical maximum is 24 hours, with no default.
// Only the hash is stored; return the random bearer token once, never log it.
func (s *Store) IssueSession(ctx context.Context, principalID string, lifetime time.Duration) (string, error) {
	if lifetime < time.Second || lifetime > 24*time.Hour {
		return "", ErrSession
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", ErrSession
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	var id string
	err := s.pool.QueryRow(ctx, `INSERT INTO auth_sessions(token_hash,principal_id,expires_at)
 SELECT $1,id,now()+make_interval(secs=>$3) FROM principals WHERE id=$2 AND active FOR SHARE
 RETURNING principal_id::text`, hash[:], principalID, lifetime.Seconds()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrSession
	}
	if err != nil {
		return "", err
	}
	return token, nil
}

func tokenDigest(token string) ([]byte, error) {
	if len(token) != 43 {
		return nil, ErrSession
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 {
		return nil, ErrSession
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}

// SessionPrincipal validates expiry/revocation and active status on every call.
// Authorization still requires a separate explicit location/action grant.
func (s *Store) SessionPrincipal(ctx context.Context, token string) (string, error) {
	hash, err := tokenDigest(token)
	if err != nil {
		return "", ErrSession
	}
	var principalID string
	err = s.pool.QueryRow(ctx, `SELECT p.id::text FROM auth_sessions s JOIN principals p ON p.id=s.principal_id
 WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND p.active`, hash).Scan(&principalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrSession
	}
	if err != nil {
		return "", err
	}
	return principalID, nil
}

func (s *Store) RevokeSession(ctx context.Context, token string) error {
	hash, err := tokenDigest(token)
	if err != nil {
		return ErrSession
	}
	_, err = s.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1`, hash)
	return err
}
