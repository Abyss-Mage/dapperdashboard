package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCustomerBookingRequestSnapshots(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_customer_request_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if _, e := admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error(e)
		}
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
	var org, loc, staff, actor, coowner string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic request organization') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic request location') RETURNING id::text`, org).Scan(&loc); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		name string
		id   *string
	}{{"synthetic-request-staff", &staff}, {"synthetic-request-customer", &actor}, {"synthetic-request-coowner", &coowner}} {
		if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) RETURNING id::text`, entry.name).Scan(entry.id); err != nil {
			t.Fatal(err)
		}
	}
	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: staff}
	for _, action := range []string{"customers.create", "catalogue.manage", "scheduling.manage", "scheduling.reserve", "bookings.read"} {
		if _, err = pool.Exec(ctx, `INSERT INTO location_grants(organization_id,location_id,principal_id,action) VALUES($1,$2,$3,$4)`, org, loc, staff, action); err != nil {
			t.Fatal(err)
		}
	}
	customer, err := s.CreateCustomer(ctx, scope, "Synthetic request owner", "owner@example.invalid", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{actor, coowner} {
		if _, err = pool.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, id, org, loc, customer.ID); err != nil {
			t.Fatal(err)
		}
	}
	vehicle, err := s.CreateCustomerVehicle(ctx, actor, customer.ID, Vehicle{Description: "Synthetic original car", CategoryDescription: "Synthetic car", Identifier: "SYNTHETIC-REQUEST"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(24 * time.Hour).Add(33 * time.Hour)
	resource, err := s.CreateSchedulingResource(ctx, scope, SchedulingResource{Name: "Synthetic request bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}}}})
	if err != nil {
		t.Fatal(err)
	}
	service, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic request service", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := s.SaveSchedulingRule(ctx, scope, SchedulingRule{ResourceID: resource.ID, ServiceID: service.ID, DurationMinutes: 60, BeforeMinutes: 15, AfterMinutes: 15, Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	c := BookingOfferConfig{ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: 1, Title: "Synthetic request offer", EquipmentCategory: "Synthetic car", Mode: "fixed", Currency: "CAD", SubtotalCents: 10000, TaxCents: 1300, DepositCents: 2000, Terms: "Original synthetic terms", CancellationTerms: "Original synthetic cancellation", LeadMinutes: 0, AdvanceDays: 30, SlotMinutes: 15, Published: true}
	offer, err := s.SaveBookingOffer(ctx, scope, "", 0, true, c)
	if err != nil {
		t.Fatal(err)
	}
	input := CustomerBookingRequestInput{RequestKey: "synthetic-customer-request-001", OfferID: offer.ID, OfferVersion: 1, Start: start.Add(15 * time.Minute), CustomerNote: "Synthetic private request note", TermsReviewed: true}
	bad := input
	bad.TermsReviewed = false
	if _, err = s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, bad); !errors.Is(err, ErrCustomerBookingRequest) {
		t.Fatal("unreviewed request accepted", err)
	}
	if _, err = s.SubmitCustomerBookingRequest(ctx, staff, customer.ID, vehicle.ID, input); !errors.Is(err, ErrForbidden) {
		t.Fatal("staff fallback granted customer ownership", err)
	}
	bad = input
	bad.OfferVersion = 2
	if _, err = s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, bad); !errors.Is(err, ErrBookingOfferConflict) {
		t.Fatal("stale offer requested", err)
	}
	bad = input
	bad.Start = bad.Start.Add(time.Minute)
	if _, err = s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, bad); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("off-grid request accepted", err)
	}
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, rule.ID, 1, start.Add(3*time.Hour+15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	bad = input
	bad.Start = start.Add(3*time.Hour + 15*time.Minute)
	if _, err = s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, bad); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("occupied request accepted", err)
	}
	type result struct {
		request CustomerBookingRequest
		err     error
	}
	out := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			r, e := s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, input)
			out <- result{r, e}
		}()
	}
	one, two := <-out, <-out
	if one.err != nil || two.err != nil || one.request.ID == "" || one.request.ID != two.request.ID || one.request.Reserved || one.request.Status != "requested" || one.request.Offer.SubtotalCents != 10000 || one.request.Vehicle.Description != vehicle.Description {
		t.Fatal("concurrent request retry/snapshot failed", one.err, two.err)
	}
	bad = input
	bad.CustomerNote = "Changed note"
	if _, err = s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, bad); !errors.Is(err, ErrCustomerBookingRequestConflict) {
		t.Fatal("changed retry payload accepted", err)
	}
	if _, err = s.SubmitCustomerBookingRequest(ctx, coowner, customer.ID, vehicle.ID, input); err != nil {
		t.Fatal("actor-specific request key collision", err)
	}
	history, err := s.CustomerBookingRequests(ctx, actor, customer.ID)
	if err != nil || len(history) != 1 {
		t.Fatal("customer history leaked another actor request", err)
	}
	staffRequests, err := s.StaffCustomerBookingRequests(ctx, scope)
	if err != nil || len(staffRequests) != 2 {
		t.Fatal("authorized staff request inbox missing", err)
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT synthetic_request_failure CHECK(action<>'customer_booking_request.submitted') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	bad = input
	bad.RequestKey = "synthetic-request-audit-failure"
	bad.Start = start.Add(5*time.Hour + 15*time.Minute)
	if _, err = s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, bad); err == nil {
		t.Fatal("audit failure committed request")
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT synthetic_request_failure`); err != nil {
		t.Fatal(err)
	}
	var count, events, bookings, reservations int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM customer_booking_requests),(SELECT count(*) FROM customer_booking_request_events),(SELECT count(*) FROM bookings),(SELECT count(*) FROM capacity_reservations)`).Scan(&count, &events, &bookings, &reservations); err != nil || count != 2 || events != 2 || bookings != 0 || reservations != 1 {
		t.Fatal("request created booking/capacity or audit orphan", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE customer_booking_requests SET customer_note='Tampered' WHERE id=$1`, one.request.ID); err == nil {
		t.Fatal("immutable submitted request changed")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM customer_booking_requests WHERE id=$1`, one.request.ID); err == nil {
		t.Fatal("immutable submitted request deleted")
	}
	if _, err = pool.Exec(ctx, `UPDATE vehicles SET description='Synthetic revised car' WHERE id=$1`, vehicle.ID); err != nil {
		t.Fatal(err)
	}
	c.Mode = "quote_required"
	c.SubtotalCents = 0
	c.TaxCents = 0
	c.DepositCents = 0
	c.Terms = "Synthetic quote-required terms"
	if _, err = s.SaveBookingOffer(ctx, scope, offer.ID, 1, true, c); err != nil {
		t.Fatal(err)
	}
	quoteInput := input
	quoteInput.RequestKey = "synthetic-customer-quote-request"
	quoteInput.OfferVersion = 2
	quoteInput.Start = start.Add(5*time.Hour + 15*time.Minute)
	quoteRequest, err := s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, quoteInput)
	if err != nil || quoteRequest.Offer.Mode != "quote_required" || quoteRequest.Offer.SubtotalCents != 0 || quoteRequest.Status != "requested" || quoteRequest.Reserved {
		t.Fatal("quote-required request pretended price/booking confirmation", err)
	}
	c.Published = false
	c.Mode = "fixed"
	c.SubtotalCents = 20000
	c.Terms = "Revised synthetic terms"
	if _, err = s.SaveBookingOffer(ctx, scope, offer.ID, 2, false, c); err != nil {
		t.Fatal(err)
	}
	retry, err := s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, input)
	if err != nil || retry.ID != one.request.ID || retry.Offer.SubtotalCents != 10000 || retry.Offer.Terms != "Original synthetic terms" || retry.Vehicle.Description != vehicle.Description {
		t.Fatal("retry changed historical offer/equipment snapshot", err)
	}
	bad = input
	bad.RequestKey = "synthetic-request-withdrawn-offer"
	if _, err = s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, bad); !errors.Is(err, ErrBookingOfferConflict) {
		t.Fatal("new request used withdrawn offer", err)
	}
	encoded, _ := json.Marshal(retry)
	if strings.Contains(string(encoded), "ruleId") || strings.Contains(string(encoded), "reviewedBy") || strings.Contains(string(encoded), "resourceId") {
		t.Fatal("request leaked internal snapshot selectors")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM customer_account_links WHERE principal_id=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SubmitCustomerBookingRequest(ctx, actor, customer.ID, vehicle.ID, input); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked owner replay accepted", err)
	}
	if _, err = s.CustomerBookingRequests(ctx, actor, customer.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked owner history allowed", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='bookings.read'`, staff); err != nil {
		t.Fatal(err)
	}
	if _, err = s.StaffCustomerBookingRequests(ctx, scope); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked staff inbox allowed", err)
	}
}
