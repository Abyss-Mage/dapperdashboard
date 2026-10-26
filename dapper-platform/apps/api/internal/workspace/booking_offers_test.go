package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/scheduling"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func verifyBookingOfferHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, handler http.Handler, actor, org, loc, customer, token, origin string) {
	t.Helper()
	scope := postgres.Scope{ActorID: actor, OrganizationID: org, LocationID: loc}
	// Anchor to tomorrow at Toronto noon so the 4h window never straddles local midnight; a UTC-hour anchor made the
	// local-day assertion below depend on the wall-clock hour the suite happened to run.
	torontoZone, zoneErr := time.LoadLocation("America/Toronto")
	if zoneErr != nil {
		t.Fatal(zoneErr)
	}
	torontoNow := time.Now().In(torontoZone)
	start := time.Date(torontoNow.Year(), torontoNow.Month(), torontoNow.Day()+1, 12, 0, 0, 0, torontoZone).UTC()
	resource, err := store.CreateSchedulingResource(ctx, scope, postgres.SchedulingResource{Name: "Synthetic HTTP offer bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(4 * time.Hour)}}}})
	if err != nil {
		t.Fatal(err)
	}
	service, err := store.SaveService(ctx, scope, postgres.CatalogService{Name: "Synthetic HTTP offer main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := store.SaveSchedulingRule(ctx, scope, postgres.SchedulingRule{ResourceID: resource.ID, ServiceID: service.ID, DurationMinutes: 30, Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"resourceId": resource.ID, "ruleId": rule.ID, "ruleVersion": 1, "title": "Synthetic HTTP offer", "equipmentCategory": "Synthetic car", "mode": "fixed", "currency": "CAD", "subtotalCents": 10000, "taxCents": 1300, "depositCents": 2000, "terms": "Synthetic test terms", "cancellationTerms": "Synthetic cancellation terms", "leadMinutes": 0, "advanceDays": 30, "slotMinutes": 15, "published": true, "reviewed": true, "expectedVersion": 0}
	send := func(method, path string, input map[string]any, bearer, requestOrigin string) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(input)
		r := httptest.NewRequest(method, path, strings.NewReader(string(encoded)))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Origin", requestOrigin)
		r.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/api/v1/organizations/" + org + "/locations/" + loc + "/booking-offers"
	if send("PUT", path, body, "", origin).Code != 401 || send("PUT", path, body, token, "https://untrusted.invalid").Code != 403 {
		t.Fatal("offer auth/origin bypass")
	}
	delete(body, "leadMinutes")
	if send("PUT", path, body, token, origin).Code != 422 {
		t.Fatal("omitted lead-time policy assumed")
	}
	body["leadMinutes"] = 0
	body["reviewed"] = false
	if send("PUT", path, body, token, origin).Code != 422 {
		t.Fatal("unreviewed publication HTTP accepted")
	}
	body["reviewed"] = true
	body["title"] = strings.Repeat("x", 129*1024)
	if send("PUT", path, body, token, origin).Code != 400 {
		t.Fatal("oversized offer body accepted")
	}
	body["title"] = "Synthetic HTTP offer"
	body["terms"] = strings.Repeat("Synthetic long terms ", 450)
	body["cancellationTerms"] = strings.Repeat("Synthetic long cancellation ", 300)
	body["actorId"] = actor
	if send("PUT", path, body, token, origin).Code != 400 {
		t.Fatal("forged publication actor accepted")
	}
	delete(body, "actorId")
	created := send("PUT", path, body, token, origin)
	var offer postgres.BookingOffer
	if created.Code != 201 || json.Unmarshal(created.Body.Bytes(), &offer) != nil || !offer.Published || offer.LeadMinutes != 0 || offer.SubtotalCents != 10000 || offer.ReviewedBy == nil {
		t.Fatal("reviewed offer HTTP save failed", created.Code, created.Body.String())
	}
	if send("GET", path+"/options", nil, token, origin).Code != 200 {
		t.Fatal("offer management choices unavailable")
	}
	var portalActor, vehicle string
	if err = setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic-http-offer-customer') RETURNING id::text`).Scan(&portalActor); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `INSERT INTO vehicles(organization_id,location_id,customer_id,description,category_description) VALUES($1,$2,$3,'Synthetic HTTP car','Synthetic car') RETURNING id::text`, org, loc, customer).Scan(&vehicle); err != nil {
		t.Fatal(err)
	}
	portalToken, err := store.IssueSession(ctx, portalActor, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	portalPath := "/api/v1/portal/accounts/" + customer + "/vehicles/" + vehicle + "/booking-offers"
	if send("GET", portalPath, nil, "", origin).Code != 401 || send("GET", portalPath, nil, portalToken, origin).Code != 403 {
		t.Fatal("unowned offers disclosed over HTTP")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, portalActor, org, loc, customer); err != nil {
		t.Fatal(err)
	}
	visible := send("GET", portalPath, nil, portalToken, origin)
	var portal struct {
		Items []postgres.PortalBookingOffer `json:"items"`
	}
	if visible.Code != 200 || json.Unmarshal(visible.Body.Bytes(), &portal) != nil || len(portal.Items) != 1 || portal.Items[0].ID != offer.ID || strings.Contains(visible.Body.String(), "reviewedBy") || strings.Contains(visible.Body.String(), "ruleId") {
		t.Fatal("owned HTTP offer contract failed", visible.Code)
	}
	availabilityPath := portalPath + "/" + offer.ID + "/availability?offerVersion=1&from=" + start.Format(time.RFC3339Nano) + "&to=" + start.Add(2*time.Hour).Format(time.RFC3339Nano)
	available := send("GET", availabilityPath, nil, portalToken, origin)
	var times postgres.CustomerAvailability
	if available.Code != 200 || json.Unmarshal(available.Body.Bytes(), &times) != nil || len(times.Starts) != 8 || times.Reserved || strings.Contains(available.Body.String(), "resourceId") || strings.Contains(available.Body.String(), "capacity") {
		t.Fatal("owned HTTP availability failed/leaked", available.Code, available.Body.String())
	}
	zone, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	dayPath := portalPath + "/" + offer.ID + "/availability?offerVersion=1&day=" + start.In(zone).Format("2006-01-02")
	dayResponse := send("GET", dayPath, nil, portalToken, origin)
	if dayResponse.Code != 200 || json.Unmarshal(dayResponse.Body.Bytes(), &times) != nil || len(times.Starts) != 15 || times.Timezone != "America/Toronto" {
		t.Fatal("location date availability failed", dayResponse.Code, dayResponse.Body.String())
	}
	if send("GET", dayPath+"&from="+start.Format(time.RFC3339Nano), nil, portalToken, origin).Code != 422 {
		t.Fatal("mixed date/range accepted")
	}
	if send("GET", strings.Replace(availabilityPath, "offerVersion=1", "offerVersion=2", 1), nil, portalToken, origin).Code != 409 || send("GET", availabilityPath, nil, "", origin).Code != 401 {
		t.Fatal("availability stale/anonymous guard failed")
	}
	requestPath := "/api/v1/portal/accounts/" + customer + "/vehicles/" + vehicle + "/booking-requests"
	requestBody := map[string]any{"requestKey": "synthetic-http-customer-request-001", "offerId": offer.ID, "offerVersion": 1, "start": start, "customerNote": "Synthetic HTTP customer note", "termsReviewed": true}
	if send("POST", requestPath, requestBody, "", origin).Code != 401 || send("POST", requestPath, requestBody, portalToken, "https://untrusted.invalid").Code != 403 {
		t.Fatal("customer request auth/origin bypass")
	}
	requestBody["termsReviewed"] = false
	if send("POST", requestPath, requestBody, portalToken, origin).Code != 422 {
		t.Fatal("unreviewed HTTP request accepted")
	}
	requestBody["termsReviewed"] = true
	requestBody["subtotalCents"] = 1
	if send("POST", requestPath, requestBody, portalToken, origin).Code != 400 {
		t.Fatal("client request price accepted")
	}
	delete(requestBody, "subtotalCents")
	requestBody["status"] = "confirmed"
	if send("POST", requestPath, requestBody, portalToken, origin).Code != 400 {
		t.Fatal("client request confirmation accepted")
	}
	delete(requestBody, "status")
	var requestID string
	for i := 0; i < 2; i++ {
		response := send("POST", requestPath, requestBody, portalToken, origin)
		var result postgres.CustomerBookingRequest
		if response.Code != 201 || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Status != "requested" || result.Reserved || result.Offer.SubtotalCents != 10000 || result.Vehicle.Description != "Synthetic HTTP car" || strings.Contains(response.Body.String(), "reviewedBy") {
			t.Fatal("customer request HTTP snapshot/retry failed", response.Code, response.Body.String())
		}
		if i == 0 {
			requestID = result.ID
		} else if result.ID != requestID {
			t.Fatal("HTTP retry duplicated request")
		}
	}
	requestBody["customerNote"] = "Different note"
	if send("POST", requestPath, requestBody, portalToken, origin).Code != 409 {
		t.Fatal("request payload conflict accepted")
	}
	requestBody["customerNote"] = "Synthetic HTTP customer note"
	requestsPath := "/api/v1/portal/accounts/" + customer + "/booking-requests"
	if send("GET", requestsPath, nil, portalToken, origin).Code != 200 {
		t.Fatal("owned request history unavailable")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'bookings.read') ON CONFLICT DO NOTHING`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	if send("GET", "/api/v1/organizations/"+org+"/locations/"+loc+"/customer-booking-requests", nil, token, origin).Code != 200 {
		t.Fatal("staff request inbox unavailable")
	}
	body["id"] = offer.ID
	body["expectedVersion"] = 1
	body["published"] = false
	body["reviewed"] = false
	if send("PUT", path, body, token, origin).Code != 200 || send("PUT", path, body, token, origin).Code != 409 {
		t.Fatal("withdraw or optimistic offer conflict failed")
	}
	if send("POST", requestPath, requestBody, portalToken, origin).Code != 201 {
		t.Fatal("withdrawn offer invalidated original request receipt")
	}
	hidden := send("GET", portalPath, nil, portalToken, origin)
	if hidden.Code != 200 || json.Unmarshal(hidden.Body.Bytes(), &portal) != nil || len(portal.Items) != 0 {
		t.Fatal("withdrawn offer still visible")
	}
	if _, err = setup.Exec(ctx, `DELETE FROM customer_account_links WHERE principal_id=$1`, portalActor); err != nil {
		t.Fatal(err)
	}
	if send("GET", portalPath, nil, portalToken, origin).Code != 403 {
		t.Fatal("revoked ownership retained offer read")
	}
	if send("GET", availabilityPath, nil, portalToken, origin).Code != 403 {
		t.Fatal("revoked owner retained availability")
	}
	if send("GET", requestsPath, nil, portalToken, origin).Code != 403 || send("POST", requestPath, requestBody, portalToken, origin).Code != 403 {
		t.Fatal("revoked customer request access retained")
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.manage'`, actor); err != nil {
		t.Fatal(err)
	}
	if send("GET", path, nil, token, origin).Code != 403 || send("PUT", path, body, token, origin).Code != 403 {
		t.Fatal("catalogue grant implied publication control")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'scheduling.manage')`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
}
