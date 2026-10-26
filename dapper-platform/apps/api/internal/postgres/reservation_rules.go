package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type reservationRuleChoice struct {
	ID      string
	Version int
}

// ReserveServiceCapacity resolves all timing/units from the current exact rule.
// The caller supplies only resource/rule selectors, expected version and start.
func (s *Store) ReserveServiceCapacity(ctx context.Context, scope Scope, resourceID, ruleID string, ruleVersion int, start time.Time) (CapacityReservation, error) {
	if ruleID == "" || ruleVersion < 1 {
		return CapacityReservation{}, ErrSchedulingRule
	}
	return s.writeReservation(ctx, scope, resourceID, "", 0, scheduling.Request{Start: start}, &reservationRuleChoice{ruleID, ruleVersion})
}

// RescheduleServiceCapacity rechecks both reservation and rule versions, keeps
// exact combination identity and resolves current timing under the same lock.
func (s *Store) RescheduleServiceCapacity(ctx context.Context, scope Scope, resourceID, ruleID, id string, ruleVersion, version int, start time.Time) (CapacityReservation, error) {
	if ruleID == "" || id == "" || ruleVersion < 1 || version < 1 {
		return CapacityReservation{}, ErrReservationInput
	}
	return s.writeReservation(ctx, scope, resourceID, id, version, scheduling.Request{Start: start}, &reservationRuleChoice{ruleID, ruleVersion})
}
func resolveReservationRule(ctx context.Context, tx pgx.Tx, scope Scope, resourceID string, choice reservationRuleChoice) (SchedulingRule, error) {
	rule, err := scanRule(tx.QueryRow(ctx, `SELECT `+ruleColumns+` FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND id=$4 FOR SHARE`, scope.OrganizationID, scope.LocationID, resourceID, choice.ID))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && rule.Version != choice.Version {
		return SchedulingRule{}, ErrSchedulingRuleConflict
	}
	if err != nil {
		return SchedulingRule{}, err
	}
	var main string
	err = tx.QueryRow(ctx, `SELECT id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='main' AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, rule.ServiceID).Scan(&main)
	if errors.Is(err, pgx.ErrNoRows) {
		return SchedulingRule{}, ErrSchedulingRule
	}
	if err != nil {
		return SchedulingRule{}, err
	}
	for _, id := range rule.AddonIDs {
		var child string
		err = tx.QueryRow(ctx, `SELECT id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='addon' AND parent_id=$4 AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, id, rule.ServiceID).Scan(&child)
		if errors.Is(err, pgx.ErrNoRows) {
			return SchedulingRule{}, ErrSchedulingRule
		}
		if err != nil {
			return SchedulingRule{}, err
		}
	}
	return rule, nil
}
