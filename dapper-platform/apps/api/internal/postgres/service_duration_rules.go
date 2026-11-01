package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// syncServiceDurationRules keeps the booking rule of every main service in step with the "time to complete" the owner typed on
// the service. For each active main service that has a duration, each booking resource of its franchise gets a base rule (no
// add-ons, one unit, no buffers) with exactly that duration; a changed duration revises the rule and keeps its history.
// Rules that the owner built by hand for add-on combinations are never touched, and nothing is invented for a service with no
// duration. location may be empty to cover every franchise of the organisation.
func syncServiceDurationRules(ctx context.Context, tx pgx.Tx, org, location, actor string) error {
	rows, err := tx.Query(ctx, `SELECT c.location_id::text,c.id::text,c.duration_minutes,r.id::text
FROM catalog_services c JOIN scheduling_resources r ON r.organization_id=c.organization_id AND r.location_id=c.location_id
WHERE c.organization_id=$1 AND ($2='' OR c.location_id=NULLIF($2,'')::uuid) AND c.kind='main' AND NOT c.archived AND c.duration_minutes IS NOT NULL`, org, location)
	if err != nil {
		return err
	}
	type item struct {
		location, service, resource string
		minutes                     int
	}
	var todo []item
	for rows.Next() {
		var it item
		if err = rows.Scan(&it.location, &it.service, &it.minutes, &it.resource); err != nil {
			rows.Close()
			return err
		}
		todo = append(todo, it)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, it := range todo {
		var id string
		var duration, version int
		e := tx.QueryRow(ctx, `SELECT id::text,duration_minutes,version FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND service_id=$4 AND addon_ids='[]'::jsonb FOR UPDATE`, org, it.location, it.resource, it.service).Scan(&id, &duration, &version)
		var rule SchedulingRule
		switch {
		case e == pgx.ErrNoRows:
			rule, err = scanRule(tx.QueryRow(ctx, `INSERT INTO service_scheduling_rules(organization_id,location_id,resource_id,service_id,addon_ids,duration_minutes,before_minutes,after_minutes,units,working_time,slots) VALUES($1,$2,$3,$4,'[]'::jsonb,$5,0,0,1,false,(SELECT LEAST(1000,GREATEST(1,capacity)) FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3)) RETURNING `+ruleColumns, org, it.location, it.resource, it.service, it.minutes))
		case e != nil:
			return e
		case duration == it.minutes:
			continue
		default:
			rule, err = scanRule(tx.QueryRow(ctx, `UPDATE service_scheduling_rules SET duration_minutes=$5,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND resource_id=$4 RETURNING `+ruleColumns, org, it.location, id, it.resource, it.minutes))
		}
		if err != nil {
			return err
		}
		snapshot, e2 := json.Marshal(rule)
		if e2 != nil {
			return e2
		}
		if _, err = tx.Exec(ctx, `INSERT INTO service_scheduling_revisions(rule_id,version,snapshot,actor_id) VALUES($1,$2,$3,$4)`, rule.ID, rule.Version, snapshot, actor); err != nil {
			return err
		}
	}
	return nil
}
