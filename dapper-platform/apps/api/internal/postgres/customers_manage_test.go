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
	if _, err = s.UpdateCustomer(ctx, a, free.ID, "X", "", ""); !errors.Is(err, ErrCustomerInput) {
		t.Fatal("one-letter name accepted", err)
	}
	if _, err = s.UpdateCustomer(ctx, b, free.ID, "Foreign edit", "", ""); err == nil {
		t.Fatal("another franchise edited this customer")
	}
	if err = s.DeleteCustomer(ctx, a, used.ID); !errors.Is(err, ErrCustomerInUse) {
		t.Fatal("a customer with a work order was deleted", err)
	}
	if err = s.DeleteCustomer(ctx, b, free.ID); err == nil {
		t.Fatal("another franchise deleted this customer")
	}
	if err = s.DeleteCustomer(ctx, a, free.ID); err != nil {
		t.Fatal("deleting a customer without history failed", err)
	}
	if err = s.DeleteCustomer(ctx, a, free.ID); !errors.Is(err, ErrCustomerNotFound) {
		t.Fatal("double delete not reported", err)
	}
	var vehicles int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE customer_id=$1`, free.ID).Scan(&vehicles); err != nil || vehicles != 0 {
		t.Fatal("deleted customer's vehicles remain", err)
	}
	// Once its history is gone the second customer can be removed too (also leaves the shared fixture unchanged).
	if _, err = s.pool.Exec(ctx, `DELETE FROM jobs WHERE customer_id=$1`, used.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteCustomer(ctx, a, used.ID); err != nil {
		t.Fatal("customer without history could not be deleted", err)
	}
}
