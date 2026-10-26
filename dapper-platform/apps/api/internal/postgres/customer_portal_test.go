package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func verifyCustomerPortal(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	var actor, foreignCustomer string
	if err := s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic-customer-portal') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Foreign portal customer','test@example.test') RETURNING id::text`, b.OrganizationID, b.LocationID).Scan(&foreignCustomer); err != nil {
		t.Fatal(err)
	}
	// Existing staff grants and matching emails confer no customer ownership.
	if _, err := s.CustomerRecords(ctx, a.ActorID, customer); !errors.Is(err, ErrForbidden) {
		t.Fatalf("staff fallback granted portal access: %v", err)
	}
	if accounts, err := s.CustomerAccounts(ctx, actor); err != nil || len(accounts) != 0 {
		t.Fatalf("unlinked account leaked profiles: %v", err)
	}
	if _, err := s.CustomerRecords(ctx, actor, customer); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unlinked customer access: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, actor, a.OrganizationID, a.LocationID, foreignCustomer); err == nil {
		t.Fatal("cross-scope account link accepted")
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, actor, a.OrganizationID, a.LocationID, customer); err != nil {
		t.Fatal(err)
	}
	accounts, err := s.CustomerAccounts(ctx, actor)
	if err != nil || len(accounts) != 1 || accounts[0].CustomerID != customer {
		t.Fatalf("explicit account missing: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time,notes) VALUES($1,$2,$3,'Synthetic portal booking','internal','pending',now(),'INTERNAL_SECRET_SENTINEL')`, a.OrganizationID, a.LocationID, customer); err != nil {
		t.Fatal(err)
	}
	var published string
	for _, status := range []string{"draft", "issued"} {
		var id string
		if err := s.pool.QueryRow(ctx, `INSERT INTO invoices(organization_id,location_id,customer_id,invoice_number,status,subtotal_cents,tax_cents,total_cents,due_cents,notes) VALUES($1,$2,$3,$4,'draft',1000,0,1000,1000,'INTERNAL_SECRET_SENTINEL') RETURNING id::text`, a.OrganizationID, a.LocationID, customer, "PORTAL-"+status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if status == "issued" {
			if _, err := s.pool.Exec(ctx, `INSERT INTO invoice_items(invoice_id,organization_id,location_id,description,quantity,unit_price_cents,total_price_cents) VALUES($1,$2,$3,'Synthetic portal item',1,1000,1000)`, id, a.OrganizationID, a.LocationID); err != nil {
				t.Fatal(err)
			}
			issueInvoiceForTest(t, ctx, s, a, Invoice{ID: id})
			published = id
		}
	}
	records, err := s.CustomerRecords(ctx, actor, customer)
	if err != nil {
		t.Fatal(err)
	}
	if len(records.Vehicles) == 0 || len(records.Bookings) == 0 {
		t.Fatal("owned records missing")
	}
	found := false
	for _, i := range records.Invoices {
		if i.Status == "draft" {
			t.Fatal("draft invoice disclosed")
		}
		if i.ID == published {
			found = true
		}
	}
	if !found {
		t.Fatal("issued own invoice missing")
	}
	vehicle, err := s.CreateCustomerVehicle(ctx, actor, customer, Vehicle{CustomerID: foreignCustomer, Description: "Synthetic customer-owned boat", CategoryDescription: "Marine", Identifier: "SYNTHETIC-PORTAL"})
	if err != nil || vehicle.CustomerID != customer {
		t.Fatalf("server-resolved vehicle owner failed: %v", err)
	}
	if _, err := s.CreateCustomerVehicle(ctx, actor, foreignCustomer, Vehicle{Description: "Foreign owner attempt"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign customer vehicle mutation: %v", err)
	}
	if _, err := s.CreateCustomerVehicle(ctx, actor, customer, Vehicle{Description: "x"}); !errors.Is(err, ErrVehicle) {
		t.Fatalf("invalid portal vehicle accepted: %v", err)
	}
	var auditCount int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE entity_id=$1 AND actor_id=$2 AND action='vehicle.created'`, vehicle.ID, actor).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatal("portal vehicle audit missing", err)
	}
	if _, err := s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT portal_vehicle_audit_failure CHECK(action <> 'vehicle.created') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCustomerVehicle(ctx, actor, customer, Vehicle{Description: "Must roll back portal equipment"}); err == nil {
		t.Fatal("audit failure did not reject portal write")
	}
	var rolledBack int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE customer_id=$1 AND description='Must roll back portal equipment'`, customer).Scan(&rolledBack); err != nil || rolledBack != 0 {
		t.Fatal("failed audit left portal equipment", err)
	}
	if _, err := s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT portal_vehicle_audit_failure`); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "INTERNAL_SECRET_SENTINEL") || strings.Contains(string(encoded), "providerReference") {
		t.Fatal("staff-only information disclosed")
	}
	if _, err := s.CustomerRecords(ctx, actor, foreignCustomer); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign customer disclosed: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE principals SET active=false WHERE id=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CustomerRecords(ctx, actor, customer); !errors.Is(err, ErrForbidden) {
		t.Fatalf("disabled principal read: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE principals SET active=true WHERE id=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM customer_account_links WHERE principal_id=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CustomerRecords(ctx, actor, customer); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked membership read: %v", err)
	}
	if _, err := s.CreateCustomerVehicle(ctx, actor, customer, Vehicle{Description: "Revoked link attempt"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked membership write: %v", err)
	}
}
