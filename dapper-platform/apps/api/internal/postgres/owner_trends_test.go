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

func TestOwnerTrendsAreScopedFilteredAndAuthorizedPerBlock(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_trends_%d", time.Now().UnixNano())
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
	id := func(query string, args ...any) string {
		t.Helper()
		var v string
		if err := pool.QueryRow(ctx, query, args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	org := id(`INSERT INTO organizations(name) VALUES('Synthetic Trends Org') RETURNING id::text`)
	site := id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic trends site') RETURNING id::text`, org)
	otherSite := id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic other site') RETURNING id::text`, org)
	actor := id(`INSERT INTO principals(external_subject) VALUES('synthetic:trends-actor') RETURNING id::text`)
	for _, a := range []string{"bookings.read", "jobs.read"} {
		exec(`INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, actor, org, site, a)
	}
	customer := id(`INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Synthetic Customer','synthetic@example.invalid') RETURNING id::text`, org, site)
	otherCustomer := id(`INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Synthetic Other','other@example.invalid') RETURNING id::text`, org, otherSite)
	day := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	book := func(site, customer, channel, status, service string, at time.Time) {
		exec(`INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,$4,$5,$6,$7)`, org, site, customer, service, channel, status, at)
	}
	book(site, customer, "online", "scheduled", "Synthetic wash", day)
	book(site, customer, "online", "completed", "Synthetic wash", day)
	book(site, customer, "walk_in", "scheduled", "Synthetic polish", day.AddDate(0, 0, 1))
	book(site, customer, "phone", "cancelled", "Synthetic wash", day)
	book(otherSite, otherCustomer, "online", "scheduled", "Foreign wash", day)
	exec(`INSERT INTO jobs(organization_id,location_id,customer_id,service_name) VALUES($1,$2,$3,'Synthetic job')`, org, site, customer)
	scope := Scope{ActorID: actor, OrganizationID: org, LocationID: site}

	got, err := s.OwnerTrends(ctx, scope, "2026-10-19", "2026-10-22", "America/Toronto", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Bookings == nil || got.Jobs == nil || got.Payments != nil || got.Invoices != nil {
		t.Fatalf("blocks must follow the grants: %+v", got)
	}
	if got.Bookings.Total != 3 || len(got.Bookings.ByDay) != 4 || got.Bookings.ByDay[1].Count != 2 || got.Bookings.ByDay[2].Count != 1 {
		t.Fatalf("daily booking counts wrong (cancelled excluded, other site excluded): %+v", got.Bookings.ByDay)
	}
	if len(got.Bookings.ByStatus) != 3 || got.Bookings.TopServices[0].Key != "Synthetic wash" || got.Bookings.TopServices[0].Count != 2 {
		t.Fatalf("status/service breakdown wrong: %+v %+v", got.Bookings.ByStatus, got.Bookings.TopServices)
	}
	phone, err := s.OwnerTrends(ctx, scope, "2026-10-19", "2026-10-22", "America/Toronto", "walk_in")
	if err != nil || phone.Bookings.Total != 1 || len(phone.Bookings.ByChannel) != 2 {
		t.Fatalf("channel filter must narrow the series but keep channels comparable: %+v %v", phone.Bookings, err)
	}
	if got.Jobs.Open != 1 {
		t.Fatalf("open job count wrong: %+v", got.Jobs)
	}
	if _, err = s.OwnerTrends(ctx, scope, "2026-10-19", "2026-10-22", "Not/AZone", ""); !errors.Is(err, ErrTrendsInput) {
		t.Fatal("bad timezone accepted", err)
	}
	if _, err = s.OwnerTrends(ctx, scope, "2010-01-01", "2027-12-31", "UTC", ""); !errors.Is(err, ErrTrendsInput) {
		t.Fatal("range over ten years accepted", err)
	}
	if _, err = s.OwnerTrends(ctx, Scope{ActorID: actor, OrganizationID: org, LocationID: otherSite}, "2026-10-19", "2026-10-22", "UTC", ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("another site's trends were readable", err)
	}
}
