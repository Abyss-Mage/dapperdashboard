package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCentralServiceDependentLocationConflict(t *testing.T) {
	response := httptest.NewRecorder()
	centralServiceFailure(response, postgres.ErrCentralServiceHasChildren)
	if response.Code != http.StatusConflict {
		t.Fatalf("dependent linked services must return a recoverable conflict, got %d", response.Code)
	}
}

func TestPublicContentHTTPBoundary(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_content_http_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error("isolated cleanup failed")
		}
	}()
	config := admin.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	setup, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("test URL unavailable")
	}
	query := parsed.Query()
	query.Set("search_path", namespace)
	parsed.RawQuery = query.Encode()
	store, err := postgres.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal("scoped store unavailable")
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var actor, org, location string
	if err = setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic-content-http') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic content HTTP') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic content HTTP') RETURNING id::text`, org).Scan(&location); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"content.manage", "catalogue.manage"} {
		if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, actor, org, location, action); err != nil {
			t.Fatal(err)
		}
	}
	token, err := store.IssueSession(ctx, actor, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, "https://127.0.0.1:8443")
	base := fmt.Sprintf("/api/v1/organizations/%s/locations/%s/content", org, location)
	send := func(method, path, body string, auth bool, origin string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Host = "127.0.0.1:8121"
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", origin)
		if auth {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		record := httptest.NewRecorder()
		handler.ServeHTTP(record, request)
		return record
	}
	if r := send("GET", base, "", false, ""); r.Code != 401 {
		t.Fatal("management allowed anonymously", r.Code)
	}
	if r := send("GET", base+"/source-images", "", false, ""); r.Code != 401 {
		t.Fatal("source archive exposed anonymously", r.Code)
	}
	if r := send("GET", base+"/source-images", "", true, ""); r.Code != 200 || !strings.Contains(r.Body.String(), "portfolio-18.jpg") {
		t.Fatal("authorized source inventory unavailable", r.Code)
	}
	if r := send("GET", base+"/source-images/portfolio-01.jpg", "", true, ""); r.Code != 200 || r.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal("authorized source photo unavailable", r.Code)
	}
	if r := send("GET", base+"/source-images/not-in-manifest.jpg", "", true, ""); r.Code != 404 {
		t.Fatal("source allowlist bypass accepted", r.Code)
	}
	if r := send("GET", "/api/v1/public/sites/no-site", "", false, ""); r.Code != 404 {
		t.Fatal("unknown publication not hidden", r.Code)
	}
	if r := send("GET", "/api/v1/public/media/not-a-uuid", "", false, ""); r.Code != 404 {
		t.Fatal("invalid media not rejected", r.Code)
	}
	body := `{"slug":"http-dapper","name":"Synthetic HTTP site","published":true,"version":0}`
	if r := send("PUT", base+"/site", body, true, "https://foreign.invalid"); r.Code != 403 {
		t.Fatal("cross-origin write accepted", r.Code)
	}
	if r := send("PUT", base+"/site", strings.TrimSuffix(body, "}")+`,"actorId":"injected"}`, true, "https://127.0.0.1:8443"); r.Code != 400 {
		t.Fatal("actor injection accepted", r.Code)
	}
	if r := send("PUT", base+"/site", body, true, "https://127.0.0.1:8443"); r.Code != 200 {
		t.Fatal("authorized site write failed", r.Code, r.Body.String())
	}
	if r := send("GET", "/api/v1/public/sites/http-dapper", "", false, ""); r.Code != 200 || strings.Contains(r.Body.String(), actor) || strings.Contains(r.Body.String(), org) {
		t.Fatal("public response wrong or leaked actor/scope", r.Code)
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='content.manage'`, actor); err != nil {
		t.Fatal(err)
	}
	if r := send("PUT", base+"/site", body, true, "https://127.0.0.1:8443"); r.Code != 403 {
		t.Fatal("revoked content capability accepted", r.Code)
	}
	if r := send("GET", base+"/source-images/portfolio-01.jpg", "", true, ""); r.Code != 403 {
		t.Fatal("revoked publisher retained archive access", r.Code)
	}
	if r := send(http.MethodPost, base+"/media", "<svg/>", true, "https://127.0.0.1:8443"); r.Code != 403 {
		t.Fatal("upload processed after grant revoke", r.Code)
	}
}
