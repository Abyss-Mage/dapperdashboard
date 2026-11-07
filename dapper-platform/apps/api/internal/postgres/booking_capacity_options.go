package postgres

import (
	"context"
)

type BookingResourceChoice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}
type BookingRuleChoice struct {
	SchedulingRule
	ServiceName string     `json:"serviceName"`
	Addons      []JobAddon `json:"addons"`
	// AvailableAddons are the active add-ons of this service that can be attached to a base rule (one with no add-ons of its own).
	AvailableAddons []JobAddon `json:"availableAddons"`
}
type BookingCapacityOptions struct {
	Resources []BookingResourceChoice `json:"resources"`
	Rules     []BookingRuleChoice     `json:"rules"`
	Limit     int                     `json:"limit"`
}

func (s *Store) BookingCapacityChoices(ctx context.Context, scope Scope) (BookingCapacityOptions, error) {
	return s.bookingCapacityChoices(ctx, scope, []string{"bookings.create", "scheduling.reserve"})
}
func (s *Store) BookingOfferChoices(ctx context.Context, scope Scope) (BookingCapacityOptions, error) {
	return s.bookingCapacityChoices(ctx, scope, []string{"catalogue.manage", "scheduling.manage"})
}
func (s *Store) BookingRescheduleChoices(ctx context.Context, scope Scope) (BookingCapacityOptions, error) {
	return s.bookingCapacityChoices(ctx, scope, []string{"bookings.manage", "scheduling.reserve", "scheduling.release"})
}
func (s *Store) bookingCapacityChoices(ctx context.Context, scope Scope, actions []string) (BookingCapacityOptions, error) {
	result := BookingCapacityOptions{Resources: []BookingResourceChoice{}, Rules: []BookingRuleChoice{}, Limit: 100}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	for _, action := range actions {
		if err = authorizeTransaction(ctx, tx, scope, action); err != nil {
			return result, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,timezone FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 ORDER BY name,id LIMIT 100`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item BookingResourceChoice
		if err = rows.Scan(&item.ID, &item.Name, &item.Timezone); err != nil {
			rows.Close()
			return result, err
		}
		result.Resources = append(result.Resources, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = tx.Query(ctx, `SELECT sr.id::text,sr.resource_id::text,sr.service_id::text,sr.addon_ids,sr.duration_minutes,sr.before_minutes,sr.after_minutes,sr.units,sr.version,c.name,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('serviceId',a.id::text,'name',a.name) ORDER BY a.id) FROM catalog_services a WHERE a.organization_id=sr.organization_id AND a.location_id=sr.location_id AND sr.addon_ids ? a.id::text),'[]'::jsonb)
 FROM service_scheduling_rules sr JOIN catalog_services c ON (c.organization_id,c.location_id,c.id)=(sr.organization_id,sr.location_id,sr.service_id)
 WHERE sr.organization_id=$1 AND sr.location_id=$2 AND c.kind='main' AND NOT c.archived
 AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text(sr.addon_ids) aid(value) WHERE NOT EXISTS(SELECT 1 FROM catalog_services a WHERE a.organization_id=sr.organization_id AND a.location_id=sr.location_id AND a.id::text=aid.value AND a.kind='addon' AND catalog_addon_offered_with(a.id,sr.service_id) AND NOT a.archived))
 ORDER BY c.name,sr.id LIMIT 100`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item BookingRuleChoice
		if err = rows.Scan(&item.ID, &item.ResourceID, &item.ServiceID, &item.AddonIDs, &item.DurationMinutes, &item.BeforeMinutes, &item.AfterMinutes, &item.Units, &item.Version, &item.ServiceName, &item.Addons); err != nil {
			rows.Close()
			return result, err
		}
		result.Rules = append(result.Rules, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	children := map[string][]JobAddon{}
	arows, err := tx.Query(ctx, `SELECT l.main_id::text,a.id::text,a.name FROM catalog_addon_parents l JOIN catalog_services a ON a.id=l.addon_id WHERE a.organization_id=$1 AND a.location_id=$2 AND a.kind='addon' AND NOT a.archived ORDER BY a.name,a.id,l.main_id LIMIT 5000`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return result, err
	}
	for arows.Next() {
		var parent string
		var a JobAddon
		if err = arows.Scan(&parent, &a.ServiceID, &a.Name); err != nil {
			arows.Close()
			return result, err
		}
		children[parent] = append(children[parent], a)
	}
	arows.Close()
	if err = arows.Err(); err != nil {
		return result, err
	}
	for i := range result.Rules {
		result.Rules[i].AvailableAddons = []JobAddon{}
		if len(result.Rules[i].AddonIDs) == 0 {
			result.Rules[i].AvailableAddons = append(result.Rules[i].AvailableAddons, children[result.Rules[i].ServiceID]...)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
