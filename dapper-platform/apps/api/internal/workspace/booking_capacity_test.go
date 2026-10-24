package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/scheduling"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyBookingCapacityHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, handler http.Handler, actor, org, loc, customer, token, origin string) {
	t.Helper()
	for _, action := range []string{"scheduling.manage", "catalogue.manage", "bookings.create"} {
		if _, err := setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor, org, loc, action); err != nil {
			t.Fatal(err)
		}
	}
	scope := postgres.Scope{ActorID: actor, OrganizationID: org, LocationID: loc}
	start := time.Date(2027, 1, 2, 9, 0, 0, 0, time.UTC)
	resource, err := store.CreateSchedulingResource(ctx, scope, postgres.SchedulingResource{Name: "Isolated HTTP booking resource", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(2 * time.Hour)}}}})
	if err != nil {
		t.Fatal(err)
	}
	main, err := store.SaveService(ctx, scope, postgres.CatalogService{Name: "Isolated HTTP atomic service", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := store.SaveSchedulingRule(ctx, scope, postgres.SchedulingRule{ResourceID: resource.ID, ServiceID: main.ID, DurationMinutes: 30, BeforeMinutes: 0, AfterMinutes: 0, Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	send := func(method, path, body, bearer, requestOrigin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
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
	base := "/api/v1/organizations/" + org + "/locations/" + loc + "/bookings"
	input := postgres.ReserveBookingInput{RequestKey: "isolated-http-booking-request-001", CustomerID: customer, ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: 1, Start: start, Channel: "walk_in", Notes: "HTTP private booking note"}
	encoded, _ := json.Marshal(input)
	if send("POST", base+"/reserve", string(encoded), "", origin).Code != 401 {
		t.Fatal("anonymous reserved booking accepted")
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.reserve'`, actor); err != nil {
		t.Fatal(err)
	}
	if send("POST", base+"/reserve", string(encoded), token, origin).Code != 403 {
		t.Fatal("booking create implied capacity permission")
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'scheduling.reserve')`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	choices := send("GET", base+"/reservation-options", "", token, origin)
	if choices.Code != 200 || !strings.Contains(choices.Body.String(), rule.ID) || !strings.Contains(choices.Body.String(), main.Name) {
		t.Fatal("reserved booking choices unavailable", choices.Code)
	}
	if send("POST", base+"/reserve", string(encoded), token, "https://foreign.invalid").Code != 403 {
		t.Fatal("foreign origin reserve-and-book accepted")
	}
	forged := strings.TrimSuffix(string(encoded), "}") + `,"status":"confirmed"}`
	if send("POST", base+"/reserve", forged, token, origin).Code != 400 {
		t.Fatal("caller forged booking status")
	}
	var booking, replayed postgres.Booking
	created := send("POST", base+"/reserve", string(encoded), token, origin)
	if created.Code != 201 || json.Unmarshal(created.Body.Bytes(), &booking) != nil || booking.Status != "pending" || booking.ReservationID == nil || booking.Notes != "" || booking.CustomerEmail != "" {
		t.Fatal("HTTP atomic booking failed or leaked private fields", created.Code)
	}
	replay := send("POST", base+"/reserve", string(encoded), token, origin)
	if replay.Code != 201 || json.Unmarshal(replay.Body.Bytes(), &replayed) != nil || replayed.ID != booking.ID {
		t.Fatal("HTTP booking retry duplicated record")
	}
	input.Notes = "Changed request note"
	changed, _ := json.Marshal(input)
	if send("POST", base+"/reserve", string(changed), token, origin).Code != 409 {
		t.Fatal("HTTP booking retry payload conflict accepted")
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.reserve'`, actor); err != nil {
		t.Fatal(err)
	}
	if send("POST", base+"/reserve", string(encoded), token, origin).Code != 403 {
		t.Fatal("HTTP revoked reserve retry accepted")
	}
	for _, action := range []string{"bookings.manage", "scheduling.release"} {
		if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor, org, loc, action); err != nil {
			t.Fatal(err)
		}
	}
	cancelPath := base + "/" + booking.ID + "/status"
	if send("POST", cancelPath, `{"toStatus":"cancelled"}`, token, origin).Code != 422 {
		t.Fatal("public cancellation omitted reservation version")
	}
	if send("POST", cancelPath, `{"toStatus":" cancelled "}`, token, origin).Code != 422 {
		t.Fatal("whitespace bypassed cancellation version guard")
	}
	if send("POST", cancelPath, `{"toStatus":"cancelled","reservationVersion":0}`, token, origin).Code != 409 {
		t.Fatal("public cancellation ignored stale version")
	}
	for i := 0; i < 2; i++ {
		cancelled := send("POST", cancelPath, `{"toStatus":"cancelled","reservationVersion":1,"cancellationReason":"HTTP cancellation","notes":"HTTP cancellation note"}`, token, origin)
		var result postgres.Booking
		if cancelled.Code != 200 || json.Unmarshal(cancelled.Body.Bytes(), &result) != nil || result.ID != booking.ID || result.ReservationVersion == nil || *result.ReservationVersion != 2 || result.CustomerEmail != "" || result.Notes != "" {
			t.Fatal("HTTP atomic cancellation/retry failed", cancelled.Code)
		}
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'scheduling.reserve')`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	input.RequestKey = "isolated-http-reschedule-source"
	input.Start = start
	moveSource, err := store.ReserveBooking(ctx, scope, input)
	if err != nil {
		t.Fatal(err)
	}
	move := postgres.RescheduleBookingInput{RequestKey: "isolated-http-reschedule-request", ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: 1, ReservationVersion: 1, Start: start.Add(time.Hour), Notes: "Synthetic HTTP move"}
	path := base + "/" + moveSource.ID + "/reschedule"
	moveBody, _ := json.Marshal(move)
	if send("GET", base+"/reschedule-options", "", token, origin).Code != 200 {
		t.Fatal("managed reschedule choices unavailable")
	}
	if send("POST", path, string(moveBody), "", origin).Code != 401 {
		t.Fatal("anonymous move accepted")
	}
	if send("POST", path, string(moveBody), token, "https://untrusted.invalid").Code != 403 {
		t.Fatal("foreign origin move accepted")
	}
	if send("POST", path, strings.TrimSuffix(string(moveBody), "}")+`,"status":"confirmed"}`, token, origin).Code != 400 {
		t.Fatal("forged status accepted during move")
	}
	missing := move
	missing.ReservationVersion = 0
	missingBody, _ := json.Marshal(missing)
	if send("POST", path, string(missingBody), token, origin).Code != 422 {
		t.Fatal("move omitted observed version")
	}
	for i := 0; i < 2; i++ {
		response := send("POST", path, string(moveBody), token, origin)
		var result postgres.Booking
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.ID != moveSource.ID || result.ReservationVersion == nil || *result.ReservationVersion != 2 || !result.BookingTime.Equal(move.Start) || result.CustomerEmail != "" || result.Notes != "" {
			t.Fatal("HTTP atomic reschedule/retry failed", response.Code)
		}
	}
	move.RequestKey = "isolated-http-stale-move-request"
	staleBody, _ := json.Marshal(move)
	if send("POST", path, string(staleBody), token, origin).Code != 409 {
		t.Fatal("stale reschedule accepted")
	}
	if _, err = setup.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.release'`, actor); err != nil {
		t.Fatal(err)
	}
	if send("POST", path, string(moveBody), token, origin).Code != 403 || send("GET", base+"/reschedule-options", "", token, origin).Code != 403 {
		t.Fatal("revoked release grant allowed move or choices")
	}
}
