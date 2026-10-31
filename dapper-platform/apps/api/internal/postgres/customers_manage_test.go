package postgres

import (
	"context"
	"errors"
	"testing"
)

func verifyCustomerManagement(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	for _, g := range []string{"customers.read", "customers.create", "customer_accounts.manage"} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID, g); err != nil {
			t.Fatal(err)
		}
	}
	// Only this test holds customer_accounts.manage; the shared fixture must not keep it (other checks assert it is absent).
	defer func() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND location_id=$2 AND action='customer_accounts.manage'`, a.ActorID, a.LocationID)
	}()
	free, err := s.CreateCustomer(ctx, a, "Synthetic Removable", "removable@example.invalid", "")
	if err != nil {
		t.Fatal(err)
	}
	used, err := s.CreateCustomer(ctx, a, "Synthetic With History", "history@example.invalid", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO vehicles(organization_id,location_id,customer_id,description) VALUES($1,$2,$3,'Synthetic removable car')`, a.OrganizationID, a.LocationID, free.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO jobs(organization_id,location_id,customer_id,service_name) VALUES($1,$2,$3,'Synthetic history job')`, a.OrganizationID, a.LocationID, used.ID); err != nil {
		t.Fatal(err)
	}
	dir, err := s.CustomerDirectory(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]DirectoryCustomer{}
	for _, c := range dir {
		counts[c.ID] = c
	}
	if counts[free.ID].Vehicles != 1 || counts[used.ID].Jobs != 1 {
		t.Fatalf("directory counts wrong: %+v %+v", counts[free.ID], counts[used.ID])
	}
	up, err := s.UpdateCustomer(ctx, a, free.ID, "Synthetic Renamed", "renamed@example.invalid", "+14165550199")
	if err != nil || up.Name != "Synthetic Renamed" || up.Phone != "+14165550199" {
		t.Fatal("customer edit failed", err)
	}
	if err = s.SetCustomerAddress(ctx, a, free.ID, "  12 Synthetic Road, Peterborough  "); err != nil {
		t.Fatal("customer address not saved", err)
	}
	if dir, err = s.CustomerDirectory(ctx, a); err != nil {
		t.Fatal(err)
	}
	for _, c := range dir {
		if c.ID == free.ID && c.Address != "12 Synthetic Road, Peterborough" {
			t.Fatalf("directory address wrong: %q", c.Address)
		}
	}
	if _, err = s.UpdateCustomer(ctx, a, free.ID, "X", "", ""); !errors.Is(err, ErrCustomerInput) {
		t.Fatal("one-letter name accepted", err)
	}
	if _, err = s.UpdateCustomer(ctx, b, free.ID, "Foreign edit", "", ""); err == nil {
		t.Fatal("another franchise edited this customer")
	}
	// One customer per email and per phone at a franchise.
	if _, err = s.CreateCustomer(ctx, a, "Synthetic Same Email", "HISTORY@example.invalid", ""); !errors.Is(err, ErrCustomerDuplicate) {
		t.Fatal("a second customer with the same email was added", err)
	}
	if _, err = s.CreateCustomer(ctx, a, "Synthetic Same Phone", "", "+1 (416) 555-0199"); !errors.Is(err, ErrCustomerDuplicate) {
		t.Fatal("a second customer with the same phone was added", err)
	}
	if _, err = s.UpdateCustomer(ctx, a, used.ID, "Synthetic With History", "renamed@example.invalid", ""); !errors.Is(err, ErrCustomerDuplicate) {
		t.Fatal("an edit took another customer's email", err)
	}
	// Opening a customer lists their records.
	records, err := s.CustomerRecordsFor(ctx, a, used.ID)
	if err != nil || len(records.Jobs) != 1 || records.Jobs[0].Title != "Synthetic history job" {
		t.Fatalf("customer records wrong: %+v %v", records, err)
	}
	if _, err = s.DeleteCustomer(ctx, b, free.ID); err == nil {
		t.Fatal("another franchise deleted this customer")
	}
	if res, err := s.DeleteCustomer(ctx, a, free.ID); err != nil || !res.Deleted {
		t.Fatal("deleting a customer without history failed", res, err)
	}
	if _, err = s.DeleteCustomer(ctx, a, free.ID); !errors.Is(err, ErrCustomerNotFound) {
		t.Fatal("double delete not reported", err)
	}
	var vehicles int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE customer_id=$1`, free.ID).Scan(&vehicles); err != nil || vehicles != 0 {
		t.Fatal("deleted customer's vehicles remain", err)
	}
	// A customer with a job is deleted together with the job (nothing issued or paid to keep).
	if res, err := s.DeleteCustomer(ctx, a, used.ID); err != nil || !res.Deleted {
		t.Fatal("customer with a job could not be deleted", res, err)
	}
	var jobs int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE customer_id=$1`, used.ID).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatal("deleted customer's jobs remain", err)
	}
}
