package postgres

import "context"

type LocationAccess struct {
	OrganizationID   string   `json:"organizationId"`
	OrganizationName string   `json:"organizationName"`
	LocationID       string   `json:"locationId"`
	LocationName     string   `json:"locationName"`
	Actions          []string `json:"actions"`
}
type OrganizationAccess struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

func (s *Store) Access(ctx context.Context, actor string) ([]LocationAccess, []OrganizationAccess, error) {
	rows, err := s.pool.Query(ctx, `
		WITH effective_location_actions AS (
			-- Direct location grants
			SELECT l.organization_id, o.name AS organization_name, l.id AS location_id, l.name AS location_name, g.action
			FROM location_grants g
			JOIN principals p ON p.id=g.principal_id
			JOIN locations l ON l.organization_id=g.organization_id AND l.id=g.location_id
			JOIN organizations o ON o.id=l.organization_id
			WHERE p.id=$1 AND p.active

			UNION

			-- Scoped role assignment at location scope
			SELECT l.organization_id, o.name AS organization_name, l.id AS location_id, l.name AS location_name, rc.action
			FROM member_role_assignments a
			JOIN principals p ON p.id=a.principal_id
			JOIN locations l ON l.organization_id=a.organization_id AND l.id=a.location_id
			JOIN organizations o ON o.id=l.organization_id
			JOIN role_capabilities rc ON rc.role_id=a.role_id
			WHERE p.id=$1 AND p.active AND a.scope_type='location'

			UNION

			-- Scoped role assignment at franchise scope
			SELECT l.organization_id, o.name AS organization_name, l.id AS location_id, l.name AS location_name, rc.action
			FROM member_role_assignments a
			JOIN principals p ON p.id=a.principal_id
			JOIN locations l ON l.organization_id=a.organization_id AND l.franchise_id=a.franchise_id
			JOIN organizations o ON o.id=l.organization_id
			JOIN role_capabilities rc ON rc.role_id=a.role_id
			WHERE p.id=$1 AND p.active AND a.scope_type='franchise'

			UNION

			-- Scoped role assignment at organization scope
			SELECT l.organization_id, o.name AS organization_name, l.id AS location_id, l.name AS location_name, rc.action
			FROM member_role_assignments a
			JOIN principals p ON p.id=a.principal_id
			JOIN locations l ON l.organization_id=a.organization_id
			JOIN organizations o ON o.id=l.organization_id
			JOIN role_capabilities rc ON rc.role_id=a.role_id
			WHERE p.id=$1 AND p.active AND a.scope_type='organization'
		)
		SELECT organization_id::text, organization_name, location_id::text, location_name, array_agg(action ORDER BY action)
		FROM effective_location_actions
		GROUP BY organization_id, organization_name, location_id, location_name
		ORDER BY organization_name, location_name, location_id
		LIMIT 100
	`, actor)
	if err != nil {
		return nil, nil, err
	}
	locations := []LocationAccess{}
	for rows.Next() {
		var item LocationAccess
		if err = rows.Scan(&item.OrganizationID, &item.OrganizationName, &item.LocationID, &item.LocationName, &item.Actions); err != nil {
			rows.Close()
			return nil, nil, err
		}
		locations = append(locations, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	rows, err = s.pool.Query(ctx, `
		WITH effective_org_actions AS (
			-- Direct organization grants
			SELECT o.id AS organization_id, o.name AS organization_name, g.action
			FROM organization_grants g
			JOIN principals p ON p.id=g.principal_id
			JOIN organizations o ON o.id=g.organization_id
			WHERE p.id=$1 AND p.active

			UNION

			-- Scoped role assignment at organization scope
			SELECT o.id AS organization_id, o.name AS organization_name, rc.action
			FROM member_role_assignments a
			JOIN principals p ON p.id=a.principal_id
			JOIN organizations o ON o.id=a.organization_id
			JOIN role_capabilities rc ON rc.role_id=a.role_id
			WHERE p.id=$1 AND p.active AND a.scope_type='organization'
		)
		SELECT organization_id::text, organization_name, array_agg(action ORDER BY action)
		FROM effective_org_actions
		GROUP BY organization_id, organization_name
		ORDER BY organization_name, organization_id
		LIMIT 100
	`, actor)
	if err != nil {
		return nil, nil, err
	}
	organizations := []OrganizationAccess{}
	for rows.Next() {
		var item OrganizationAccess
		if err = rows.Scan(&item.ID, &item.Name, &item.Actions); err != nil {
			rows.Close()
			return nil, nil, err
		}
		organizations = append(organizations, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	return locations, organizations, nil
}
