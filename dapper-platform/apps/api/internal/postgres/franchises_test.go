package postgres

import (
	"context"
	"errors"
	"testing"
)

var testAddress = Address{Street: "1 Synthetic Way", City: "Peterborough", Region: "ON", PostalCode: "K9J 1A1", Country: "Canada"}

func verifyFranchiseStructure(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	if _, err := s.Structure(ctx, a.ActorID, a.OrganizationID); err != ErrForbidden {
		t.Fatal("location grants implied organization management")
	}
	for _, scope := range []Scope{a, b} {
		for _, action := range []string{"structure.read", "structure.manage"} {
			if _, err := s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,$3)`, scope.ActorID, scope.OrganizationID, action); err != nil {
				t.Fatal(err)
			}
		}
	}
	var centralMain, centralAddon string
	if err := s.pool.QueryRow(ctx, `INSERT INTO central_catalog_services(organization_id,name,kind,price_cents) VALUES($1,'Synthetic central main','main',5000) RETURNING id::text`, a.OrganizationID).Scan(&centralMain); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `INSERT INTO central_catalog_services(organization_id,name,kind,parent_id,price_cents) VALUES($1,'Synthetic central add-on','addon',$2,1000) RETURNING id::text`, a.OrganizationID, centralMain).Scan(&centralAddon); err != nil {
		t.Fatal(err)
	}
	north, err := s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "north", Name: "Synthetic North", Address: Address{Street: "1 Synthetic Way", City: "Peterborough", Region: "ON", PostalCode: "K9J 7C8", Country: "Canada"}})
	if err != nil || north.Code != "NORTH" || north.ID == "" || north.FranchiseID == nil || north.Address.City != "Peterborough" {
		t.Fatalf("franchise with address %+v %v", north, err)
	}
	var copied, linkedAddon int
	if err = s.pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE central_service_id=$2 AND parent_id IS NOT NULL) FROM catalog_services WHERE location_id=$1 AND central_service_id IS NOT NULL`, north.ID, centralAddon).Scan(&copied, &linkedAddon); err != nil || copied != 2 || linkedAddon != 1 {
		t.Fatalf("a new franchise must start with the central catalogue: %v %d %d", err, copied, linkedAddon)
	}
	// A later franchise starts bookable from an existing franchise's bays, and an unused one can still be removed although it starts filled in.
	var northBay string
	if err = s.pool.QueryRow(ctx, `INSERT INTO scheduling_resources(organization_id,location_id,name,capacity,timezone,rules) VALUES($1,$2,'Synthetic bay',2,'America/Toronto','{}'::jsonb) RETURNING id::text`, a.OrganizationID, north.ID).Scan(&northBay); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO service_scheduling_rules(organization_id,location_id,resource_id,service_id,addon_ids,duration_minutes,before_minutes,after_minutes,units,working_time,slots) SELECT organization_id,location_id,$2::uuid,id,'[]'::jsonb,45,0,0,1,false,1 FROM catalog_services WHERE location_id=$1 AND central_service_id=$3`, north.ID, northBay, centralMain); err != nil {
		t.Fatal(err)
	}
	spare, err := s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "SPARE", Name: "Synthetic Spare", Address: testAddress})
	if err != nil {
		t.Fatal(err)
	}
	var spareBays, spareRules int
	if err = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM scheduling_resources WHERE location_id=$1),(SELECT count(*) FROM service_scheduling_rules WHERE location_id=$1 AND duration_minutes=45)`, spare.ID).Scan(&spareBays, &spareRules); err != nil || spareBays != 1 || spareRules != 1 {
		t.Fatalf("a new franchise must be bookable: %v %d bays %d rules", err, spareBays, spareRules)
	}
	if res, e := s.DeleteFranchise(ctx, a.ActorID, a.OrganizationID, spare.ID); e != nil || !res.Permanent {
		t.Fatalf("a franchise holding only copied services must delete: %v %+v", e, res)
	}
	// Leave the organization's central catalogue as the later checks expect it.
	if _, err = s.pool.Exec(ctx, `DELETE FROM service_scheduling_rules WHERE resource_id=$1`, northBay); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM scheduling_resources WHERE id=$1`, northBay); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`DELETE FROM catalog_service_revisions r USING catalog_services c WHERE r.service_id=c.id AND c.central_service_id = ANY($1::uuid[])`,
		`DELETE FROM catalog_services WHERE central_service_id = ANY($1::uuid[])`,
		`DELETE FROM central_catalog_services WHERE id = ANY($1::uuid[])`,
	} {
		if _, err = s.pool.Exec(ctx, q, []string{centralMain, centralAddon}); err != nil {
			t.Fatal(err)
		}
	}
	south, err := s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "SOUTH", Name: "Synthetic South", Address: testAddress})
	if err != nil || south.Address != testAddress {
		t.Fatalf("franchise address not kept %+v %v", south, err)
	}
	// The full address is required, and contact people are saved with the franchise.
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "NOADDR", Name: "Synthetic No Address", Address: Address{City: "Toronto"}}); !errors.Is(err, ErrFranchiseAddress) {
		t.Fatalf("incomplete address accepted: %v", err)
	}
	withContacts, err := s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "CONTACT", Name: "Synthetic Contacts", Address: testAddress, Contacts: []FranchiseContact{{Name: "Synthetic Manager", Email: "manager@example.invalid", Phone: "+15555550102"}, {Name: "Synthetic Deputy"}}})
	if err != nil || len(withContacts.Contacts) != 2 {
		t.Fatalf("contacts not saved %+v %v", withContacts, err)
	}
	listed, err := s.Structure(ctx, a.ActorID, a.OrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range listed {
		if f.ID == withContacts.ID && (len(f.Contacts) != 2 || f.Contacts[0].Name != "Synthetic Manager") {
			t.Fatalf("contacts not listed in order %+v", f.Contacts)
		}
	}
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "OTHER", Name: " synthetic north ", Address: testAddress}); err == nil {
		t.Fatal("duplicate franchise name accepted")
	}
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "NORTH", Name: "Synthetic Another", Address: testAddress}); !errors.Is(err, ErrFranchiseCode) {
		t.Fatal("duplicate franchise code accepted", err)
	}
	if foreign, err := s.CreateFranchise(ctx, b.ActorID, b.OrganizationID, FranchiseInput{Code: "NORTH", Name: "Synthetic Foreign North", Address: testAddress}); err != nil || foreign.ID == north.ID {
		t.Fatal("codes must be unique per organization, not globally", err)
	}
	for _, bad := range []string{"", "a", "has space", "WAYTOOLONGCODE1", "bad_code"} {
		if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: bad, Name: "Synthetic Bad Code", Address: testAddress}); !errors.Is(err, ErrStructureInput) {
			t.Fatalf("invalid code %q accepted: %v", bad, err)
		}
	}
	if _, err = s.Structure(ctx, a.ActorID, b.OrganizationID); err != ErrForbidden {
		t.Fatal("foreign organization structure leaked")
	}
	sites, err := s.Structure(ctx, a.ActorID, a.OrganizationID)
	coded := 0
	for _, f := range sites {
		if f.Code == "NORTH" || f.Code == "SOUTH" {
			coded++
		}
	}
	if err != nil || coded != 2 || len(sites) < 3 {
		t.Fatal("franchises not listed with codes", err, len(sites))
	}

	// Edit: rename, recode and change the address.
	edited, err := s.UpdateFranchise(ctx, a.ActorID, a.OrganizationID, north.ID, FranchiseInput{Code: "NTH2", Name: "Synthetic North Two", Address: Address{Street: "3 Synthetic Way", City: "Toronto", Region: "ON", PostalCode: "M5V 1A1", Country: "Canada"}})
	if err != nil || edited.Code != "NTH2" || edited.Name != "Synthetic North Two" || edited.Address.City != "Toronto" || edited.Address.Street != "3 Synthetic Way" {
		t.Fatalf("franchise edit %+v %v", edited, err)
	}
	var shadowName string
	if err = s.pool.QueryRow(ctx, `SELECT f.name FROM franchises f JOIN locations l ON l.franchise_id=f.id WHERE l.id=$1`, north.ID).Scan(&shadowName); err != nil || shadowName != "Synthetic North Two" {
		t.Fatal("grouping row did not follow the franchise name", err)
	}
	if _, err = s.UpdateFranchise(ctx, a.ActorID, a.OrganizationID, south.ID, FranchiseInput{Code: "NTH2", Name: "Synthetic South", Address: testAddress}); !errors.Is(err, ErrFranchiseCode) {
		t.Fatal("recode to an existing code accepted", err)
	}
	// A pre-existing site with no code or grouping row becomes a normal franchise when edited.
	var legacy string
	if err = s.pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic Legacy Site') RETURNING id::text`, a.OrganizationID).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	adopted, err := s.UpdateFranchise(ctx, a.ActorID, a.OrganizationID, legacy, FranchiseInput{Code: "LEGACY", Name: "Synthetic Legacy Site", Address: testAddress})
	if err != nil || adopted.FranchiseID == nil || adopted.Code != "LEGACY" {
		t.Fatalf("legacy site not adopted %+v %v", adopted, err)
	}
	if _, err = s.UpdateFranchise(ctx, b.ActorID, b.OrganizationID, north.ID, FranchiseInput{Code: "HIJACK", Name: "Hijack", Address: testAddress}); !errors.Is(err, ErrStructureInput) {
		t.Fatal("foreign organization edited a franchise", err)
	}

	// Delete: a site with operational records is protected; an unused one is removed with its grouping row.
	if _, err = s.pool.Exec(ctx, `INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Synthetic customer','c@example.invalid')`, a.OrganizationID, south.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DeleteFranchise(ctx, a.ActorID, a.OrganizationID, south.ID); !errors.Is(err, ErrFranchiseInUse) {
		t.Fatal("franchise with a customer was deleted", err)
	}
	if res, err := s.DeleteFranchise(ctx, a.ActorID, a.OrganizationID, adopted.ID); err != nil || !res.Permanent {
		t.Fatal("unused franchise not deleted", err)
	}
	var left int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM franchises WHERE id=$1`, adopted.FranchiseID).Scan(&left); err != nil || left != 0 {
		t.Fatal("grouping row left behind", err)
	}

	if _, err = s.pool.Exec(ctx, `ALTER TABLE organization_audit_events ADD CONSTRAINT test_structure_audit_failure CHECK(action<>'franchise.created') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Structure(ctx, a.ActorID, a.OrganizationID)
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "ROLL", Name: "Rollback franchise", Address: testAddress}); err == nil {
		t.Fatal("franchise committed without audit")
	}
	after, err := s.Structure(ctx, a.ActorID, a.OrganizationID)
	if err != nil || len(after) != len(before) {
		t.Fatal("structure audit rollback failed")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE organization_audit_events DROP CONSTRAINT test_structure_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM organization_grants WHERE principal_id=$1 AND action='structure.manage'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "REVOKED", Name: "Revoked fixture", Address: testAddress}); err != ErrForbidden {
		t.Fatal("revoked structure grant retained access")
	}
}
