package postgres

import (
	"context"
	"testing"
	"time"
)

func verifyManagedPasswords(t *testing.T, ctx context.Context, s *Store, principalID string) {
	t.Helper()
	a, err := NewPasswordAuthenticator(s)
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic credentials exist only inside this disposable database fixture.
	first := "Synthetic fixture password alpha"
	second := "Synthetic fixture password bravo"
	if err = a.SetPassword(ctx, principalID, " Fictional.Account@example.invalid ", first); err != nil {
		t.Fatal(err)
	}
	token, err := a.SignIn(ctx, "FICTIONAL.ACCOUNT@example.invalid", first, time.Hour)
	if err != nil {
		t.Fatal("valid synthetic sign-in failed", err)
	}
	if id, err := s.SessionPrincipal(ctx, token); err != nil || id != principalID {
		t.Fatal("wrong session principal")
	}
	for _, entry := range []struct{ email, password string }{
		{"fictional.account@example.invalid", "wrong"},
		{"unknown@example.invalid", first},
		{"Display Name <fictional.account@example.invalid>", first},
	} {
		if _, err = a.SignIn(ctx, entry.email, entry.password, time.Hour); err != ErrSession {
			t.Fatal("invalid credentials accepted")
		}
	}
	if err = a.SetPassword(ctx, principalID, "fictional.account@example.invalid", "short"); err != ErrSession {
		t.Fatal("short password accepted")
	}
	if err = a.SetPassword(ctx, principalID, "fictional.account@example.invalid", second); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SessionPrincipal(ctx, token); err != ErrSession {
		t.Fatal("password replacement did not revoke session")
	}
	if _, err = a.SignIn(ctx, "fictional.account@example.invalid", first, time.Hour); err != ErrSession {
		t.Fatal("old password accepted")
	}
	fresh, err := a.SignIn(ctx, "fictional.account@example.invalid", second, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=false WHERE id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
	if _, err = a.SignIn(ctx, "fictional.account@example.invalid", second, time.Hour); err != ErrSession {
		t.Fatal("disabled account signed in")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=true WHERE id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SessionPrincipal(ctx, fresh); err != ErrSession {
		t.Fatal("reactivation revived session")
	}
	a.slots <- struct{}{}
	a.slots <- struct{}{}
	if _, err = a.SignIn(ctx, "fictional.account@example.invalid", second, time.Hour); err != ErrAuthenticationBusy {
		t.Fatal("hash concurrency limit missing")
	}
	<-a.slots
	<-a.slots
	if _, err = s.pool.Exec(ctx, `DELETE FROM password_credentials WHERE principal_id=$1`, principalID); err != nil {
		t.Fatal(err)
	}
	if _, err = a.SignIn(ctx, "fictional.account@example.invalid", second, time.Hour); err != ErrSession {
		t.Fatal("deleted credentials accepted")
	}
}
