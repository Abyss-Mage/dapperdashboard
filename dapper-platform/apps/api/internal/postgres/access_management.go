package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type AccessMember struct {
	PrincipalID string `json:"principalId"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Active      bool   `json:"active"`
}
type ManagedLocationGrant struct {
	PrincipalID string `json:"principalId"`
	LocationID  string `json:"locationId"`
	Action      string `json:"action"`
}
type AccessManagement struct {
	Members             []AccessMember             `json:"members"`
	Locations           []Location                 `json:"locations"`
	Grants              []ManagedLocationGrant     `json:"grants"`
	Actions             []string                   `json:"actions"`
	Limit               int                        `json:"limit"`
	OrganizationActions []string                   `json:"organizationActions"`
	OrganizationGrants  []ManagedOrganizationGrant `json:"organizationGrants"`
	Franchises          []Franchise                `json:"franchises"`
}

type ManagedOrganizationGrant struct {
	PrincipalID string `json:"principalId"`
	Action      string `json:"action"`
}

var ErrLastAccessManager = errors.New("cannot revoke the last active organization access manager")

func OrganizationGrantActions() []string {
	return []string{"structure.read", "structure.manage", "catalogue.read", "catalogue.manage", "attendance.read", "attendance.manage", "bookings.read", "bookings.manage", "invoices.read", "invoices.manage", "invoices.configure", "payments.read", "sop.read", "sop.manage", "access.manage"}
}

func (s *Store) SetOrganizationGrant(ctx context.Context, actor, org, principal, action string, enabled bool) error {
	valid := false
	for _, candidate := range OrganizationGrantActions() {
		if candidate == action {
			valid = true
		}
	}
	if !valid {
		return ErrForbidden
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	// Unified organization access authority lock serializes all grant and role changes.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, org+"/access-authority"); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT id::text FROM organizations WHERE id=$1`, org).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	} else if err != nil {
		return err
	}
	if err = accessManagementGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	var active bool
	err = tx.QueryRow(ctx, `SELECT p.active FROM organization_memberships m JOIN principals p ON p.id=m.principal_id WHERE m.organization_id=$1 AND m.principal_id=$2 FOR SHARE OF m,p`, org, principal).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if enabled && !active {
		return ErrForbidden
	}
	var changed int64
	if enabled {
		tag, e := tx.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, principal, org, action)
		if e != nil {
			return e
		}
		changed = tag.RowsAffected()
	} else {
		tag, e := tx.Exec(ctx, `DELETE FROM organization_grants WHERE principal_id=$1 AND organization_id=$2 AND action=$3`, principal, org, action)
		if e != nil {
			return e
		}
		changed = tag.RowsAffected()

		// Post-change effective last-admin check
		if action == "access.manage" || action == "roles.manage" {
			postAdminCount, err := countActiveOrgAdmins(ctx, tx, org)
			if err != nil {
				return err
			}
			if postAdminCount < 1 {
				return ErrLastAccessManager
			}
		}
	}
	if changed > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO organization_grant_events(organization_id,principal_id,actor_id,action,enabled) VALUES($1,$2,$3,$4,$5)`, org, principal, actor, action, enabled); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func LocationGrantActions() []string {
	return []string{"customers.read", "customers.create", "customer_accounts.manage", "vehicles.read", "vehicles.create",
		"jobs.read", "jobs.create", "jobs.complete", "jobs.contribute", "catalogue.read", "catalogue.manage", "content.manage",
		"scheduling.manage", "scheduling.reserve", "scheduling.release", "scheduling.read",
		"attendance.record", "attendance.read", "attendance.manage", "bookings.read", "bookings.create", "bookings.manage",
		"invoices.read", "invoices.create", "invoices.issue", "invoices.manage", "payments.record", "payments.read", "sop.read", "sop.acknowledge", "sop.manage"}
}
func validLocationGrantAction(action string) bool {
	for _, candidate := range LocationGrantActions() {
		if candidate == action {
			return true
		}
	}
	return false
}

func accessManagementGrant(ctx context.Context, tx pgx.Tx, actor, org string) error {
	if actor == "" || org == "" {
		return ErrForbidden
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT p.id::text FROM principals p
 JOIN organization_memberships m ON m.principal_id=p.id AND m.organization_id=$2
 JOIN organization_grants g ON g.principal_id=p.id AND g.organization_id=m.organization_id AND g.action='access.manage'
 WHERE p.id=$1 AND p.active FOR SHARE OF p,m,g`, actor, org).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	return err
}

