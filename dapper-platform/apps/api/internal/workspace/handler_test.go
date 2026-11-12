package workspace

import (
	"context"
	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
	"bytes"
	"encoding/json"
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

func TestScopedHTTPBoundaryWithPostgres(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("test DB unavailable")
	}
	defer admin.Close()
	var publicBefore string
	if err = admin.QueryRow(ctx, `SELECT COALESCE(to_regclass('public.dapper_schema_migrations')::text,'')`).Scan(&publicBefore); err != nil {
		t.Fatal(err)
	}
	defer func() {
		var after string
		if err := admin.QueryRow(context.Background(), `SELECT COALESCE(to_regclass('public.dapper_schema_migrations')::text,'')`).Scan(&after); err != nil || after != publicBefore {
			t.Error("isolated HTTP test changed the public application schema")
		}
	}()
	namespace := fmt.Sprintf("dapper_http_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error("test schema cleanup failed")
		}
	}()
	config := admin.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	setup, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close()
	// ConnString retains the original text, so mutating pgx RuntimeParams above
	// is not sufficient when opening another pool from a connection string.
	testURL, err := url.Parse(dsn)
	if err != nil || testURL.Host == "" {
		t.Fatal("test database must be configured with a PostgreSQL URL")
	}
	query := testURL.Query()
	query.Set("search_path", namespace)
	testURL.RawQuery = query.Encode()
	store, err := postgres.Open(ctx, testURL.String())
	if err != nil {
		t.Fatal("scoped DB unavailable")
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var actor, org, location, otherOrg, otherLocation string
	if err = setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic-http-user') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		name          string
		org, location *string
	}{{"Synthetic HTTP A", &org, &location}, {"Synthetic HTTP B", &otherOrg, &otherLocation}} {
		if err = setup.QueryRow(ctx, `INSERT INTO organizations(name) VALUES($1) RETURNING id::text`, entry.name).Scan(entry.org); err != nil {
			t.Fatal(err)
		}
		if err = setup.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic location') RETURNING id::text`, *entry.org).Scan(entry.location); err != nil {
			t.Fatal(err)
		}
	}
	for _, action := range []string{"customers.read", "customers.create", "vehicles.read", "vehicles.create", "jobs.read", "jobs.create", "jobs.complete", "catalogue.read", "catalogue.manage"} {
		if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, actor, org, location, action); err != nil {
			t.Fatal(err)
		}
	}
	token, err := store.IssueSession(ctx, actor, time.Hour)
	if err != nil {
		t.Fatal("session setup failed")
	}
	mockSquare := &square.MockClient{}
	handler := NewHandlerWithServices(store, "http://127.0.0.1:3006", nil, mockSquare)
	send := func(method, path, body, auth, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8121"
		if auth != "" {
			r.Header.Set("Authorization", "Bearer "+auth)
		}
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	base := "/api/v1/organizations/" + org + "/locations/" + location + "/"
	otherBase := "/api/v1/organizations/" + otherOrg + "/locations/" + otherLocation + "/"
	origin := "http://127.0.0.1:3006"
	if send("GET", base+"scheduling/resources", "", token, origin).Code != 403 {
		t.Fatal("ungranted scheduling list accepted")
	}
	for _, action := range []string{"scheduling.manage", "scheduling.reserve", "scheduling.release"} {
		if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, actor, org, location, action); err != nil {
			t.Fatal(err)
		}
	}
	verifySchedulingHTTP(t, send, base, otherBase, token, origin)
	readPath := base + "scheduling/reservations?from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z"
	if send("GET", readPath, "", token, origin).Code != 403 {
		t.Fatal("write grants implied reservation read")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'scheduling.read')`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	if response := send("GET", readPath, "", token, origin); response.Code != 200 || !strings.Contains(response.Body.String(), `"nextCursor"`) || !strings.Contains(response.Body.String(), `"occupiedStart"`) {
		t.Fatal("reservation page response failed", response.Code)
	}
	if send("GET", readPath, "", "", origin).Code != 401 {
		t.Fatal("anonymous reservation read accepted")
	}
	if send("GET", otherBase+"scheduling/reservations?from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z", "", token, origin).Code != 403 {
		t.Fatal("foreign reservation read accepted")
	}
	if send("GET", readPath+"&cursor=invalid!", "", token, origin).Code != 422 {
		t.Fatal("invalid cursor accepted by HTTP")
	}
	if send("GET", base+"scheduling/reservations", "", token, origin).Code != 422 {
		t.Fatal("missing explicit range accepted")
	}

	orgBase := "/api/v1/organizations/" + org + "/"
	if send("GET", orgBase+"structure", "", token, origin).Code != 403 {
		t.Fatal("location grants implied structure access")
	}
	for _, action := range []string{"structure.read", "structure.manage"} {
		if _, err = setup.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,$3)`, actor, org, action); err != nil {
			t.Fatal(err)
		}
	}
	accessResponse := send("GET", "/api/v1/auth/access", "", token, origin)
	if accessResponse.Code != 200 || !strings.Contains(accessResponse.Body.String(), location) || strings.Contains(accessResponse.Body.String(), otherOrg) {
		t.Fatal("access discovery leaked or omitted granted scope")
	}
	if send("GET", orgBase+"structure", "", "", origin).Code != 401 {
		t.Fatal("anonymous structure access")
	}
	fresponse := send("POST", orgBase+"franchises", `{"code":"httpf","name":"Synthetic HTTP franchise","address":{"street":"1 Synthetic Way","city":"Peterborough","region":"ON","postalCode":"K9J 1A1","country":"Canada"}}`, token, origin)
	if fresponse.Code != 201 {
		t.Fatal("franchise route failed", fresponse.Code, fresponse.Body.String())
	}
	var franchise postgres.Franchise
	if json.Unmarshal(fresponse.Body.Bytes(), &franchise) != nil || franchise.ID == "" || franchise.Code != "HTTPF" || franchise.Address.City != "Peterborough" {
		t.Fatal("invalid franchise response", fresponse.Body.String())
	}
	if send("POST", orgBase+"locations", `{"name":"Synthetic HTTP location","franchiseId":"`+franchise.ID+`"}`, token, origin).Code == 201 {
		t.Fatal("a separate location can still be created")
	}
	if send("POST", orgBase+"franchises", `{"code":"OTHR","name":"Synthetic HTTP franchise","address":{"street":"1 Synthetic Way","city":"Peterborough","region":"ON","postalCode":"K9J 1A1","country":"Canada"}}`, token, origin).Code != 409 {
		t.Fatal("duplicate franchise name not actionable")
	}
	if send("POST", orgBase+"franchises", `{"code":"HTTPF","name":"Synthetic duplicate code","address":{"street":"1 Synthetic Way","city":"Peterborough","region":"ON","postalCode":"K9J 1A1","country":"Canada"}}`, token, origin).Code != 409 {
		t.Fatal("duplicate franchise code not actionable")
	}
	if send("POST", orgBase+"franchises", `{"name":"No code"}`, token, origin).Code != 422 {
		t.Fatal("franchise without a code accepted")
	}
	if send("POST", orgBase+"franchises", `{"code":"INJ","name":"Injected actor","actorId":"forged"}`, token, origin).Code != 400 {
		t.Fatal("structure actor injection accepted")
	}
	if send("PUT", orgBase+"franchises/"+franchise.ID, `{"code":"HTTPF","name":"Synthetic HTTP franchise","address":{"street":"1 Test Rd","city":"Peterborough","region":"ON","postalCode":"K9J 1A1","country":"Canada"},"taxRegistration":"SYN-HST-1"}`, token, origin).Code != 200 {
		t.Fatal("franchise edit failed")
	}
	if send("GET", "/api/v1/organizations/"+otherOrg+"/structure", "", token, origin).Code != 403 {
		t.Fatal("foreign organization structure leaked")
	}
	if list := send("GET", orgBase+"structure", "", token, origin); list.Code != 200 || !strings.Contains(list.Body.String(), "HTTPF") || !strings.Contains(list.Body.String(), "1 Test Rd") {
		t.Fatal("persisted structure missing")
	}
	if listed := send("GET", orgBase+"structure", "", token, origin); listed.Code != 200 || !strings.Contains(listed.Body.String(), "SYN-HST-1") {
		t.Fatal("franchise HST number was not saved through the route", listed.Code)
	}
	if del := send("DELETE", orgBase+"franchises/"+franchise.ID, "", token, origin); del.Code != 200 {
		t.Fatal("unused franchise could not be deleted", del.Code, del.Body.String())
	}
	if send("GET", orgBase+"central-services", "", token, origin).Code != 403 {
		t.Fatal("central services implicitly allowed")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'catalogue.read'),($1,$2,'catalogue.manage')`, actor, org); err != nil {
		t.Fatal(err)
	}
	centralMainRes := send("POST", orgBase+"central-services", `{"name":"Central HTTP Detail","kind":"main"}`, token, origin)
	if centralMainRes.Code != 201 {
		t.Fatal("central main creation failed", centralMainRes.Code, centralMainRes.Body.String())
	}
	var centralMain postgres.CentralService
	if json.Unmarshal(centralMainRes.Body.Bytes(), &centralMain) != nil || centralMain.ID == "" {
		t.Fatal("invalid central main response")
	}
	centralAddonRes := send("POST", orgBase+"central-services", `{"name":"Central HTTP Wax","kind":"addon","parentId":"`+centralMain.ID+`"}`, token, origin)
	if centralAddonRes.Code != 201 {
		t.Fatal("central addon creation failed", centralAddonRes.Code)
	}
	if list := send("GET", orgBase+"central-services", "", token, origin); list.Code != 200 || !strings.Contains(list.Body.String(), "Central HTTP Detail") {
		t.Fatal("central services list failed", list.Code)
	}
	if send("GET", "/api/v1/auth/login", "", "", origin).Code != 503 {
		t.Fatal("unconfigured login exposed a session route")
	}
	cookieRequest := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	cookieRequest.Host = "127.0.0.1:8121"
	cookieRequest.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: token})
	cookieResponse := httptest.NewRecorder()
	handler.ServeHTTP(cookieResponse, cookieRequest)
	if cookieResponse.Code != 200 {
		t.Fatal("verified workspace cookie rejected")
	}
	cookieRequest.Header.Set("Authorization", "malformed")
	cookieResponse = httptest.NewRecorder()
	handler.ServeHTTP(cookieResponse, cookieRequest)
	if cookieResponse.Code != 401 {
		t.Fatal("malformed explicit authorization fell back to cookie")
	}
	if send("GET", base+"customers", "", "", origin).Code != 401 {
		t.Fatal("anonymous scoped access")
	}
	if send("GET", base+"customers", "", "invalid", origin).Code != 401 {
		t.Fatal("invalid token accepted")
	}
	if send("GET", otherBase+"customers", "", token, origin).Code != 403 {
		t.Fatal("path selector granted foreign scope")
	}
	if send("POST", base+"customers", `{"name":"Synthetic customer"}`, token, "https://untrusted.example").Code != 403 {
		t.Fatal("cross-origin mutation")
	}
	if send("POST", base+"customers", `{"name":"Synthetic customer","actorId":"forged"}`, token, origin).Code != 400 {
		t.Fatal("actor injection accepted")
	}
	if send("POST", base+"customers", `{"name":"x","email":"invalid"}`, token, origin).Code != 422 {
		t.Fatal("invalid customer accepted")
	}
	if send("POST", base+"customers", `{"name":"Valid"}{"name":"Extra"}`, token, origin).Code != 400 {
		t.Fatal("multiple JSON objects accepted")
	}
	created := send("POST", base+"customers", `{"name":"Synthetic HTTP customer","email":"http@example.test","phone":""}`, token, origin)
	if created.Code != 201 {
		t.Fatal("customer creation failed", created.Code)
	}
	var customer postgres.Customer
	if json.Unmarshal(created.Body.Bytes(), &customer) != nil || customer.ID == "" {
		t.Fatal("invalid customer response")
	}

	if send("GET", base+"jobs", "", "", origin).Code != 401 {
		t.Fatal("anonymous job list accepted")
	}
	if send("GET", otherBase+"jobs", "", token, origin).Code != 403 {
		t.Fatal("foreign job scope accepted")
	}

	if send("GET", base+"services", "", "", origin).Code != 401 {
		t.Fatal("anonymous service access")
	}
	if send("GET", otherBase+"services", "", token, origin).Code != 403 {
		t.Fatal("foreign catalogue access")
	}
	createdService := send("POST", base+"services", `{"name":"Synthetic HTTP service","kind":"main"}`, token, origin)
	if createdService.Code != 201 {
		t.Fatal("service creation failed", createdService.Code)
	}
	var service postgres.CatalogService
	if json.Unmarshal(createdService.Body.Bytes(), &service) != nil || service.ID == "" {
		t.Fatal("invalid service response")
	}
	if send("PUT", base+"services/"+service.ID, `{"name":"Edited service","kind":"main","version":1,"actorId":"forged"}`, token, origin).Code != 400 {
		t.Fatal("service actor injection accepted")
	}
	if send("PUT", base+"services/"+service.ID, `{"name":"Edited service","kind":"main","version":1}`, token, origin).Code != 200 {
		t.Fatal("service edit failed")
	}
	if send("PUT", base+"services/"+service.ID, `{"name":"Stale editor","kind":"main","version":1}`, token, origin).Code != 409 {
		t.Fatal("stale edit accepted")
	}
	linked := send("POST", base+"jobs", `{"customerId":"`+customer.ID+`","serviceId":"`+service.ID+`"}`, token, origin)
	if linked.Code != 201 || !strings.Contains(linked.Body.String(), `"serviceName":"Edited service"`) || !strings.Contains(linked.Body.String(), `"serviceId":"`+service.ID+`"`) {
		t.Fatal("linked job snapshot not resolved", linked.Code)
	}
	if send("POST", base+"jobs", `{"customerId":"`+customer.ID+`","serviceId":"`+service.ID+`","serviceName":"Forged"}`, token, origin).Code != 422 {
		t.Fatal("client service name override accepted")
	}
	addonResponse := send("POST", base+"services", `{"name":"HTTP add-on","kind":"addon","parentId":"`+service.ID+`"}`, token, origin)
	var addon postgres.CatalogService
	if addonResponse.Code != 201 || json.Unmarshal(addonResponse.Body.Bytes(), &addon) != nil {
		t.Fatal("add-on fixture failed")
	}
	withAddon := send("POST", base+"jobs", `{"customerId":"`+customer.ID+`","serviceId":"`+service.ID+`","addonIds":["`+addon.ID+`"]}`, token, origin)
	var addonJob postgres.Job
	if withAddon.Code != 201 || json.Unmarshal(withAddon.Body.Bytes(), &addonJob) != nil || len(addonJob.Addons) != 1 || addonJob.Addons[0].Name != "HTTP add-on" {
		t.Fatal("HTTP add-on snapshot failed", withAddon.Code)
	}
	for _, payload := range []string{
		`{"customerId":"` + customer.ID + `","serviceId":"` + service.ID + `","addonIds":["` + addon.ID + `","` + addon.ID + `"]}`,
		`{"customerId":"` + customer.ID + `","serviceId":"` + service.ID + `","addonIds":["invalid"]}`,
		`{"customerId":"` + customer.ID + `","serviceName":"Manual","addonIds":["` + addon.ID + `"]}`,
	} {
		if send("POST", base+"jobs", payload, token, origin).Code != 422 {
			t.Fatal("invalid add-on selection accepted")
		}
	}
	if send("POST", base+"jobs", `{"customerId":"`+customer.ID+`","serviceId":"`+service.ID+`","addons":[{"name":"forged"}]}`, token, origin).Code != 400 {
		t.Fatal("add-on name injection accepted")
	}
	if send("POST", base+"services/"+addon.ID+"/archive", `{"version":1}`, token, origin).Code != 200 {
		t.Fatal("add-on archive failed")
	}
	if send("POST", base+"jobs", `{"customerId":"`+customer.ID+`","serviceId":"`+service.ID+`","addonIds":["`+addon.ID+`"]}`, token, origin).Code != 422 {
		t.Fatal("archived HTTP add-on accepted")
	}
	if send("POST", base+"services/"+service.ID+"/archive", `{"version":2}`, token, origin).Code != 200 {
		t.Fatal("archive failed")
	}
	if list := send("GET", base+"services", "", token, origin); list.Code != 200 || !strings.Contains(list.Body.String(), `"archived":true`) {
		t.Fatal("archive identity missing")
	}
	if send("POST", base+"jobs", `{"customerId":"`+customer.ID+`","serviceId":"`+service.ID+`"}`, token, origin).Code != 422 {
		t.Fatal("archived service accepted for new work")
	}
	if send("POST", base+"jobs", `{"customerId":"`+customer.ID+`","serviceName":"Invalid res","reservationId":"00000000-0000-0000-0000-000000000000"}`, token, origin).Code != 422 {
		t.Fatal("nonexistent reservation accepted for job")
	}
	createdJob := send("POST", base+"jobs", `{"customerId":"`+customer.ID+`","serviceName":"Synthetic HTTP job"}`, token, origin)
	if createdJob.Code != 201 {
		t.Fatal("job creation failed", createdJob.Code, createdJob.Body.String())
	}
	var job postgres.Job
	if json.Unmarshal(createdJob.Body.Bytes(), &job) != nil || job.ID == "" {
		t.Fatal("invalid job response")
	}
	if send("POST", base+"jobs/"+job.ID+"/contribute", `{}`, token, origin).Code != 403 {
		t.Fatal("contribution implicitly granted")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'jobs.contribute')`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	if send("POST", base+"jobs/"+job.ID+"/contribute", `{"principalId":"forged"}`, token, origin).Code != 400 {
		t.Fatal("contributor spoof accepted")
	}
	for i := 0; i < 2; i++ {
		if response := send("POST", base+"jobs/"+job.ID+"/contribute", `{}`, token, origin); response.Code != 200 || !strings.Contains(response.Body.String(), actor) {
			t.Fatal("session contribution failed", response.Code)
		}
	}
	if send("POST", base+"jobs/"+job.ID+"/complete", `{"completedBy":"forged"}`, token, origin).Code != 400 {
		t.Fatal("job actor injection accepted")
	}
	if send("POST", base+"jobs/"+job.ID+"/complete", `{}`, token, "https://untrusted.example").Code != 403 {
		t.Fatal("cross-origin completion accepted")
	}
	if send("POST", base+"jobs/"+job.ID+"/complete", `{}`, token, origin).Code != 200 {
		t.Fatal("completion failed")
	}
	if send("POST", base+"jobs/"+job.ID+"/complete", `{}`, token, origin).Code != 409 {
		t.Fatal("duplicate completion accepted")
	}
	if list := send("GET", base+"jobs", "", token, origin); list.Code != 200 || !strings.Contains(list.Body.String(), `"completedBy":"`+actor+`"`) {
		t.Fatal("job performer missing")
	}
	if send("POST", otherBase+"vehicles", `{"customerId":"`+customer.ID+`","description":"Wrong scope"}`, token, origin).Code != 403 {
		t.Fatal("foreign scope mutation accepted")
	}
	if send("POST", base+"vehicles", `{"customerId":"`+customer.ID+`","description":"Synthetic boat"}`, token, origin).Code != 201 {
		t.Fatal("vehicle creation failed")
	}
	var foreignCustomer string
	if err = setup.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Other synthetic customer','') RETURNING id::text`, otherOrg, otherLocation).Scan(&foreignCustomer); err != nil {
		t.Fatal(err)
	}
	foreignLink := send("POST", base+"vehicles", `{"customerId":"`+foreignCustomer+`","description":"Invalid foreign link"}`, token, origin)
	if foreignLink.Code != 422 || strings.Contains(foreignLink.Body.String(), "SQLSTATE") {
		t.Fatal("foreign owner link error leaked or was accepted")
	}
	listed := send("GET", base+"customers", "", token, origin)
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), "Synthetic HTTP customer") || listed.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("scoped persisted list failed")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES
($1,$2,$3,'attendance.record'),
($1,$2,$3,'attendance.read') ON CONFLICT DO NOTHING`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	if send("GET", base+"attendance/active", "", token, origin).Code != 200 {
		t.Fatal("active attendance read failed")
	}
	// Time is recorded on the shop iPad now: the personal clock-in is refused, and the PIN clock does the whole shift.
	if w := send("POST", base+"attendance/clock-in", `{"notes":"HTTP morning shift"}`, token, origin); w.Code != 409 || !strings.Contains(w.Body.String(), "shop_clock") {
		t.Fatal("personal clock in still accepted", w.Code, w.Body.String())
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'attendance.manage') ON CONFLICT DO NOTHING`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	// The shop iPad lists the franchise's team by role, as the Employees page does (error404 #48).
	var clockRole string
	if err = setup.QueryRow(ctx, `INSERT INTO roles(organization_id,name,description,version) VALUES($1,'Synthetic clock staff','Synthetic test role',1) RETURNING id::text`, org).Scan(&clockRole); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO role_capabilities(role_id,action) VALUES($1,'attendance.record')`, clockRole); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, org, actor); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO member_role_assignments(organization_id,principal_id,role_id,scope_type,scope_id,location_id,version,assigned_by) VALUES($1,$2,$3,'location',$4,$4,1,$2)`, org, actor, clockRole, location); err != nil {
		t.Fatal(err)
	}
	if w := send("PUT", base+"shop-clock/staff/"+actor+"/pin", `{"pin":"4821"}`, token, origin); w.Code != 422 {
		t.Fatal("a PIN that is not 6 digits was accepted", w.Code)
	}
	if w := send("PUT", base+"shop-clock/staff/"+actor+"/pin", `{"pin":"482193"}`, token, origin); w.Code != 200 {
		t.Fatal("PIN not saved", w.Code, w.Body.String())
	}
	if w := send("GET", base+"shop-clock/staff", "", token, origin); w.Code != 200 || !strings.Contains(w.Body.String(), `"hasPin":true`) || strings.Contains(w.Body.String(), "482193") {
		t.Fatal("PIN list wrong or shows the PIN", w.Code, w.Body.String())
	}
	if w := send("PUT", base+"shop-clock/login", `{"username":"Synthetic.Shop","password":"short"}`, token, origin); w.Code != 422 {
		t.Fatal("a short shop password was accepted", w.Code)
	}
	if w := send("PUT", base+"shop-clock/login", `{"username":"Synthetic.Shop","password":"synthetic-shop-pass"}`, token, origin); w.Code != 200 {
		t.Fatal("shop login not saved", w.Code, w.Body.String())
	}
	if w := send("GET", base+"shop-clock/login", "", token, origin); w.Code != 200 || !strings.Contains(w.Body.String(), `"username":"synthetic.shop"`) || strings.Contains(w.Body.String(), "synthetic-shop-pass") {
		t.Fatal("shop login read wrong", w.Code, w.Body.String())
	}
	// The iPad: its own cookie jar, no bearer token.
	jar := map[string]string{}
	ipad := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		for k, v := range jar {
			r.AddCookie(&http.Cookie{Name: k, Value: v})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		for _, c := range w.Result().Cookies() {
			if c.MaxAge < 0 {
				delete(jar, c.Name)
			} else {
				jar[c.Name] = c.Value
			}
		}
		return w
	}
	if w := ipad("GET", "/api/v1/shop/workers?release=1", ""); w.Code != 401 {
		t.Fatal("worker list shown without the shop sign-in", w.Code)
	}
	if w := ipad("POST", "/api/v1/shop/login", `{"username":"synthetic.shop","password":"wrong-password-x"}`); w.Code != 401 {
		t.Fatal("wrong shop password accepted", w.Code)
	}
	if w := ipad("POST", "/api/v1/shop/login", `{"username":"synthetic.shop","password":"synthetic-shop-pass"}`); w.Code != 200 || jar["__Host-dapper_shop"] == "" {
		t.Fatal("shop sign-in failed", w.Code, w.Body.String())
	}
	// The shop sees every workspace page: signing in opens the workspace as the franchise's Shop account (not the owner, not a worker).
	if w := ipad("GET", "/api/v1/employee/workspace", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"employee":"Shop iPad"`) || !strings.Contains(w.Body.String(), `"bookings.read"`) {
		t.Fatal("the shop login did not open the Shop workspace", w.Code, w.Body.String())
	}
	if w := send("GET", base+"shop-clock/staff", "", token, origin); w.Code != 200 || strings.Contains(w.Body.String(), "Shop iPad") || strings.Contains(w.Body.String(), "system:shop") {
		t.Fatal("the Shop account shows in the staff PIN list", w.Code, w.Body.String())
	}
	if w := ipad("POST", "/api/v1/shop/workers/"+actor+"/availability", `{"pin":"000001","week":"2026-11-09"}`); w.Code != 401 {
		t.Fatal("availability opened with a wrong PIN", w.Code, w.Body.String())
	}
	if w := ipad("GET", "/api/v1/shop/workers?release=1", ""); w.Code != 200 || !strings.Contains(w.Body.String(), actor) {
		t.Fatal("worker list wrong", w.Code, w.Body.String())
	}
	if w := ipad("GET", "/api/v1/shop/me", ""); w.Code != 401 {
		t.Fatal("shop bar answered before a PIN", w.Code)
	}
	unlock := func(pin string) *httptest.ResponseRecorder {
		return ipad("POST", "/api/v1/shop/workers/"+actor+"/unlock", `{"pin":"`+pin+`"}`)
	}
	if w := unlock("482193"); w.Code != 200 || jar["__Host-dapper_workspace"] == "" || jar["dapper_shop_worker"] != "1" {
		t.Fatal("PIN unlock failed", w.Code, w.Body.String())
	}
	// The workspace's clock page: name + PIN punches without opening the dashboard; a wrong PIN is refused.
	if w := ipad("POST", "/api/v1/shop/workers/"+actor+"/punch", `{"pin":"482193","action":"status"}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"shift":null`) {
		t.Fatal("PIN status on the clock page failed", w.Code, w.Body.String())
	}
	// The unlocked session is the worker's own dashboard session.
	if w := ipad("GET", "/api/v1/auth/me", ""); w.Code != 200 || !strings.Contains(w.Body.String(), actor) {
		t.Fatal("worker session not usable for the dashboard", w.Code, w.Body.String())
	}
	if w := ipad("GET", "/api/v1/shop/me", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"shift":null`) {
		t.Fatal("status before clocking in wrong", w.Code, w.Body.String())
	}
	for _, step := range []struct{ action, status string }{{"clock-in", "clocked_in"}, {"start-break", "on_break"}, {"end-break", "clocked_in"}, {"clock-out", "clocked_out"}} {
		if w := ipad("POST", "/api/v1/shop/me", `{"action":"`+step.action+`"}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"`+step.status+`"`) {
			t.Fatal("shop "+step.action+" failed", w.Code, w.Body.String())
		}
	}
	// Staff hours in the browser's zone, including an old alias the database may not know (error404 #47).
	if w := send("GET", base+"attendance/totals?from="+time.Now().AddDate(0, 0, -3).Format("2006-01-02")+"&to="+time.Now().AddDate(0, 0, 3).Format("2006-01-02")+"&tz=Asia%2FCalcutta", "", token, origin); w.Code != 200 || !strings.Contains(w.Body.String(), actor) {
		t.Fatal("staff hours in Asia/Calcutta failed", w.Code, w.Body.String())
	}
	if w := ipad("POST", "/api/v1/shop/me", `{"action":"clock-out"}`); w.Code != 409 {
		t.Fatal("clock out without a shift accepted", w.Code)
	}
	// The owner's own signed-in session cannot punch through the shop bar (it was not opened by a PIN on the iPad).
	if w := send("GET", "/api/v1/shop/me", "", token, origin); w.Code != 401 {
		t.Fatal("a normal session used the shop bar", w.Code)
	}
	worker := jar["__Host-dapper_workspace"]
	if w := ipad("POST", "/api/v1/shop/done", ""); w.Code != 204 || jar["__Host-dapper_workspace"] != "" {
		t.Fatal("Done did not sign the worker out", w.Code)
	}
	if w := send("GET", "/api/v1/auth/me", "", worker, origin); w.Code != 401 {
		t.Fatal("the worker session still works after Done", w.Code)
	}
	// An owner's PIN never opens their dashboard on the shared iPad.
	if _, err = setup.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage') ON CONFLICT DO NOTHING`, actor, org); err != nil {
		t.Fatal(err)
	}
	if w := unlock("482193"); w.Code != 403 || jar["__Host-dapper_workspace"] != "" {
		t.Fatal("an owner was signed in with a shop PIN", w.Code, w.Body.String())
	}
	if _, err = setup.Exec(ctx, `DELETE FROM organization_grants WHERE principal_id=$1 AND organization_id=$2 AND action='access.manage'`, actor, org); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ { // one wrong PIN was already tried on availability
		if w := unlock("000000"); w.Code != 401 {
			t.Fatal("wrong PIN not refused", w.Code)
		}
	}
	if w := unlock("482193"); w.Code != 429 {
		t.Fatal("wrong PINs not limited", w.Code)
	}
	// Changing the shop password signs the iPad out.
	if w := send("PUT", base+"shop-clock/login", `{"username":"synthetic.shop","password":"another-shop-pass"}`, token, origin); w.Code != 200 {
		t.Fatal("shop login change failed", w.Code)
	}
	if w := ipad("GET", "/api/v1/shop/workers?release=1", ""); w.Code != 401 {
		t.Fatal("the iPad stayed signed in after the password changed", w.Code)
	}
	if _, err = setup.Exec(ctx, `DELETE FROM member_role_assignments WHERE role_id=$1`, clockRole); err != nil {
		t.Fatal(err)
	}
	if hist := send("GET", base+"attendance/history", "", token, origin); hist.Code != 200 || !strings.Contains(hist.Body.String(), `"status":"clocked_out"`) {
		t.Fatal("attendance history failed", hist.Code, hist.Body.String())
	}

	// Dealership invoices over HTTP: a draft with VIN lines, the filtered list, its PDF and the ZIP of the list; another franchise is refused.
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) SELECT $1,$2,$3,a FROM unnest(ARRAY['dealership.read','dealership.manage','invoices.read','invoices.create','invoices.manage','customers.create','customers.read']) a ON CONFLICT DO NOTHING`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	dlCustomer := send("POST", base+"customers", `{"name":"Synthetic HTTP Motors","email":"motors@example.test","phone":""}`, token, origin)
	var dlc struct{ ID string }
	if dlCustomer.Code != 201 || json.Unmarshal(dlCustomer.Body.Bytes(), &dlc) != nil {
		t.Fatal("dealership customer failed", dlCustomer.Code, dlCustomer.Body.String())
	}
	dlRes := send("POST", base+"dealerships", `{"customerId":"`+dlc.ID+`","accountName":"Synthetic HTTP Motors","accountCode":"HTTPMOT","accountType":"dealership","contacts":[{"name":"Synthetic Buyer","email":"buyer@example.test"}]}`, token, origin)
	var dl struct{ ID string }
	if dlRes.Code != 201 || json.Unmarshal(dlRes.Body.Bytes(), &dl) != nil {
		t.Fatal("dealership failed", dlRes.Code, dlRes.Body.String())
	}
	if w := send("POST", base+"dealer-invoices", `{"dealershipId":"`+dl.ID+`","discountCents":9000,"lines":[{"description":"Lot detail","quantity":1,"unitPriceCents":4500}]}`, token, origin); w.Code != 422 {
		t.Fatal("a discount above the subtotal was accepted", w.Code, w.Body.String())
	}
	dlInv := send("POST", base+"dealer-invoices", `{"dealershipId":"`+dl.ID+`","poNumber":"PO-1","discountPercent":5,"lines":[{"description":"Lot detail","detail":"Full interior","quantity":2,"unitPriceCents":4500,"vin":"1HGCM82633A004352","stockNumber":"S-1"}]}`, token, origin)
	var di struct {
		ID    string
		Lines []struct{ VIN string }
	}
	if dlInv.Code != 201 || json.Unmarshal(dlInv.Body.Bytes(), &di) != nil || len(di.Lines) != 1 || di.Lines[0].VIN != "1HGCM82633A004352" {
		t.Fatal("dealership invoice failed", dlInv.Code, dlInv.Body.String())
	}
	if w := send("GET", base+"dealer-invoices?dealershipId="+dl.ID+"&status=draft", "", token, origin); w.Code != 200 || !strings.Contains(w.Body.String(), di.ID) {
		t.Fatal("dealership invoice list failed", w.Code, w.Body.String())
	}
	if w := send("GET", base+"dealer-invoices?status=nonsense", "", token, origin); w.Code != 422 {
		t.Fatal("unknown status filter accepted", w.Code)
	}
	if w := send("GET", base+"dealer-invoices/"+di.ID+"/pdf", "", token, origin); w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("dealership invoice PDF failed", w.Code)
	}
	if w := send("GET", base+"dealer-invoices/zip?dealershipId="+dl.ID, "", token, origin); w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte("PK")) {
		t.Fatal("dealership invoice ZIP failed", w.Code, w.Body.String())
	}
	if w := send("GET", "/api/v1/public/invoice-pay/"+strings.Repeat("A", 43), "", "", origin); w.Code != 404 {
		t.Fatal("an unknown pay link opened something", w.Code, w.Body.String())
	}
	if w := send("POST", "/api/v1/organizations/"+org+"/dealer-invoice-template/sample?location="+location, `{"template":{"title":"Invoice","accentColor":"#E5222F","showLogo":true,"billToLabel":"Bill to","itemLabel":"Item","quantityLabel":"Qty","unitPriceLabel":"Rate","amountLabel":"Amount","paperSize":"letter","footerNote":"Thank you"}}`, token, origin); w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) || !bytes.Contains(w.Body.Bytes(), []byte("Client Details")) {
		t.Fatal("dealership template sample failed", w.Code, w.Body.String())
	}
	if w := send("POST", base+"dealer-invoices/preview", `{"discountPercent":10,"lines":[{"quantity":2,"unitPriceCents":4500}]}`, token, origin); w.Code != 200 || !strings.Contains(w.Body.String(), `"discountCents":900`) {
		t.Fatal("dealership invoice preview failed", w.Code, w.Body.String())
	}
	if w := send("GET", base+"dealer-invoices/"+di.ID+"/held-payments", "", token, origin); w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal("held payments list failed", w.Code, w.Body.String())
	}
	if w := send("POST", base+"dealer-invoices/"+di.ID+"/release-held-payments", "", token, origin); w.Code != 200 || !strings.Contains(w.Body.String(), `"released":0`) {
		t.Fatal("releasing nothing failed", w.Code, w.Body.String())
	}
	if w := send("POST", base+"dealer-invoices/"+di.ID+"/reopen", "", token, origin); w.Code != 409 {
		t.Fatal("a draft was reopened", w.Code, w.Body.String())
	}
	// The dealership portal: the owner gives a login; the portal answers only with its own cookie.
	if w := send("POST", base+"dealerships/"+dl.ID+"/portal-users", `{"name":"Synthetic Buyer","email":"http-buyer@example.test","password":"portal-pass-123"}`, token, origin); w.Code != 201 {
		t.Fatal("portal login not created", w.Code, w.Body.String())
	}
	if w := send("GET", base+"dealerships/"+dl.ID+"/portal-users", "", token, origin); w.Code != 200 || !strings.Contains(w.Body.String(), "http-buyer@example.test") || strings.Contains(w.Body.String(), "portal-pass-123") {
		t.Fatal("portal login list wrong", w.Code, w.Body.String())
	}
	if w := send("GET", "/api/v1/dealer-portal/invoices", "", token, origin); w.Code != 401 {
		t.Fatal("the portal answered a staff session", w.Code)
	}
	dealerPortal := func(method, path, body, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "__Host-dapper_dealer", Value: cookie})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := dealerPortal("POST", "/api/v1/dealer-portal/login", `{"email":"http-buyer@example.test","password":"wrong-pass-123"}`, ""); w.Code != 401 {
		t.Fatal("wrong portal password accepted", w.Code)
	}
	login := dealerPortal("POST", "/api/v1/dealer-portal/login", `{"email":"http-buyer@example.test","password":"portal-pass-123"}`, "")
	var dealerCookie string
	for _, c := range login.Result().Cookies() {
		if c.Name == "__Host-dapper_dealer" {
			dealerCookie = c.Value
		}
	}
	if login.Code != 200 || dealerCookie == "" {
		t.Fatal("portal sign in failed", login.Code, login.Body.String())
	}
	if w := dealerPortal("GET", "/api/v1/dealer-portal/me", "", dealerCookie); w.Code != 200 || !strings.Contains(w.Body.String(), "Synthetic HTTP Motors") {
		t.Fatal("portal me wrong", w.Code, w.Body.String())
	}
	if w := dealerPortal("GET", "/api/v1/dealer-portal/invoices", "", dealerCookie); w.Code != 200 || strings.Contains(w.Body.String(), di.ID) {
		t.Fatal("portal list failed or showed a draft", w.Code, w.Body.String())
	}
	if w := dealerPortal("GET", "/api/v1/dealer-portal/invoices/"+di.ID, "", dealerCookie); w.Code != 404 {
		t.Fatal("portal opened a draft", w.Code)
	}
	if w := dealerPortal("GET", "/api/v1/dealer-portal/statement?from=2026-01-01&to=2026-12-31&status=all", "", dealerCookie); w.Code != 200 {
		t.Fatal("portal statement failed", w.Code, w.Body.String())
	}
	if w := dealerPortal("GET", "/api/v1/dealer-portal/statement/pdf?from=2026-01-01&to=2026-12-31&payments=1&aging=1", "", dealerCookie); w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("portal statement PDF failed", w.Code)
	}
	if w := dealerPortal("POST", "/api/v1/dealer-portal/pay-many", `{"invoiceIds":[],"cardId":"x","idempotencyKey":"key-12345"}`, dealerCookie); w.Code != 422 {
		t.Fatal("paying no invoices was accepted", w.Code, w.Body.String())
	}
	if w := dealerPortal("POST", "/api/v1/dealer-portal/pay-many", `{"invoiceIds":["`+di.ID+`"],"cardId":"x","idempotencyKey":"key-12345"}`, dealerCookie); w.Code != 404 {
		t.Fatal("a draft was offered for payment", w.Code, w.Body.String())
	}
	if w := dealerPortal("GET", "/api/v1/dealer-portal/cards", "", dealerCookie); w.Code != 200 {
		t.Fatal("portal cards failed", w.Code, w.Body.String())
	}
	if w := dealerPortal("POST", "/api/v1/dealer-portal/link", `{"token":"`+strings.Repeat("A", 43)+`"}`, ""); w.Code != 404 {
		t.Fatal("an unknown email link opened the portal", w.Code)
	}
	if w := dealerPortal("POST", "/api/v1/dealer-portal/logout", "", dealerCookie); w.Code != 204 {
		t.Fatal("portal sign out failed", w.Code)
	}
	if w := dealerPortal("GET", "/api/v1/dealer-portal/me", "", dealerCookie); w.Code != 401 {
		t.Fatal("portal session survived sign out", w.Code)
	}
	if w := send("GET", otherBase+"dealer-invoices", "", token, origin); w.Code != 403 {
		t.Fatal("another franchise's dealership invoices were listed", w.Code)
	}
	// The checks below decide invoice rights for themselves.
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND organization_id=$2 AND location_id=$3 AND action IN ('dealership.read','dealership.manage','invoices.read','invoices.create','invoices.manage')`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	if send("GET", otherBase+"attendance/history", "", token, origin).Code != 403 {
		t.Fatal("foreign attendance history leaked")
	}

	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES
