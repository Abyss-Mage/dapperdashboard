package postgres

import "context"

// The actor ID must be obtained from a verified identity provider/session.
// Never use a client-supplied actor ID as authentication. This package does not
// authenticate an HTTP request, and no production auth routes are enabled yet.
func (s *Store) ResolveScope(ctx context.Context, actorID, organizationID, locationID, action string) (Scope, error) {
	scope := Scope{OrganizationID: organizationID, LocationID: locationID, ActorID: actorID}
	if err := s.authorize(ctx, scope, action); err != nil {
		return Scope{}, err
	}
	return scope, nil
}

func (s *Store) authorize(ctx context.Context, scope Scope, action string) error {
	if err := scope.validate(); err != nil {
		return err
	}
	switch action {
	case "customers.read", "customers.create", "customer_accounts.manage", "vehicles.read", "vehicles.create",
		"jobs.read", "jobs.create", "jobs.complete", "jobs.contribute",
		"catalogue.read", "catalogue.manage",
		"scheduling.manage", "scheduling.reserve", "scheduling.release", "scheduling.read",
		"attendance.record", "attendance.read", "attendance.manage",
		"bookings.read", "bookings.create", "bookings.manage",
		"invoices.read", "invoices.create", "invoices.issue", "invoices.manage", "payments.record", "payments.read",
		"sop.read", "sop.acknowledge", "sop.manage",
		"quotes.read", "quotes.create", "quotes.manage", "quotes.issue",
		"dealership.read", "dealership.manage",
		"marketing.read", "marketing.manage", "content.manage":
	default:
		return ErrForbidden
	}
	var granted bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(
		-- 1. Direct location grant
		SELECT 1 FROM location_grants g
		JOIN principals p ON p.id=g.principal_id
		WHERE p.active AND g.principal_id=$1 AND g.organization_id=$2 AND g.location_id=$3 AND g.action=$4

		UNION ALL

		-- 2. Scoped role assignment at location scope
		SELECT 1 FROM member_role_assignments a
		JOIN principals p ON p.id=a.principal_id
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
		  AND a.scope_type='location' AND a.location_id=$3 AND rc.action=$4

		UNION ALL

		-- 3. Scoped role assignment at franchise scope (location belongs to franchise)
		SELECT 1 FROM member_role_assignments a
		JOIN principals p ON p.id=a.principal_id
		JOIN locations l ON l.organization_id=a.organization_id AND l.id=$3
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
		  AND a.scope_type='franchise' AND a.franchise_id=l.franchise_id AND rc.action=$4

		UNION ALL

		-- 4. Scoped role assignment at organization scope
		SELECT 1 FROM member_role_assignments a
		JOIN principals p ON p.id=a.principal_id
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
		  AND a.scope_type='organization' AND a.scope_id=$2 AND rc.action=$4
	)`, scope.ActorID, scope.OrganizationID, scope.LocationID, action).Scan(&granted)
	if err != nil {
		return err
	}
	if !granted {
		return ErrForbidden
	}
	return nil
}
