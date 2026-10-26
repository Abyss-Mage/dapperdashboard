package postgres

import "context"

func (s *Store) SchedulingResources(ctx context.Context, scope Scope) ([]SchedulingResource, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.manage"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,timezone,capacity,rules,weekly_hours FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 ORDER BY name,id LIMIT 100`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	items := []SchedulingResource{}
	for rows.Next() {
		var item SchedulingResource
		var capacity int
		if err = rows.Scan(&item.ID, &item.Name, &item.Timezone, &capacity, &item.Plan, &item.WeeklyHours); err != nil {
			rows.Close()
			return nil, err
		}
		item.Plan.Capacity = capacity
		item.Plan.Existing = nil
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