($1,$2,$3,'bookings.create'),
($1,$2,$3,'bookings.read'),
($1,$2,$3,'bookings.manage') ON CONFLICT DO NOTHING`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	bookingSvcRes := send("POST", base+"services", `{"name":"Executive Ceramic Service","kind":"main"}`, token, origin)
	if bookingSvcRes.Code != 201 {
		t.Fatal("failed to create service for booking", bookingSvcRes.Code)
	}
	var bookingSvc postgres.CatalogService
	if json.Unmarshal(bookingSvcRes.Body.Bytes(), &bookingSvc) != nil {
		t.Fatal("invalid service unmarshal")
	}
	bookingPayload := `{"customerId":"` + customer.ID + `","serviceId":"` + bookingSvc.ID + `","channel":"internal","status":"confirmed","bookingTime":"2026-11-15T10:00:00Z","notes":"HTTP Executive booking"}`
	createBookingRes := send("POST", base+"bookings", bookingPayload, token, origin)
	if createBookingRes.Code != 201 {
		t.Fatal("booking creation failed", createBookingRes.Code, createBookingRes.Body.String())
	}
	var createdBooking postgres.Booking
	if json.Unmarshal(createBookingRes.Body.Bytes(), &createdBooking) != nil || createdBooking.ID == "" {
		t.Fatal("invalid created booking JSON")
	}
	conversionPath := base + "bookings/" + createdBooking.ID + "/work-orders"
	conversionPayload := `{"requestKey":"http-booking-conversion-key-001","description":"HTTP booking conversion"}`
	if send("POST", conversionPath, `{"requestKey":"http-booking-injection-key-001","customerId":"`+customer.ID+`"}`, token, origin).Code != 400 {
		t.Fatal("conversion accepted browser-owned customer fields")
	}
	converted := send("POST", conversionPath, conversionPayload, token, origin)
	if converted.Code != 201 {
		t.Fatal("booking conversion failed", converted.Code)
	}
	var convertedJob, retriedJob postgres.Job
	if json.Unmarshal(converted.Body.Bytes(), &convertedJob) != nil || convertedJob.BookingID == nil || *convertedJob.BookingID != createdBooking.ID || convertedJob.CustomerID != customer.ID {
		t.Fatal("conversion did not resolve server booking snapshot")
	}
	retried := send("POST", conversionPath, conversionPayload, token, origin)
	if retried.Code != 201 || json.Unmarshal(retried.Body.Bytes(), &retriedJob) != nil || retriedJob.ID != convertedJob.ID {
		t.Fatal("HTTP conversion retry created different work")
	}
	if send("POST", conversionPath, `{"requestKey":"http-booking-conversion-key-001","description":"Changed request"}`, token, origin).Code != 409 {
		t.Fatal("HTTP conversion key conflict accepted")
	}
	if bList := send("GET", base+"bookings", "", token, origin); bList.Code != 200 || !strings.Contains(bList.Body.String(), "HTTP Executive booking") {
		t.Fatal("booking list failed", bList.Code, bList.Body.String())
	}
	if bDetail := send("GET", base+"bookings/"+createdBooking.ID, "", token, origin); bDetail.Code != 200 || !strings.Contains(bDetail.Body.String(), createdBooking.ID) {
		t.Fatal("booking detail failed", bDetail.Code, bDetail.Body.String())
	}
	updateStatusRes := send("POST", base+"bookings/"+createdBooking.ID+"/status", `{"toStatus":"scheduled","notes":"Bay assigned"}`, token, origin)
	if updateStatusRes.Code != 200 {
		t.Fatal("booking status update failed", updateStatusRes.Code, updateStatusRes.Body.String())
	}
	if send("POST", otherBase+"bookings", bookingPayload, token, origin).Code != 403 {
		t.Fatal("foreign booking creation leaked")
	}

	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='customers.read'`, actor); err != nil {
		t.Fatal(err)
	}
	if send("GET", base+"customers", "", token, origin).Code != 403 {
		t.Fatal("revoked grant remained effective over HTTP")
	}
	if send("GET", "/api/v1/portal/accounts", "", "", origin).Code != 401 {
		t.Fatal("anonymous customer portal request accepted")
	}
	portalPath := "/api/v1/portal/accounts/" + customer.ID + "/records"
	if send("GET", portalPath, "", token, origin).Code != 403 {
		t.Fatal("staff session bypassed explicit customer membership")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, actor, org, location, customer.ID); err != nil {
		t.Fatal(err)
	}
	portal := send("GET", portalPath, "", token, origin)
	if portal.Code != 200 || portal.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("explicit customer account did not resolve through session", portal.Code)
	}
	var customerRecords postgres.PortalRecords
	if json.Unmarshal(portal.Body.Bytes(), &customerRecords) != nil || customerRecords.Account.CustomerID != customer.ID {
		t.Fatal("customer portal returned wrong owner")
	}
	if send("POST", "/api/v1/portal/accounts", `{"customerId":"`+customer.ID+`"}`, token, origin).Code != 405 {
		t.Fatal("public customer self-linking must remain unavailable")
	}
	vehiclePortalPath := "/api/v1/portal/accounts/" + customer.ID + "/vehicles"
	if send("POST", vehiclePortalPath, `{"description":"Synthetic portal trailer","customerId":"`+customer.ID+`"}`, token, origin).Code != 400 {
		t.Fatal("portal owner injection accepted")
	}
	if send("POST", vehiclePortalPath, `{"description":"Synthetic portal trailer"}`, token, "https://foreign.invalid").Code != 403 {
		t.Fatal("portal foreign origin accepted")
	}
	portalVehicle := send("POST", vehiclePortalPath, `{"description":"Synthetic portal trailer","categoryDescription":"Trailer","identifier":"SYNTHETIC-HTTP"}`, token, origin)
	if portalVehicle.Code != 201 {
		t.Fatal("linked portal vehicle creation failed", portalVehicle.Code)
	}
	var createdPortalVehicle postgres.Vehicle
	if json.Unmarshal(portalVehicle.Body.Bytes(), &createdPortalVehicle) != nil || createdPortalVehicle.CustomerID != customer.ID {
		t.Fatal("wrong portal vehicle owner")
	}
	if _, err = setup.Exec(ctx, `DELETE FROM customer_account_links WHERE principal_id=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if send("GET", portalPath, "", token, origin).Code != 403 {
		t.Fatal("revoked customer link remained effective over HTTP")
	}
	if send("POST", vehiclePortalPath, `{"description":"Revoked portal trailer"}`, token, origin).Code != 403 {
		t.Fatal("revoked link allowed portal vehicle creation")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.create'),($1,$2,$3,'invoices.read'),($1,$2,$3,'invoices.issue'),($1,$2,$3,'payments.record') ON CONFLICT DO NOTHING`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	fiscalResponse := send("POST", base+"invoices", `{"customerId":"`+customer.ID+`","invoiceNumber":"HTTP-CASH-SAFETY","taxCents":0,"notes":"INTERNAL-FINANCE-SENTINEL","items":[{"description":"Synthetic HTTP cash safety","quantity":1,"unitPriceCents":1000}]}`, token, origin)
	var fiscalInvoice postgres.Invoice
	if fiscalResponse.Code != 201 || json.Unmarshal(fiscalResponse.Body.Bytes(), &fiscalInvoice) != nil {
		t.Fatal("HTTP cash fixture invoice creation failed", fiscalResponse.Code)
	}
	if fiscalInvoice.Status != "draft" || fiscalInvoice.IssuedAt != nil {
		t.Fatal("HTTP invoice skipped draft review")
	}
	if send("GET", base+"invoices/"+fiscalInvoice.ID+"/issuance-preview", ``, token, origin).Code != 409 {
		t.Fatal("invoice issued without configured issuer")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, org, actor); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'invoices.configure')`, actor, org); err != nil {
		t.Fatal(err)
	}
	issuerBase := "/api/v1/organizations/" + org + "/invoice-issuers"
	issuerResponse := send("POST", issuerBase, `{"legalName":"Isolated HTTP issuer","address":"Isolated HTTP issuer address","taxRegistration":"TEST-REG","currency":"CAD","numberPrefix":"HTTP-","startNumber":1}`, token, origin)
	var httpIssuer postgres.InvoiceIssuer
	if issuerResponse.Code != 200 || json.Unmarshal(issuerResponse.Body.Bytes(), &httpIssuer) != nil || httpIssuer.ID == "" {
		t.Fatal("HTTP issuer configuration failed", issuerResponse.Code)
	}
	if send("PUT", issuerBase+"/locations/"+location, `{"issuerId":"`+httpIssuer.ID+`"}`, token, origin).Code != 200 {
		t.Fatal("HTTP issuer binding failed")
	}
	if send("POST", base+"invoices/"+fiscalInvoice.ID+"/issue", `{"invoiceNumber":"FORGED"}`, token, origin).Code != 400 {
		t.Fatal("HTTP issuance accepted forged number")
	}
	previewResponse := send("GET", base+"invoices/"+fiscalInvoice.ID+"/issuance-preview", ``, token, origin)
	if previewResponse.Code != 200 {
		t.Fatal("HTTP issuer review preview unavailable")
	}
	if send("POST", base+"invoices/"+fiscalInvoice.ID+"/issue", `{}`, token, origin).Code != 422 {
		t.Fatal("HTTP issuance accepted missing review identity")
	}
	if _, err = setup.Exec(ctx, `UPDATE locations SET tax_registration='FRANCHISE-HST' WHERE id=$1`, location); err != nil {
		t.Fatal(err)
	}
	issuedResponse := send("POST", base+"invoices/"+fiscalInvoice.ID+"/issue", `{"issuerId":"`+httpIssuer.ID+`","issuerVersion":1,"invoiceVersion":1}`, token, origin)
	if issuedResponse.Code != 200 || json.Unmarshal(issuedResponse.Body.Bytes(), &fiscalInvoice) != nil || fiscalInvoice.Status != "issued" || fiscalInvoice.InvoiceNumber != "HTTP-1" || fiscalInvoice.IssuerSnapshot == nil {
		t.Fatal("HTTP explicit invoice issuance failed", issuedResponse.Code)
	}
	if fiscalInvoice.IssuerSnapshot.TaxRegistration != "FRANCHISE-HST" {
		t.Fatal("the franchise's own HST number was not frozen onto the invoice", fiscalInvoice.IssuerSnapshot.TaxRegistration)
	}
	if renumbered := send("PUT", issuerBase+"/"+httpIssuer.ID, `{"legalName":"Isolated HTTP issuer","address":"Isolated HTTP issuer address","taxRegistration":"TEST-REG","currency":"CAD","numberPrefix":"HTTP-","version":1,"nextNumber":500,"quotePrefix":"hq-","quoteNextNumber":9}`, token, origin); renumbered.Code != 200 || !strings.Contains(renumbered.Body.String(), `"nextNumber":500`) || !strings.Contains(renumbered.Body.String(), `"quotePrefix":"HQ-"`) || !strings.Contains(renumbered.Body.String(), `"quoteNextNumber":9`) {
		t.Fatal("document numbering could not be changed", renumbered.Code, renumbered.Body.String())
	}
	if pdf := send("GET", base+"invoices/"+fiscalInvoice.ID+"/pdf", "", token, origin); pdf.Code != 200 || pdf.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(pdf.Body.String(), "%PDF-") || !strings.Contains(pdf.Body.String(), "HST: FRANCHISE-HST") {
		t.Fatal("invoice PDF unavailable", pdf.Code)
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='invoices.read'`, actor); err != nil {
		t.Fatal(err)
	}
	paymentPath := base + "invoices/" + fiscalInvoice.ID + "/payments"
	if send("POST", paymentPath, `{"amountCents":100,"method":"cash"}`, token, origin).Code != 422 {
		t.Fatal("HTTP cash accepted missing request key")
	}
	if send("POST", paymentPath, `{"amountCents":100,"method":"square_card","providerReference":"unverified-reference"}`, token, origin).Code != 409 {
		t.Fatal("HTTP manual Square reference settled invoice")
	}
	cashPayload := `{"amountCents":100,"method":"cash","idempotencyKey":"http-cash-retry-001"}`
	var cashReceipt, replayedReceipt struct {
		Payment postgres.Payment `json:"payment"`
		Invoice postgres.Invoice `json:"invoice"`
	}
	cashResponse := send("POST", paymentPath, cashPayload, token, origin)
	if cashResponse.Code != 201 || json.Unmarshal(cashResponse.Body.Bytes(), &cashReceipt) != nil || cashReceipt.Payment.ID == "" || cashReceipt.Invoice.PaidCents != 100 || cashReceipt.Invoice.Notes != "" || cashReceipt.Invoice.CustomerEmail != "" {
		t.Fatal("cash-only permission response failed or leaked private details", cashResponse.Code)
	}
	replayedCash := send("POST", paymentPath, cashPayload, token, origin)
	if replayedCash.Code != 201 || json.Unmarshal(replayedCash.Body.Bytes(), &replayedReceipt) != nil || replayedReceipt.Payment.ID != cashReceipt.Payment.ID {
		t.Fatal("HTTP cash retry duplicated receipt")
	}
	if send("POST", paymentPath, `{"amountCents":101,"method":"cash","idempotencyKey":"http-cash-retry-001"}`, token, origin).Code != 409 {
		t.Fatal("HTTP cash key conflict accepted")
	}

	// Verify customer online payment for invoice via portal endpoint
	custPayPath := "/api/v1/portal/accounts/" + customer.ID + "/invoices/" + fiscalInvoice.ID + "/pay"
	if send("POST", custPayPath, `{"sourceId":"cnon:card-nonce-ok","amountCents":900,"idempotencyKey":"http-cust-pay-00001"}`, token, origin).Code != 403 {
		t.Fatal("unlinked actor allowed to pay customer invoice")
	}

	if _, err = setup.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, actor, org, location, customer.ID); err != nil {
		t.Fatal(err)
	}

	unconfHandler := NewHandler(store, origin)
	unconfReq := httptest.NewRequest("POST", custPayPath, strings.NewReader(`{"sourceId":"cnon:card-nonce-ok","amountCents":900,"idempotencyKey":"http-cust-pay-00001"}`))
	unconfReq.Host = "127.0.0.1:8121"
	unconfReq.Header.Set("Authorization", "Bearer "+token)
	unconfReq.Header.Set("Origin", origin)
	unconfReq.Header.Set("Content-Type", "application/json")
	unconfRec := httptest.NewRecorder()
	unconfHandler.ServeHTTP(unconfRec, unconfReq)
	if unconfRec.Code != 503 {
		t.Fatalf("expected 503 for unconfigured square client, got %d: %s", unconfRec.Code, unconfRec.Body.String())
	}
	// Portal payments charge the account saved for the invoice's location, like every other card payment.
	if _, err = setup.Exec(ctx, `INSERT INTO square_merchant_binding_revisions(organization_id,location_id,version,environment,merchant_id,square_location_id,credential_reference,reviewed_by) VALUES($1,$2,1,'sandbox','synthetic-merchant','synthetic-provider-location','synthetic-secret-selector',$3) ON CONFLICT DO NOTHING`, org, location, actor); err != nil {
		t.Fatal(err)
	}
	store.SetSquareCredentialResolver(func(context.Context, string, string) (postgres.VerifiedSquareClient, error) {
		return verifiedMockSquare{mockSquare}, nil
	})
	if overpayRes := send("POST", custPayPath, `{"sourceId":"cnon:card-nonce-ok","amountCents":901,"idempotencyKey":"http-cust-pay-00002"}`, token, origin); overpayRes.Code != 422 {
		t.Fatalf("expected 422 for overpay, got %d: %s", overpayRes.Code, overpayRes.Body.String())
	}
	custPayRes := send("POST", custPayPath, `{"sourceId":"cnon:card-nonce-ok","amountCents":900,"idempotencyKey":"http-cust-pay-00001"}`, token, origin)
	if custPayRes.Code != 201 {
		t.Fatalf("customer invoice payment failed: %d: %s", custPayRes.Code, custPayRes.Body.String())
	}
	var custPayReceipt struct {
		Payment postgres.Payment `json:"payment"`
		Invoice postgres.Invoice `json:"invoice"`
	}
	if err := json.Unmarshal(custPayRes.Body.Bytes(), &custPayReceipt); err != nil {
		t.Fatal(err)
	}
	if custPayReceipt.Invoice.Status != "paid" || custPayReceipt.Invoice.DueCents != 0 || custPayReceipt.Payment.AmountCents != 900 {
		t.Fatalf("unexpected customer payment receipt: %+v", custPayReceipt)
	}

	if _, err = setup.Exec(ctx, `DELETE FROM customer_account_links WHERE principal_id=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if send("GET", base+"invoices/"+fiscalInvoice.ID, "", token, origin).Code != 403 {
		t.Fatal("cash receipt implicitly granted invoice history")
	}
	accessPath := "/api/v1/organizations/" + org + "/access"
	voidDraftResponse := send("POST", base+"invoices", `{"customerId":"`+customer.ID+`","notes":"PRIVATE-VOID-SENTINEL","items":[{"description":"Isolated HTTP void item","quantity":1,"unitPriceCents":1000}]}`, token, origin)
	var voidDraft postgres.Invoice
	if voidDraftResponse.Code != 201 || json.Unmarshal(voidDraftResponse.Body.Bytes(), &voidDraft) != nil {
		t.Fatal("HTTP void draft creation failed")
	}
	voidPath := base + "invoices/" + voidDraft.ID + "/void"
	draftPath := base + "invoices/" + voidDraft.ID + "/draft"
	draftPayload := `{"version":1,"taxCents":50,"notes":"Revised isolated HTTP note","items":[{"description":"Revised isolated HTTP item","quantity":2,"unitPriceCents":600}]}`
	if send("PUT", draftPath, draftPayload, token, origin).Code != 403 {
		t.Fatal("draft changed without manage grant")
	}
	if send("POST", voidPath, `{"notes":"Not authorized"}`, token, origin).Code != 403 {
		t.Fatal("void accepted without manage grant")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.manage')`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	editedResponse := send("PUT", draftPath, draftPayload, token, origin)
	var editedDraft postgres.Invoice
	if editedResponse.Code != 200 || json.Unmarshal(editedResponse.Body.Bytes(), &editedDraft) != nil || editedDraft.Version != 2 || editedDraft.TotalCents != 1250 || editedDraft.Notes != "" || len(editedDraft.Items) != 0 {
		t.Fatal("HTTP manage-only draft edit failed or leaked", editedResponse.Code)
	}
	if send("PUT", draftPath, draftPayload, token, origin).Code != 409 {
		t.Fatal("HTTP stale edit accepted")
	}
	if send("POST", base+"invoices/"+voidDraft.ID+"/issue", `{"issuerId":"`+httpIssuer.ID+`","issuerVersion":1,"invoiceVersion":1}`, token, origin).Code != 409 {
		t.Fatal("HTTP stale invoice issuance accepted")
	}
	if send("GET", base+"invoices/"+voidDraft.ID+"/draft-history", "", token, origin).Code != 403 {
		t.Fatal("manage implicitly granted draft history")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.read')`, actor, org, location); err != nil {
		t.Fatal(err)
	}
	historyResponse := send("GET", base+"invoices/"+voidDraft.ID+"/draft-history", "", token, origin)
	if historyResponse.Code != 200 || !strings.Contains(historyResponse.Body.String(), `"toVersion":2`) || !strings.Contains(historyResponse.Body.String(), `"unitPriceCents":600`) {
		t.Fatal("HTTP draft snapshots missing")
	}
	if fiscalInvoice.IssuerSnapshot.TaxRegistration != "FRANCHISE-HST" {
		t.Fatal("the franchise's own HST number was not frozen onto the invoice", fiscalInvoice.IssuerSnapshot.TaxRegistration)
	}
	if pdf := send("GET", base+"invoices/"+fiscalInvoice.ID+"/pdf", "", token, origin); pdf.Code != 200 || pdf.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(pdf.Body.String(), "%PDF-") || !strings.Contains(pdf.Body.String(), "HST: FRANCHISE-HST") {
		t.Fatal("invoice PDF unavailable", pdf.Code)
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='invoices.read'`, actor); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		response := send("POST", voidPath, `{"notes":"Isolated HTTP cancellation"}`, token, origin)
		var summary postgres.Invoice
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &summary) != nil || summary.Status != "void" || summary.Notes != "" || summary.CustomerEmail != "" || len(summary.Items) != 0 {
			t.Fatal("manage-only HTTP void failed or leaked details", response.Code)
		}
	}
	if send("POST", base+"invoices/"+fiscalInvoice.ID+"/void", `{"notes":"Has cash receipt"}`, token, origin).Code != 409 {
		t.Fatal("HTTP paid void did not return conflict")
	}
	if send("GET", base+"invoices/"+voidDraft.ID, "", token, origin).Code != 403 {
		t.Fatal("void authority granted invoice history")
	}
	if send("GET", accessPath, "", token, origin).Code != 403 {
		t.Fatal("access administration implicitly granted")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, org, actor); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage')`, actor, org); err != nil {
		t.Fatal(err)
	}
	if send("GET", accessPath, "", token, origin).Code != 200 {
		t.Fatal("explicit administrator cannot inspect scoped access")
	}
	organizationGrantPath := accessPath + "/organization-grants"
	if send("PUT", organizationGrantPath, `{"principalId":"`+actor+`","action":"access.manage","enabled":false}`, token, origin).Code != 409 {
		t.Fatal("HTTP last access manager revocation accepted")
	}
	if send("PUT", organizationGrantPath, `{"principalId":"`+actor+`","action":"invoices.configure","enabled":false}`, token, origin).Code != 200 || send("GET", issuerBase, "", token, origin).Code != 403 {
		t.Fatal("HTTP organization finance revoke ineffective")
	}
	if send("PUT", organizationGrantPath, `{"principalId":"`+actor+`","action":"invoices.configure","enabled":true}`, token, origin).Code != 200 || send("GET", issuerBase, "", token, origin).Code != 200 {
		t.Fatal("HTTP explicit organization finance grant ineffective")
	}
	if send("PUT", accessPath+"/location-grants", `{"principalId":"`+actor+`","locationId":"`+location+`","action":"customers.read"}`, token, origin).Code != 422 {
		t.Fatal("implicit enabled state accepted")
	}
	grantPayload := `{"principalId":"` + actor + `","locationId":"` + location + `","action":"customers.read","enabled":true}`
	if send("PUT", accessPath+"/location-grants", grantPayload, token, "https://foreign.invalid").Code != 403 {
		t.Fatal("foreign-origin grant change accepted")
	}
	if send("PUT", accessPath+"/location-grants", grantPayload, token, origin).Code != 200 {
		t.Fatal("explicit scoped grant failed over HTTP")
	}
	if send("GET", base+"customers", "", token, origin).Code != 200 {
		t.Fatal("new explicit grant ineffective over HTTP")
	}
	revokePayload := `{"principalId":"` + actor + `","locationId":"` + location + `","action":"customers.read","enabled":false}`
	if send("PUT", accessPath+"/location-grants", revokePayload, token, origin).Code != 200 || send("GET", base+"customers", "", token, origin).Code != 403 {
		t.Fatal("HTTP grant revocation ineffective")
	}
	verifyInvitationHTTP(t, ctx, store, setup, actor, org, otherOrg, token)
	verifyCustomerAccountManagementHTTP(t, ctx, store, setup, handler, actor, org, location, customer.ID, token, origin)
	verifyBookingCapacityHTTP(t, ctx, store, setup, handler, actor, org, location, customer.ID, token, origin)
	verifyBookingOfferHTTP(t, ctx, store, setup, handler, actor, org, location, customer.ID, token, origin)
	verifyQuotationHTTP(t, ctx, store, setup, handler, actor, org, location, customer.ID, token, origin)
	verifySquareWebhookHTTP(t, ctx, store, setup, actor, org, location, customer.ID, token, origin)
	verifyDealershipHTTP(t, ctx, store, setup, handler, actor, org, location, customer.ID, token, origin)
	verifyMarketingHTTP(t, ctx, store, setup, handler, actor, org, location, token, origin)
	verifyOwnerOverviewHTTP(t, ctx, store, setup, handler, org, location, origin)
	verifyRolesHTTP(t, ctx, store, setup, handler, actor, org, location, token, origin)
	if _, err = setup.Exec(ctx, `DELETE FROM organization_memberships WHERE organization_id=$1 AND principal_id=$2`, org, actor); err != nil {
		t.Fatal(err)
	}
	if send("GET", accessPath, "", token, origin).Code != 403 {
		t.Fatal("revoked manager membership retained HTTP access")
	}
	logout := send("POST", "/api/v1/auth/logout", "", token, origin)
	if logout.Code != 204 {
		t.Fatal("logout failed")
	}
	cleared := false
	for _, cookie := range logout.Result().Cookies() {
		if cookie.Name == identity.SessionCookieName && cookie.MaxAge < 0 && cookie.Secure && cookie.HttpOnly {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout did not clear the protected cookie")
	}
	if send("GET", "/api/v1/auth/me", "", token, origin).Code != 401 {
		t.Fatal("logged-out session accepted")
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r.Host = "untrusted.example"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("Host boundary bypass")
	}
}

type verifiedMockSquare struct{ *square.MockClient }

func (verifiedMockSquare) VerifyLocation(context.Context, square.LocationBinding) error { return nil }
