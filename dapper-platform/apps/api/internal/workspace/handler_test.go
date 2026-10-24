package workspace

import (
	"context"
	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
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
	fresponse := send("POST", orgBase+"franchises", `{"code":"httpf","name":"Synthetic HTTP franchise","address":{"city":"Peterborough"}}`, token, origin)
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
	if send("POST", orgBase+"franchises", `{"code":"OTHR","name":"Synthetic HTTP franchise"}`, token, origin).Code != 409 {
		t.Fatal("duplicate franchise name not actionable")
	}
	if send("POST", orgBase+"franchises", `{"code":"HTTPF","name":"Synthetic duplicate code"}`, token, origin).Code != 409 {
		t.Fatal("duplicate franchise code not actionable")
	}
	if send("POST", orgBase+"franchises", `{"name":"No code"}`, token, origin).Code != 422 {
		t.Fatal("franchise without a code accepted")
	}
	if send("POST", orgBase+"franchises", `{"code":"INJ","name":"Injected actor","actorId":"forged"}`, token, origin).Code != 400 {
		t.Fatal("structure actor injection accepted")
	}
	if send("PUT", orgBase+"franchises/"+franchise.ID, `{"code":"HTTPF","name":"Synthetic HTTP franchise","address":{"street":"1 Test Rd"}}`, token, origin).Code != 200 {
		t.Fatal("franchise edit failed")
	}
	if send("GET", "/api/v1/organizations/"+otherOrg+"/structure", "", token, origin).Code != 403 {
		t.Fatal("foreign organization structure leaked")
	}
	if list := send("GET", orgBase+"structure", "", token, origin); list.Code != 200 || !strings.Contains(list.Body.String(), "HTTPF") || !strings.Contains(list.Body.String(), "1 Test Rd") {
		t.Fatal("persisted structure missing")
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
	clockInRes := send("POST", base+"attendance/clock-in", `{"notes":"HTTP morning shift"}`, token, origin)
	if clockInRes.Code != 201 {
		t.Fatal("clock in failed", clockInRes.Code, clockInRes.Body.String())
	}
	if send("POST", base+"attendance/clock-in", `{"notes":"Double clock in"}`, token, origin).Code != 409 {
		t.Fatal("double clock in accepted")
	}
	if send("POST", base+"attendance/start-break", `{"notes":"Coffee"}`, token, origin).Code != 200 {
		t.Fatal("start break failed")
	}
	if send("POST", base+"attendance/end-break", `{"notes":"Work resumed"}`, token, origin).Code != 200 {
		t.Fatal("end break failed")
	}
	clockOutRes := send("POST", base+"attendance/clock-out", `{"notes":"Shift done"}`, token, origin)
	if clockOutRes.Code != 200 {
		t.Fatal("clock out failed", clockOutRes.Code, clockOutRes.Body.String())
	}
	if hist := send("GET", base+"attendance/history", "", token, origin); hist.Code != 200 || !strings.Contains(hist.Body.String(), "HTTP morning shift") {
		t.Fatal("attendance history failed", hist.Code, hist.Body.String())
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
	issuedResponse := send("POST", base+"invoices/"+fiscalInvoice.ID+"/issue", `{"issuerId":"`+httpIssuer.ID+`","issuerVersion":1,"invoiceVersion":1}`, token, origin)
	if issuedResponse.Code != 200 || json.Unmarshal(issuedResponse.Body.Bytes(), &fiscalInvoice) != nil || fiscalInvoice.Status != "issued" || fiscalInvoice.InvoiceNumber != "HTTP-1" || fiscalInvoice.IssuerSnapshot == nil {
		t.Fatal("HTTP explicit invoice issuance failed", issuedResponse.Code)
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
