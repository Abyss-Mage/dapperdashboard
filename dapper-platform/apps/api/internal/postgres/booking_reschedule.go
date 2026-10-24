package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"dapper.local/platform/internal/scheduling"
	"github.com/jackc/pgx/v5"
)

type RescheduleBookingInput struct {
	RequestKey         string    `json:"requestKey"`
	ResourceID         string    `json:"resourceId"`
	RuleID             string    `json:"ruleId"`
	RuleVersion        int       `json:"ruleVersion"`
	ReservationVersion int       `json:"reservationVersion"`
	Start              time.Time `json:"start"`
	Notes              string    `json:"notes"`
}

// RescheduleBooking moves the existing reservation and booking atomically. It
// retains the booking's identity/status and does not modify payment or jobs.
func (s *Store) RescheduleBooking(ctx context.Context, scope Scope, id string, input RescheduleBookingInput) (Booking, error) {
	input.Notes = strings.TrimSpace(input.Notes)
	input.Start = input.Start.UTC()
	if id == "" || input.ResourceID == "" || input.RuleID == "" || input.RuleVersion < 1 || input.ReservationVersion < 1 || input.Start.IsZero() || !exactTime(input.Start) || utf8.RuneCountInString(input.Notes) > 2000 || len(input.RequestKey) < 16 || len(input.RequestKey) > 128 {
		return Booking{}, ErrBookingInput
	}
	for _, c := range input.RequestKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return Booking{}, ErrBookingInput
		}
	}
	encoded, err := json.Marshal(struct {
		BookingID string
		Input     RescheduleBookingInput
	}{id, input})
	if err != nil {
		return Booking{}, err
	}
	digest := sha256.Sum256(encoded)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	for _, action := range []string{"bookings.manage", "scheduling.reserve", "scheduling.release"} {
		if err = authorizeTransaction(ctx, tx, scope, action); err != nil {
			return Booking{}, err
		}
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.OrganizationID+"/"+scope.LocationID+"/booking-reschedule/"+input.RequestKey); err != nil {
		return Booking{}, err
	}
	var receiptID, receiptActor string
	var receiptHash []byte
	err = tx.QueryRow(ctx, `SELECT booking_id::text,actor_id::text,request_hash FROM booking_reschedule_requests WHERE organization_id=$1 AND location_id=$2 AND request_key=$3`, scope.OrganizationID, scope.LocationID, input.RequestKey).Scan(&receiptID, &receiptActor, &receiptHash)
	if err == nil {
		if receiptID != id || receiptActor != scope.ActorID || string(receiptHash) != string(digest[:]) {
			return Booking{}, ErrBookingCapacityRequest
		}
		b, e := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3 FOR SHARE OF b`, scope.OrganizationID, scope.LocationID, id))
		if e != nil {
			return Booking{}, e
		}
		if err = tx.Commit(ctx); err != nil {
			return Booking{}, err
		}
		return bookingMutationSummary(b), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, err
	}
	var reservation, oldResource *string
	err = tx.QueryRow(ctx, `SELECT b.reservation_id::text,r.resource_id::text FROM bookings b LEFT JOIN capacity_reservations r ON (r.organization_id,r.location_id,r.id)=(b.organization_id,b.location_id,b.reservation_id) WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3`, scope.OrganizationID, scope.LocationID, id).Scan(&reservation, &oldResource)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if reservation == nil || oldResource == nil {
		return Booking{}, ErrBookingReservation
	}
	resources := []string{*oldResource}
	if *oldResource != input.ResourceID {
		resources = append(resources, input.ResourceID)
	}
	sort.Strings(resources)
	for _, resource := range resources {
		var locked string
		err = tx.QueryRow(ctx, `SELECT id::text FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, resource).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrReservationConflict
		}
		if err != nil {
			return Booking{}, err
		}
	}
	var resource string
	var version int
	var cancelled *time.Time
	err = tx.QueryRow(ctx, `SELECT resource_id::text,version,cancelled_at FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, *reservation).Scan(&resource, &version, &cancelled)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrReservationConflict
	}
	if err != nil {
		return Booking{}, err
	}
	if resource != *oldResource || version != input.ReservationVersion || cancelled != nil {
		return Booking{}, ErrReservationConflict
	}
	b, err := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3 FOR UPDATE OF b`, scope.OrganizationID, scope.LocationID, id))
	if err != nil {
		return Booking{}, err
	}
	if b.ReservationID == nil || *b.ReservationID != *reservation {
		return Booking{}, ErrReservationConflict
	}
	switch b.Status {
	case "pending", "awaiting_payment", "confirmed", "scheduled":
	default:
		return Booking{}, ErrBookingTransition
	}
	choice := reservationRuleChoice{ID: input.RuleID, Version: input.RuleVersion}
	rule, err := resolveReservationRule(ctx, tx, scope, input.ResourceID, choice)
	if err != nil {
		return Booking{}, err
	}
	if b.ServiceID == nil || *b.ServiceID != rule.ServiceID || len(rule.AddonIDs) > 0 && len(b.Addons) != len(rule.AddonIDs) {
		return Booking{}, ErrBookingReservation
	}
	ids := map[string]bool{}
	for _, addon := range b.Addons {
		ids[addon.ServiceID] = true
	}
	for _, addon := range rule.AddonIDs {
		if !ids[addon] {
			return Booking{}, ErrBookingReservation
		}
	}
	// These intermediate selector changes are private to this transaction; the
	// shared evaluator and deferred binding checks must succeed before commit.
	if _, err = tx.Exec(ctx, `UPDATE capacity_reservations SET resource_id=$2,rule_id=$3,rule_version=$4 WHERE id=$1`, *reservation, input.ResourceID, input.RuleID, input.RuleVersion); err != nil {
		return Booking{}, err
	}
	moved, err := s.writeReservationInTransaction(ctx, tx, scope, input.ResourceID, *reservation, version, scheduling.Request{Start: input.Start}, &choice)
	if err != nil {
		return Booking{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bookings SET booking_time=$4,updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id, moved.Request.Start); err != nil {
		return Booking{}, err
	}
	eventNotes := "Rescheduled from " + b.BookingTime.UTC().Format(time.RFC3339Nano) + " to " + moved.Request.Start.UTC().Format(time.RFC3339Nano)
	if input.Notes != "" {
		eventNotes += " | " + input.Notes
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_events(organization_id,location_id,booking_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,$5,$5,$6)`, scope.OrganizationID, scope.LocationID, id, scope.ActorID, b.Status, eventNotes); err != nil {
		return Booking{}, err
	}
	if err = jobAudit(ctx, tx, scope, "bookings.rescheduled", id); err != nil {
		return Booking{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_reschedule_requests(organization_id,location_id,request_key,request_hash,actor_id,booking_id,from_version,to_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.OrganizationID, scope.LocationID, input.RequestKey, digest[:], scope.ActorID, id, version, moved.Version); err != nil {
		return Booking{}, err
	}
	b.BookingTime = moved.Request.Start
	b.ReservationVersion = &moved.Version
	if err = tx.QueryRow(ctx, `SELECT updated_at FROM bookings WHERE id=$1`, id).Scan(&b.UpdatedAt); err != nil {
		return Booking{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, bookingWriteError(err)
	}
	return bookingMutationSummary(b), nil
}
