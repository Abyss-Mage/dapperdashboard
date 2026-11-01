package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"sort"
)

var ErrSchedulingRule = errors.New("explicit active main/add-on combination and resource inputs required")
var ErrSchedulingRuleConflict = errors.New("scheduling rule duplicated, changed or unavailable")

type SchedulingRule struct {
	ID              string   `json:"id"`
	ResourceID      string   `json:"resourceId"`
	ServiceID       string   `json:"serviceId"`
	AddonIDs        []string `json:"addonIds"`
	DurationMinutes int      `json:"durationMinutes"`
	BeforeMinutes   int      `json:"beforeMinutes"`
	AfterMinutes    int      `json:"afterMinutes"`
	Units           int      `json:"units"`
	Slots           int      `json:"slots"` // how many bookings of this service can run at the same time
	WorkingTime     bool     `json:"workingTime"`
	Version         int      `json:"version"`
}

const ruleColumns = `id::text,resource_id::text,service_id::text,addon_ids,duration_minutes,before_minutes,after_minutes,units,working_time,version,slots`

func scanRule(row pgx.Row) (SchedulingRule, error) {
	var item SchedulingRule
	err := row.Scan(&item.ID, &item.ResourceID, &item.ServiceID, &item.AddonIDs, &item.DurationMinutes, &item.BeforeMinutes, &item.AfterMinutes, &item.Units, &item.WorkingTime, &item.Version, &item.Slots)
	return item, err
}

func (s *Store) SchedulingRules(ctx context.Context, scope Scope, resourceID string) ([]SchedulingRule, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.manage"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+ruleColumns+` FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 ORDER BY service_id,id LIMIT 100`, scope.OrganizationID, scope.LocationID, resourceID)
	if err != nil {
		return nil, err
	}
	items := []SchedulingRule{}
	for rows.Next() {
		item, err := scanRule(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

// SaveSchedulingRule configures the complete combination duration explicitly;
// adding an add-on never silently sums an inferred or unapproved time value.
// Resource/service/combination identity is fixed on edits, revisions are retained.
func (s *Store) SaveSchedulingRule(ctx context.Context, scope Scope, input SchedulingRule) (SchedulingRule, error) {
	if input.ResourceID == "" || input.ServiceID == "" || input.DurationMinutes < 1 || input.DurationMinutes > 10080 || input.BeforeMinutes < 0 || input.BeforeMinutes > 10080 || input.AfterMinutes < 0 || input.AfterMinutes > 10080 || input.Units < 1 || input.Units > 1000000 || input.Slots < 0 || input.Slots > 1000 || len(input.AddonIDs) > 100 || input.ID != "" && input.Version < 1 || input.ID == "" && input.Version != 0 {
		return SchedulingRule{}, ErrSchedulingRule
	}
	input.AddonIDs = append([]string{}, input.AddonIDs...)
	sort.Strings(input.AddonIDs)
	for i, id := range input.AddonIDs {
		if id == "" || i > 0 && id == input.AddonIDs[i-1] {
			return SchedulingRule{}, ErrSchedulingRule
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SchedulingRule{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.manage"); err != nil {
		return SchedulingRule{}, err
	}
	var capacity int
	err = tx.QueryRow(ctx, `SELECT capacity FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, scope.OrganizationID, scope.LocationID, input.ResourceID).Scan(&capacity)
	if errors.Is(err, pgx.ErrNoRows) {
		return SchedulingRule{}, ErrSchedulingRule
	}
	if err != nil {
		return SchedulingRule{}, err
	}
	if input.Units > capacity {
		return SchedulingRule{}, ErrSchedulingRule
	}
	if input.Slots == 0 {
		// Not given: keep what the shared calendar allowed before slots belonged to each service.
		input.Slots = capacity / input.Units
		if input.Slots < 1 {
			input.Slots = 1
		}
	}
	var main string
	err = tx.QueryRow(ctx, `SELECT id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='main' AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, input.ServiceID).Scan(&main)
	if errors.Is(err, pgx.ErrNoRows) {
		return SchedulingRule{}, ErrSchedulingRule
	}
	if err != nil {
		return SchedulingRule{}, err
	}
	for _, id := range input.AddonIDs {
		var child string
		err = tx.QueryRow(ctx, `SELECT id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='addon' AND parent_id=$4 AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, id, input.ServiceID).Scan(&child)
		if errors.Is(err, pgx.ErrNoRows) {
			return SchedulingRule{}, ErrSchedulingRule
		}
		if err != nil {
			return SchedulingRule{}, err
		}
	}
	var result SchedulingRule
	if input.ID == "" {
		result, err = scanRule(tx.QueryRow(ctx, `INSERT INTO service_scheduling_rules(organization_id,location_id,resource_id,service_id,addon_ids,duration_minutes,before_minutes,after_minutes,units,working_time,slots) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+ruleColumns, scope.OrganizationID, scope.LocationID, input.ResourceID, input.ServiceID, input.AddonIDs, input.DurationMinutes, input.BeforeMinutes, input.AfterMinutes, input.Units, input.WorkingTime, input.Slots))
	} else {
		result, err = scanRule(tx.QueryRow(ctx, `UPDATE service_scheduling_rules SET duration_minutes=$8,before_minutes=$9,after_minutes=$10,units=$11,working_time=$12,slots=$13,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND version=$4 AND resource_id=$5 AND service_id=$6 AND addon_ids=$7::jsonb RETURNING `+ruleColumns, scope.OrganizationID, scope.LocationID, input.ID, input.Version, input.ResourceID, input.ServiceID, input.AddonIDs, input.DurationMinutes, input.BeforeMinutes, input.AfterMinutes, input.Units, input.WorkingTime, input.Slots))
	}
	var pgErr *pgconn.PgError
	if errors.Is(err, pgx.ErrNoRows) || errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return SchedulingRule{}, ErrSchedulingRuleConflict
	}
	if err != nil {
		return SchedulingRule{}, err
	}
	snapshot, err := json.Marshal(result)
	if err != nil {
		return SchedulingRule{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO service_scheduling_revisions(rule_id,version,snapshot,actor_id) VALUES($1,$2,$3,$4)`, result.ID, result.Version, snapshot, scope.ActorID); err != nil {
		return SchedulingRule{}, err
	}
	if err = jobAudit(ctx, tx, scope, "scheduling.rule.revised", result.ID); err != nil {
		return SchedulingRule{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SchedulingRule{}, err
	}
	return result, nil
}