func accessReadGrant(ctx context.Context, tx pgx.Tx, actor, org string) error {
	if actor == "" || org == "" {
		return ErrForbidden
	}
	var has bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM organization_grants g
		JOIN principals p ON p.id=g.principal_id
		JOIN organization_memberships m ON m.principal_id=p.id AND m.organization_id=g.organization_id
		WHERE p.active AND g.principal_id=$1 AND g.organization_id=$2 AND g.action IN ('access.manage', 'roles.manage', 'roles.assign')

		UNION ALL

		SELECT 1 FROM member_role_assignments a
		JOIN principals p ON p.id=a.principal_id
		JOIN organization_memberships m ON m.principal_id=p.id AND m.organization_id=a.organization_id
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
		  AND rc.action IN ('access.manage', 'roles.manage', 'roles.assign')
	)`, actor, org).Scan(&has)
	if err != nil {
		return err
	}
	if !has {
		return ErrForbidden
	}
	return nil
}

func (s *Store) ManagedAccess(ctx context.Context, actor, org string) (AccessManagement, error) {
	result := AccessManagement{
		Members:             []AccessMember{},
		Locations:           []Location{},
		Grants:              []ManagedLocationGrant{},
		Actions:             LocationGrantActions(),
		Limit:               100,
		OrganizationActions: OrganizationGrantActions(),
		OrganizationGrants:  []ManagedOrganizationGrant{},
		Franchises:          []Franchise{},
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AccessManagement{}, err
	}
	defer tx.Rollback(ctx)
	if err = accessReadGrant(ctx, tx, actor, org); err != nil {
		return AccessManagement{}, err
	}
	rows, err := tx.Query(ctx, `SELECT p.id::text,coalesce(c.email,''),coalesce(mp.full_name,''),p.active FROM organization_memberships m JOIN principals p ON p.id=m.principal_id LEFT JOIN password_credentials c ON c.principal_id=p.id LEFT JOIN member_profiles mp ON mp.principal_id=p.id WHERE m.organization_id=$1 AND p.external_subject NOT LIKE 'system:%' ORDER BY c.email,p.id LIMIT 100`, org)
	if err != nil {
		return AccessManagement{}, err
	}
	for rows.Next() {
		var m AccessMember
		if err = rows.Scan(&m.PrincipalID, &m.Email, &m.Name, &m.Active); err != nil {
			rows.Close()
			return AccessManagement{}, err
		}
		result.Members = append(result.Members, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return AccessManagement{}, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text,name,franchise_id::text FROM locations WHERE organization_id=$1 ORDER BY name,id LIMIT 100`, org)
	if err != nil {
		return AccessManagement{}, err
	}
	for rows.Next() {
		var l Location
		if err = rows.Scan(&l.ID, &l.Name, &l.FranchiseID); err != nil {
			rows.Close()
			return AccessManagement{}, err
		}
		result.Locations = append(result.Locations, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return AccessManagement{}, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text,name FROM franchises WHERE organization_id=$1 ORDER BY name,id LIMIT 100`, org)
	if err != nil {
		return AccessManagement{}, err
	}
	for rows.Next() {
		var f Franchise
		if err = rows.Scan(&f.ID, &f.Name); err != nil {
			rows.Close()
			return AccessManagement{}, err
		}
		result.Franchises = append(result.Franchises, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return AccessManagement{}, err
	}
	rows, err = tx.Query(ctx, `SELECT g.principal_id::text,g.location_id::text,g.action FROM location_grants g JOIN organization_memberships m ON m.principal_id=g.principal_id AND m.organization_id=g.organization_id WHERE g.organization_id=$1 ORDER BY g.principal_id,g.location_id,g.action LIMIT 1000`, org)
	if err != nil {
		return AccessManagement{}, err
	}
	for rows.Next() {
		var g ManagedLocationGrant
		if err = rows.Scan(&g.PrincipalID, &g.LocationID, &g.Action); err != nil {
			rows.Close()
			return AccessManagement{}, err
		}
		result.Grants = append(result.Grants, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return AccessManagement{}, err
	}
	rows, err = tx.Query(ctx, `SELECT g.principal_id::text,g.action FROM organization_grants g JOIN organization_memberships m ON m.principal_id=g.principal_id AND m.organization_id=g.organization_id WHERE g.organization_id=$1 ORDER BY g.principal_id,g.action LIMIT 1000`, org)
	if err != nil {
		return AccessManagement{}, err
	}
	for rows.Next() {
		var g ManagedOrganizationGrant
		if err = rows.Scan(&g.PrincipalID, &g.Action); err != nil {
			rows.Close()
			return AccessManagement{}, err
		}
		result.OrganizationGrants = append(result.OrganizationGrants, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return AccessManagement{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AccessManagement{}, err
	}
	return result, nil
}

// SetLocationGrant changes exactly one capability for an explicit member. No
// business roles, inherited scope, account creation or customer linking occurs.
func (s *Store) SetLocationGrant(ctx context.Context, actor, org, principal, location, action string, enabled bool) error {
	if !validLocationGrantAction(action) {
		return ErrForbidden
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = accessManagementGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	var active bool
	err = tx.QueryRow(ctx, `SELECT p.active FROM organization_memberships m JOIN principals p ON p.id=m.principal_id WHERE m.organization_id=$1 AND m.principal_id=$2 FOR SHARE OF m,p`, org, principal).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if enabled && !active {
		return ErrForbidden
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM locations WHERE organization_id=$1 AND id=$2 FOR SHARE`, org, location).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, org+"/access-authority"); err != nil {
		return err
	}
	var changed int64
	if enabled {
		tag, err := tx.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, principal, org, location, action)
		if err != nil {
			return err
		}
		changed = tag.RowsAffected()
	} else {
		tag, err := tx.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND organization_id=$2 AND location_id=$3 AND action=$4`, principal, org, location, action)
		if err != nil {
			return err
		}
		changed = tag.RowsAffected()
	}
	if changed > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO access_grant_events(organization_id,location_id,principal_id,actor_id,action,enabled) VALUES($1,$2,$3,$4,$5,$6)`, org, location, principal, actor, action, enabled); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
