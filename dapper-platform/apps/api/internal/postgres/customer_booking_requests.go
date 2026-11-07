package postgres

import (
	"context"
	"crypto/sha256"
	"dapper.local/platform/internal/scheduling"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrCustomerBookingRequest = errors.New("explicit reviewed customer request inputs required")
var ErrCustomerBookingRequestConflict = errors.New("customer request key conflicts with an earlier payload")

type CustomerBookingRequestInput struct {
	RequestKey    string    `json:"requestKey"`
	OfferID       string    `json:"offerId"`
	OfferVersion  int       `json:"offerVersion"`
	Start         time.Time `json:"start"`
	CustomerNote  string    `json:"customerNote"`
	TermsReviewed bool      `json:"termsReviewed"`
}
type CustomerBookingRequest struct {
	CustomerName   string             `json:"customerName"`
	Vehicle        Vehicle            `json:"vehicle"`
	ID             string             `json:"id"`
	CustomerID     string             `json:"customerId"`
	VehicleID      string             `json:"vehicleId"`
	Offer          PortalBookingOffer `json:"offer"`
	RequestedStart time.Time          `json:"requestedStart"`
	CustomerNote   string             `json:"customerNote"`
	CreatedAt      time.Time          `json:"createdAt"`
	Status         string             `json:"status"`
	Reserved       bool               `json:"reserved"`
}

const customerRequestColumns = `r.id::text,r.customer_id::text,(SELECT c.name FROM customers c WHERE (c.organization_id,c.location_id,c.id)=(r.organization_id,r.location_id,r.customer_id)),r.vehicle_id::text,r.vehicle_snapshot,r.offer_snapshot->'public',r.requested_start,r.customer_note,r.created_at`

func scanCustomerRequest(row pgx.Row) (CustomerBookingRequest, error) {
	r := CustomerBookingRequest{Status: "requested"}
	err := row.Scan(&r.ID, &r.CustomerID, &r.CustomerName, &r.VehicleID, &r.Vehicle, &r.Offer, &r.RequestedStart, &r.CustomerNote, &r.CreatedAt)
	return r, err
}
func (s *Store) SubmitCustomerBookingRequest(ctx context.Context, actor, customer, vehicle string, input CustomerBookingRequestInput) (CustomerBookingRequest, error) {
	input.Start = input.Start.UTC()
	input.CustomerNote = strings.TrimSpace(input.CustomerNote)
	if actor == "" || customer == "" || vehicle == "" {
		return CustomerBookingRequest{}, ErrForbidden
	}
	if !input.TermsReviewed || input.OfferID == "" || input.OfferVersion < 1 || input.Start.IsZero() || !exactTime(input.Start) || utf8.RuneCountInString(input.CustomerNote) > 2000 || len(input.RequestKey) < 16 || len(input.RequestKey) > 128 {
		return CustomerBookingRequest{}, ErrCustomerBookingRequest
	}
	for _, c := range input.RequestKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return CustomerBookingRequest{}, ErrCustomerBookingRequest
		}
	}
	payload, err := json.Marshal(struct {
		Customer, Vehicle string
		Input             CustomerBookingRequestInput
	}{customer, vehicle, input})
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	digest := sha256.Sum256(payload)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	defer tx.Rollback(ctx)
	var org, loc, category string
	var equipment Vehicle
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text,lower(btrim(v.category_description)),v.id::text,v.customer_id::text,v.description,v.category_description,v.identifier FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active JOIN vehicles v ON (v.organization_id,v.location_id,v.customer_id)=(a.organization_id,a.location_id,a.customer_id) WHERE a.principal_id=$1 AND a.customer_id=$2 AND v.id=$3 FOR SHARE OF a,p,v`, actor, customer, vehicle).Scan(&org, &loc, &category, &equipment.ID, &equipment.CustomerID, &equipment.Description, &equipment.CategoryDescription, &equipment.Identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerBookingRequest{}, ErrForbidden
	}
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, actor+"/customer-booking-request/"+input.RequestKey); err != nil {
		return CustomerBookingRequest{}, err
	}
	var existingID string
	var hash []byte
	err = tx.QueryRow(ctx, `SELECT id::text,request_hash FROM customer_booking_requests WHERE actor_id=$1 AND request_key=$2`, actor, input.RequestKey).Scan(&existingID, &hash)
	if err == nil {
		if string(hash) != string(digest[:]) {
			return CustomerBookingRequest{}, ErrCustomerBookingRequestConflict
		}
		saved, e := scanCustomerRequest(tx.QueryRow(ctx, `SELECT `+customerRequestColumns+` FROM customer_booking_requests r WHERE r.id=$1 AND r.actor_id=$2 AND r.customer_id=$3 AND r.vehicle_id=$4`, existingID, actor, customer, vehicle))
		if e != nil {
			return CustomerBookingRequest{}, e
		}
		if err = tx.Commit(ctx); err != nil {
			return CustomerBookingRequest{}, err
		}
		return saved, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CustomerBookingRequest{}, err
	}
	var resource string
	err = tx.QueryRow(ctx, `SELECT resource_id::text FROM booking_offers WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND category_key=$4 AND published AND version=$5`, org, loc, input.OfferID, category, input.OfferVersion).Scan(&resource)
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerBookingRequest{}, ErrBookingOfferConflict
	}
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	var plan scheduling.Plan
	var capacity int
	var timezone string
	err = tx.QueryRow(ctx, `SELECT capacity,rules,timezone FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, org, loc, resource).Scan(&capacity, &plan, &timezone)
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	plan.Capacity = capacity
	plan.Existing = nil
	offer, err := scanBookingOffer(tx.QueryRow(ctx, `SELECT `+offerColumns+` FROM booking_offers WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND category_key=$4 AND published AND version=$5 FOR SHARE`, org, loc, input.OfferID, category, input.OfferVersion))
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerBookingRequest{}, ErrBookingOfferConflict
	}
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	if offer.ResourceID != resource {
		return CustomerBookingRequest{}, ErrBookingOfferConflict
	}
	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: actor}
	rule, err := resolveReservationRule(ctx, tx, scope, resource, reservationRuleChoice{offer.RuleID, offer.RuleVersion})
	if errors.Is(err, ErrSchedulingRule) || errors.Is(err, ErrSchedulingRuleConflict) {
		return CustomerBookingRequest{}, ErrBookingOfferConflict
	}
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	now := time.Now().UTC()
	if input.Start.Before(now.Add(time.Duration(offer.LeadMinutes)*time.Minute)) || !input.Start.Before(now.Add(time.Duration(offer.AdvanceDays)*24*time.Hour)) {
		return CustomerBookingRequest{}, ErrReservationConflict
	}
	request := scheduling.Request{Duration: time.Duration(rule.DurationMinutes) * time.Minute, Before: time.Duration(rule.BeforeMinutes) * time.Minute, After: time.Duration(rule.AfterMinutes) * time.Minute, Units: 1, WorkingTime: rule.WorkingTime}
	plan.Capacity = rule.Slots // each service has its own number of slots
	reach := request.Duration + request.After
	if request.WorkingTime {
		reach = 30 * 24 * time.Hour // a working-time job can span many days
	}
	rows, err := tx.Query(ctx, `SELECT occupied_start,occupied_end,units,COALESCE(rule_snapshot->>'serviceId','') FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND cancelled_at IS NULL AND occupied_start<$5 AND occupied_end>$4 LIMIT 1001`, org, loc, resource, input.Start.Add(-request.Before), input.Start.Add(reach))
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	for rows.Next() {
		var allocation scheduling.Allocation
		var service string
		if err = rows.Scan(&allocation.Start, &allocation.End, &allocation.Units, &service); err != nil {
			rows.Close()
			return CustomerBookingRequest{}, err
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
		return CustomerBookingRequest{}, err
	}
	if len(plan.Existing) > 1000 || len(plan.Open) > 1000 || len(plan.Closed) > 1000 {
		return CustomerBookingRequest{}, ErrAvailabilityLimit
	}
	page, err := scheduling.AvailableStarts(plan, request, input.Start, input.Start.Add(time.Microsecond), time.Duration(offer.SlotMinutes)*time.Minute, 1, 1)
	if err != nil {
		return CustomerBookingRequest{}, ErrCustomerBookingRequest
	}
	if len(page.Starts) != 1 || !page.Starts[0].Equal(input.Start) {
		return CustomerBookingRequest{}, ErrReservationConflict
	}
	public := PortalBookingOffer{ID: offer.ID, Version: offer.Version, Title: offer.Title, EquipmentCategory: offer.EquipmentCategory, Mode: offer.Mode, Currency: offer.Currency, SubtotalCents: offer.SubtotalCents, TaxCents: offer.TaxCents, DepositCents: offer.DepositCents, Terms: offer.Terms, CancellationTerms: offer.CancellationTerms, DurationMinutes: rule.DurationMinutes, Timezone: timezone}
	snapshot := struct {
		Public        PortalBookingOffer `json:"public"`
		Configuration BookingOffer       `json:"configuration"`
	}{public, offer}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO customer_booking_requests(organization_id,location_id,customer_id,vehicle_id,actor_id,request_key,request_hash,offer_id,offer_version,offer_snapshot,rule_snapshot,requested_start,customer_note,terms_reviewed,vehicle_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,true,$14) RETURNING id::text`, org, loc, customer, vehicle, actor, input.RequestKey, digest[:], offer.ID, offer.Version, snapshot, rule, input.Start, input.CustomerNote, equipment).Scan(&id)
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO customer_booking_request_events(request_id,organization_id,location_id,actor_id,action) VALUES($1,$2,$3,$4,'submitted')`, id, org, loc, actor); err != nil {
		return CustomerBookingRequest{}, err
	}
	if err = jobAudit(ctx, tx, scope, "customer_booking_request.submitted", id); err != nil {
		return CustomerBookingRequest{}, err
	}
	saved, err := scanCustomerRequest(tx.QueryRow(ctx, `SELECT `+customerRequestColumns+` FROM customer_booking_requests r WHERE r.id=$1`, id))
	if err != nil {
		return CustomerBookingRequest{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CustomerBookingRequest{}, err
	}
	return saved, nil
}
func customerRequestsInTransaction(ctx context.Context, tx pgx.Tx, org, loc, customer, actor string) ([]CustomerBookingRequest, error) {
	rows, err := tx.Query(ctx, `SELECT `+customerRequestColumns+` FROM customer_booking_requests r WHERE r.organization_id=$1 AND r.location_id=$2 AND ($3='' OR r.customer_id::text=$3) AND ($4='' OR r.actor_id::text=$4) ORDER BY r.created_at DESC,r.id LIMIT 100`, org, loc, customer, actor)
	if err != nil {
		return nil, err
	}
	items := []CustomerBookingRequest{}
	for rows.Next() {
		r, e := scanCustomerRequest(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		items = append(items, r)
	}
	err = rows.Err()
	rows.Close()
	return items, err
}
func (s *Store) CustomerBookingRequests(ctx context.Context, actor, customer string) ([]CustomerBookingRequest, error) {
	if actor == "" || customer == "" {
		return nil, ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var org, loc string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`, actor, customer).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, err
	}
	items, err := customerRequestsInTransaction(ctx, tx, org, loc, customer, actor)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}
func (s *Store) StaffCustomerBookingRequests(ctx context.Context, scope Scope) ([]CustomerBookingRequest, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.read"); err != nil {
		return nil, err
	}
	items, err := customerRequestsInTransaction(ctx, tx, scope.OrganizationID, scope.LocationID, "", "")
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}
