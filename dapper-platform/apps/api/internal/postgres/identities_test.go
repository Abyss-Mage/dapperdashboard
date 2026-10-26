package postgres

import (
	"context"
	"testing"
)

func verifyOIDCIdentityMapping(t *testing.T, ctx context.Context, s *Store, principalID string) {
	t.Helper()
	issuer := "https://synthetic-idp.example.test"
	subject := "synthetic-subject"
	if _, err := s.pool.Exec(ctx, `INSERT INTO oidc_identities(issuer,subject,principal_id) VALUES($1,$2,$3)`, issuer, subject, principalID); err != nil {
		t.Fatal(err)
	}
	if id, err := s.FindOIDCPrincipal(ctx, issuer, subject); err != nil || id != principalID {
		t.Fatal("pre-provisioned identity failed")
	}
	if _, err := s.FindOIDCPrincipal(ctx, "https://other-idp.example.test", subject); err != ErrSession {
		t.Fatal("issuer boundary bypass")
	}
	if _, err := s.FindOIDCPrincipal(ctx, issuer, "unknown-subject"); err != ErrSession {
		t.Fatal("unknown identity automatically provisioned")
	}
	if _, err := s.pool.Exec(ctx, `UPDATE principals SET active=false WHERE id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FindOIDCPrincipal(ctx, issuer, subject); err != ErrSession {
		t.Fatal("disabled identity accepted")
	}
	if _, err := s.pool.Exec(ctx, `UPDATE principals SET active=true WHERE id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
}
