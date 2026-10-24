package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestSyntheticWorkspaceBootstrap(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real PostgreSQL fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err = admin.CreateSyntheticFixture(ctx, "Synthetic fixture password only"); err == nil {
		t.Fatal("default schema used for synthetic seed")
	}
	namespace := fmt.Sprintf("dapper_workspace_demo_test_%d", time.Now().UnixNano())
	config := admin.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	defer s.Close()
	fixture, err := s.CreateSyntheticFixture(ctx, "Synthetic fixture password only")
	if err != nil {
		t.Fatal("fresh bootstrap failed", err)
	}
	defer func() {
		_, err := admin.pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{namespace}.Sanitize()+" CASCADE")
		if err != nil {
			t.Error(err)
		}
	}()
	if err = s.ValidateSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateSyntheticFixture(ctx, "Other synthetic password only"); err == nil {
		t.Fatal("existing fixture/account overwritten")
	}
	auth, err := NewPasswordAuthenticator(s)
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.SignIn(ctx, fixture.Email, "Synthetic fixture password only", time.Hour)
	if err != nil {
		t.Fatal("fixture login failed", err)
	}
	actor, err := s.SessionPrincipal(ctx, token)
	if err != nil || actor != fixture.PrincipalID {
		t.Fatal("fixture session invalid")
	}
	locations, organizations, err := s.Access(ctx, actor)
	if err != nil || len(locations) != 1 || len(organizations) != 1 {
		t.Fatal("fixture scope discovery invalid")
	}
	jobs, err := s.Jobs(ctx, Scope{OrganizationID: fixture.OrganizationID, LocationID: fixture.LocationID, ActorID: actor})
	if err != nil || len(jobs) != 1 {
		t.Fatal("fixture work order missing")
	}
}
