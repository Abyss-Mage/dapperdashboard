package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Most older tests build their own invoices for bookings, so the automatic draft invoice is switched off for them; this test turns it on.
func init() { autoInvoiceEnabled = false }

func TestOwnerWaveFeatures(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_wave_%d", time.Now().UnixNano())
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var org, actor string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Wave Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_wave_actor', true) RETURNING id::text`).Scan(&actor))
	_, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org, actor)
	must(err)
	for _, action := range []string{"structure.read", "structure.manage", "catalogue.read", "catalogue.manage", "invoices.configure"} {
		_, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, $3)`, org, actor, action)
		must(err)
	}
	fran, err := s.CreateFranchise(ctx, actor, org, FranchiseInput{Code: "WAVE", Name: "Wave Franchise", Address: testAddress})
	must(err)
	loc := fran.ID
	for _, action := range []string{"customers.read", "customers.create", "vehicles.read", "vehicles.create", "catalogue.read", "catalogue.manage", "scheduling.manage", "scheduling.reserve", "bookings.create", "bookings.read", "dealership.manage", "dealership.read"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id, organization_id, location_id, action) VALUES($1, $2, $3, $4)`, actor, org, loc, action)
		must(err)
	}
	scope := Scope{ActorID: actor, OrganizationID: org, LocationID: loc}
	customer, err := s.CreateCustomer(ctx, scope, "Synthetic Wave Customer", "wave@example.invalid", "+15555550111")
	must(err)

	t.Run("a service duration becomes a booking rule, and slots set the capacity", func(t *testing.T) {
		minutes := 90
		svc, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic Wave Detail", Kind: "main", DurationMinutes: &minutes})
		must(err)
		if svc.DurationMinutes == nil || *svc.DurationMinutes != 90 {
			t.Fatalf("duration not stored: %+v", svc)
		}
		must(s.SaveBookingSlots(ctx, scope, 3, "America/Toronto"))
		var capacity, rules int
		must(pool.QueryRow(ctx, `SELECT capacity FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&capacity))
		must(pool.QueryRow(ctx, `SELECT count(*) FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND service_id=$3 AND duration_minutes=90 AND addon_ids='[]'::jsonb`, org, loc, svc.ID).Scan(&rules))
		if capacity != 3 || rules != 1 {
			t.Fatalf("slots %d, rules %d", capacity, rules)
		}
		minutes = 120
		svc, err = s.SaveService(ctx, scope, CatalogService{ID: svc.ID, Version: svc.Version, Name: svc.Name, Kind: "main", DurationMinutes: &minutes})
		must(err)
		var version int
		must(pool.QueryRow(ctx, `SELECT version FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND service_id=$3 AND duration_minutes=120`, org, loc, svc.ID).Scan(&version))
		if version != 2 {
			t.Fatalf("changed duration should revise the rule, version %d", version)
		}
		if err = s.SaveBookingSlots(ctx, scope, 0, "America/Toronto"); err == nil {
			t.Fatal("zero slots accepted")
		}
	})

	t.Run("a central service carries its duration to every franchise", func(t *testing.T) {
		minutes := 45
		central, err := s.SaveCentralService(ctx, actor, org, CentralServiceInput{Name: "Synthetic Central Wash", Kind: "main", DurationSet: true, DurationMinutes: &minutes})
		must(err)
		var local *int
		must(pool.QueryRow(ctx, `SELECT duration_minutes FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND central_service_id=$3`, org, loc, central.ID).Scan(&local))
		if local == nil || *local != 45 {
			t.Fatalf("franchise copy has duration %v", local)
		}
		var rules int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND duration_minutes=45`, org, loc).Scan(&rules))
		if rules != 1 {
			t.Fatalf("central duration did not create the franchise rule: %d", rules)
		}
	})

	t.Run("a booking raises a draft invoice and is visible to staff", func(t *testing.T) {
		autoInvoiceEnabled = true
		defer func() { autoInvoiceEnabled = false }()
		booking, err := s.CreateBooking(ctx, scope, Booking{CustomerID: customer.ID, ServiceName: "Synthetic automatic invoice", Channel: "phone", BookingTime: time.Now().Add(48 * time.Hour)})
		must(err)
		var count, lines int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM invoices WHERE booking_id=$1 AND status='draft'`, booking.ID).Scan(&count))
		must(pool.QueryRow(ctx, `SELECT count(*) FROM invoice_items WHERE invoice_id=(SELECT id FROM invoices WHERE booking_id=$1)`, booking.ID).Scan(&lines))
		if count != 1 || lines != 1 {
			t.Fatalf("draft invoices %d, lines %d", count, lines)
		}
		var linked int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM employee_booking_worksites WHERE booking_id=$1 AND dealership_id IS NULL`, booking.ID).Scan(&linked))
		if linked != 1 {
			t.Fatal("booking is not part of the location's staff work")
		}
	})

	t.Run("dealership per-car rate and several contacts", func(t *testing.T) {
		rate := int64(4500)
		dealer, err := s.CreateDealershipAccount(ctx, scope, CreateDealershipInput{CustomerID: customer.ID, AccountName: "Synthetic Motors", AccountCode: "SYNMOT", AccountType: "dealership", PerCarRateCents: &rate,
			Contacts: []DealershipContact{{Name: "Synthetic Sales", Email: "sales@example.invalid", Phone: "+15555550112"}, {Name: "Synthetic Service", Email: "service@example.invalid"}}})
		must(err)
		if dealer.PerCarRateCents == nil || *dealer.PerCarRateCents != 4500 || len(dealer.Contacts) != 2 || dealer.ContactName != "Synthetic Sales" || dealer.TaxExempt || dealer.BillingTermsDays != 30 {
			t.Fatalf("dealership not saved as expected: %+v", dealer)
		}
		if _, err = s.CreateDealershipAccount(ctx, scope, CreateDealershipInput{CustomerID: customer.ID, AccountName: "Synthetic No Contact", AccountCode: "NOCON", AccountType: "dealership"}); err == nil {
			t.Fatal("a dealership with no contact person was accepted")
		}
		updated, err := s.UpdateDealershipAccount(ctx, scope, dealer.ID, UpdateDealershipInput{AccountName: "Synthetic Motors", Active: true, Contacts: []DealershipContact{{Name: "Synthetic Manager", Email: "manager@example.invalid"}}})
		must(err)
		if len(updated.Contacts) != 1 || updated.ContactName != "Synthetic Manager" || updated.PerCarRateCents != nil || updated.BillingTermsDays != 30 {
			t.Fatalf("update did not replace contacts or keep terms: %+v", updated)
		}
		list, err := s.ListDealershipAccounts(ctx, scope)
		must(err)
		if len(list) != 1 || len(list[0].Contacts) != 1 {
			t.Fatalf("list lost the contacts: %+v", list)
		}
	})

	t.Run("square application keys are saved sealed and never read back", func(t *testing.T) {
		if err := s.SaveSquareAppCredentials(ctx, actor, org, "sandbox", "sandbox-app-id", "synthetic-secret-value"); err == nil {
			t.Fatal("keys saved without the server key")
		}
		must(s.SetMFAKey(make([]byte, 32)))
		must(s.SaveSquareAppCredentials(ctx, actor, org, "sandbox", "sandbox-app-id", "synthetic-secret-value"))
		items, err := s.SquareAppStatuses(ctx, actor, org)
		must(err)
		if len(items) != 2 || !items[0].Configured || items[0].ApplicationID != "sandbox-app-id" || items[0].Source != "saved" || items[1].Configured {
			t.Fatalf("statuses %+v", items)
		}
		if !s.SquareOAuthAvailable("sandbox") || s.SquareOAuthAvailable("production") {
			t.Fatal("saved keys are not in use")
		}
		var sealed []byte
		must(pool.QueryRow(ctx, `SELECT secret_sealed FROM square_app_credentials WHERE environment='sandbox'`).Scan(&sealed))
		if string(sealed) == "synthetic-secret-value" || len(sealed) < 29 {
			t.Fatal("secret is not sealed")
		}
		// A restart loads them again.
		s.setSquareApp("sandbox", "", "")
		must(s.LoadSquareAppCredentials(ctx))
		if !s.SquareOAuthAvailable("sandbox") {
			t.Fatal("saved keys not reloaded")
		}
		if err = s.SaveSquareAppCredentials(ctx, "00000000-0000-4000-8000-000000000000", org, "production", "live-app-id", "synthetic-secret-value"); err == nil {
			t.Fatal("a stranger saved keys")
		}
		must(s.ClearSquareAppCredentials(ctx, actor, org, "sandbox"))
		if s.SquareOAuthAvailable("sandbox") {
			t.Fatal("cleared keys still in use")
		}
	})
}
