package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyCustomerAccountManagementHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, handler http.Handler, actor, org, loc, customer, token, origin string) {
	t.Helper()
	var target string
	if err := setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-http-customer-owner') RETURNING id::text`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	verifier, err := postgres.NewPasswordAuthenticator(store)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifier.SetPassword(ctx, target, "http-customer-owner@example.invalid", "Isolated HTTP customer account password"); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, target); err != nil {
		t.Fatal(err)
	}
	ownerSession, err := store.IssueSession(ctx, target, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	send := func(method, path, body, bearer, requestOrigin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", requestOrigin)
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/api/v1/organizations/" + org + "/locations/" + loc + "/customer-accounts"
	portal := "/api/v1/portal/accounts/" + customer + "/records"
	payload := `{"customerId":"` + customer + `","principalId":"` + target + `","enabled":true,"ownershipReviewed":true}`
	if send("PUT", path, payload, "", origin).Code != 401 {
		t.Fatal("anonymous customer linking accepted")
	}
	if send("PUT", path, payload, token, origin).Code != 403 {
		t.Fatal("organization access alone granted customer ownership")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'customer_accounts.manage')`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	if send("GET", portal, "", ownerSession, origin).Code != 403 {
		t.Fatal("enrollment implied customer ownership")
	}
	if send("PUT", path, payload, token, "https://foreign.invalid").Code != 403 {
		t.Fatal("foreign-origin customer linking accepted")
	}
	if send("PUT", path, `{"customerId":"`+customer+`","principalId":"`+target+`","enabled":true}`, token, origin).Code != 422 {
		t.Fatal("HTTP link omitted review")
	}
	if send("PUT", path, payload, token, origin).Code != 200 || send("PUT", path, payload, token, origin).Code != 200 {
		t.Fatal("explicit reviewed customer link failed")
	}
	records := send("GET", portal, "", ownerSession, origin)
	if records.Code != 200 || !strings.Contains(records.Body.String(), customer) {
		t.Fatal("customer portal did not honor explicit ownership", records.Code)
	}
	listed := send("GET", path, "", token, origin)
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), target) || strings.Contains(listed.Body.String(), "password_hash") {
		t.Fatal("customer link management read failed or leaked credential")
	}
	if send("GET", path, "", ownerSession, origin).Code != 403 {
		t.Fatal("customer owner gained staff link administration")
	}
	revoke := `{"customerId":"` + customer + `","principalId":"` + target + `","enabled":false}`
	if send("PUT", path, revoke, token, origin).Code != 200 {
		t.Fatal("HTTP customer revoke failed")
	}
	if send("GET", portal, "", ownerSession, origin).Code != 403 {
		t.Fatal("revoked customer retained record access")
	}
	equipment := send("POST", "/api/v1/portal/accounts/"+customer+"/vehicles", `{"description":"Rejected after revoke","categoryDescription":"Test","identifier":""}`, ownerSession, origin)
	if equipment.Code != 403 {
		t.Fatal("revoked customer retained equipment write")
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='customer_accounts.manage'`, actor); err != nil {
		t.Fatal(err)
	}
	if send("GET", path, "", token, origin).Code != 403 {
		t.Fatal("revoked link manager retained metadata access")
	}
}
