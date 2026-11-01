package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFirstFranchiseSetup(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_first_%d", time.Now().UnixNano())
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := s.InitializeAdministrator(ctx, "Synthetic Setup Business", "owner@example.invalid", "a-long-synthetic-password-1")
	must(err)
	bad := FirstFranchiseInput{Name: "Synthetic Franchise", Code: "d", Address: Address{Street: "1 Test St", City: "Testville", Region: "ON", PostalCode: "A1A 1A1", Country: "Canada"}}
	if _, err = s.ProvisionFirstFranchise(ctx, owner, bad); err == nil {
		t.Fatal("a one-letter franchise code was accepted")
	}
	good := bad
	good.Code, good.Phone = "ddp", "705-555-0100"
	franchise, err := s.ProvisionFirstFranchise(ctx, owner, good)
	must(err)
	if franchise.Code != "DDP" || len(franchise.Contacts) != 1 || franchise.Contacts[0].Phone != "705-555-0100" {
		t.Fatalf("franchise wrong: %+v", franchise)
	}
	var capabilities, assigned int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM roles r JOIN role_capabilities c ON c.role_id=r.id WHERE r.organization_id=$1 AND r.name='Super Admin'`, owner.OrganizationID).Scan(&capabilities))
	if capabilities != len(ServerKnownCapabilities()) {
		t.Fatal("the Super Admin role does not hold every capability", capabilities)
	}
	must(pool.QueryRow(ctx, `SELECT count(*) FROM member_role_assignments a JOIN roles r ON r.id=a.role_id WHERE a.principal_id=$1 AND r.name='Super Admin' AND a.scope_type='organization' AND a.scope_id=$2`, owner.PrincipalID, owner.OrganizationID).Scan(&assigned))
	if assigned != 1 {
		t.Fatal("the first owner was not made Super Admin", assigned)
	}
	var published bool
	var slug string
	must(pool.QueryRow(ctx, `SELECT slug,published FROM public_sites WHERE organization_id=$1 AND location_id=$2`, owner.OrganizationID, franchise.ID).Scan(&slug, &published))
	if slug != "ddp" || !published {
		t.Fatal("the franchise has no published website entry", slug, published)
	}
	// the new Super Admin can run the roles screen and the content screens
	if roles, e := s.ListRoles(ctx, owner.PrincipalID, owner.OrganizationID); e != nil || len(roles) != 1 {
		t.Fatal("the Super Admin cannot list roles", e)
	}
	scope, err := s.ResolveScope(ctx, owner.PrincipalID, owner.OrganizationID, franchise.ID, "content.manage")
	if err != nil || scope.LocationID != franchise.ID {
		t.Fatal("the Super Admin cannot manage the franchise website content", err)
	}
}
