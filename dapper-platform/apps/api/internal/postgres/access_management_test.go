package postgres

import (
	"context"
	"errors"
	"testing"
)

func verifyAccessManagement(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	if _, err := s.ManagedAccess(ctx, a.ActorID, a.OrganizationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("access management implicitly granted")
	}
	var target string
	if err := s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-access-management-target') RETURNING id::text`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2),($1,$3) ON CONFLICT DO NOTHING`, a.OrganizationID, a.ActorID, target); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage')`, a.ActorID, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ManagedAccess(ctx, a.ActorID, b.OrganizationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign organization access leaked")
	}
	for _, item := range []struct{ principal, location, action string }{{b.ActorID, a.LocationID, "customers.read"}, {target, b.LocationID, "customers.read"}, {target, a.LocationID, "unknown.action"}} {
		if err := s.SetLocationGrant(ctx, a.ActorID, a.OrganizationID, item.principal, item.location, item.action, true); !errors.Is(err, ErrForbidden) {
			t.Fatalf("invalid membership/location/action accepted: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := s.SetLocationGrant(ctx, a.ActorID, a.OrganizationID, target, a.LocationID, "customers.read", true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ResolveScope(ctx, target, a.OrganizationID, a.LocationID, "customers.read"); err != nil {
		t.Fatal("explicit managed grant not effective", err)
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM access_grant_events WHERE principal_id=$1 AND action='customers.read' AND enabled`, target).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate grant request duplicated audit", err)
	}
	managed, err := s.ManagedAccess(ctx, a.ActorID, a.OrganizationID)
	if err != nil || len(managed.Members) != 2 {
		t.Fatal("bounded scoped management read failed", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE access_grant_events ADD CONSTRAINT fail_access_grant_audit CHECK(action<>'customers.create') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err = s.SetLocationGrant(ctx, a.ActorID, a.OrganizationID, target, a.LocationID, "customers.create", true); err == nil {
		t.Fatal("audit failure committed grant")
	}
	if _, err := s.ResolveScope(ctx, target, a.OrganizationID, a.LocationID, "customers.create"); !errors.Is(err, ErrForbidden) {
		t.Fatal("failed audit left granted capability")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE access_grant_events DROP CONSTRAINT fail_access_grant_audit`); err != nil {
		t.Fatal(err)
	}
	if err = s.SetLocationGrant(ctx, a.ActorID, a.OrganizationID, target, a.LocationID, "customers.read", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveScope(ctx, target, a.OrganizationID, a.LocationID, "customers.read"); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked managed grant remains effective")
	}
	if err = s.SetOrganizationGrant(ctx, a.ActorID, a.OrganizationID, a.ActorID, "access.manage", false); !errors.Is(err, ErrLastAccessManager) {
		t.Fatal("last access manager revoked its own grant")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE organization_grant_events ADD CONSTRAINT fail_org_grant_event CHECK(action<>'invoices.configure') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err = s.SetOrganizationGrant(ctx, a.ActorID, a.OrganizationID, target, "invoices.configure", true); err == nil {
		t.Fatal("organization grant audit failure committed")
	}
	if _, err = s.InvoiceConfiguration(ctx, target, a.OrganizationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("failed organization audit left financial authority")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE organization_grant_events DROP CONSTRAINT fail_org_grant_event`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.SetOrganizationGrant(ctx, a.ActorID, a.OrganizationID, target, "invoices.configure", true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.InvoiceConfiguration(ctx, target, a.OrganizationID); err != nil {
		t.Fatal("explicit organization finance grant ineffective", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM organization_grant_events WHERE principal_id=$1 AND action='invoices.configure' AND enabled`, target).Scan(&count); err != nil || count != 1 {
		t.Fatal("organization grant audit duplicated", err)
	}
	if err = s.SetOrganizationGrant(ctx, a.ActorID, a.OrganizationID, target, "invoices.configure", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.InvoiceConfiguration(ctx, target, a.OrganizationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked organization finance grant effective")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=false WHERE id=$1`, target); err != nil {
		t.Fatal(err)
	}
	if err = s.SetLocationGrant(ctx, a.ActorID, a.OrganizationID, target, a.LocationID, "customers.read", true); !errors.Is(err, ErrForbidden) {
		t.Fatal("grant enabled for disabled principal")
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM organization_memberships WHERE organization_id=$1 AND principal_id=$2`, a.OrganizationID, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ManagedAccess(ctx, a.ActorID, a.OrganizationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked manager membership retained access")
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM organization_grants WHERE organization_id=$1 AND principal_id=$2 AND action='access.manage'`, a.OrganizationID, a.ActorID); err != nil {
		t.Fatal(err)
	}
}
