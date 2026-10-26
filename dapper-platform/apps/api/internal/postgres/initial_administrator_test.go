package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestInitialAdministratorAtomicBoundary(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_initial_admin_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.pool.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
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
	if _, err = s.InitializeAdministrator(ctx, "Test organization", "invalid", "short"); !errors.Is(err, ErrAdministratorInput) {
		t.Fatal("invalid input accepted")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE organization_audit_events ADD CONSTRAINT fail_initial_admin_audit CHECK(action<>'administrator.initialized') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.InitializeAdministrator(ctx, "Test organization", "administrator@example.invalid", "Synthetic test password only"); err == nil {
		t.Fatal("failed audit committed administrator")
	}
	var organizations, principals, credentials int
	if err = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM organizations),(SELECT count(*) FROM principals),(SELECT count(*) FROM password_credentials)`).Scan(&organizations, &principals, &credentials); err != nil || organizations != 0 || principals != 0 || credentials != 0 {
		t.Fatal("failed initialization left partial accounts", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE organization_audit_events DROP CONSTRAINT fail_initial_admin_audit`); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		admin InitialAdministrator
		err   error
	}
	outcomes := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() {
			a, e := s.InitializeAdministrator(ctx, "Test organization", "administrator@example.invalid", "Synthetic test password only")
			outcomes <- outcome{a, e}
		}()
	}
	successes := 0
	var actual InitialAdministrator
	for i := 0; i < 2; i++ {
		result := <-outcomes
		if result.err == nil {
			successes++
			actual = result.admin
		} else if !errors.Is(result.err, ErrAdministratorInitialized) {
			t.Fatal(result.err)
		}
	}
	if successes != 1 || actual.PrincipalID == "" || actual.OrganizationID == "" {
		t.Fatal("concurrent initialization did not create one administrator")
	}
	if _, err = s.InitializeAdministrator(ctx, "Other organization", "other@example.invalid", "Another synthetic test password"); !errors.Is(err, ErrAdministratorInitialized) {
		t.Fatal("second initialization replaced existing application")
	}
	var hash string
	if err = s.pool.QueryRow(ctx, `SELECT password_hash FROM password_credentials WHERE principal_id=$1`, actual.PrincipalID).Scan(&hash); err != nil || hash == "Synthetic test password only" {
		t.Fatal("credential was not stored as hash", err)
	}
	auth, err := NewPasswordAuthenticator(s)
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.SignIn(ctx, actual.Email, "Synthetic test password only", time.Hour)
	if err != nil {
		t.Fatal("initialized administrator cannot sign in")
	}
	actor, err := s.SessionPrincipal(ctx, token)
	if err != nil || actor != actual.PrincipalID {
		t.Fatal("wrong administrator session actor")
	}
	locations, orgs, err := s.Access(ctx, actor)
	if err != nil || len(locations) != 0 || len(orgs) != 1 {
		t.Fatal("administrator acquired implicit location access", err)
	}
	expected := map[string]bool{"structure.read": true, "structure.manage": true, "catalogue.read": true, "catalogue.manage": true, "access.manage": true}
	if len(orgs[0].Actions) != len(expected) {
		t.Fatal("unexpected initial administrative grants")
	}
	for _, action := range orgs[0].Actions {
		if !expected[action] {
			t.Fatal("unapproved initial capability")
		}
	}
	var membership, audit int
	if err = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM organization_memberships),(SELECT count(*) FROM organization_audit_events WHERE action='administrator.initialized')`).Scan(&membership, &audit); err != nil || membership != 1 || audit != 1 {
		t.Fatal("initial administrator membership/audit missing", err)
	}
}
