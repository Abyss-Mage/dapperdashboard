package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestScopeRequiresVerifiedContext(t *testing.T) {
	for _, scope := range []Scope{{}, {OrganizationID: "org"}, {OrganizationID: "org", LocationID: "location"}} {
		if scope.validate() != ErrScope {
			t.Fatal("incomplete scope accepted")
		}
	}
}

func TestPostgresScopedPersistence(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("DAPPER_TEST_DATABASE_URL required for real PostgreSQL evidence")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.pool.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error("could not remove isolated test schema")
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
	if err = s.ValidateSchema(ctx); err == nil {
		t.Fatal("unmigrated schema accepted at runtime")
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal("migration was not repeatable", err)
	}
	if err = s.ValidateSchema(ctx); err != nil {
		t.Fatal("matching schema rejected", err)
	}
	var orgA, orgB, locA, locB string
	if err = s.pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic A') RETURNING id::text`).Scan(&orgA); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic B') RETURNING id::text`).Scan(&orgB); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		org string
		loc *string
	}{{orgA, &locA}, {orgB, &locB}} {
		if err = s.pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Demo location') RETURNING id::text`, item.org).Scan(item.loc); err != nil {
			t.Fatal(err)
		}
	}
	scopeA := Scope{OrganizationID: orgA, LocationID: locA, ActorID: orgA}
	scopeB := Scope{OrganizationID: orgB, LocationID: locB, ActorID: orgB}
	for _, scope := range []Scope{scopeA, scopeB} {
		if _, err = s.pool.Exec(ctx, `INSERT INTO principals(id,external_subject) VALUES($1,$2)`, scope.ActorID, "synthetic:"+scope.ActorID); err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"customers.read", "customers.create", "vehicles.read", "vehicles.create"} {
			if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, scope.ActorID, scope.OrganizationID, scope.LocationID, action); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = s.ResolveScope(ctx, scopeA.ActorID, orgB, locB, "customers.read"); err != ErrForbidden {
		t.Fatal("scope selector granted cross-organization access")
	}
	if _, err = s.ResolveScope(ctx, scopeA.ActorID, orgA, locA, "unknown.action"); err != ErrForbidden {
		t.Fatal("unknown action allowed")
	}
	verifySessionLifecycle(t, ctx, s, scopeA.ActorID)
	verifyOIDCIdentityMapping(t, ctx, s, scopeA.ActorID)
	verifyManagedPasswords(t, ctx, s, scopeA.ActorID)
	c, err := s.CreateCustomer(ctx, scopeA, "Synthetic customer", "test@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	own, err := s.Customers(ctx, scopeA)
	verifyJobPool(t, ctx, s, scopeA, scopeB, c.ID)
	verifyCatalogServices(t, ctx, s, scopeA, scopeB, c.ID)
	verifyFranchiseStructure(t, ctx, s, scopeA, scopeB)
	verifyCapacityReservations(t, ctx, s, scopeA, scopeB)
	verifySchedulingRules(t, ctx, s, scopeA, scopeB)
	verifySchedulingWeeklyHours(t, ctx, s, scopeA, scopeB)
	verifyWorkingTimeRule(t, ctx, s, scopeA)
	verifyCustomerManagement(t, ctx, s, scopeA, scopeB)
	// invoices.configure is organization-level authority: grant it through an organization-scoped role.
	for _, step := range []struct {
		q    string
		args []any
	}{
		{`INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.read') ON CONFLICT DO NOTHING`, []any{scopeA.ActorID, scopeA.OrganizationID, scopeA.LocationID}},
		{`INSERT INTO organization_memberships(organization_id,principal_id) VALUES($2,$1) ON CONFLICT DO NOTHING`, []any{scopeA.ActorID, scopeA.OrganizationID}},
		{`WITH r AS (INSERT INTO roles(organization_id,name) VALUES($2,'Synthetic tax admin') RETURNING id),
 c AS (INSERT INTO role_capabilities(role_id,action) SELECT id,'invoices.configure' FROM r)
 INSERT INTO member_role_assignments(organization_id,principal_id,role_id,scope_type,scope_id,assigned_by) SELECT $2,$1,id,'organization',$2,$1 FROM r`, []any{scopeA.ActorID, scopeA.OrganizationID}},
	} {
		if _, err = s.pool.Exec(ctx, step.q, step.args...); err != nil {
			t.Fatal(err)
		}
	}
	verifyFranchiseTaxes(t, ctx, s, scopeA, scopeB)
	verifyInvoiceTaxCalculation(t, ctx, s, scopeA)
	verifyInvoicePrintTemplate(t, ctx, s, scopeA, scopeB)
	verifyQuotePrintTemplate(t, ctx, s, scopeA, scopeB)
	verifyOutletTemplates(t, ctx, s, scopeA, scopeB)
	verifyLegacyBilling(t, ctx, s, scopeA, scopeB)
	verifyBookingNumbers(t, ctx, s, scopeA, scopeB)
	// Remove the temporary role so later membership clean-up in this test is unaffected.
	for _, q := range []string{
		`DELETE FROM member_role_assignments WHERE role_id IN (SELECT id FROM roles WHERE name='Synthetic tax admin')`,
		`DELETE FROM role_capabilities WHERE role_id IN (SELECT id FROM roles WHERE name='Synthetic tax admin')`,
		`DELETE FROM roles WHERE name='Synthetic tax admin'`,
	} {
		if _, err = s.pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	verifyReservationRead(t, ctx, s, scopeA, scopeB)
	verifyJobContributors(t, ctx, s, scopeA, scopeB, c.ID)
	verifyCentralServices(t, ctx, s, scopeA, scopeB)
	verifyJobWithReservation(t, ctx, s, scopeA, scopeB, c.ID)
	verifyAttendance(t, ctx, s, scopeA, scopeB)
	verifyBookings(t, ctx, s, scopeA, scopeB, c.ID)
	verifyBookingJobs(t, ctx, s, scopeA, scopeB, c.ID)
	verifyBookingCapacity(t, ctx, s, scopeA, scopeB, c.ID)
	verifyWorkingTimeAndAddonBookings(t, ctx, s, scopeA, c.ID)
	verifyBookingReschedule(t, ctx, s, scopeA, scopeB, c.ID)
	verifyBookingOffers(t, ctx, s, scopeA, scopeB, c.ID)
	verifyInvoices(t, ctx, s, scopeA, scopeB, c.ID)
	verifyPaymentSafety(t, ctx, s, scopeA, scopeB, c.ID)
	verifyInvoiceIssuance(t, ctx, s, scopeA, scopeB, c.ID)
	verifyInvoiceVoid(t, ctx, s, scopeA, scopeB, c.ID)
	verifyInvoiceDrafts(t, ctx, s, scopeA, scopeB, c.ID)
	verifyOneInvoicePerBooking(t, ctx, s, scopeA, c.ID)
	verifySOPDocuments(t, ctx, s, scopeA, scopeB)
	if err != nil || len(own) != 1 || own[0].ID != c.ID {
		t.Fatal("own scoped customer missing")
	}
	vehicle, err := s.CreateVehicle(ctx, scopeA, Vehicle{CustomerID: c.ID, Description: "Synthetic aircraft", CategoryDescription: "Aircraft", Identifier: "DEMO-AIR-1"})
	if err != nil {
		t.Fatal("valid owned equipment rejected", err)
	}
	if _, err = s.CreateVehicle(ctx, scopeB, Vehicle{CustomerID: c.ID, Description: "Wrong owner"}); err == nil {
		t.Fatal("cross-scope equipment owner accepted")
	}
	if _, err = s.CreateVehicle(ctx, scopeA, Vehicle{CustomerID: c.ID, Description: "x"}); err != ErrVehicle {
		t.Fatal("invalid vehicle accepted")
	}
	vehicles, err := s.Vehicles(ctx, scopeA)
	if err != nil || len(vehicles) != 1 || vehicles[0].ID != vehicle.ID {
		t.Fatal("owned equipment missing")
	}
	vehicles, err = s.Vehicles(ctx, scopeB)
	if err != nil || len(vehicles) != 0 {
		t.Fatal("equipment crossed organization boundary")
	}
	var vehicleAudits int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE entity_id=$1 AND action='vehicle.created'`, vehicle.ID).Scan(&vehicleAudits); err != nil || vehicleAudits != 1 {
		t.Fatal("vehicle audit missing")
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='vehicles.create'`, scopeA.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateVehicle(ctx, scopeA, Vehicle{CustomerID: c.ID, Description: "Revoked write"}); err != ErrForbidden {
		t.Fatal("revoked equipment grant allowed mutation")
	}
	other, err := s.Customers(ctx, scopeB)
	if err != nil || len(other) != 0 {
		t.Fatal("cross-organization read leaked")
	}
	if _, err = s.CreateCustomer(ctx, Scope{OrganizationID: orgB, LocationID: locA, ActorID: orgB}, "Invalid scope", "bad@example.test", ""); err == nil {
		t.Fatal("cross-organization location accepted")
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO vehicles(organization_id,location_id,customer_id,description) VALUES($1,$2,$3,'Invalid owner link')`, orgB, locB, c.ID); err == nil {
		t.Fatal("cross-scope vehicle link accepted")
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE entity_id=$1 AND action='customer.created'`, c.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("transactional audit missing")
	}
	// Force an audit insert failure after the customer insert; both must roll back. Counted right here, because the checks above add
	// customers of their own (a new email is a new customer even with a phone already on file, error404 #98).
	if own, err = s.Customers(ctx, scopeA); err != nil {
		t.Fatal(err)
	}
	before := len(own)
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_audit_failure CHECK (action <> 'customer.created') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateCustomer(ctx, scopeA, "Rollback customer", "rollback@example.test", ""); err == nil {
		t.Fatal("audit failure did not reject mutation")
	}
	own, err = s.Customers(ctx, scopeA)
	if err != nil || len(own) != before {
		t.Fatal("failed audit did not roll back customer")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=false WHERE id=$1`, scopeA.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Customers(ctx, scopeA); err != ErrForbidden {
		t.Fatal("disabled principal retained access")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=true WHERE id=$1`, scopeA.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='customers.read'`, scopeA.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Customers(ctx, scopeA); err != ErrForbidden {
		t.Fatal("revoked grant retained access")
	}
	verifyCustomerPortal(t, ctx, s, scopeA, scopeB, c.ID)
	verifyAccessManagement(t, ctx, s, scopeA, scopeB)
	verifyMemberInvitations(t, ctx, s, scopeA, scopeB)
	verifyCustomerAccountManagement(t, ctx, s, scopeA, scopeB, c.ID)
	if _, err = s.pool.Exec(ctx, `UPDATE dapper_schema_migrations SET checksum='changed' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if s.ValidateSchema(ctx) == nil {
		t.Fatal("runtime accepted changed migration history")
	}
	if s.Migrate(ctx) == nil {
		t.Fatal("edited migration history accepted")
	}
}
