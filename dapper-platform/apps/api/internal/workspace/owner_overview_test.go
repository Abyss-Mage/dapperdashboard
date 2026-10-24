package workspace

import (
	"context"
	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func verifyOwnerOverviewHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, handler http.Handler, org, loc, origin string) {
	t.Helper()
	var actor string
	if err := setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) RETURNING id::text`, fmt.Sprintf("synthetic-overview-%d", time.Now().UnixNano())).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'customers.read')`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	token, err := store.IssueSession(ctx, actor, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/organizations/" + org + "/locations/" + loc + "/overview"
	send := func(route, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", route, nil)
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Origin", origin)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: cookie})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if send(path, "").Code != 401 {
		t.Fatal("anonymous overview accepted")
	}
	response := send(path, token)
	if response.Code != 200 {
		t.Fatalf("overview failed%d: %s", response.Code, response.Body.String())
	}
	var result postgres.OwnerOverview
	if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var expected int64
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM customers WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if result.Metrics["customers"] == nil || *result.Metrics["customers"] != expected {
		t.Fatal("scoped customer metric mismatch")
	}
	for _, key := range []string{"upcomingBookings", "openJobs", "outstandingCents", "netPaymentsCents"} {
		if result.Metrics[key] != nil {
			t.Fatalf("unauthorized metric leaked%s", key)
		}
	}
	if result.WindowEnd.Sub(result.GeneratedAt) != 7*24*time.Hour {
		t.Fatal("overview window mismatch")
	}
	if send("/api/v1/organizations/"+org+"/locations/00000000-0000-4000-8000-000000000099/overview", token).Code != 403 {
		t.Fatal("other location overview accepted")
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if send(path, token).Code != 403 {
		t.Fatal("revoked overview retained access")
	}
}
