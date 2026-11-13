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
	if roles, e := s.ListRoles(ctx, owner.PrincipalID, owner.OrganizationID); e != nil || len(roles) != 3 {
		t.Fatal("the Super Admin cannot list the three built-in roles", len(roles), e)
	}
	// error404 #89: the Franchise Owner role is there from the start, with its full list of capabilities.
	var ownerCaps int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM roles r JOIN role_capabilities c ON c.role_id=r.id WHERE r.organization_id=$1 AND r.name=$2`, owner.OrganizationID, FranchiseOwnerRoleName).Scan(&ownerCaps))
	if ownerCaps != len(FranchiseOwnerRoleCapabilities) || ownerCaps != 37 {
		t.Fatal("Franchise Owner role incomplete", ownerCaps)
	}
	scope, err := s.ResolveScope(ctx, owner.PrincipalID, owner.OrganizationID, franchise.ID, "content.manage")
	if err != nil || scope.LocationID != franchise.ID {
		t.Fatal("the Super Admin cannot manage the franchise website content", err)
	}
	// The built-in Employee role exists and makes someone workspace-only; the owner is not.
	var employeeCaps int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM roles r JOIN role_capabilities c ON c.role_id=r.id WHERE r.organization_id=$1 AND r.name=$2`, owner.OrganizationID, EmployeeRoleName).Scan(&employeeCaps))
	if employeeCaps != len(EmployeeRoleCapabilities) {
		t.Fatal("the Employee role is missing or incomplete", employeeCaps)
	}
	var worker, roleID string
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:worker') RETURNING id::text`).Scan(&worker))
	_, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, owner.OrganizationID, worker)
	must(err)
	must(pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE organization_id=$1 AND name=$2`, owner.OrganizationID, EmployeeRoleName).Scan(&roleID))
	_, err = pool.Exec(ctx, `INSERT INTO member_role_assignments(organization_id,principal_id,role_id,scope_type,scope_id,location_id,version,assigned_by) VALUES($1,$2,$3,'location',$4,$4,1,$5)`, owner.OrganizationID, worker, roleID, franchise.ID, owner.PrincipalID)
	must(err)
	if only, e := s.WorkspaceOnly(ctx, worker); e != nil || !only {
		t.Fatal("an Employee-only person is not workspace-only", only, e)
	}
	if only, e := s.WorkspaceOnly(ctx, owner.PrincipalID); e != nil || only {
		t.Fatal("the Super Admin was treated as workspace-only", only, e)
	}
	var manager string
	must(pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE organization_id=$1 AND name='Super Admin'`, owner.OrganizationID).Scan(&manager))
	_, err = pool.Exec(ctx, `INSERT INTO member_role_assignments(organization_id,principal_id,role_id,scope_type,scope_id,version,assigned_by) VALUES($1,$2,$3,'organization',$1,1,$4)`, owner.OrganizationID, worker, manager, owner.PrincipalID)
	must(err)
	if only, e := s.WorkspaceOnly(ctx, worker); e != nil || only {
		t.Fatal("someone who is also an administrator was treated as workspace-only", only, e)
	}
	var nobody string
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:nobody') RETURNING id::text`).Scan(&nobody))
	if only, e := s.WorkspaceOnly(ctx, nobody); e != nil || only {
		t.Fatal("someone with no role was treated as workspace-only", only, e)
	}
}
