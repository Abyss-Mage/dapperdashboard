package postgres

import (
	"context"
	"testing"
)

func verifyCatalogServices(t *testing.T, ctx context.Context, s *Store, a, b Scope, customerID string) {
	t.Helper()
	for _, scope := range []Scope{a, b} {
		for _, action := range []string{"catalogue.read", "catalogue.manage"} {
			if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, scope.ActorID, scope.OrganizationID, scope.LocationID, action); err != nil {
				t.Fatal(err)
			}
		}
	}
	main, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic add-on", Kind: "addon", ParentID: &main.ID})
	if err != nil {
		t.Fatal(err)
	}
	linked, err := s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, ServiceName: "Client-supplied stale name", AddonIDs: []string{child.ID}, Addons: []JobAddon{{ServiceID: child.ID, Name: "Forged name"}}})
	if err != nil || linked.ServiceName != main.Name || linked.ServiceID == nil || *linked.ServiceID != main.ID {
		t.Fatal("main service identity/snapshot not resolved", err)
	}
	if len(linked.Addons) != 1 || linked.Addons[0].Name != child.Name || linked.Addons[0].ServiceID != child.ID {
		t.Fatal("add-on snapshot not resolved by server")
	}
	for _, ids := range [][]string{{child.ID, child.ID}, {main.ID}} {
		if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, AddonIDs: ids}); err != ErrJobAddons {
			t.Fatal("duplicate or main-as-add-on accepted", err)
		}
	}
	otherMain, err := s.SaveService(ctx, a, CatalogService{Name: "Another main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &otherMain.ID, AddonIDs: []string{child.ID}}); err != ErrJobAddons {
		t.Fatal("unrelated main add-on accepted", err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceName: "Manual", AddonIDs: []string{child.ID}}); err != ErrJobAddons {
		t.Fatal("unlinked add-on accepted", err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &child.ID}); err != ErrJobInput {
		t.Fatal("standalone add-on job accepted")
	}
	if _, err = s.SaveService(ctx, b, CatalogService{Name: "Foreign parent", Kind: "addon", ParentID: &main.ID}); err != ErrServiceInput {
		t.Fatal("foreign parent accepted", err)
	}
	if _, err = s.ArchiveService(ctx, a, main.ID, main.Version); err != ErrServiceConflict {
		t.Fatal("parent archived with active children")
	}
	if items, err := s.Services(ctx, b); err != nil || len(items) != 0 {
		t.Fatal("foreign catalogue leaked")
	}
	foreignMain, err := s.SaveService(ctx, b, CatalogService{Name: "Foreign main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	foreignAddon, err := s.SaveService(ctx, b, CatalogService{Name: "Foreign add-on", Kind: "addon", ParentID: &foreignMain.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, AddonIDs: []string{foreignAddon.ID}}); err != ErrJobAddons {
		t.Fatal("cross-scope add-on accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_addon_job_audit_failure CHECK(action<>'job.created') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, AddonIDs: []string{child.ID}, Description: "Add-on rollback fixture"}); err == nil {
		t.Fatal("add-on work order committed without audit")
	}
	var rolledBack int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE description='Add-on rollback fixture'`).Scan(&rolledBack); err != nil || rolledBack != 0 {
		t.Fatal("add-on job audit rollback failed", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_addon_job_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveService(ctx, b, main); err != ErrServiceConflict {
		t.Fatal("foreign service updated")
	}
	main.Name = "Updated main"
	updated, err := s.SaveService(ctx, a, main)
	if err != nil || updated.Version != 2 {
		t.Fatal("revision missing", err)
	}
	if _, err = s.SaveService(ctx, a, main); err != ErrServiceConflict {
		t.Fatal("stale editor overwrote newer revision")
	}
	if _, err = s.ArchiveService(ctx, a, child.ID, child.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, AddonIDs: []string{child.ID}}); err != ErrJobAddons {
		t.Fatal("archived add-on accepted", err)
	}
	archived, err := s.ArchiveService(ctx, a, main.ID, updated.Version)
	if err != nil || !archived.Archived || archived.Version != 3 {
		t.Fatal("archive revision missing", err)
	}
	jobs, err := s.Jobs(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, job := range jobs {
		if job.ID == linked.ID {
			found = job.ServiceName == "Synthetic main" && len(job.Addons) == 1 && job.Addons[0].Name == "Synthetic add-on"
		}
	}
	if !found {
		t.Fatal("service rename changed historical work-order snapshot")
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID}); err != ErrJobInput {
		t.Fatal("archived main accepted for new work")
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM catalog_service_revisions WHERE service_id=$1`, main.ID).Scan(&count); err != nil || count != 3 {
		t.Fatal("service revision history lost")
	}
	if _, err = s.SaveService(ctx, a, CatalogService{Name: "Archived parent", Kind: "addon", ParentID: &main.ID}); err != ErrServiceInput {
		t.Fatal("archived parent accepted")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_service_audit_failure CHECK(action<>'service.revised') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveService(ctx, a, CatalogService{Name: "Rollback fixture", Kind: "main"}); err == nil {
		t.Fatal("service committed without audit")
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM catalog_services WHERE name='Rollback fixture'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("service audit rollback failed")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_service_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='catalogue.manage'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveService(ctx, a, CatalogService{Name: "Revoked fixture", Kind: "main"}); err != ErrForbidden {
		t.Fatal("revoked manager retained access")
	}
}
