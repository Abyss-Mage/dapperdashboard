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

func TestRolesAndScopedAssignments(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()

	namespace := fmt.Sprintf("dapper_roles_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if _, e := admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error(e)
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

	// 1. Setup two organizations, franchises, locations, and principals
	var org1, org2 string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Org North') RETURNING id::text`).Scan(&org1); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Org South') RETURNING id::text`).Scan(&org2); err != nil {
		t.Fatal(err)
	}

	var fran1A, fran1B, fran2A string
	if err = pool.QueryRow(ctx, `INSERT INTO franchises(organization_id, name) VALUES($1, 'North Urban') RETURNING id::text`, org1).Scan(&fran1A); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO franchises(organization_id, name) VALUES($1, 'North Rural') RETURNING id::text`, org1).Scan(&fran1B); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO franchises(organization_id, name) VALUES($1, 'South Metro') RETURNING id::text`, org2).Scan(&fran2A); err != nil {
		t.Fatal(err)
	}

	var loc1A, loc1B, loc2A string
	if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id, franchise_id, name) VALUES($1, $2, 'Downtown Bay') RETURNING id::text`, org1, fran1A).Scan(&loc1A); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id, franchise_id, name) VALUES($1, $2, 'Country Bay') RETURNING id::text`, org1, fran1B).Scan(&loc1B); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id, franchise_id, name) VALUES($1, $2, 'Metro Bay') RETURNING id::text`, org2, fran2A).Scan(&loc2A); err != nil {
		t.Fatal(err)
	}

	var owner1, tech1, techInactive, owner2 string
	// Active owner in org1
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_owner1', true) RETURNING id::text`).Scan(&owner1); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org1, owner1); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, 'access.manage'), ($1, $2, 'roles.manage'), ($1, $2, 'roles.assign')`, org1, owner1); err != nil {
		t.Fatal(err)
	}

	// Active tech in org1
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_tech1', true) RETURNING id::text`).Scan(&tech1); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org1, tech1); err != nil {
		t.Fatal(err)
	}

	// Inactive member in org1
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_tech_inactive', false) RETURNING id::text`).Scan(&techInactive); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org1, techInactive); err != nil {
		t.Fatal(err)
	}

	// Active owner in org2
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_owner2', true) RETURNING id::text`).Scan(&owner2); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org2, owner2); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, 'access.manage'), ($1, $2, 'roles.manage'), ($1, $2, 'roles.assign')`, org2, owner2); err != nil {
		t.Fatal(err)
	}

	t.Run("Capability Catalogue", func(t *testing.T) {
		cat := GetCapabilityCatalogue()
		if len(cat.LocationCapabilities) == 0 || len(cat.OrganizationCapabilities) == 0 {
			t.Fatal("empty capability catalogue")
		}
		if !IsValidCapability("jobs.read") {
			t.Fatal("jobs.read should be valid")
		}
		if !IsValidCapability("roles.manage") {
			t.Fatal("roles.manage should be valid")
		}
		if IsValidCapability("invalid.unknown.action") {
			t.Fatal("unknown action should be invalid")
		}
	})

	t.Run("Create Role - Validation", func(t *testing.T) {
		// Unknown capability rejected
		_, err := s.CreateRole(ctx, owner1, org1, "Invalid Role", "Desc", []string{"unknown.fake.cap"})
		if !errors.Is(err, ErrInvalidCapability) {
			t.Fatalf("expected ErrInvalidCapability, got: %v", err)
		}

		// Empty capabilities rejected
		_, err = s.CreateRole(ctx, owner1, org1, "Empty Role", "Desc", []string{})
		if err == nil {
			t.Fatal("expected error for empty capabilities")
		}

		// Forbidden actor (not member or not manager)
		_, err = s.CreateRole(ctx, tech1, org1, "Tech Created", "Desc", []string{"jobs.read"})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for non-manager actor, got: %v", err)
		}

		// Cross-org actor rejected
		_, err = s.CreateRole(ctx, owner2, org1, "Cross Org Role", "Desc", []string{"jobs.read"})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for cross-org actor, got: %v", err)
		}
	})

	var techRole, leadRole Role
	t.Run("Create Role - Success and Duplicate Name", func(t *testing.T) {
		role, err := s.CreateRole(ctx, owner1, org1, "Detailing Technician", "Standard shop-floor tech", []string{"jobs.read", "jobs.contribute", "attendance.record"})
		if err != nil {
			t.Fatalf("failed to create role: %v", err)
		}
		if role.Version != 1 || role.Name != "Detailing Technician" || len(role.Capabilities) != 3 {
			t.Fatalf("unexpected role details: %+v", role)
		}
		techRole = role

		lead, err := s.CreateRole(ctx, owner1, org1, "Shop Lead", "Franchise-wide supervisor", []string{"jobs.read", "jobs.create", "jobs.complete", "scheduling.read"})
		if err != nil {
			t.Fatalf("failed to create lead role: %v", err)
		}
		leadRole = lead

		// Duplicate name in same org rejected
		_, err = s.CreateRole(ctx, owner1, org1, "Detailing Technician", "Duplicate", []string{"jobs.read"})
		if !errors.Is(err, ErrRoleNameConflict) {
			t.Fatalf("expected ErrRoleNameConflict, got: %v", err)
		}

		// Same name in different org allowed
		_, err = s.CreateRole(ctx, owner2, org2, "Detailing Technician", "Org 2 tech", []string{"jobs.read"})
		if err != nil {
			t.Fatalf("failed to create same-name role in org 2: %v", err)
		}
	})

	t.Run("List Roles and Get Role", func(t *testing.T) {
		roles, err := s.ListRoles(ctx, owner1, org1)
		if err != nil {
			t.Fatalf("ListRoles failed: %v", err)
		}
		if len(roles) != 2 {
			t.Fatalf("expected 2 roles in org1, got %d", len(roles))
		}

		fetched, err := s.GetRole(ctx, owner1, org1, techRole.ID)
		if err != nil {
			t.Fatalf("GetRole failed: %v", err)
		}
		if fetched.ID != techRole.ID || fetched.Name != techRole.Name {
			t.Fatalf("GetRole mismatch: got %+v", fetched)
		}

		// Cross-org get fails with ErrRoleNotFound
		_, err = s.GetRole(ctx, owner2, org2, techRole.ID)
		if !errors.Is(err, ErrRoleNotFound) {
			t.Fatalf("expected ErrRoleNotFound for cross-org role, got: %v", err)
		}
	})

	t.Run("Update Role - Optimistic Concurrency", func(t *testing.T) {
		// Wrong expected version rejected
		_, err := s.UpdateRole(ctx, owner1, org1, techRole.ID, "Detailing Technician", "Updated desc", []string{"jobs.read"}, 99)
		if !errors.Is(err, ErrRoleStaleVersion) {
			t.Fatalf("expected ErrRoleStaleVersion, got: %v", err)
		}

		// Correct expected version updates version to 2
		updated, err := s.UpdateRole(ctx, owner1, org1, techRole.ID, "Senior Technician", "Updated desc", []string{"jobs.read", "jobs.contribute", "jobs.complete"}, 1)
		if err != nil {
			t.Fatalf("UpdateRole failed: %v", err)
		}
		if updated.Version != 2 || updated.Name != "Senior Technician" || len(updated.Capabilities) != 3 {
			t.Fatalf("unexpected updated role: %+v", updated)
		}
		techRole = updated
	})

	t.Run("Assign Role - Validation", func(t *testing.T) {
		// Inactive member rejected
		_, err := s.AssignMemberRole(ctx, owner1, org1, techInactive, techRole.ID, "location", loc1A)
		if !errors.Is(err, ErrInactiveMember) {
			t.Fatalf("expected ErrInactiveMember, got: %v", err)
		}

		// Cross-org location rejected
		_, err = s.AssignMemberRole(ctx, owner1, org1, tech1, techRole.ID, "location", loc2A)
		if !errors.Is(err, ErrInvalidScope) {
			t.Fatalf("expected ErrInvalidScope for cross-org location, got: %v", err)
		}

		// Cross-org franchise rejected
		_, err = s.AssignMemberRole(ctx, owner1, org1, tech1, techRole.ID, "franchise", fran2A)
		if !errors.Is(err, ErrInvalidScope) {
			t.Fatalf("expected ErrInvalidScope for cross-org franchise, got: %v", err)
		}

		// Non-existent role rejected
		_, err = s.AssignMemberRole(ctx, owner1, org1, tech1, "00000000-0000-0000-0000-000000000000", "location", loc1A)
		if !errors.Is(err, ErrRoleNotFound) {
			t.Fatalf("expected ErrRoleNotFound, got: %v", err)
		}
	})

	var locAssign, franAssign MemberRoleAssignment
	t.Run("Assign Role - Location Scope and Effective Authorization", func(t *testing.T) {
		assignment, err := s.AssignMemberRole(ctx, owner1, org1, tech1, techRole.ID, "location", loc1A)
		if err != nil {
			t.Fatalf("AssignMemberRole at location scope failed: %v", err)
		}
		if assignment.ScopeType != "location" || assignment.ScopeID != loc1A {
			t.Fatalf("unexpected assignment: %+v", assignment)
		}
		locAssign = assignment

		// Verify ResolveScope: tech1 has jobs.read at loc1A
		scope, err := s.ResolveScope(ctx, tech1, org1, loc1A, "jobs.read")
		if err != nil {
			t.Fatalf("ResolveScope for assigned location failed: %v", err)
		}
		if scope.LocationID != loc1A {
			t.Fatalf("scope location mismatch: %+v", scope)
		}

		// tech1 does NOT have jobs.read at loc1B
		_, err = s.ResolveScope(ctx, tech1, org1, loc1B, "jobs.read")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden at other location, got: %v", err)
		}

		// tech1 does NOT have scheduling.manage (not in role)
		_, err = s.ResolveScope(ctx, tech1, org1, loc1A, "scheduling.manage")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for ungranted action, got: %v", err)
		}
	})

	t.Run("Assign Role - Franchise Scope Inheritance and Isolation", func(t *testing.T) {
		assignment, err := s.AssignMemberRole(ctx, owner1, org1, tech1, leadRole.ID, "franchise", fran1A)
		if err != nil {
			t.Fatalf("AssignMemberRole at franchise scope failed: %v", err)
		}
		franAssign = assignment

		// loc1A belongs to fran1A -> tech1 has scheduling.read at loc1A
		_, err = s.ResolveScope(ctx, tech1, org1, loc1A, "scheduling.read")
		if err != nil {
			t.Fatalf("expected franchise role to grant access to child location loc1A: %v", err)
		}

		// loc1B belongs to fran1B -> tech1 does NOT have access at loc1B (isolated sibling franchise)
		_, err = s.ResolveScope(ctx, tech1, org1, loc1B, "scheduling.read")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden at sibling franchise location, got: %v", err)
		}
	})

	t.Run("Overlapping Roles & Independent Grant Sources", func(t *testing.T) {
		// tech1 has both Senior Technician (location loc1A) and Shop Lead (franchise fran1A)
		// Revoke the location assignment
		err := s.RevokeMemberRoleAssignment(ctx, owner1, org1, locAssign.ID, locAssign.Version)
		if err != nil {
			t.Fatalf("RevokeMemberRoleAssignment failed: %v", err)
		}

		// Senior Technician had jobs.complete; Shop Lead ALSO has jobs.complete
		// jobs.complete must STILL be granted via Shop Lead!
		_, err = s.ResolveScope(ctx, tech1, org1, loc1A, "jobs.complete")
		if err != nil {
			t.Fatalf("expected jobs.complete to remain granted via remaining franchise role: %v", err)
		}

		// But jobs.contribute was ONLY in Senior Technician!
		// Now it must be denied:
		_, err = s.ResolveScope(ctx, tech1, org1, loc1A, "jobs.contribute")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected jobs.contribute to be revoked after role removal, got: %v", err)
		}
	})

	t.Run("Role In-Use Deletion Protection", func(t *testing.T) {
		// leadRole is currently assigned to tech1 -> delete must be refused
		err := s.DeleteRole(ctx, owner1, org1, leadRole.ID, leadRole.Version)
		if !errors.Is(err, ErrRoleInUse) {
			t.Fatalf("expected ErrRoleInUse when deleting assigned role, got: %v", err)
		}

		// Revoke franchise assignment
		err = s.RevokeMemberRoleAssignment(ctx, owner1, org1, franAssign.ID, franAssign.Version)
		if err != nil {
			t.Fatalf("failed to revoke franchise assignment: %v", err)
		}

		// Now delete succeeds
		err = s.DeleteRole(ctx, owner1, org1, leadRole.ID, leadRole.Version)
		if err != nil {
			t.Fatalf("DeleteRole failed after revoking assignment: %v", err)
		}
	})

	t.Run("Last Admin Lockout Protection", func(t *testing.T) {
		// Create admin role
		adminRole, err := s.CreateRole(ctx, owner1, org1, "Organization Admin", "Full admin", []string{"access.manage", "roles.manage"})
		if err != nil {
			t.Fatal(err)
		}

		// Assign admin role to tech1 at org scope
		assign, err := s.AssignMemberRole(ctx, owner1, org1, tech1, adminRole.ID, "organization", org1)
		if err != nil {
			t.Fatal(err)
		}

		// Now remove direct grant from owner1 so tech1 is the ONLY active admin
		_, err = pool.Exec(ctx, `DELETE FROM organization_grants WHERE organization_id=$1 AND principal_id=$2 AND action IN ('access.manage', 'roles.manage')`, org1, owner1)
		if err != nil {
			t.Fatal(err)
		}

		// Attempting to revoke tech1's admin assignment must be refused
		err = s.RevokeMemberRoleAssignment(ctx, tech1, org1, assign.ID, assign.Version)
		if !errors.Is(err, ErrLastAdminLockout) {
			t.Fatalf("expected ErrLastAdminLockout, got: %v", err)
		}

		// Attempting to edit adminRole to remove access.manage/roles.manage must also be refused
		_, err = s.UpdateRole(ctx, tech1, org1, adminRole.ID, "Organization Admin", "Demoted", []string{"jobs.read"}, adminRole.Version)
		if !errors.Is(err, ErrLastAdminLockout) {
			t.Fatalf("expected ErrLastAdminLockout on role capability demotion, got: %v", err)
		}
	})

	t.Run("Audit Ledger Verification", func(t *testing.T) {
		var defCount, assignCount int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM role_definition_events WHERE organization_id=$1`, org1).Scan(&defCount); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM role_assignment_events WHERE organization_id=$1`, org1).Scan(&assignCount); err != nil {
			t.Fatal(err)
		}
		if defCount < 3 {
			t.Fatalf("expected at least 3 role definition events, got %d", defCount)
		}
		if assignCount < 3 {
			t.Fatalf("expected at least 3 role assignment events, got %d", assignCount)
		}
	})
}

