package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrReservationInput = errors.New("explicit valid resource and scheduling inputs required")
var ErrReservationConflict = errors.New("reservation unavailable, stale or capacity exhausted")

type SchedulingResource struct {
	ID, Name, Timezone string
	Plan               scheduling.Plan
	WeeklyHours        []HourWindow
}
type CapacityReservation struct {
	ID, ResourceID string
	Request        scheduling.Request
	Occupied       scheduling.Interval
	Version        int
	CancelledAt    *time.Time
	CancelledBy    *string
	Rule           *SchedulingRule
}

func exactTime(t time.Time) bool {
	return t.Year() >= 1 && t.Year() <= 9999 && t.Nanosecond()%1000 == 0
}
func exactRequest(r scheduling.Request) bool {
	return exactTime(r.Start) && r.Duration%time.Microsecond == 0 && r.Before%time.Microsecond == 0 && r.After%time.Microsecond == 0
}

// CreateSchedulingResource requires explicit dated rules and a separate grant.
// No public API exposes rule approval or derives values from legacy data.
func (s *Store) CreateSchedulingResource(ctx context.Context, scope Scope, input SchedulingResource) (SchedulingResource, error) {
	input.Name = strings.TrimSpace(input.Name)
	if n := utf8.RuneCountInString(input.Name); n < 2 || n > 120 {
		return SchedulingResource{}, ErrReservationInput
	}
	if input.Timezone == "" || input.Timezone == "Local" || len(input.Plan.Existing) > 0 {
		return SchedulingResource{}, ErrReservationInput
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return SchedulingResource{}, ErrReservationInput
	}
	for _, intervals := range [][]scheduling.Interval{input.Plan.Open, input.Plan.Closed} {
		for _, i := range intervals {
			if !exactTime(i.Start) || !exactTime(i.End) {
				return SchedulingResource{}, ErrReservationInput
			}
		}
	}
	if err := scheduling.ValidatePlan(input.Plan); err != nil {
		return SchedulingResource{}, ErrReservationInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SchedulingResource{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.manage"); err != nil {
		return SchedulingResource{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO scheduling_resources(organization_id,location_id,name,capacity,timezone,rules) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, scope.OrganizationID, scope.LocationID, input.Name, input.Plan.Capacity, input.Timezone, input.Plan).Scan(&input.ID)
	if err != nil {
		return SchedulingResource{}, err
	}
	if err = jobAudit(ctx, tx, scope, "scheduling.resource.created", input.ID); err != nil {
		return SchedulingResource{}, err
	}
	// A new bay can take every service the franchise offers straight away. Most services have no typed duration, so their timing comes from the bays already there.
	if _, err = tx.Exec(ctx, `WITH added AS (
INSERT INTO service_scheduling_rules(organization_id,location_id,resource_id,service_id,addon_ids,duration_minutes,before_minutes,after_minutes,units,working_time,slots)
SELECT DISTINCT ON (r.service_id) r.organization_id,r.location_id,$3::uuid,r.service_id,'[]'::jsonb,r.duration_minutes,r.before_minutes,r.after_minutes,r.units,r.working_time,LEAST(1000,GREATEST(1,$4::int/r.units))
FROM service_scheduling_rules r JOIN catalog_services c ON c.id=r.service_id AND NOT c.archived
WHERE r.organization_id=$1::uuid AND r.location_id=$2::uuid AND r.resource_id<>$3::uuid AND r.addon_ids='[]'::jsonb AND r.units<=$4::int
ORDER BY r.service_id,r.resource_id RETURNING *)
INSERT INTO service_scheduling_revisions(rule_id,version,snapshot,actor_id) SELECT a.id,a.version,to_jsonb(a),$5::uuid FROM added a`, scope.OrganizationID, scope.LocationID, input.ID, input.Plan.Capacity, scope.ActorID); err != nil {
		return SchedulingResource{}, err
	}
	if err = syncServiceDurationRules(ctx, tx, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return SchedulingResource{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SchedulingResource{}, err
	}
	return input, nil
}

// ReserveCapacity is a technical reservation, not booking/payment confirmation.
func (s *Store) ReserveCapacity(ctx context.Context, scope Scope, resourceID string, request scheduling.Request) (CapacityReservation, error) {
	return s.writeReservation(ctx, scope, resourceID, "", 0, request, nil)
}

// RescheduleCapacity keeps the same authorized resource and replaces its old
// interval only if the new one fits. A failed/stale move retains the old slot.
func (s *Store) RescheduleCapacity(ctx context.Context, scope Scope, resourceID, id string, version int, request scheduling.Request) (CapacityReservation, error) {
	if id == "" || version < 1 {
		return CapacityReservation{}, ErrReservationInput
	}
	return s.writeReservation(ctx, scope, resourceID, id, version, request, nil)
}
func (s *Store) writeReservation(ctx context.Context, scope Scope, resourceID, id string, version int, request scheduling.Request, choice *reservationRuleChoice) (CapacityReservation, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return CapacityReservation{}, err
	}
	defer tx.Rollback(ctx)
	result, err := s.writeReservationInTransaction(ctx, tx, scope, resourceID, id, version, request, choice)
	if err != nil {
		return CapacityReservation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CapacityReservation{}, reservationCommitError(err)
	}
	return result, nil
}
func (s *Store) writeReservationInTransaction(ctx context.Context, tx pgx.Tx, scope Scope, resourceID, id string, version int, request scheduling.Request, choice *reservationRuleChoice) (CapacityReservation, error) {
	if !exactRequest(request) {
		return CapacityReservation{}, ErrReservationInput
	}

	var err error
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.reserve"); err != nil {
		return CapacityReservation{}, err
	}
	var plan scheduling.Plan
	var capacity int
	var weekly []HourWindow
	var zone string
	// Every cooperating create/move locks the resource BEFORE reading allocations.
	// READ COMMITTED's subsequent query sees prior lock-holder commits.
	err = tx.QueryRow(ctx, `SELECT capacity,rules,timezone,weekly_hours FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, resourceID).Scan(&capacity, &plan, &zone, &weekly)
	if errors.Is(err, pgx.ErrNoRows) {
		return CapacityReservation{}, ErrReservationConflict
	}
	if err != nil {
		return CapacityReservation{}, err
	}
	plan.Capacity = capacity
	plan.Existing = nil
	// A time that has already passed can only be entered by someone who manages bookings (to record work that already happened).
	if !request.Start.IsZero() && request.Start.Before(time.Now()) && !insideOpenWindow(plan, request.Start) {
		if err = authorizeTransaction(ctx, tx, scope, "bookings.manage"); err != nil {
			return CapacityReservation{}, err
		}
		addPastWindows(&plan, zone, weekly, request.Start.Add(-24*time.Hour), request.Start.Add(48*time.Hour))
	}
	var resolvedRule *SchedulingRule
	if choice != nil {
		rule, resolveErr := resolveReservationRule(ctx, tx, scope, resourceID, *choice)
		if resolveErr != nil {
			return CapacityReservation{}, resolveErr
		}
		resolvedRule = &rule
		request.Duration = time.Duration(rule.DurationMinutes) * time.Minute
		request.Before = time.Duration(rule.BeforeMinutes) * time.Minute
		request.After = time.Duration(rule.AfterMinutes) * time.Minute
		request.Units = rule.Units
		request.WorkingTime = rule.WorkingTime
	}
	// With a service rule, only bookings of the same service count and the service has its own number of slots.
	evalRequest := request
	if resolvedRule != nil {
		evalRequest.Units = 1
		plan.Capacity = resolvedRule.Slots
	}
	if id != "" {
		var current int
		var existingRule *string
		err = tx.QueryRow(ctx, `SELECT version,rule_id::text FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND id=$4 AND cancelled_at IS NULL FOR UPDATE`, scope.OrganizationID, scope.LocationID, resourceID, id).Scan(&current, &existingRule)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && current != version {
			return CapacityReservation{}, ErrReservationConflict
		}
		if err != nil {
			return CapacityReservation{}, err
		}
		if existingRule != nil && (choice == nil || *existingRule != choice.ID) || existingRule == nil && choice != nil {
			return CapacityReservation{}, ErrReservationInput
		}
	}
	// Bound work before loading allocations. Only intersecting rows are needed.
	probe, err := scheduling.Evaluate(plan, evalRequest)
	if err != nil {
		return CapacityReservation{}, ErrReservationInput
	}
	if !probe.Available {
		return CapacityReservation{}, ErrReservationConflict
	}
	if !exactTime(probe.Occupied.Start) || !exactTime(probe.Occupied.End) {
		return CapacityReservation{}, ErrReservationInput
	}
	rows, err := tx.Query(ctx, `SELECT occupied_start,occupied_end,units,COALESCE(rule_snapshot->>'serviceId','') FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND cancelled_at IS NULL AND ($4='' OR id::text<>$4) AND occupied_start<$6 AND occupied_end>$5 LIMIT 10001`, scope.OrganizationID, scope.LocationID, resourceID, id, probe.Occupied.Start, probe.Occupied.End)
	if err != nil {
		return CapacityReservation{}, err
	}
	for rows.Next() {
		var a scheduling.Allocation
		var service string
		if err = rows.Scan(&a.Start, &a.End, &a.Units, &service); err != nil {
			rows.Close()
			return CapacityReservation{}, err
		}
		if resolvedRule != nil {
			if service != resolvedRule.ServiceID {
				continue
			}
			a.Units = 1
		}
		plan.Existing = append(plan.Existing, a)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return CapacityReservation{}, err
	}
	decision, err := scheduling.Evaluate(plan, evalRequest)
	if err != nil {
		return CapacityReservation{}, ErrReservationInput
	}
	if !decision.Available {
		return CapacityReservation{}, ErrReservationConflict
	}
	result := CapacityReservation{ResourceID: resourceID, Request: request, Occupied: decision.Occupied, Rule: resolvedRule}
	action := "scheduling.reserved"
	if id == "" {
		err = tx.QueryRow(ctx, `INSERT INTO capacity_reservations(organization_id,location_id,resource_id,occupied_start,occupied_end,units,request) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id::text,version`, scope.OrganizationID, scope.LocationID, resourceID, decision.Occupied.Start, decision.Occupied.End, request.Units, request).Scan(&result.ID, &result.Version)
	} else {
		action = "scheduling.rescheduled"
		err = tx.QueryRow(ctx, `UPDATE capacity_reservations SET occupied_start=$5,occupied_end=$6,units=$7,request=$8,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND id=$4 RETURNING id::text,version`, scope.OrganizationID, scope.LocationID, resourceID, id, decision.Occupied.Start, decision.Occupied.End, request.Units, request).Scan(&result.ID, &result.Version)
	}
	if err != nil {
		return CapacityReservation{}, err
	}
	if resolvedRule != nil {
		if _, err = tx.Exec(ctx, `UPDATE capacity_reservations SET rule_id=$2,rule_version=$3,rule_snapshot=$4 WHERE id=$1`, result.ID, resolvedRule.ID, resolvedRule.Version, resolvedRule); err != nil {
			return CapacityReservation{}, err
		}
	}
	if err = reservationRevision(ctx, tx, scope, result); err != nil {
		return CapacityReservation{}, err
	}
	if err = jobAudit(ctx, tx, scope, action, result.ID); err != nil {
		return CapacityReservation{}, err
	}
	return result, nil
}
