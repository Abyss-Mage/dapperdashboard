package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

var ErrAvailabilityLimit = errors.New("availability window exceeds safe resource evaluation limits")

type CustomerAvailability struct {
	OfferID         string      `json:"offerId"`
	OfferVersion    int         `json:"offerVersion"`
	Timezone        string      `json:"timezone"`
	DurationMinutes int         `json:"durationMinutes"`
	Starts          []time.Time `json:"starts"`
	NextFrom        *time.Time  `json:"nextFrom,omitempty"`
	To              time.Time   `json:"to"`
	Reserved        bool        `json:"reserved"`
}

func (s *Store) CustomerOfferAvailability(ctx context.Context, actor, customer, vehicle, offer string, version int, from, to time.Time, days ...string) (CustomerAvailability, error) {
	if len(days) > 1 {
		return CustomerAvailability{}, ErrBookingOffer
	}
	if len(days) == 1 {
		parsed, e := time.Parse("2006-01-02", days[0])
		if e != nil {
			return CustomerAvailability{}, ErrBookingOffer
		}
		from = parsed
		to = parsed.AddDate(0, 0, 1)
	}
	result := CustomerAvailability{OfferID: offer, OfferVersion: version, Starts: []time.Time{}, To: to}
	if actor == "" || customer == "" || vehicle == "" {
		return result, ErrForbidden
	}
	if offer == "" || version < 1 || from.IsZero() || !exactTime(from) || !exactTime(to) || !from.Before(to) || to.Sub(from) > 26*time.Hour {
		return result, ErrBookingOffer
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var org, loc, category string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text,lower(btrim(v.category_description)) FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active JOIN vehicles v ON (v.organization_id,v.location_id,v.customer_id)=(a.organization_id,a.location_id,a.customer_id) WHERE a.principal_id=$1 AND a.customer_id=$2 AND v.id=$3 FOR SHARE OF a,p,v`, actor, customer, vehicle).Scan(&org, &loc, &category)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrForbidden
	}
	if err != nil {
		return result, err
	}
	var resource string
	err = tx.QueryRow(ctx, `SELECT resource_id::text FROM booking_offers WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND category_key=$4 AND published AND version=$5`, org, loc, offer, category, version).Scan(&resource)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrBookingOfferConflict
	}
	if err != nil {
		return result, err
	}
	var plan scheduling.Plan
	var capacity int
	err = tx.QueryRow(ctx, `SELECT capacity,rules,timezone FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, org, loc, resource).Scan(&capacity, &plan, &result.Timezone)
	if err != nil {
		return result, err
	}
	plan.Capacity = capacity
	plan.Existing = nil
	if len(days) == 1 {
		zone, e := time.LoadLocation(result.Timezone)
		if e != nil {
			return result, e
		}
		from, e = time.ParseInLocation("2006-01-02", days[0], zone)
		if e != nil || from.Format("2006-01-02") != days[0] {
			return result, ErrBookingOffer
		}
		to = from.AddDate(0, 0, 1)
		result.To = to
	}
	config, err := scanBookingOffer(tx.QueryRow(ctx, `SELECT `+offerColumns+` FROM booking_offers WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND category_key=$4 AND published AND version=$5 FOR SHARE`, org, loc, offer, category, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrBookingOfferConflict
	}
	if err != nil {
		return result, err
	}
	if config.ResourceID != resource {
		return result, ErrBookingOfferConflict
	}
	rule, err := resolveReservationRule(ctx, tx, Scope{OrganizationID: org, LocationID: loc, ActorID: actor}, resource, reservationRuleChoice{config.RuleID, config.RuleVersion})
	if errors.Is(err, ErrSchedulingRule) || errors.Is(err, ErrSchedulingRuleConflict) {
		return result, ErrBookingOfferConflict
	}
	if err != nil {
		return result, err
	}
	result.DurationMinutes = rule.DurationMinutes
	now := time.Now().UTC()
	earliest := now.Add(time.Duration(config.LeadMinutes) * time.Minute)
	latest := now.Add(time.Duration(config.AdvanceDays) * 24 * time.Hour)
	if from.Before(earliest) {
		from = earliest
	}
	if to.After(latest) {
		to = latest
	}
	if !from.Before(to) {
		if err = tx.Commit(ctx); err != nil {
			return result, err
		}
		return result, nil
	}
	request := scheduling.Request{Duration: time.Duration(rule.DurationMinutes) * time.Minute, Before: time.Duration(rule.BeforeMinutes) * time.Minute, After: time.Duration(rule.AfterMinutes) * time.Minute, Units: 1, WorkingTime: rule.WorkingTime}
	plan.Capacity = rule.Slots // each service has its own number of slots
	reach := request.Duration + request.After
	if request.WorkingTime {
		reach = 30 * 24 * time.Hour // a working-time job can span many days
	}
	rows, err := tx.Query(ctx, `SELECT occupied_start,occupied_end,units,COALESCE(rule_snapshot->>'serviceId','') FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND cancelled_at IS NULL AND occupied_start<$5 AND occupied_end>$4 LIMIT 1001`, org, loc, resource, from.Add(-request.Before), to.Add(reach))
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var allocation scheduling.Allocation
		var service string
		if err = rows.Scan(&allocation.Start, &allocation.End, &allocation.Units, &service); err != nil {
			rows.Close()
			return result, err
		}
		if service != rule.ServiceID {
			continue
		}
		allocation.Units = 1
		plan.Existing = append(plan.Existing, allocation)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(plan.Existing) > 1000 || len(plan.Open) > 1000 || len(plan.Closed) > 1000 {
		return result, ErrAvailabilityLimit
	}
	page, err := scheduling.AvailableStarts(plan, request, from, to, time.Duration(config.SlotMinutes)*time.Minute, 100, 1000)
	if err != nil {
		return result, ErrBookingOffer
	}
	result.Starts = page.Starts
	result.NextFrom = page.NextFrom
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
