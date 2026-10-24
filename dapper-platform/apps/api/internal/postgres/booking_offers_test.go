package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func verifyBookingOffers(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	start := time.Now().UTC().Truncate(24 * time.Hour).Add(33 * time.Hour)
	resource, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic offer bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}}}})
	if err != nil {
		t.Fatal(err)
	}
	service, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic offer main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := s.SaveSchedulingRule(ctx, a, SchedulingRule{ResourceID: resource.ID, ServiceID: service.ID, DurationMinutes: 60, Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	c := BookingOfferConfig{ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: 1, Title: "Synthetic customer offer", EquipmentCategory: "Synthetic aircraft", Mode: "fixed", Currency: "CAD", SubtotalCents: 10000, TaxCents: 1300, DepositCents: 2000, Terms: "Synthetic test service terms only", CancellationTerms: "Synthetic cancellation terms only", LeadMinutes: 60, AdvanceDays: 30, SlotMinutes: 15}
	if _, err = s.SaveBookingOffer(ctx, b, "", 0, false, c); !errors.Is(err, ErrBookingOfferConflict) {
		t.Fatal("foreign offer configured", err)
	}
	invalid := c
	invalid.Published = true
	if _, err = s.SaveBookingOffer(ctx, a, "", 0, false, invalid); !errors.Is(err, ErrBookingOffer) {
		t.Fatal("unreviewed publication accepted", err)
	}
	invalid = c
	invalid.DepositCents = 11301
	if _, err = s.SaveBookingOffer(ctx, a, "", 0, false, invalid); !errors.Is(err, ErrBookingOffer) {
		t.Fatal("over-total deposit accepted", err)
	}
	invalid = c
	invalid.Mode = "quote_required"
	if _, err = s.SaveBookingOffer(ctx, a, "", 0, false, invalid); !errors.Is(err, ErrBookingOffer) {
		t.Fatal("quote invented fixed amounts", err)
	}
	draft, err := s.SaveBookingOffer(ctx, a, "", 0, false, c)
	if err != nil || draft.Published || draft.ReviewedBy != nil {
		t.Fatal("draft unexpectedly published", err)
	}
	duplicate := c
	duplicate.EquipmentCategory = "  SYNTHETIC AIRCRAFT "
	if _, err = s.SaveBookingOffer(ctx, a, "", 0, false, duplicate); !errors.Is(err, ErrBookingOfferConflict) {
		t.Fatal("normalized duplicate offer accepted", err)
	}
	var actor, vehicle string
	if err = s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic-offer-customer') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `INSERT INTO vehicles(organization_id,location_id,customer_id,description,category_description) VALUES($1,$2,$3,'Synthetic offer aircraft','synthetic aircraft') RETURNING id::text`, a.OrganizationID, a.LocationID, customer).Scan(&vehicle); err != nil {
		t.Fatal(err)
	}
	defer s.pool.Exec(ctx, `DELETE FROM vehicles WHERE id=$1`, vehicle)
	if _, err = s.CustomerBookingOffers(ctx, actor, customer, vehicle); !errors.Is(err, ErrForbidden) {
		t.Fatal("unlinked portal saw offers", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, actor, a.OrganizationID, a.LocationID, customer); err != nil {
		t.Fatal(err)
	}
	offers, err := s.CustomerBookingOffers(ctx, actor, customer, vehicle)
	if err != nil || len(offers) != 0 {
		t.Fatal("draft visible to customer", err)
	}
	c.Published = true
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT synthetic_offer_audit_failure CHECK(action<>'booking_offer.saved') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveBookingOffer(ctx, a, draft.ID, 1, true, c); err == nil {
		t.Fatal("offer audit failure committed")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT synthetic_offer_audit_failure`); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		offer BookingOffer
		err   error
	}
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() { o, e := s.SaveBookingOffer(ctx, a, draft.ID, 1, true, c); results <- outcome{o, e} }()
	}
	wins, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err == nil {
			wins++
			if r.offer.Version != 2 || !r.offer.Published || r.offer.ReviewedBy == nil {
				t.Fatal("publication review/version missing")
			}
		} else if errors.Is(r.err, ErrBookingOfferConflict) {
			conflicts++
		} else {
			t.Fatal(r.err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("concurrent publication lost update")
	}
	offers, err = s.CustomerBookingOffers(ctx, actor, customer, vehicle)
	if err != nil || len(offers) != 1 || offers[0].SubtotalCents != 10000 || offers[0].TaxCents != 1300 || offers[0].DepositCents != 2000 || offers[0].DurationMinutes != 60 {
		t.Fatal("published customer offer missing/wrong", err)
	}
	body, _ := json.Marshal(offers)
	if strings.Contains(string(body), "ruleId") || strings.Contains(string(body), "reviewedBy") || strings.Contains(string(body), "resourceId") {
		t.Fatal("portal leaked internal selectors/reviewer")
	}
	availability, err := s.CustomerOfferAvailability(ctx, actor, customer, vehicle, draft.ID, 2, start, start.Add(4*time.Hour))
	if err != nil || len(availability.Starts) != 16 || !availability.Starts[0].Equal(start) || availability.Reserved {
		t.Fatal("owned current offer availability failed", err, len(availability.Starts))
	}
	dstDay, err := s.CustomerOfferAvailability(ctx, actor, customer, vehicle, draft.ID, 2, time.Time{}, time.Time{}, "2026-11-01")
	if err != nil || !dstDay.To.Equal(time.Date(2026, 11, 2, 5, 0, 0, 0, time.UTC)) {
		t.Fatal("location DST day did not end at actual next midnight", err, dstDay.To)
	}
	if _, err = s.CustomerOfferAvailability(ctx, actor, customer, vehicle, draft.ID, 1, start, start.Add(4*time.Hour)); !errors.Is(err, ErrBookingOfferConflict) {
		t.Fatal("stale offer availability accepted", err)
	}
	if _, err = s.CustomerOfferAvailability(ctx, actor, customer, vehicle, draft.ID, 2, start, start.Add(27*time.Hour)); !errors.Is(err, ErrBookingOffer) {
		t.Fatal("unbounded availability accepted", err)
	}
	blocked, err := s.ReserveServiceCapacity(ctx, a, resource.ID, rule.ID, rule.Version, start)
	if err != nil {
		t.Fatal(err)
	}
	availability, err = s.CustomerOfferAvailability(ctx, actor, customer, vehicle, draft.ID, 2, start, start.Add(4*time.Hour))
	if err != nil || len(availability.Starts) != 12 || !availability.Starts[0].Equal(start.Add(time.Hour)) {
		t.Fatal("customer availability ignored staff capacity", err, len(availability.Starts))
	}
	if _, err = s.ReleaseCapacity(ctx, a, resource.ID, blocked.ID, blocked.Version); err != nil {
		t.Fatal(err)
	}
	availability, err = s.CustomerOfferAvailability(ctx, actor, customer, vehicle, draft.ID, 2, start, start.Add(4*time.Hour))
	if err != nil || len(availability.Starts) != 16 {
		t.Fatal("released staff capacity not visible", err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE vehicles SET category_description='Other category' WHERE id=$1`, vehicle); err != nil {
		t.Fatal(err)
	}
	if offers, err = s.CustomerBookingOffers(ctx, actor, customer, vehicle); err != nil || len(offers) != 0 {
		t.Fatal("wrong category offered", err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE vehicles SET category_description='Synthetic aircraft' WHERE id=$1`, vehicle); err != nil {
		t.Fatal(err)
	}
	rule.DurationMinutes = 90
	rule, err = s.SaveSchedulingRule(ctx, a, rule)
	if err != nil {
		t.Fatal(err)
	}
	if offers, err = s.CustomerBookingOffers(ctx, actor, customer, vehicle); err != nil || len(offers) != 0 {
		t.Fatal("stale timing offer visible", err)
	}
	if _, err = s.CustomerOfferAvailability(ctx, actor, customer, vehicle, draft.ID, 2, start, start.Add(4*time.Hour)); !errors.Is(err, ErrBookingOfferConflict) {
		t.Fatal("stale rule used for customer availability", err)
	}
	if _, err = s.SaveBookingOffer(ctx, a, draft.ID, 2, true, c); !errors.Is(err, ErrBookingOfferConflict) {
		t.Fatal("stale rule republished", err)
	}
	c.Published = false
	if _, err = s.SaveBookingOffer(ctx, a, draft.ID, 2, false, c); err != nil {
		t.Fatal("stale offer could not be withdrawn", err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM customer_account_links WHERE principal_id=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CustomerBookingOffers(ctx, actor, customer, vehicle); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked ownership read offers", err)
	}
	if _, err = s.CustomerOfferAvailability(ctx, actor, customer, vehicle, draft.ID, 3, start, start.Add(4*time.Hour)); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked customer availability permitted", err)
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM booking_offer_revisions WHERE offer_id=$1`, draft.ID).Scan(&count); err != nil || count != 3 {
		t.Fatal("revision audit count wrong", err)
	}
}
