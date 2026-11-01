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

func TestReports(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_report_%d", time.Now().UnixNano())
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
	var org, loc, staff, stranger, ann, ben string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Report Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic report site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:report-staff') RETURNING id::text`).Scan(&staff))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:report-stranger') RETURNING id::text`).Scan(&stranger))
	for _, action := range []string{"bookings.read", "payments.read", "invoices.read", "invoices.create", "customers.read"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, staff, org, loc, action)
		must(err)
	}
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Ann Report','ann@example.invalid','') RETURNING id::text`, org, loc).Scan(&ann))
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Ben Report','','') RETURNING id::text`, org, loc).Scan(&ben))
	scope := Scope{ActorID: staff, OrganizationID: org, LocationID: loc}
	day := time.Now().UTC().Format("2006-01-02")
	book := func(customer, service, status string) string {
		var id string
		must(pool.QueryRow(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,$4,'internal',$5,now()) RETURNING id::text`, org, loc, customer, service, status).Scan(&id))
		return id
	}
	b1 := book(ann, "Synthetic Detail", "completed")
	book(ann, "Synthetic Detail", "cancelled")
	book(ben, "Synthetic Tint", "confirmed")
	inv, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: ann, BookingID: &b1, Items: []CreateInvoiceItemInput{{Description: "Synthetic Detail", Quantity: 1, UnitPriceCents: 20000}}})
	must(err)
	_, err = pool.Exec(ctx, `INSERT INTO payments(organization_id,location_id,invoice_id,customer_id,actor_id,amount_cents,method,status,recorded_at) VALUES($1,$2,$3,$4,$5,15000,'cash','completed',now())`, org, loc, inv.ID, ann, staff)
	must(err)

	// the same moment is a different calendar day in India near midnight
	indiaDay := time.Now().UTC().Add(5*time.Hour + 30*time.Minute).Format("2006-01-02")
	f := ReportFilter{From: day, To: day, TZ: "UTC"}
	if _, err = s.RunReport(ctx, Scope{ActorID: stranger, OrganizationID: org, LocationID: loc}, "appointments", f); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access ran a report", err)
	}
	if _, err = s.RunReport(ctx, scope, "nonsense", f); !errors.Is(err, ErrReportInput) {
		t.Fatal("an unknown report ran", err)
	}
	if _, err = s.RunReport(ctx, scope, "appointments", ReportFilter{From: "2020-01-01", To: "2026-01-01", TZ: "UTC"}); !errors.Is(err, ErrReportInput) {
		t.Fatal("a range of more than two years ran", err)
	}
	if _, err = s.RunReport(ctx, scope, "appointments", ReportFilter{From: day, To: day, TZ: "Not/AZone"}); !errors.Is(err, ErrReportInput) {
		t.Fatal("a bad time zone ran", err)
	}

	all, err := s.RunReport(ctx, scope, "appointments", f)
	must(err)
	if len(all.Rows) != 3 || all.Stats[0].Value != "3" || all.Stats[2].Value != "1" || all.Stats[3].Value != "1" || len(all.Series) != 1 || all.Series[0].Count != 3 || len(all.ServiceNames) != 2 {
		t.Fatalf("appointments report wrong: %+v", all)
	}
	if only, e := s.RunReport(ctx, scope, "appointments", ReportFilter{From: day, To: day, TZ: "UTC", Service: "Synthetic Detail", Status: "completed"}); e != nil || len(only.Rows) != 1 || only.Rows[0][6] != "$200.00" {
		t.Fatal("service and status filters wrong", only.Rows, e)
	}
	if approved, e := s.RunReport(ctx, scope, "appointments", ReportFilter{From: indiaDay, To: indiaDay, TZ: "Asia/Calcutta", Status: "confirmed"}); e != nil || len(approved.Rows) != 1 {
		t.Fatal("approved filter or a legacy time zone name failed", approved.Rows, e)
	}
	money, err := s.RunReport(ctx, scope, "revenue", f)
	must(err)
	if len(money.Rows) != 1 || money.Stats[0].Value != "$150.00" || money.Series[0].Cents != 15000 {
		t.Fatalf("revenue report wrong: %+v", money)
	}
	if none, e := s.RunReport(ctx, scope, "revenue", ReportFilter{From: day, To: day, TZ: "UTC", Service: "Synthetic Tint"}); e != nil || len(none.Rows) != 0 {
		t.Fatal("revenue service filter wrong", none.Rows, e)
	}
	people, err := s.RunReport(ctx, scope, "customers", f)
	must(err)
	if len(people.Rows) != 2 || people.Rows[0][0] != "Ann Report" || people.Rows[0][3] != "2" || people.Stats[1].Value != "2" {
		t.Fatalf("customers report wrong: %+v", people)
	}
}
