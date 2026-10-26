package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"time"
)

// SaveBookingSlots sets how many bookings a franchise can take at the same time (its "slots") on every booking resource.
// A franchise with no resource yet gets one, named after the franchise, in the given time zone. Bookings that already exist are
// never moved or cancelled by lowering the number. It also brings the franchise's service booking rules in step with the
// durations entered on its services.
func (s *Store) SaveBookingSlots(ctx context.Context, scope Scope, slots int, timezone string) error {
	if slots < 1 || slots > 1000 {
		return ErrReservationInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.manage"); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,rules FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 ORDER BY id FOR UPDATE`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return err
	}
	type res struct {
		id   string
		plan scheduling.Plan
	}
	var list []res
	for rows.Next() {
		var r res
		if err = rows.Scan(&r.id, &r.plan); err != nil {
			rows.Close()
			return err
		}
		list = append(list, r)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if len(list) == 0 {
		if timezone == "" || timezone == "Local" {
			return ErrReservationInput
		}
		if _, e := time.LoadLocation(timezone); e != nil {
			return ErrReservationInput
		}
		var name string
		if err = tx.QueryRow(ctx, `SELECT name FROM locations WHERE organization_id=$1 AND id=$2`, scope.OrganizationID, scope.LocationID).Scan(&name); err != nil {
			return err
		}
		plan := scheduling.Plan{Capacity: slots}
		if _, err = tx.Exec(ctx, `INSERT INTO scheduling_resources(organization_id,location_id,name,capacity,timezone,rules,weekly_hours) VALUES($1,$2,$3,$4,$5,$6,'[]'::jsonb)`, scope.OrganizationID, scope.LocationID, name, slots, timezone, plan); err != nil {
			return err
		}
	}
	for _, r := range list {
		r.plan.Capacity = slots
		if err = scheduling.ValidatePlan(r.plan); err != nil {
			return ErrReservationInput
		}
		if _, err = tx.Exec(ctx, `UPDATE scheduling_resources SET capacity=$4,rules=$5 WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, r.id, slots, r.plan); err != nil {
			return err
		}
	}
	if err = syncServiceDurationRules(ctx, tx, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return err
	}
	if err = jobAudit(ctx, tx, scope, "scheduling.slots.saved", scope.LocationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
