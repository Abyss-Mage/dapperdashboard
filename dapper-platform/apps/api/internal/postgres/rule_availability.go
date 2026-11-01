package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrAvailabilityInput = errors.New("choose a configured service, a resource and up to 31 days")

// RuleAvailabilityDay lists the start times that can be booked on one calendar day of the resource's own time zone.
type RuleAvailabilityDay struct {
	Date   string      `json:"date"`
	Starts []time.Time `json:"starts"`
}

type RuleAvailability struct {
	Timezone        string                `json:"timezone"`
	DurationMinutes int                   `json:"durationMinutes"`
	Days            []RuleAvailabilityDay `json:"days"`
}

const ruleAvailabilityStep = 30 * time.Minute

// RuleAvailability answers "which days and times can this service be booked?" for staff screens. It uses the same Evaluate
// authority as saving a booking, so a time offered here is one the save will accept (unless someone else books it first).
// The caller has already checked the person may book at this scope.
func (s *Store) RuleAvailability(ctx context.Context, scope Scope, resourceID, ruleID string, ruleVersion int, firstDay string, days int, allowPast ...bool) (RuleAvailability, error) {
	past := len(allowPast) > 0 && allowPast[0] // administrators entering a booking that already happened
	out := RuleAvailability{Days: []RuleAvailabilityDay{}}
	if resourceID == "" || ruleID == "" || ruleVersion < 1 || days < 1 || days > 31 {
		return out, ErrAvailabilityInput
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var plan scheduling.Plan
	var capacity int
	err = tx.QueryRow(ctx, `SELECT capacity,rules,timezone FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, scope.OrganizationID, scope.LocationID, resourceID).Scan(&capacity, &plan, &out.Timezone)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrAvailabilityInput
	}
	if err != nil {
		return out, err
	}
	plan.Capacity = capacity
	plan.Existing = nil
	zone, err := time.LoadLocation(out.Timezone)
	if err != nil {
		return out, err
	}
	first, err := time.ParseInLocation("2006-01-02", firstDay, zone)
	if err != nil || first.Format("2006-01-02") != firstDay {
		return out, ErrAvailabilityInput
	}
	rule, err := resolveReservationRule(ctx, tx, scope, resourceID, reservationRuleChoice{ruleID, ruleVersion})
	if err != nil {
		return out, err
	}
	out.DurationMinutes = rule.DurationMinutes
	request := scheduling.Request{Duration: time.Duration(rule.DurationMinutes) * time.Minute, Before: time.Duration(rule.BeforeMinutes) * time.Minute, After: time.Duration(rule.AfterMinutes) * time.Minute, Units: 1, WorkingTime: rule.WorkingTime}
	plan.Capacity = rule.Slots // each service has its own number of slots
	end := first.AddDate(0, 0, days)
	reach := request.Duration + request.After
	if request.WorkingTime {
		reach = 30 * 24 * time.Hour // a working-time job can run across many days
	}
	rows, err := tx.Query(ctx, `SELECT occupied_start,occupied_end,units,COALESCE(rule_snapshot->>'serviceId','') FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND cancelled_at IS NULL AND occupied_start<$5 AND occupied_end>$4 LIMIT 1001`, scope.OrganizationID, scope.LocationID, resourceID, first.Add(-request.Before), end.Add(reach))
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var allocation scheduling.Allocation
		var service string
		if err = rows.Scan(&allocation.Start, &allocation.End, &allocation.Units, &service); err != nil {
			rows.Close()
			return out, err
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
		return out, err
	}
	if len(plan.Existing) > 1000 || len(plan.Open) > 1000 || len(plan.Closed) > 1000 {
		return out, ErrAvailabilityLimit
	}
	now := time.Now().UTC()
	for i := 0; i < days; i++ {
		dayStart := first.AddDate(0, 0, i)
		dayEnd := first.AddDate(0, 0, i+1)
		item := RuleAvailabilityDay{Date: dayStart.Format("2006-01-02"), Starts: []time.Time{}}
		from := dayStart
		if from.Before(now) && !past {
			from = now.Truncate(ruleAvailabilityStep).Add(ruleAvailabilityStep)
		}
		if from.Before(dayEnd) {
			page, e := scheduling.AvailableStarts(plan, request, from, dayEnd, ruleAvailabilityStep, 100, 1000)
			if e != nil {
				return out, ErrAvailabilityInput
			}
			item.Starts = page.Starts
		}
		out.Days = append(out.Days, item)
	}
	return out, tx.Commit(ctx)
}

// EmployeeRuleAvailability is RuleAvailability for the employee's own worksite (needs the walk-in permission).
func (s *Store) EmployeeRuleAvailability(ctx context.Context, actor, resourceID, ruleID string, ruleVersion int, firstDay string, days int) (RuleAvailability, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RuleAvailability{}, err
	}
	a, err := employeeWorksite(ctx, tx, actor)
	if err == nil {
		err = authorizeTransaction(ctx, tx, employeeScope(a), "customers.create")
	}
	tx.Rollback(ctx)
	if err != nil {
		return RuleAvailability{}, err
	}
	return s.RuleAvailability(ctx, employeeScope(a), resourceID, ruleID, ruleVersion, firstDay, days)
}

// EmployeeBookingTaxes lets the guided booking screen show a total with tax before saving (the invoice recalculates it).
func (s *Store) EmployeeBookingTaxes(ctx context.Context, actor string) (FranchiseTaxes, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FranchiseTaxes{}, err
	}
	a, err := employeeWorksite(ctx, tx, actor)
	if err == nil {
		err = authorizeTransaction(ctx, tx, employeeScope(a), "customers.create")
	}
	tx.Rollback(ctx)
	if err != nil {
		return FranchiseTaxes{}, err
	}
	return s.franchiseTaxesUnchecked(ctx, employeeScope(a))
}
