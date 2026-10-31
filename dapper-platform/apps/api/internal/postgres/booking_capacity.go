package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"dapper.local/platform/internal/scheduling"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrBookingReservation = errors.New("booking requires an available matching service reservation not used by another booking")
var ErrBookingCapacityRequest = errors.New("booking reservation request key conflicts with an earlier actor or payload")

func bookingWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && ((pgErr.Code == "23505" && pgErr.ConstraintName == "bookings_reservation_identity") || (pgErr.Code == "23514" && pgErr.ConstraintName == "booking_capacity_consistency")) {
		return ErrBookingReservation
	}
	return err
}
func reservationCommitError(err error) error {
	if errors.Is(bookingWriteError(err), ErrBookingReservation) {
		return ErrReservationConflict
	}
	return err
}

type ReserveBookingInput struct {
	RequestKey  string    `json:"requestKey"`
	CustomerID  string    `json:"customerId"`
	VehicleID   *string   `json:"vehicleId"`
	ResourceID  string    `json:"resourceId"`
	RuleID      string    `json:"ruleId"`
	RuleVersion int       `json:"ruleVersion"`
	Start       time.Time `json:"start"`
	Channel     string    `json:"channel"`
	Notes       string    `json:"notes"`
	// ServiceAddress is where mobile or valet work happens (empty for in-shop work).
	ServiceAddress string `json:"serviceAddress"`
	// AddonIDs attach add-ons to a base rule (a rule that already lists its add-ons cannot take more).
	AddonIDs []string `json:"addonIds"`
}

// ReserveBooking shares the authoritative scheduling/booking helpers and writes
// capacity, rule/service snapshot, booking, histories and retry receipt together.
// This staff operation does not confirm payment or approve booking policy.
func (s *Store) ReserveBooking(ctx context.Context, scope Scope, input ReserveBookingInput) (Booking, error) {
	input.Notes = strings.TrimSpace(input.Notes)
	input.Start = input.Start.UTC()
	if len(input.RequestKey) < 16 || len(input.RequestKey) > 128 || input.CustomerID == "" || input.ResourceID == "" || input.RuleID == "" || input.RuleVersion < 1 || input.Start.IsZero() || !exactTime(input.Start) || utf8.RuneCountInString(input.Notes) > 2000 {
		return Booking{}, ErrBookingInput
	}
	for _, c := range input.RequestKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return Booking{}, ErrBookingInput
		}
	}
	switch input.Channel {
	case "internal", "phone", "walk_in", "dealership", "online":
	default:
		return Booking{}, ErrBookingInput
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return Booking{}, ErrBookingInput
	}
	hash := sha256.Sum256(encoded)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	for _, action := range []string{"bookings.create", "scheduling.reserve"} {
		if err = authorizeTransaction(ctx, tx, scope, action); err != nil {
			return Booking{}, err
		}
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.OrganizationID+"/"+scope.LocationID+"/booking-capacity/"+input.RequestKey); err != nil {
		return Booking{}, err
	}
	var existingID, existingActor string
	var existingHash []byte
	err = tx.QueryRow(ctx, `SELECT booking_id::text,actor_id::text,request_hash FROM booking_capacity_requests WHERE organization_id=$1 AND location_id=$2 AND request_key=$3`, scope.OrganizationID, scope.LocationID, input.RequestKey).Scan(&existingID, &existingActor, &existingHash)
	if err == nil {
		if existingActor != scope.ActorID || len(existingHash) != len(hash) || string(existingHash) != string(hash[:]) {
			return Booking{}, ErrBookingCapacityRequest
		}
		booking, e := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3 FOR SHARE OF b`, scope.OrganizationID, scope.LocationID, existingID))
		if e != nil {
			return Booking{}, e
		}
		if err = tx.Commit(ctx); err != nil {
			return Booking{}, bookingWriteError(err)
		}
		return bookingMutationSummary(booking), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, err
	}
	reservation, err := s.writeReservationInTransaction(ctx, tx, scope, input.ResourceID, "", 0, scheduling.Request{Start: input.Start}, &reservationRuleChoice{ID: input.RuleID, Version: input.RuleVersion})
	if err != nil {
		return Booking{}, err
	}
	rule := reservation.Rule
	if rule == nil {
		return Booking{}, ErrSchedulingRule
	}
	addons := rule.AddonIDs
	if len(input.AddonIDs) > 0 {
		if len(rule.AddonIDs) > 0 || len(input.AddonIDs) > 20 {
			return Booking{}, ErrBookingInput
		}
		seen := map[string]bool{}
		for _, id := range input.AddonIDs {
			var ok bool
			if seen[id] || tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND kind='addon' AND parent_id::text=$4 AND NOT archived)`, scope.OrganizationID, scope.LocationID, id, rule.ServiceID).Scan(&ok) != nil || !ok {
				return Booking{}, ErrBookingInput
			}
			seen[id] = true
		}
		addons = append([]string{}, input.AddonIDs...)
	}
	booking, err := s.createBookingInTransaction(ctx, tx, scope, Booking{CustomerID: input.CustomerID, VehicleID: input.VehicleID, ServiceID: &rule.ServiceID, AddonIDs: addons, ReservationID: &reservation.ID, Channel: input.Channel, Status: "confirmed", BookingTime: reservation.Request.Start, Notes: input.Notes, ServiceAddress: input.ServiceAddress})
	if err != nil {
		return Booking{}, bookingWriteError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_capacity_requests(organization_id,location_id,request_key,request_hash,actor_id,booking_id) VALUES($1,$2,$3,$4,$5,$6)`, scope.OrganizationID, scope.LocationID, input.RequestKey, hash[:], scope.ActorID, booking.ID); err != nil {
		return Booking{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, bookingWriteError(err)
	}
	return bookingMutationSummary(booking), nil
}

func bookingMutationSummary(b Booking) Booking {
	return Booking{ReservationVersion: b.ReservationVersion, ID: b.ID, OrganizationID: b.OrganizationID, LocationID: b.LocationID, CustomerID: b.CustomerID, VehicleID: b.VehicleID, ServiceID: b.ServiceID, ServiceName: b.ServiceName, Addons: b.Addons, ReservationID: b.ReservationID, Channel: b.Channel, Status: b.Status, BookingTime: b.BookingTime, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt}
}
