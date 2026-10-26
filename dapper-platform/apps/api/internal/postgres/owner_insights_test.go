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

func TestOwnerInsightsPerSectionAndGrant(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_insights_%d", time.Now().UnixNano())
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
	id := func(q string, a ...any) string {
		t.Helper()
		var v string
		if err := pool.QueryRow(ctx, q, a...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	exec := func(q string, a ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	org := id(`INSERT INTO organizations(name) VALUES('Synthetic Insights Org') RETURNING id::text`)
	site := id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic insights site') RETURNING id::text`, org)
	actor := id(`INSERT INTO principals(external_subject) VALUES('synthetic:insights-actor') RETURNING id::text`)
	for _, g := range []string{"customers.read", "vehicles.read", "bookings.read", "jobs.read", "invoices.read"} {
		exec(`INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, actor, org, site, g)
	}
	one := id(`INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Synthetic One','one@example.invalid') RETURNING id::text`, org, site)
	two := id(`INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Synthetic Two','two@example.invalid') RETURNING id::text`, org, site)
	_ = two
	exec(`INSERT INTO vehicles(organization_id,location_id,customer_id,description) VALUES($1,$2,$3,'Synthetic car')`, org, site, one)
	exec(`INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,'Synthetic wash','online','scheduled',now())`, org, site, one)
	exec(`INSERT INTO jobs(organization_id,location_id,customer_id,service_name) VALUES($1,$2,$3,'Synthetic wash')`, org, site, one)
	// The issuance trigger needs issuer snapshots that are irrelevant to this reporting test; bypass it in this disposable schema only.
	exec(`ALTER TABLE invoices DISABLE TRIGGER USER`)
	exec(`INSERT INTO invoices(organization_id,location_id,customer_id,invoice_number,status,subtotal_cents,tax_cents,total_cents,paid_cents,due_cents,issued_at,due_at) VALUES($1,$2,$3,'INV-SYN-1','issued',10000,1300,11300,0,11300,now()-interval '40 days',now()-interval '35 days')`, org, site, one)
	exec(`ALTER TABLE invoices ENABLE TRIGGER USER`)
	scope := Scope{ActorID: actor, OrganizationID: org, LocationID: site}
	from, to := time.Now().AddDate(0, 0, -60).Format("2006-01-02"), time.Now().Format("2006-01-02")

	c, err := s.OwnerInsights(ctx, scope, "customers", from, to, "UTC")
	if err != nil || c.Customers == nil || c.Customers.Total != 2 || c.Customers.New != 2 || c.Customers.Vehicles == nil || *c.Customers.Vehicles != 1 || len(c.Customers.BookingsSplit) != 2 {
		t.Fatalf("customer insights wrong: %+v %v", c.Customers, err)
	}
	w, err := s.OwnerInsights(ctx, scope, "work", from, to, "UTC")
	if err != nil || w.Work == nil || len(w.Work.OpenAge) != 1 || w.Work.OpenAge[0].Key != "Under 1 day" || len(w.Work.ByService) != 1 {
		t.Fatalf("work insights wrong: %+v %v", w.Work, err)
	}
	f, err := s.OwnerInsights(ctx, scope, "finance", from, to, "UTC")
	if err != nil || f.Finance == nil || f.Finance.Invoiced != 11300 || len(f.Finance.Aging) != 1 || f.Finance.Aging[0].Key != "31–60 days overdue" || f.Finance.Aging[0].Due != 11300 || f.Finance.Quotes != nil {
		t.Fatalf("finance insights wrong (quotes must be nil without quotes.read): %+v %v", f.Finance, err)
	}
	if _, err = s.OwnerInsights(ctx, scope, "bogus", from, to, "UTC"); !errors.Is(err, ErrTrendsInput) {
		t.Fatal("unknown section accepted", err)
	}
	other := Scope{ActorID: actor, OrganizationID: org, LocationID: id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic other insights site') RETURNING id::text`, org)}
	if _, err = s.OwnerInsights(ctx, other, "customers", from, to, "UTC"); !errors.Is(err, ErrForbidden) {
		t.Fatal("another site's insights were readable", err)
	}
}
