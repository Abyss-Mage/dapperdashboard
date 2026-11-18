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

func TestLifecycleControls(t *testing.T) {
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

	namespace := fmt.Sprintf("dapper_lifecycle_%d", time.Now().UnixNano())
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

	var org, actor string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Lifecycle Org') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_lifecycle_actor', true) RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org, actor); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"structure.read", "structure.manage", "catalogue.read", "catalogue.manage"} {
		if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, $3)`, org, actor, action); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("Franchise Lifecycle", func(t *testing.T) {
		fran, err := s.CreateFranchise(ctx, actor, org, FranchiseInput{Code: "WEST", Name: "West Coast", Address: testAddress})
		if err != nil {
			t.Fatalf("CreateFranchise failed: %v", err)
		}
		updated, err := s.UpdateFranchise(ctx, actor, org, fran.ID, FranchiseInput{Code: "PAC", Name: "Pacific Region", Address: Address{Street: "2 Synthetic Way", City: "Vancouver", Region: "BC", PostalCode: "V6B 1A1", Country: "Canada"}})
		if err != nil {
			t.Fatalf("UpdateFranchise failed: %v", err)
		}
		if updated.Name != "Pacific Region" || updated.Code != "PAC" || updated.Address.Region != "BC" {
			t.Fatalf("unexpected franchise: %+v", updated)
		}
		// A franchise with an active franchise-scoped role assignment cannot be deleted.
		var role, member string
		if err = pool.QueryRow(ctx, `INSERT INTO roles(organization_id,name) VALUES($1,'Lifecycle Role') RETURNING id::text`, org).Scan(&role); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('sub_lifecycle_member') RETURNING id::text`).Scan(&member); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, member); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO member_role_assignments(organization_id,principal_id,role_id,scope_type,scope_id,franchise_id,assigned_by) VALUES($1,$2,$3,'franchise',$4,$4,$5)`, org, member, role, updated.FranchiseID, actor); err != nil {
			t.Fatalf("seed role assignment: %v", err)
		}
		if _, err = s.DeleteFranchise(ctx, actor, org, fran.ID); !errors.Is(err, ErrFranchiseInUse) {
			t.Fatalf("expected ErrFranchiseInUse, got: %v", err)
		}
		if _, err = pool.Exec(ctx, `DELETE FROM member_role_assignments WHERE organization_id=$1 AND franchise_id=$2`, org, updated.FranchiseID); err != nil {
			t.Fatal(err)
		}
		delRes, err := s.DeleteFranchise(ctx, actor, org, fran.ID)
		if err != nil || !delRes.Permanent {
			t.Fatalf("expected permanent deletion of an unused franchise: %+v %v", delRes, err)
		}
	})

	t.Run("Location Service Lifecycle", func(t *testing.T) {
		fran, err := s.CreateFranchise(ctx, actor, org, FranchiseInput{Code: "ABN", Name: "Alberta North", Address: testAddress})
		if err != nil {
			t.Fatal(err)
		}
		loc := fran

		for _, action := range []string{"catalogue.manage", "catalogue.read"} {
			if _, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id, organization_id, location_id, action) VALUES($1, $2, $3, $4)`, actor, org, loc.ID, action); err != nil {
				t.Fatal(err)
			}
		}

		scope := Scope{OrganizationID: org, LocationID: loc.ID, ActorID: actor}

		// Create main service
		mainSvc, err := s.SaveService(ctx, scope, CatalogService{Name: "Paint Correction", Kind: "main"})
		if err != nil {
			t.Fatalf("SaveService main failed: %v", err)
		}

		// Create add-on service
		addonSvc, err := s.SaveService(ctx, scope, CatalogService{Name: "Ceramic Sealant", Kind: "addon", ParentID: &mainSvc.ID})
		if err != nil {
			t.Fatalf("SaveService addon failed: %v", err)
		}

		// Delete unreferenced add-on permanently
		delAddon, err := s.DeleteService(ctx, scope, addonSvc.ID, addonSvc.Version)
		if err != nil {
			t.Fatalf("DeleteService addon failed: %v", err)
		}
		if !delAddon.Permanent {
			t.Fatalf("expected permanent deletion for unreferenced addon: %+v", delAddon)
		}

		// Delete unreferenced main service permanently
		delMain, err := s.DeleteService(ctx, scope, mainSvc.ID, mainSvc.Version)
		if err != nil {
			t.Fatalf("DeleteService main failed: %v", err)
		}
		if !delMain.Permanent {
			t.Fatalf("expected permanent deletion for unreferenced main: %+v", delMain)
		}

		// error404 #160: deleting (or archiving) a main service that still has an add-on offered only with it is no longer refused:
		// the add-on is archived, and the main is archived too because that add-on still belongs to it.
		withAddon, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic Main With Add-on", Kind: "main"})
		if err != nil {
			t.Fatal(err)
		}
		onlyAddon, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic Only Add-on", Kind: "addon", ParentID: &withAddon.ID})
		if err != nil {
			t.Fatal(err)
		}
		kept, err := s.DeleteService(ctx, scope, withAddon.ID, withAddon.Version)
		if err != nil || !kept.Archived {
			t.Fatalf("a main with an add-on was not archived: %+v %v", kept, err)
		}
		var addonArchived bool
		if err = s.pool.QueryRow(ctx, `SELECT archived FROM catalog_services WHERE id=$1`, onlyAddon.ID).Scan(&addonArchived); err != nil || !addonArchived {
			t.Fatal("the add-on offered only with it stayed active", err)
		}
	})

	t.Run("Central Service Lifecycle", func(t *testing.T) {
		centralMain, err := s.SaveCentralService(ctx, actor, org, CentralServiceInput{Name: "Central Interior Detail", Kind: "main"})
		if err != nil {
			t.Fatalf("SaveCentralService main failed: %v", err)
		}

		centralAddon, err := s.SaveCentralService(ctx, actor, org, CentralServiceInput{Name: "Leather Conditioning", Kind: "addon", ParentID: &centralMain.ID})
		if err != nil {
			t.Fatalf("SaveCentralService addon failed: %v", err)
		}

		// Delete unreferenced central addon permanently
		delAddon, err := s.DeleteCentralService(ctx, actor, org, centralAddon.ID, centralAddon.Version)
		if err != nil {
			t.Fatalf("DeleteCentralService addon failed: %v", err)
		}
		if !delAddon.Permanent {
			t.Fatalf("expected permanent deletion for unreferenced central addon: %+v", delAddon)
		}

		// Delete unreferenced central main permanently
		delMain, err := s.DeleteCentralService(ctx, actor, org, centralMain.ID, centralMain.Version)
		if err != nil {
			t.Fatalf("DeleteCentralService main failed: %v", err)
		}
		if !delMain.Permanent {
			t.Fatalf("expected permanent deletion for unreferenced central main: %+v", delMain)
		}

		// error404 #151: a franchise copy with a booking rule (from its time to complete) but no bookings does not stop the delete.
		timed, err := s.SaveCentralService(ctx, actor, org, CentralServiceInput{Name: "Central Timed Test", Kind: "main"})
		if err != nil {
			t.Fatal(err)
		}
		site, err := s.CreateFranchise(ctx, actor, org, FranchiseInput{Code: "DELT", Name: "Delete Test", Address: testAddress})
		if err != nil {
			t.Fatal(err)
		}
		var copyID, resource string
		if err = s.pool.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind,central_service_id,duration_minutes) VALUES($1,$2,'Central Timed Test','main',$3,1) ON CONFLICT DO NOTHING RETURNING id::text`, org, site.ID, timed.ID).Scan(&copyID); err != nil {
			if err = s.pool.QueryRow(ctx, `SELECT id::text FROM catalog_services WHERE central_service_id=$1 AND location_id=$2`, timed.ID, site.ID).Scan(&copyID); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.pool.QueryRow(ctx, `INSERT INTO scheduling_resources(organization_id,location_id,name,capacity,timezone,rules) VALUES($1,$2,'Synthetic delete bay',1,'America/Toronto','{}'::jsonb) RETURNING id::text`, org, site.ID).Scan(&resource); err != nil {
			t.Fatal(err)
		}
		if _, err = s.pool.Exec(ctx, `INSERT INTO service_scheduling_rules(organization_id,location_id,resource_id,service_id,addon_ids,duration_minutes,before_minutes,after_minutes,units) VALUES($1,$2,$3,$4,'[]'::jsonb,1,0,0,1)`, org, site.ID, resource, copyID); err != nil {
			t.Fatal(err)
		}
		gone, err := s.DeleteCentralService(ctx, actor, org, timed.ID, 0)
		if err != nil || !gone.Permanent {
			t.Fatalf("an unused central service with a timed franchise copy was not deleted: %+v %v", gone, err)
		}

		// error404 #160: a central service whose franchise copy has the franchise's own add-on can be archived; the add-on goes too.
		withOwn, err := s.SaveCentralService(ctx, actor, org, CentralServiceInput{Name: "Central With Own Add-on", Kind: "main"})
		if err != nil {
			t.Fatal(err)
		}
		siteScope := Scope{OrganizationID: org, LocationID: site.ID, ActorID: actor}
		for _, action := range []string{"catalogue.manage", "catalogue.read"} {
			if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor, org, site.ID, action); err != nil {
				t.Fatal(err)
			}
		}
		ownCopy, err := s.ImportCentralService(ctx, siteScope, withOwn.ID, withOwn.Version)
		if err != nil {
			t.Fatal(err)
		}
		ownAddon, err := s.SaveService(ctx, siteScope, CatalogService{Name: "Synthetic Pet Hair", Kind: "addon", ParentID: &ownCopy.ID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ArchiveCentralService(ctx, actor, org, withOwn.ID, withOwn.Version); err != nil {
			t.Fatal("a central service whose copy has a franchise add-on could not be archived", err)
		}
		var ownArchived bool
		if err = s.pool.QueryRow(ctx, `SELECT archived FROM catalog_services WHERE id=$1`, ownAddon.ID).Scan(&ownArchived); err != nil || !ownArchived {
			t.Fatal("the franchise add-on of the archived copy stayed active", err)
		}
	})
}
