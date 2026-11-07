package postgres

import (
	"errors"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSharedAddons(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_addon_%d", time.Now().UnixNano())
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
	var org, loc, owner string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Addon Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic addon site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:addon-owner') RETURNING id::text`).Scan(&owner))
	_, err = pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'catalogue.manage'),($1,$2,'catalogue.read')`, owner, org)
	must(err)
	for _, action := range []string{"catalogue.read", "catalogue.manage"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, owner, org, loc, action)
		must(err)
	}
	scope := Scope{ActorID: owner, OrganizationID: org, LocationID: loc}
	main := func(name string) CentralService {
		svc, e := s.SaveCentralService(ctx, owner, org, CentralServiceInput{Name: name, Kind: "main"})
		must(e)
		return svc
	}
	a, b, c := main("Synthetic Main A"), main("Synthetic Main B"), main("Synthetic Main C")
	clay, err := s.SaveCentralService(ctx, owner, org, CentralServiceInput{Name: "Synthetic Claybar", Kind: "addon", ParentIDs: []string{a.ID, b.ID}})
	must(err)
	if len(clay.ParentIDs) != 2 || clay.ParentID == nil || *clay.ParentID != a.ID {
		t.Fatalf("add-on parents wrong: %+v", clay)
	}
	if _, err = s.SaveCentralService(ctx, owner, org, CentralServiceInput{Name: "Synthetic Orphan", Kind: "addon"}); !errors.Is(err, ErrCentralServiceInput) {
		t.Fatal("an add-on with no main service was accepted", err)
	}
	if _, err = s.SaveCentralService(ctx, owner, org, CentralServiceInput{Name: "Synthetic Self", Kind: "addon", ParentIDs: []string{clay.ID}}); err == nil {
		t.Fatal("an add-on was offered with another add-on")
	}

	local := func() (CatalogService, map[string]string) {
		list, e := s.Services(ctx, scope)
		must(e)
		byCentral := map[string]string{}
		var addon CatalogService
		for _, sv := range list {
			if sv.CentralServiceID != nil {
				byCentral[*sv.CentralServiceID] = sv.ID
			}
			if sv.CentralServiceID != nil && *sv.CentralServiceID == clay.ID {
				addon = sv
			}
		}
		return addon, byCentral
	}
	addon, ids := local()
	if addon.ID == "" || len(addon.ParentIDs) != 2 {
		t.Fatalf("the franchise copy is not offered with both main services: %+v", addon)
	}
	offered := func(addonID, mainID string) bool {
		var ok bool
		must(pool.QueryRow(ctx, `SELECT catalog_addon_offered_with($1::uuid,$2::uuid)`, addonID, mainID).Scan(&ok))
		return ok
	}
	if !offered(addon.ID, ids[a.ID]) || !offered(addon.ID, ids[b.ID]) || offered(addon.ID, ids[c.ID]) {
		t.Fatal("the franchise check of what is offered with what is wrong")
	}

	// offer it with a third main service and stop offering it with the first (the primary moves)
	clay, err = s.SaveCentralService(ctx, owner, org, CentralServiceInput{ID: clay.ID, Name: "Synthetic Claybar", Kind: "addon", ParentIDs: []string{b.ID, c.ID}, Version: clay.Version})
	must(err)
	if clay.ParentID == nil || *clay.ParentID != b.ID || len(clay.ParentIDs) != 2 {
		t.Fatalf("the primary main service did not move: %+v", clay)
	}
	addon, ids = local()
	if len(addon.ParentIDs) != 2 || offered(addon.ID, ids[a.ID]) || !offered(addon.ID, ids[c.ID]) || addon.ParentID == nil || *addon.ParentID != ids[b.ID] {
		t.Fatalf("the franchise copy did not follow: %+v", addon)
	}

	// archiving a main service it shares only unlinks it; its sole main service still blocks
	list, err := s.ListCentralServices(ctx, owner, org)
	must(err)
	var bVersion int
	for _, sv := range list {
		if sv.ID == b.ID {
			bVersion = sv.Version
		}
	}
	if _, err = s.ArchiveCentralService(ctx, owner, org, b.ID, bVersion); err != nil {
		t.Fatal("a main service shared with another could not be archived", err)
	}
	addon, ids = local()
	if len(addon.ParentIDs) != 1 || addon.ParentIDs[0] != ids[c.ID] {
		t.Fatalf("the add-on should now be offered with the remaining main service only: %+v", addon)
	}
	list, err = s.ListCentralServices(ctx, owner, org)
	must(err)
	var cVersion int
	for _, sv := range list {
		if sv.ID == c.ID {
			cVersion = sv.Version
		}
	}
	if _, err = s.ArchiveCentralService(ctx, owner, org, c.ID, cVersion); !errors.Is(err, ErrCentralServiceHasChildren) {
		t.Fatal("the add-on's only main service was archived from under it", err)
	}
	// duplicates folded by the migration stay out of the lists
	_, err = pool.Exec(ctx, `UPDATE central_catalog_services SET merged_into=$2 WHERE id=$1`, a.ID, c.ID)
	must(err)
	if list, err = s.ListCentralServices(ctx, owner, org); err != nil {
		t.Fatal(err)
	}
	for _, sv := range list {
		if sv.ID == a.ID {
			t.Fatal("a merged duplicate is still listed")
		}
	}
}
