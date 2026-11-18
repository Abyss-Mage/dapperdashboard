package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// error404 #154: what a customer changes on their account page is saved and shown back.
func TestPortalSavesCustomerChanges(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_portal_save_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE") }()
	parsed, _ := url.Parse(dsn)
	q := parsed.Query()
	q.Set("search_path", namespace)
	parsed.RawQuery = q.Encode()
	store, err := postgres.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	config := admin.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var org, loc, staff, customer string
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Portal Org') RETURNING id::text`).Scan(&org))
	must(db.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic portal site') RETURNING id::text`, org).Scan(&loc))
	must(db.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:portal-staff') RETURNING id::text`).Scan(&staff))
	must(db.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic Portal','portal@example.invalid','+17055550101') RETURNING id::text`, org, loc).Scan(&customer))
	principal, _, _, _, err := store.PrepareCustomerAccount(ctx, org, loc, staff, customer)
	must(err)
	token, err := store.IssueSession(ctx, principal, time.Hour)
	must(err)

	origin := "http://127.0.0.1:3006"
	handler := NewHandler(store, origin)
	send := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: token})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	base := "/api/v1/portal/accounts/" + customer
	if w := send("PUT", base+"/details", `{"name":"Synthetic Portal Renamed","phone":"+17055550199","address":"1 Synthetic Road"}`); w.Code != 200 {
		t.Fatalf("details not saved: %d %s", w.Code, w.Body.String())
	}
	if w := send("PUT", base+"/addresses", `{"id":"","label":"Home","address":"2 Synthetic Lane","isDefault":false}`); w.Code != 200 && w.Code != 201 {
		t.Fatalf("address not saved: %d %s", w.Code, w.Body.String())
	}
	// The first saved address becomes the default and the account address, so it replaces the one typed above.
	w := send("GET", base+"/records", "")
	if w.Code != 200 {
		t.Fatalf("records not read: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Account   struct{ Name, Phone, Address string }
		Addresses []struct{ Label, Address string }
	}
	must(json.Unmarshal(w.Body.Bytes(), &got))
	if got.Account.Name != "Synthetic Portal Renamed" || got.Account.Phone != "+17055550199" || got.Account.Address != "2 Synthetic Lane" || len(got.Addresses) != 1 {
		t.Fatalf("saved changes not shown back: %+v", got)
	}
}