func TestRoleDefinitionPrivilegeSeparation(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	namespace := fmt.Sprintf("dapper_roles_priv_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
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

	var org string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Org Priv') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}

	var actorManage, actorAccess, actorAssign string
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_mgt', true) RETURNING id::text`).Scan(&actorManage); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_acc', true) RETURNING id::text`).Scan(&actorAccess); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_asg', true) RETURNING id::text`).Scan(&actorAssign); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{actorManage, actorAccess, actorAssign} {
		if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org, p); err != nil {
			t.Fatal(err)
		}
	}

	// Grants
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, 'roles.manage')`, org, actorManage); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, 'access.manage')`, org, actorAccess); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, 'roles.assign')`, org, actorAssign); err != nil {
		t.Fatal(err)
	}

	// 1. actorAccess (access.manage only) CANNOT create role
	_, err = s.CreateRole(ctx, actorAccess, org, "Forbidden Role", "Desc", []string{"jobs.read"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for access.manage-only actor, got %v", err)
	}

	// 2. actorAssign (roles.assign only) CANNOT create role
	_, err = s.CreateRole(ctx, actorAssign, org, "Forbidden Role 2", "Desc", []string{"jobs.read"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for roles.assign-only actor, got %v", err)
	}

	// 3. actorManage (roles.manage) CAN create role
	role, err := s.CreateRole(ctx, actorManage, org, "Valid Role", "Desc", []string{"jobs.read"})
	if err != nil {
		t.Fatalf("roles.manage actor failed to create role: %v", err)
	}

	// 4. actorAccess CANNOT update role
	_, err = s.UpdateRole(ctx, actorAccess, org, role.ID, "Valid Role II", "Desc II", []string{"jobs.read", "jobs.create"}, role.Version)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for access.manage-only update, got %v", err)
	}

	// 5. actorAccess CANNOT delete role
	err = s.DeleteRole(ctx, actorAccess, org, role.ID, role.Version)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for access.manage-only delete, got %v", err)
	}

	// 6. actorManage CAN update and delete role
	updated, err := s.UpdateRole(ctx, actorManage, org, role.ID, "Valid Role II", "Desc II", []string{"jobs.read", "jobs.create"}, role.Version)
	if err != nil {
		t.Fatalf("roles.manage failed to update: %v", err)
	}
	if err = s.DeleteRole(ctx, actorManage, org, updated.ID, updated.Version); err != nil {
		t.Fatalf("roles.manage failed to delete: %v", err)
	}
}

func TestMultiAdminSharedRoleLockoutRefusal(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	namespace := fmt.Sprintf("dapper_roles_multiadmin_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
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

	var org string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Org Multi') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}

	var admin1, admin2 string
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_admin1', true) RETURNING id::text`).Scan(&admin1); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_admin2', true) RETURNING id::text`).Scan(&admin2); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{admin1, admin2} {
		if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org, a); err != nil {
			t.Fatal(err)
		}
	}

	// Initially admin1 has roles.manage grant so they can set up roles
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, 'roles.manage')`, org, admin1); err != nil {
		t.Fatal(err)
	}

	// Create shared Co-Admin role
	coAdminRole, err := s.CreateRole(ctx, admin1, org, "Co-Administrator", "Shared admin role", []string{"roles.manage"})
	if err != nil {
		t.Fatal(err)
	}

	// Assign coAdminRole to both admin1 and admin2 at organization scope
	_, err = s.AssignMemberRole(ctx, admin1, org, admin1, coAdminRole.ID, "organization", org)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AssignMemberRole(ctx, admin1, org, admin2, coAdminRole.ID, "organization", org)
	if err != nil {
		t.Fatal(err)
	}

	// Remove admin1's direct grant so BOTH admins derive their admin authority solely from the shared coAdminRole
	if _, err = pool.Exec(ctx, `DELETE FROM organization_grants WHERE organization_id=$1 AND principal_id=$2 AND action='roles.manage'`, org, admin1); err != nil {
		t.Fatal(err)
	}

	// Pre-change admin count is 2 (both admin1 and admin2).
	// Under flawed pre-change check, updating coAdminRole to remove roles.manage would be allowed because 2 > 1.
	// But under effective post-change evaluation, demoting coAdminRole leaves 0 admins!
	// Therefore, it MUST return ErrLastAdminLockout!
	_, err = s.UpdateRole(ctx, admin1, org, coAdminRole.ID, "Co-Administrator", "Demoted", []string{"jobs.read"}, coAdminRole.Version)
	if !errors.Is(err, ErrLastAdminLockout) {
		t.Fatalf("expected ErrLastAdminLockout when demoting shared role, got %v", err)
	}
}

func TestRoleAssignmentDelegationBoundsAndCrossFranchiseIsolation(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	namespace := fmt.Sprintf("dapper_roles_delegation_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
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

	var org string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Org Deleg') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}

	var franA, franB string
	if err = pool.QueryRow(ctx, `INSERT INTO franchises(organization_id, name) VALUES($1, 'Franchise A') RETURNING id::text`, org).Scan(&franA); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO franchises(organization_id, name) VALUES($1, 'Franchise B') RETURNING id::text`, org).Scan(&franB); err != nil {
		t.Fatal(err)
	}

	var locA, locB string
	if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id, franchise_id, name) VALUES($1, $2, 'Location A') RETURNING id::text`, org, franA).Scan(&locA); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id, franchise_id, name) VALUES($1, $2, 'Location B') RETURNING id::text`, org, franB).Scan(&locB); err != nil {
		t.Fatal(err)
	}

	var rootOwner, managerA, worker string
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_root', true) RETURNING id::text`).Scan(&rootOwner); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_mgr_a', true) RETURNING id::text`).Scan(&managerA); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_worker', true) RETURNING id::text`).Scan(&worker); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{rootOwner, managerA, worker} {
		if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org, p); err != nil {
			t.Fatal(err)
		}
	}

	// rootOwner has roles.manage org-wide
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, 'roles.manage')`, org, rootOwner); err != nil {
		t.Fatal(err)
	}

	// Create roles:
	// Role 1: Basic Tech (jobs.read, jobs.contribute)
	basicRole, err := s.CreateRole(ctx, rootOwner, org, "Basic Tech", "Basic", []string{"jobs.read", "jobs.contribute"})
	if err != nil {
		t.Fatal(err)
	}

	// Role 2: Finance Manager (invoices.manage, jobs.read)
	financeRole, err := s.CreateRole(ctx, rootOwner, org, "Finance Role", "Finance", []string{"invoices.manage", "jobs.read"})
	if err != nil {
		t.Fatal(err)
	}

	// Role 3: Franchise Manager Role (roles.assign, jobs.read, jobs.contribute)
	mgrRole, err := s.CreateRole(ctx, rootOwner, org, "Franchise Mgr", "Assigner", []string{"roles.assign", "jobs.read", "jobs.contribute"})
	if err != nil {
		t.Fatal(err)
	}

	// Assign mgrRole to managerA ONLY at Franchise A scope
	_, err = s.AssignMemberRole(ctx, rootOwner, org, managerA, mgrRole.ID, "franchise", franA)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Cross-franchise isolation: managerA CANNOT assign any role at Franchise B
	_, err = s.AssignMemberRole(ctx, managerA, org, worker, basicRole.ID, "franchise", franB)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for cross-franchise assignment, got %v", err)
	}

	// Cross-franchise location: managerA CANNOT assign any role at Location B (which is under Franchise B)
	_, err = s.AssignMemberRole(ctx, managerA, org, worker, basicRole.ID, "location", locB)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for cross-franchise location assignment, got %v", err)
	}

	// 2. Delegation bounds (anti-escalation): managerA possesses jobs.read and jobs.contribute at Franchise A,
	// but DOES NOT possess invoices.manage.
	// Attempting to assign financeRole (which has invoices.manage) at Franchise A or Location A must fail with ErrSelfEscalation!
	_, err = s.AssignMemberRole(ctx, managerA, org, worker, financeRole.ID, "location", locA)
	if !errors.Is(err, ErrSelfEscalation) {
		t.Fatalf("expected ErrSelfEscalation when delegating unpossessed capabilities, got %v", err)
	}

	// 3. Permitted assignment: managerA assigns basicRole at Location A (child of Franchise A) -> SUCCEEDS
	assign, err := s.AssignMemberRole(ctx, managerA, org, worker, basicRole.ID, "location", locA)
	if err != nil {
		t.Fatalf("valid delegation failed: %v", err)
	}

	// 4. Concurrency check on revocation: stale expected version returns ErrRoleStaleVersion
	err = s.RevokeMemberRoleAssignment(ctx, managerA, org, assign.ID, 999)
	if !errors.Is(err, ErrRoleStaleVersion) {
		t.Fatalf("expected ErrRoleStaleVersion for stale version revoke, got %v", err)
	}

	// Revoke with correct version 1 succeeds
	err = s.RevokeMemberRoleAssignment(ctx, managerA, org, assign.ID, 1)
	if err != nil {
		t.Fatalf("revocation with matching version failed: %v", err)
	}
}
