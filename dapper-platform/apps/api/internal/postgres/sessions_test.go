package postgres

import (
	"context"
	"testing"
	"time"
)

func TestTokenDigestRejectsMalformedInput(t *testing.T) {
	for _, token := range []string{"", "invalid", string(make([]byte, 10000))} {
		if _, err := tokenDigest(token); err != ErrSession {
			t.Fatal("malformed session token accepted")
		}
	}
}

func verifySessionLifecycle(t *testing.T, ctx context.Context, s *Store, principalID string) {
	t.Helper()
	if _, err := s.IssueSession(ctx, principalID, 0); err != ErrSession {
		t.Fatal("missing explicit session lifetime accepted")
	}
	if _, err := s.IssueSession(ctx, principalID, 25*time.Hour); err != ErrSession {
		t.Fatal("excess session lifetime accepted")
	}
	token, err := s.IssueSession(ctx, principalID, time.Hour)
	if err != nil {
		t.Fatal("session issuance failed", err)
	}
	second, err := s.IssueSession(ctx, principalID, time.Hour)
	if err != nil || second == token {
		t.Fatal("session entropy/issuance failure")
	}
	id, err := s.SessionPrincipal(ctx, token)
	if err != nil || id != principalID {
		t.Fatal("session identity mismatch")
	}
	digest, _ := tokenDigest(token)
	var size int
	if err = s.pool.QueryRow(ctx, `SELECT octet_length(token_hash) FROM auth_sessions WHERE token_hash=$1`, digest).Scan(&size); err != nil || size != 32 {
		t.Fatal("hashed token storage missing")
	}
	if err = s.RevokeSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err = s.RevokeSession(ctx, token); err != nil {
		t.Fatal("logout not idempotent")
	}
	if _, err = s.SessionPrincipal(ctx, token); err != ErrSession {
		t.Fatal("revoked session accepted")
	}
	secondHash, _ := tokenDigest(second)
	if _, err = s.pool.Exec(ctx, `UPDATE auth_sessions SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE token_hash=$1`, secondHash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SessionPrincipal(ctx, second); err != ErrSession {
		t.Fatal("expired session accepted")
	}
	active, err := s.IssueSession(ctx, principalID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=false WHERE id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SessionPrincipal(ctx, active); err != ErrSession {
		t.Fatal("disabled principal session accepted")
	}
	if _, err = s.IssueSession(ctx, principalID, time.Hour); err != ErrSession {
		t.Fatal("session issued for disabled principal")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=true WHERE id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SessionPrincipal(ctx, active); err != ErrSession {
		t.Fatal("re-enabled principal revived an old session")
	}
	if _, err = s.IssueSession(ctx, principalID, time.Hour); err != nil {
		t.Fatal("re-enabled principal could not establish a fresh session")
	}
	// Serialize an in-flight issuance behind a deactivation transaction.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT id FROM principals WHERE id=$1 FOR UPDATE`, principalID); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, issueErr := s.IssueSession(ctx, principalID, time.Hour); result <- issueErr }()
	if _, err = tx.Exec(ctx, `UPDATE principals SET active=false WHERE id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-result; err != ErrSession {
		t.Fatal("session issuance crossed a concurrent deactivation")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=true WHERE id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
}
