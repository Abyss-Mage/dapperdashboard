package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestActivityLogAndTermination(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_act_%d", time.Now().UnixNano())
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
	var org, boss, second, worker string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Activity Org') RETURNING id::text`).Scan(&org))
	for name, dst := range map[string]*string{"boss": &boss, "second": &second, "worker": &worker} {
		must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) RETURNING id::text`, "synthetic:act-"+name).Scan(dst))
		_, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, *dst)
		must(err)
	}
	for _, p := range []string{boss, second} {
		_, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'access.manage')`, org, p)
		must(err)
	}
	const password = "synthetic-password-1234"
	must(auth.SetPassword(ctx, worker, "act-worker@example.invalid", password))

	// Sign-ins and changes are logged with where they came from; secrets and bodies never are.
	auth.RecordSignIn(ctx, "act-worker@example.invalid", true, "203.0.113.9", "Test Browser")
	auth.RecordSignIn(ctx, "stranger@example.invalid", false, "198.51.100.4", "Other Browser")
	must(s.RecordActivity(ctx, worker, "", "write", "POST", "/api/v1/organizations/"+org+"/locations/"+worker+"/bookings/"+boss+"/status", 200, "203.0.113.9", "Test Browser"))
	if _, err = s.ActivityLog(ctx, worker, org, ActivityFilter{}); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access.manage read the activity log", err)
	}
	items, err := s.ActivityLog(ctx, boss, org, ActivityFilter{})
	must(err)
	if len(items) != 3 || items[0].What != "Changed a booking's status" || items[0].IP != "203.0.113.9" || items[0].Path == "" {
		t.Fatalf("activity log wrong: %+v", items)
	}
	if signIns, e := s.ActivityLog(ctx, boss, org, ActivityFilter{Kind: "sign_in_failed"}); e != nil || len(signIns) != 1 || signIns[0].Email != "stranger@example.invalid" {
		t.Fatal("failed sign-in filter wrong", signIns, e)
	}
	if byIP, e := s.ActivityLog(ctx, boss, org, ActivityFilter{Query: "203.0.113"}); e != nil || len(byIP) != 2 {
		t.Fatal("search by address wrong", byIP, e)
	}
	if got := ClientIP("127.0.0.1:5555", "203.0.113.9, 127.0.0.1"); got != "203.0.113.9" {
		t.Fatal("the visitor address was not taken from the proxy header", got)
	}
	if got := ClientIP("198.51.100.4:5555", "10.0.0.1"); got != "198.51.100.4" {
		t.Fatal("a forwarded header from a non-local client was trusted", got)
	}

	// Terminate and reinstate: blocks sign-in at once, never yourself or the last administrator.
	token, err := auth.SignIn(ctx, "act-worker@example.invalid", password, time.Hour)
	must(err)
	if err = s.SetMemberActive(ctx, worker, org, worker, false); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access.manage terminated someone", err)
	}
	if err = s.SetMemberActive(ctx, boss, org, boss, false); !errors.Is(err, ErrMemberSelf) {
		t.Fatal("an administrator terminated themselves", err)
	}
	must(s.SetMemberActive(ctx, boss, org, worker, false))
	if _, err = s.SessionPrincipal(ctx, token); err == nil {
		t.Fatal("a terminated member's session still works")
	}
	if _, err = auth.SignIn(ctx, "act-worker@example.invalid", password, time.Hour); err == nil {
		t.Fatal("a terminated member can sign in")
	}
	must(s.SetMemberActive(ctx, boss, org, worker, true))
	if _, err = auth.SignIn(ctx, "act-worker@example.invalid", password, time.Hour); err != nil {
		t.Fatal("a reinstated member cannot sign in", err)
	}
	must(s.SetMemberActive(ctx, boss, org, second, false))
	if err = s.SetMemberActive(ctx, second, org, boss, false); !errors.Is(err, ErrForbidden) && !errors.Is(err, ErrMemberLastAdmin) {
		t.Fatal("the last administrator could be terminated", err)
	}

	// error404 #90: removing a person takes them off the team (no membership, grants or sign-in); yourself and outsiders cannot.
	_, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'bookings.read')`, org, worker)
	must(err)
	if err = s.RemoveMember(ctx, worker, org, boss); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access.manage removed someone", err)
	}
	if err = s.RemoveMember(ctx, boss, org, boss); !errors.Is(err, ErrMemberSelf) {
		t.Fatal("an administrator removed themselves", err)
	}
	must(s.RemoveMember(ctx, boss, org, worker))
	var left int
	must(pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM organization_memberships WHERE principal_id=$1)+(SELECT count(*) FROM organization_grants WHERE principal_id=$1)`, worker).Scan(&left))
	if left != 0 {
		t.Fatal("a removed member kept membership or grants", left)
	}
	if _, err = auth.SignIn(ctx, "act-worker@example.invalid", password, time.Hour); err == nil {
		t.Fatal("a removed member can sign in")
	}
	if err = s.RemoveMember(ctx, boss, org, worker); !errors.Is(err, ErrMemberNotFound) {
		t.Fatal("removing twice should say not found", err)
	}

	// error404 #91: an accepted or expired invitation can be taken off the list; an open one cannot.
	var open, used string
	must(pool.QueryRow(ctx, `INSERT INTO member_invitations(organization_id,email,token_hash,created_by,expires_at) VALUES($1,'open@example.invalid',decode(md5('open')||md5('open'),'hex'),$2,now()+interval '1 day') RETURNING id::text`, org, boss).Scan(&open))
	must(pool.QueryRow(ctx, `INSERT INTO member_invitations(organization_id,email,token_hash,created_by,created_at,expires_at) VALUES($1,'old@example.invalid',decode(md5('old')||md5('old'),'hex'),$2,now()-interval '2 days',now()-interval '1 day') RETURNING id::text`, org, boss).Scan(&used))
	if err = s.HideMemberInvitation(ctx, boss, org, open); !errors.Is(err, ErrInvitationStillOpen) {
		t.Fatal("an open invitation was removed from the list", err)
	}
	must(s.HideMemberInvitation(ctx, boss, org, used))
	var status string
	var hidden bool
	must(pool.QueryRow(ctx, `SELECT status,hidden_at IS NOT NULL FROM member_invitations WHERE id=$1`, used).Scan(&status, &hidden))
	if status != "revoked" || !hidden {
		t.Fatal("expired invitation not revoked and hidden", status, hidden)
	}
}
