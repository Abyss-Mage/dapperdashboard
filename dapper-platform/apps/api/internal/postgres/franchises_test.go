package postgres

import (
	"context"
	"errors"
	"testing"
)

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
	north, err := s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "north", Name: "Synthetic North", Address: Address{Street: "1 Synthetic Way", City: "Peterborough", Region: "ON", PostalCode: "K9J 7C8", Country: "Canada"}})
	if err != nil || north.Code != "NORTH" || north.ID == "" || north.FranchiseID == nil || north.Address.City != "Peterborough" {
		t.Fatalf("franchise with address %+v %v", north, err)
	}
	south, err := s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "SOUTH", Name: "Synthetic South"})
	if err != nil || south.Address != (Address{}) {
		t.Fatalf("franchise without address %+v %v", south, err)
	}
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "OTHER", Name: " synthetic north "}); err == nil {
		t.Fatal("duplicate franchise name accepted")
	}
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "NORTH", Name: "Synthetic Another"}); !errors.Is(err, ErrFranchiseCode) {
		t.Fatal("duplicate franchise code accepted", err)
	}
	if foreign, err := s.CreateFranchise(ctx, b.ActorID, b.OrganizationID, FranchiseInput{Code: "NORTH", Name: "Synthetic Foreign North"}); err != nil || foreign.ID == north.ID {
		t.Fatal("codes must be unique per organization, not globally", err)
	}
	for _, bad := range []string{"", "a", "has space", "WAYTOOLONGCODE1", "bad_code"} {
		if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: bad, Name: "Synthetic Bad Code"}); !errors.Is(err, ErrStructureInput) {
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
	edited, err := s.UpdateFranchise(ctx, a.ActorID, a.OrganizationID, north.ID, FranchiseInput{Code: "NTH2", Name: "Synthetic North Two", Address: Address{City: "Toronto"}})
	if err != nil || edited.Code != "NTH2" || edited.Name != "Synthetic North Two" || edited.Address.City != "Toronto" || edited.Address.Street != "" {
		t.Fatalf("franchise edit %+v %v", edited, err)
	}
	var shadowName string
	if err = s.pool.QueryRow(ctx, `SELECT f.name FROM franchises f JOIN locations l ON l.franchise_id=f.id WHERE l.id=$1`, north.ID).Scan(&shadowName); err != nil || shadowName != "Synthetic North Two" {
		t.Fatal("grouping row did not follow the franchise name", err)
	}
	if _, err = s.UpdateFranchise(ctx, a.ActorID, a.OrganizationID, south.ID, FranchiseInput{Code: "NTH2", Name: "Synthetic South"}); !errors.Is(err, ErrFranchiseCode) {
		t.Fatal("recode to an existing code accepted", err)
	}
	// A pre-existing site with no code or grouping row becomes a normal franchise when edited.
	var legacy string
	if err = s.pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic Legacy Site') RETURNING id::text`, a.OrganizationID).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	adopted, err := s.UpdateFranchise(ctx, a.ActorID, a.OrganizationID, legacy, FranchiseInput{Code: "LEGACY", Name: "Synthetic Legacy Site"})
	if err != nil || adopted.FranchiseID == nil || adopted.Code != "LEGACY" {
		t.Fatalf("legacy site not adopted %+v %v", adopted, err)
	}
	if _, err = s.UpdateFranchise(ctx, b.ActorID, b.OrganizationID, north.ID, FranchiseInput{Code: "HIJACK", Name: "Hijack"}); !errors.Is(err, ErrStructureInput) {
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
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "ROLL", Name: "Rollback franchise"}); err == nil {
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
	if _, err = s.CreateFranchise(ctx, a.ActorID, a.OrganizationID, FranchiseInput{Code: "REVOKED", Name: "Revoked fixture"}); err != ErrForbidden {
		t.Fatal("revoked structure grant retained access")
	}
}
