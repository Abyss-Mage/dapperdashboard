package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEmployeeWorkspaceHTTPBoundary(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	namespace := fmt.Sprintf("dapper_employee_http_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = root.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := root.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", namespace)
	u.RawQuery = q.Encode()
	store, err := postgres.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	config := root.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	setup, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close()
	var actor string
	if err = setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:employee-http') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	token, err := store.IssueSession(ctx, actor, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	origin := "https://127.0.0.1:8443"
	handler := NewHandler(store, origin)
	send := func(method, path, body, auth, requestOrigin string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://127.0.0.1:8121"+path, strings.NewReader(body))
		if auth != "" {
			r.Header.Set("Authorization", "Bearer "+auth)
		}
		if requestOrigin != "" {
			r.Header.Set("Origin", requestOrigin)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := send("GET", "/api/v1/employee/workspace", "", "", ""); w.Code != 401 {
		t.Fatal("anonymous employee view", w.Code)
	}
	if w := send("GET", "/api/v1/employee/workspace", "", token, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"assignment":null`) {
		t.Fatal("unassigned view", w.Code, w.Body.String())
	}
	if w := send("GET", "/api/v1/employee/workspace?locationId="+actor, "", token, ""); w.Code != 400 {
		t.Fatal("employee selector accepted", w.Code)
	}
	if w := send("PUT", "/api/v1/employee/availability", `{"weekStart":"2026-10-26","intervals":[],"version":0,"actorId":"forged"}`, token, origin); w.Code != 400 {
		t.Fatal("actor injection", w.Code)
	}
	if w := send("PUT", "/api/v1/employee/availability", `{"weekStart":"2026-10-26","intervals":[],"version":0}`, token, "https://foreign.invalid"); w.Code != 403 {
		t.Fatal("cross origin", w.Code)
	}
	if w := send("POST", "/api/v1/employee/attendance/clock-in", `{}`, token, origin); w.Code != 409 {
		t.Fatal("unassigned employee clocked in", w.Code)
	}
	// Personal punches are retired (the shop iPad records time), whatever is sent.
	if w := send("POST", "/api/v1/employee/attendance/clock-in", `{"locationId":"`+actor+`"}`, token, origin); w.Code != 409 {
		t.Fatal("personal punch accepted", w.Code)
	}
	if w := send("GET", "/api/v1/organizations/"+actor+"/employee-worksites", "", token, ""); w.Code != 403 {
		t.Fatal("unprivileged admin access", w.Code)
	}
	if w := send("POST", "/api/v1/employee/bookings/"+actor+"/cash", `{"invoiceId":"`+actor+`","amountCents":2500,"idempotencyKey":"test-12345678","method":"square_card"}`, token, origin); w.Code != 400 {
		t.Fatal("method injection", w.Code)
	}
	if w := send("GET", "/api/v1/employee/calendar?from=2026-10-01&to=2026-11-01&locationId="+actor, "", token, ""); w.Code != 400 {
		t.Fatal("calendar worksite injection", w.Code)
	}
	if w := send("GET", "/api/v1/employee/walk-in/options?locationId="+actor, "", token, ""); w.Code != 400 {
		t.Fatal("walkin worksite injection", w.Code)
	}
	if w := send("POST", "/api/v1/employee/walk-in", `{"name":"Person","email":"p@example.ca","phone":"+14165550123","ruleId":"`+actor+`","resourceId":"`+actor+`","ruleVersion":1,"localStart":"2026-10-24T10:00","requestKey":"synthetic-inject-01","dealershipId":"`+actor+`"}`, token, origin); w.Code != 400 {
		t.Fatal("walkin dealership injection", w.Code)
	}
	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		if e := setup.QueryRow(ctx, sql, args...).Scan(&v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	org := id(`INSERT INTO organizations(name) VALUES('Synthetic HTTP employee org') RETURNING id::text`)
	loc := id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic HTTP employee location') RETURNING id::text`, org)
	owner := id(`INSERT INTO principals(external_subject) VALUES('synthetic:employee-http-owner') RETURNING id::text`)
	customer := id(`INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic dealer','dealer@example.invalid','+14165550123') RETURNING id::text`, org, loc)
	dealer := id(`INSERT INTO dealership_accounts(organization_id,location_id,customer_id,account_name,account_code,account_type,contact_name,contact_email) VALUES($1,$2,$3,'Synthetic dealership','HTTP','dealership','Synthetic','dealer@example.invalid') RETURNING id::text`, org, loc, customer)
	if _, e := setup.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2),($1,$3);`, org, owner, actor); e != nil {
		t.Fatal(e)
	}
	if _, e := setup.Exec(ctx, `INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'access.manage')`, org, owner); e != nil {
		t.Fatal(e)
	}
	if _, e := setup.Exec(ctx, `INSERT INTO location_grants(organization_id,location_id,principal_id,action) VALUES($1,$2,$3,'bookings.read')`, org, loc, actor); e != nil {
		t.Fatal(e)
	}
	if _, e := store.AssignEmployeeWorksite(ctx, owner, org, actor, loc, dealer, "America/Toronto", 0); e != nil {
		t.Fatal(e)
	}
	if w := send("GET", "/api/v1/organizations/"+org+"/locations/"+loc+"/bookings", "", token, ""); w.Code != 403 {
		t.Fatal("employee bypassed dealer scope via legacy management API", w.Code)
	}
	if w := send("GET", "/api/v1/employee/calendar?from=2026-10-01&to=2026-11-01", "", token, ""); w.Code != 200 {
		t.Fatal("scoped employee calendar blocked", w.Code, w.Body.String())
	}
}
