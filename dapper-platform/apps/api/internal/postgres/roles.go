package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Role struct {
	ID              string   `json:"id"`
	OrganizationID  string   `json:"organizationId"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Version         int      `json:"version"`
	Capabilities    []string `json:"capabilities"`
	AssignmentCount int      `json:"assignmentCount"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

type MemberRoleAssignment struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	PrincipalID    string `json:"principalId"`
	Email          string `json:"email"`
	RoleID         string `json:"roleId"`
	RoleName       string `json:"roleName"`
	ScopeType      string `json:"scopeType"` // "location", "franchise", "organization"
	ScopeID        string `json:"scopeId"`
	ScopeName      string `json:"scopeName"`
	Version        int    `json:"version"`
	AssignedBy     string `json:"assignedBy"`
	AssignedAt     string `json:"assignedAt"`
}

type CapabilityCatalogue struct {
	LocationCapabilities     []string `json:"locationCapabilities"`
	OrganizationCapabilities []string `json:"organizationCapabilities"`
	AllCapabilities          []string `json:"allCapabilities"`
}

var (
	ErrRoleNotFound       = errors.New("role not found")
	ErrRoleNameConflict   = errors.New("role name already exists in this organization")
	ErrRoleInUse          = errors.New("cannot delete role with active member assignments")
	ErrRoleStaleVersion   = errors.New("role version conflict; record was modified by another request")
	ErrInvalidCapability  = errors.New("invalid or unsupported capability")
	ErrInvalidScope       = errors.New("invalid scope target or scope type for this organization")
	ErrSelfEscalation     = errors.New("cannot assign capabilities or roles exceeding actor authority")
	ErrAssignmentNotFound = errors.New("role assignment not found")
	ErrLastAdminLockout   = errors.New("operation would revoke or remove the last active organization administrator")
	ErrAssignmentConflict = errors.New("member already has this role at the specified scope")
	ErrInactiveMember     = errors.New("cannot assign roles to an inactive member")
)

func ServerKnownLocationCapabilities() []string {
	return []string{
		"customers.read", "customers.create", "customer_accounts.manage", "vehicles.read", "vehicles.create",
		"jobs.read", "jobs.create", "jobs.complete", "jobs.contribute",
		"catalogue.read", "catalogue.manage", "content.manage",
		"scheduling.manage", "scheduling.reserve", "scheduling.release", "scheduling.read",
		"attendance.record", "attendance.read", "attendance.manage",
		"bookings.read", "bookings.create", "bookings.manage",
		"invoices.read", "invoices.create", "invoices.issue", "invoices.manage",
		"payments.record", "payments.read",
		"sop.read", "sop.acknowledge", "sop.manage",
		"quotes.read", "quotes.create", "quotes.manage", "quotes.issue",
		"dealership.read", "dealership.manage",
		"marketing.read", "marketing.manage",
	}
}

func ServerKnownOrganizationCapabilities() []string {
	return []string{
		"structure.read", "structure.manage",
		"catalogue.read", "catalogue.manage",
		"attendance.read", "attendance.manage",
		"bookings.read", "bookings.manage",
		"invoices.read", "invoices.manage", "invoices.configure",
		"payments.read",
		"sop.read", "sop.manage",
		"access.manage",
		"quotes.read", "quotes.manage",
		"dealership.read", "dealership.manage",
		"marketing.read", "marketing.manage",
		"roles.manage", "roles.assign",
	}
}

func ServerKnownCapabilities() []string {
	allMap := make(map[string]bool)
	for _, c := range ServerKnownLocationCapabilities() {
		allMap[c] = true
	}
	for _, c := range ServerKnownOrganizationCapabilities() {
		allMap[c] = true
	}
	result := make([]string, 0, len(allMap))
	for c := range allMap {
		result = append(result, c)
	}
	return result
}

func IsValidCapability(action string) bool {
	for _, c := range ServerKnownCapabilities() {
		if c == action {
			return true
		}
	}
	return false
}

func GetCapabilityCatalogue() CapabilityCatalogue {
	loc := ServerKnownLocationCapabilities()
	org := ServerKnownOrganizationCapabilities()
	all := ServerKnownCapabilities()
	return CapabilityCatalogue{
		LocationCapabilities:     loc,
		OrganizationCapabilities: org,
		AllCapabilities:          all,
	}
}

func actorHasOrgCapability(ctx context.Context, tx pgx.Tx, actor, org, action string) (bool, error) {
	if actor == "" || org == "" {
		return false, nil
	}
	var has bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM organization_grants g
		JOIN principals p ON p.id=g.principal_id
		WHERE p.active AND g.principal_id=$1 AND g.organization_id=$2 AND g.action=$3

		UNION ALL

		SELECT 1 FROM member_role_assignments a
		JOIN principals p ON p.id=a.principal_id
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
		  AND a.scope_type='organization' AND a.scope_id=$2 AND rc.action=$3
	)`, actor, org, action).Scan(&has)
	return has, err
}

func actorHasScopeCapability(ctx context.Context, tx pgx.Tx, actor, org, scopeType, scopeID, action string) (bool, error) {
	if actor == "" || org == "" {
		return false, nil
	}
	hasOrg, err := actorHasOrgCapability(ctx, tx, actor, org, action)
	if err != nil || hasOrg {
		return hasOrg, err
	}

	switch scopeType {
	case "organization":
		return false, nil

	case "franchise":
		var has bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM member_role_assignments a
			JOIN principals p ON p.id=a.principal_id
			JOIN role_capabilities rc ON rc.role_id=a.role_id
			WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
			  AND a.scope_type='franchise' AND a.scope_id=$3 AND rc.action=$4
		)`, actor, org, scopeID, action).Scan(&has)
		return has, err

	case "location":
		var has bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM location_grants g
			JOIN principals p ON p.id=g.principal_id
			WHERE p.active AND g.principal_id=$1 AND g.organization_id=$2 AND g.location_id=$3 AND g.action=$4

			UNION ALL

			SELECT 1 FROM member_role_assignments a
			JOIN principals p ON p.id=a.principal_id
			JOIN role_capabilities rc ON rc.role_id=a.role_id
			WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
			  AND a.scope_type='location' AND a.location_id=$3 AND rc.action=$4

			UNION ALL

			SELECT 1 FROM member_role_assignments a
			JOIN principals p ON p.id=a.principal_id
			JOIN locations l ON l.organization_id=a.organization_id AND l.id=$3
			JOIN role_capabilities rc ON rc.role_id=a.role_id
			WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
			  AND a.scope_type='franchise' AND a.franchise_id=l.franchise_id AND rc.action=$4
		)`, actor, org, scopeID, action).Scan(&has)
		return has, err

	default:
		return false, ErrInvalidScope
	}
}

func countActiveOrgAdmins(ctx context.Context, tx pgx.Tx, org string) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `SELECT count(DISTINCT p.id) FROM principals p
	JOIN organization_memberships m ON m.principal_id=p.id AND m.organization_id=$1
	WHERE p.active AND (
		EXISTS(
			SELECT 1 FROM organization_grants g
			WHERE g.principal_id=p.id AND g.organization_id=$1 AND g.action IN ('access.manage', 'roles.manage')
		)
		OR EXISTS(
			SELECT 1 FROM member_role_assignments a
			JOIN role_capabilities rc ON rc.role_id=a.role_id
			WHERE a.principal_id=p.id AND a.organization_id=$1
			  AND a.scope_type='organization' AND a.scope_id=$1
			  AND rc.action IN ('access.manage', 'roles.manage')
		)
	)`, org).Scan(&count)
	return count, err
}

// ListRoles returns all defined roles for the organization.
func (s *Store) ListRoles(ctx context.Context, actor, org string) ([]Role, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	hasManage, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.manage")
	if err != nil {
		return nil, err
	}
	hasAccess, err := actorHasOrgCapability(ctx, tx, actor, org, "access.manage")
	if err != nil {
		return nil, err
	}
	hasAssign, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.assign")
	if err != nil {
		return nil, err
	}
	if !hasManage && !hasAccess && !hasAssign {
		return nil, ErrForbidden
	}

	rows, err := tx.Query(ctx, `
		SELECT r.id::text, r.organization_id::text, r.name, r.description, r.version,
		       coalesce(array_agg(rc.action ORDER BY rc.action) FILTER (WHERE rc.action IS NOT NULL), '{}') AS capabilities,
		       (SELECT count(*) FROM member_role_assignments mra WHERE mra.role_id=r.id) AS assignment_count,
		       to_char(r.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       to_char(r.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM roles r
		LEFT JOIN role_capabilities rc ON rc.role_id=r.id
		WHERE r.organization_id=$1
		GROUP BY r.id, r.organization_id, r.name, r.description, r.version, r.created_at, r.updated_at
		ORDER BY r.name, r.id
	`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := []Role{}
	for rows.Next() {
		var item Role
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Description, &item.Version, &item.Capabilities, &item.AssignmentCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roles, tx.Commit(ctx)
}

// GetRole fetches a single role by ID.
func (s *Store) GetRole(ctx context.Context, actor, org, roleID string) (Role, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Role{}, err
	}
	defer tx.Rollback(ctx)

	hasManage, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.manage")
	if err != nil {
		return Role{}, err
	}
	hasAccess, err := actorHasOrgCapability(ctx, tx, actor, org, "access.manage")
	if err != nil {
		return Role{}, err
	}
	hasAssign, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.assign")
	if err != nil {
		return Role{}, err
	}
	if !hasManage && !hasAccess && !hasAssign {
		return Role{}, ErrForbidden
	}

	var item Role
	err = tx.QueryRow(ctx, `
		SELECT r.id::text, r.organization_id::text, r.name, r.description, r.version,
		       coalesce(array_agg(rc.action ORDER BY rc.action) FILTER (WHERE rc.action IS NOT NULL), '{}') AS capabilities,
		       (SELECT count(*) FROM member_role_assignments mra WHERE mra.role_id=r.id) AS assignment_count,
		       to_char(r.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       to_char(r.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM roles r
		LEFT JOIN role_capabilities rc ON rc.role_id=r.id
		WHERE r.organization_id=$1 AND r.id=$2
		GROUP BY r.id, r.organization_id, r.name, r.description, r.version, r.created_at, r.updated_at
	`, org, roleID).Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Description, &item.Version, &item.Capabilities, &item.AssignmentCount, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrRoleNotFound
	}
	if err != nil {
		return Role{}, err
	}
	return item, tx.Commit(ctx)
}

// CreateRole creates a new role with specific capabilities.
func (s *Store) CreateRole(ctx context.Context, actor, org, name, description string, capabilities []string) (Role, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if len(name) < 2 || len(name) > 80 {
		return Role{}, errors.New("role name must be between 2 and 80 characters")
	}
	if len(description) > 500 {
		return Role{}, errors.New("role description cannot exceed 500 characters")
	}
	if len(capabilities) == 0 {
		return Role{}, errors.New("at least one capability is required")
	}

	cleanCaps := make([]string, 0, len(capabilities))
	seen := make(map[string]bool)
	for _, cap := range capabilities {
		cap = strings.TrimSpace(cap)
		if !IsValidCapability(cap) {
			return Role{}, fmt.Errorf("%w: %s", ErrInvalidCapability, cap)
		}
		if !seen[cap] {
			seen[cap] = true
			cleanCaps = append(cleanCaps, cap)
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Role{}, err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, org+"/access-authority"); err != nil {
		return Role{}, err
	}

	hasManage, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.manage")
	if err != nil {
		return Role{}, err
	}
	if !hasManage {
		return Role{}, ErrForbidden
	}

	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE organization_id=$1 AND lower(name)=lower($2))`, org, name).Scan(&exists)
	if err != nil {
		return Role{}, err
	}
	if exists {
		return Role{}, ErrRoleNameConflict
	}

	var roleID string
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO roles(organization_id, name, description, version)
		VALUES($1, $2, $3, 1)
		RETURNING id::text, created_at, updated_at
	`, org, name, description).Scan(&roleID, &createdAt, &updatedAt)
	if err != nil {
		return Role{}, err
	}

	for _, cap := range cleanCaps {
		if _, err := tx.Exec(ctx, `INSERT INTO role_capabilities(role_id, action) VALUES($1, $2)`, roleID, cap); err != nil {
			return Role{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO role_definition_events(organization_id, actor_id, role_id, role_name, action, version, capabilities)
		VALUES($1, $2, $3, $4, 'created', 1, $5)
	`, org, actor, roleID, name, cleanCaps); err != nil {
		return Role{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Role{}, err
	}

	return Role{
		ID:              roleID,
		OrganizationID:  org,
		Name:            name,
		Description:     description,
		Version:         1,
		Capabilities:    cleanCaps,
		AssignmentCount: 0,
		CreatedAt:       createdAt.UTC().Format(time.RFC3339),
		UpdatedAt:       updatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// UpdateRole updates an existing role with optimistic concurrency checks.
func (s *Store) UpdateRole(ctx context.Context, actor, org, roleID, name, description string, capabilities []string, expectedVersion int) (Role, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if len(name) < 2 || len(name) > 80 {
		return Role{}, errors.New("role name must be between 2 and 80 characters")
	}
	if len(description) > 500 {
		return Role{}, errors.New("role description cannot exceed 500 characters")
	}
	if len(capabilities) == 0 {
		return Role{}, errors.New("at least one capability is required")
	}
	if expectedVersion < 1 {
		return Role{}, errors.New("positive expected version required for update")
	}

	cleanCaps := make([]string, 0, len(capabilities))
	seen := make(map[string]bool)
	for _, cap := range capabilities {
		cap = strings.TrimSpace(cap)
		if !IsValidCapability(cap) {
			return Role{}, fmt.Errorf("%w: %s", ErrInvalidCapability, cap)
		}
		if !seen[cap] {
			seen[cap] = true
			cleanCaps = append(cleanCaps, cap)
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Role{}, err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, org+"/access-authority"); err != nil {
		return Role{}, err
	}

	hasManage, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.manage")
	if err != nil {
		return Role{}, err
	}
	if !hasManage {
		return Role{}, ErrForbidden
	}

	var currentVersion int
	var currentName string
	err = tx.QueryRow(ctx, `SELECT version, name FROM roles WHERE id=$1 AND organization_id=$2 FOR UPDATE`, roleID, org).Scan(&currentVersion, &currentName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrRoleNotFound
	}
	if err != nil {
		return Role{}, err
	}
	if currentVersion != expectedVersion {
		return Role{}, ErrRoleStaleVersion
	}

	// Name conflict check if name changed
	if !strings.EqualFold(currentName, name) {
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE organization_id=$1 AND lower(name)=lower($2) AND id<>$3)`, org, name, roleID).Scan(&exists)
		if err != nil {
			return Role{}, err
		}
		if exists {
			return Role{}, ErrRoleNameConflict
		}
	}

	nextVersion := currentVersion + 1
	var updatedAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE roles
		SET name=$1, description=$2, version=$3, updated_at=clock_timestamp()
		WHERE id=$4 AND organization_id=$5
		RETURNING updated_at
	`, name, description, nextVersion, roleID, org).Scan(&updatedAt)
	if err != nil {
		return Role{}, err
	}

	if _, err = tx.Exec(ctx, `DELETE FROM role_capabilities WHERE role_id=$1`, roleID); err != nil {
		return Role{}, err
	}
	for _, cap := range cleanCaps {
		if _, err := tx.Exec(ctx, `INSERT INTO role_capabilities(role_id, action) VALUES($1, $2)`, roleID, cap); err != nil {
			return Role{}, err
		}
	}

	// Post-change effective last-admin lockout check:
	// Verify that at least one active organization administrator remains.
	postAdminCount, err := countActiveOrgAdmins(ctx, tx, org)
	if err != nil {
		return Role{}, err
	}
	if postAdminCount < 1 {
		return Role{}, ErrLastAdminLockout
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO role_definition_events(organization_id, actor_id, role_id, role_name, action, version, capabilities)
		VALUES($1, $2, $3, $4, 'updated', $5, $6)
	`, org, actor, roleID, name, nextVersion, cleanCaps); err != nil {
		return Role{}, err
	}

	var assignCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM member_role_assignments WHERE role_id=$1`, roleID).Scan(&assignCount); err != nil {
		return Role{}, err
	}

	var createdAt time.Time
	if err = tx.QueryRow(ctx, `SELECT created_at FROM roles WHERE id=$1`, roleID).Scan(&createdAt); err != nil {
		return Role{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Role{}, err
	}

	return Role{
		ID:              roleID,
		OrganizationID:  org,
		Name:            name,
		Description:     description,
		Version:         nextVersion,
		Capabilities:    cleanCaps,
		AssignmentCount: assignCount,
		CreatedAt:       createdAt.UTC().Format(time.RFC3339),
		UpdatedAt:       updatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// DeleteRole deletes an unassigned role definition.
func (s *Store) DeleteRole(ctx context.Context, actor, org, roleID string, expectedVersion int) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, org+"/access-authority"); err != nil {
		return err
	}

	hasManage, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.manage")
	if err != nil {
		return err
	}
	if !hasManage {
		return ErrForbidden
	}

	var version int
	var name string
	err = tx.QueryRow(ctx, `SELECT version, name FROM roles WHERE id=$1 AND organization_id=$2 FOR UPDATE`, roleID, org).Scan(&version, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRoleNotFound
	}
	if err != nil {
		return err
	}
	if expectedVersion > 0 && version != expectedVersion {
		return ErrRoleStaleVersion
	}

	var inUseCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM member_role_assignments WHERE role_id=$1`, roleID).Scan(&inUseCount); err != nil {
		return err
	}
	if inUseCount > 0 {
		return ErrRoleInUse
	}

	var caps []string
	rows, err := tx.Query(ctx, `SELECT action FROM role_capabilities WHERE role_id=$1 ORDER BY action`, roleID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err == nil {
			caps = append(caps, a)
		}
	}
	rows.Close()

	if _, err = tx.Exec(ctx, `DELETE FROM roles WHERE id=$1 AND organization_id=$2`, roleID, org); err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO role_definition_events(organization_id, actor_id, role_id, role_name, action, version, capabilities)
		VALUES($1, $2, $3, $4, 'deleted', $5, $6)
	`, org, actor, roleID, name, version, caps); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ListRoleAssignments returns all member role assignments for the organization.
func (s *Store) ListRoleAssignments(ctx context.Context, actor, org string) ([]MemberRoleAssignment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	hasAssign, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.assign")
	if err != nil {
		return nil, err
	}
	hasAccess, err := actorHasOrgCapability(ctx, tx, actor, org, "access.manage")
	if err != nil {
		return nil, err
	}
	hasManage, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.manage")
	if err != nil {
		return nil, err
	}
	if !hasAssign && !hasAccess && !hasManage {
		return nil, ErrForbidden
	}

	rows, err := tx.Query(ctx, `
		SELECT a.id::text, a.organization_id::text, a.principal_id::text, coalesce(c.email, ''),
		       a.role_id::text, r.name, a.scope_type, a.scope_id::text,
		       CASE
		         WHEN a.scope_type='location' THEN coalesce(l.name, 'Unknown Location')
		         WHEN a.scope_type='franchise' THEN coalesce(f.name, 'Unknown Franchise')
		         WHEN a.scope_type='organization' THEN coalesce(o.name, 'Organization Wide')
		         ELSE 'Unknown Scope'
		       END AS scope_name,
		       a.version, a.assigned_by::text,
		       to_char(a.assigned_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM member_role_assignments a
		JOIN roles r ON r.id=a.role_id
		JOIN principals p ON p.id=a.principal_id
		LEFT JOIN password_credentials c ON c.principal_id=p.id
		LEFT JOIN locations l ON l.id=a.location_id
		LEFT JOIN franchises f ON f.id=a.franchise_id
		LEFT JOIN organizations o ON o.id=a.organization_id
		WHERE a.organization_id=$1
		ORDER BY coalesce(c.email, ''), r.name, a.scope_type, a.assigned_at DESC
	`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := []MemberRoleAssignment{}
	for rows.Next() {
		var item MemberRoleAssignment
		if err := rows.Scan(
			&item.ID, &item.OrganizationID, &item.PrincipalID, &item.Email,
			&item.RoleID, &item.RoleName, &item.ScopeType, &item.ScopeID, &item.ScopeName,
			&item.Version, &item.AssignedBy, &item.AssignedAt,
		); err != nil {
			return nil, err
		}
		assignments = append(assignments, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return assignments, tx.Commit(ctx)
}

// AssignMemberRole assigns an existing role to an active member at location, franchise, or org scope.
func (s *Store) AssignMemberRole(ctx context.Context, actor, org, principalID, roleID, scopeType, scopeID string) (MemberRoleAssignment, error) {
	if scopeType != "location" && scopeType != "franchise" && scopeType != "organization" {
		return MemberRoleAssignment{}, ErrInvalidScope
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return MemberRoleAssignment{}, err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, org+"/access-authority"); err != nil {
		return MemberRoleAssignment{}, err
	}

	actorIsAdmin, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.manage")
	if err != nil {
		return MemberRoleAssignment{}, err
	}
	if !actorIsAdmin {
		hasAssign, err := actorHasScopeCapability(ctx, tx, actor, org, scopeType, scopeID, "roles.assign")
		if err != nil {
			return MemberRoleAssignment{}, err
		}
		if !hasAssign {
			return MemberRoleAssignment{}, ErrForbidden
		}
	}

	// Verify target member is active in this org
	var activeMember bool
	var memberEmail string
	err = tx.QueryRow(ctx, `
		SELECT p.active, coalesce(c.email, '')
		FROM organization_memberships m
		JOIN principals p ON p.id=m.principal_id
		LEFT JOIN password_credentials c ON c.principal_id=p.id
		WHERE m.organization_id=$1 AND m.principal_id=$2
	`, org, principalID).Scan(&activeMember, &memberEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return MemberRoleAssignment{}, ErrForbidden
	}
	if err != nil {
		return MemberRoleAssignment{}, err
	}
	if !activeMember {
		return MemberRoleAssignment{}, ErrInactiveMember
	}

	// Verify role belongs to this org
	var roleName string
	err = tx.QueryRow(ctx, `SELECT name FROM roles WHERE id=$1 AND organization_id=$2`, roleID, org).Scan(&roleName)
	if errors.Is(err, pgx.ErrNoRows) {
		return MemberRoleAssignment{}, ErrRoleNotFound
	}
	if err != nil {
		return MemberRoleAssignment{}, err
	}

	// Fetch role capabilities for delegation bounds / anti-escalation check
	var roleCaps []string
	rows, err := tx.Query(ctx, `SELECT action FROM role_capabilities WHERE role_id=$1`, roleID)
	if err != nil {
		return MemberRoleAssignment{}, err
	}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err == nil {
			roleCaps = append(roleCaps, a)
		}
	}
	rows.Close()

	// Delegation bounds & anti-escalation:
	// If actor is not an organization admin (roles.manage), they cannot assign roles with
	// capabilities they do not hold at the target scope, nor can they assign admin actions.
	if !actorIsAdmin {
		for _, cap := range roleCaps {
			if cap == "access.manage" || cap == "roles.manage" {
				return MemberRoleAssignment{}, ErrSelfEscalation
			}
			hasCap, err := actorHasScopeCapability(ctx, tx, actor, org, scopeType, scopeID, cap)
			if err != nil {
				return MemberRoleAssignment{}, err
			}
			if !hasCap {
				return MemberRoleAssignment{}, ErrSelfEscalation
			}
		}
	}

	// Validate target scope
	var locationID, franchiseID *string
	var scopeName string
	switch scopeType {
	case "location":
		var locName string
		err = tx.QueryRow(ctx, `SELECT name FROM locations WHERE id=$1 AND organization_id=$2`, scopeID, org).Scan(&locName)
		if errors.Is(err, pgx.ErrNoRows) {
			return MemberRoleAssignment{}, ErrInvalidScope
		}
		if err != nil {
			return MemberRoleAssignment{}, err
		}
		locationID = &scopeID
		scopeName = locName
	case "franchise":
		var franName string
		err = tx.QueryRow(ctx, `SELECT name FROM franchises WHERE id=$1 AND organization_id=$2`, scopeID, org).Scan(&franName)
		if errors.Is(err, pgx.ErrNoRows) {
			return MemberRoleAssignment{}, ErrInvalidScope
		}
		if err != nil {
			return MemberRoleAssignment{}, err
		}
		franchiseID = &scopeID
		scopeName = franName
	case "organization":
		if !actorIsAdmin {
			return MemberRoleAssignment{}, ErrForbidden
		}
		if scopeID != org {
			return MemberRoleAssignment{}, ErrInvalidScope
		}
		var orgName string
		err = tx.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1`, org).Scan(&orgName)
		if err != nil {
			return MemberRoleAssignment{}, err
		}
		scopeName = orgName
	}

	// Check if assignment already exists
	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM member_role_assignments
		WHERE organization_id=$1 AND principal_id=$2 AND role_id=$3 AND scope_type=$4 AND scope_id=$5
	`, org, principalID, roleID, scopeType, scopeID).Scan(&existingID)
	if err == nil {
		return MemberRoleAssignment{}, ErrAssignmentConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return MemberRoleAssignment{}, err
	}

	var assignID string
	var assignedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO member_role_assignments(organization_id, principal_id, role_id, scope_type, scope_id, location_id, franchise_id, version, assigned_by)
		VALUES($1, $2, $3, $4, $5, $6, $7, 1, $8)
		RETURNING id::text, assigned_at
	`, org, principalID, roleID, scopeType, scopeID, locationID, franchiseID, actor).Scan(&assignID, &assignedAt)
	if err != nil {
		return MemberRoleAssignment{}, err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO role_assignment_events(organization_id, actor_id, principal_id, role_id, scope_type, scope_id, action, version)
		VALUES($1, $2, $3, $4, $5, $6, 'assigned', 1)
	`, org, actor, principalID, roleID, scopeType, scopeID); err != nil {
		return MemberRoleAssignment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return MemberRoleAssignment{}, err
	}

	return MemberRoleAssignment{
		ID:             assignID,
		OrganizationID: org,
		PrincipalID:    principalID,
		Email:          memberEmail,
		RoleID:         roleID,
		RoleName:       roleName,
		ScopeType:      scopeType,
		ScopeID:        scopeID,
		ScopeName:      scopeName,
		Version:        1,
		AssignedBy:     actor,
		AssignedAt:     assignedAt.UTC().Format(time.RFC3339),
	}, nil
}

// RevokeMemberRoleAssignment revokes an assignment with last-admin protection, optimistic version check, and audit logging.
func (s *Store) RevokeMemberRoleAssignment(ctx context.Context, actor, org, assignmentID string, expectedVersion int) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, org+"/access-authority"); err != nil {
		return err
	}

	var principalID, roleID, scopeType, scopeID string
	var version int
	err = tx.QueryRow(ctx, `
		SELECT principal_id::text, role_id::text, scope_type, scope_id::text, version
		FROM member_role_assignments
		WHERE id=$1 AND organization_id=$2
		FOR UPDATE
	`, assignmentID, org).Scan(&principalID, &roleID, &scopeType, &scopeID, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAssignmentNotFound
	}
	if err != nil {
		return err
	}

	if expectedVersion > 0 && version != expectedVersion {
		return ErrRoleStaleVersion
	}

	actorIsAdmin, err := actorHasOrgCapability(ctx, tx, actor, org, "roles.manage")
	if err != nil {
		return err
	}
	if !actorIsAdmin {
		hasAssign, err := actorHasScopeCapability(ctx, tx, actor, org, scopeType, scopeID, "roles.assign")
		if err != nil {
			return err
		}
		if !hasAssign {
			return ErrForbidden
		}
	}

	if _, err = tx.Exec(ctx, `DELETE FROM member_role_assignments WHERE id=$1 AND organization_id=$2`, assignmentID, org); err != nil {
		return err
	}

	// Post-change effective last-admin lockout check:
	// Verify that at least one active organization administrator remains.
	postAdminCount, err := countActiveOrgAdmins(ctx, tx, org)
	if err != nil {
		return err
	}
	if postAdminCount < 1 {
		return ErrLastAdminLockout
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO role_assignment_events(organization_id, actor_id, principal_id, role_id, scope_type, scope_id, action, version)
		VALUES($1, $2, $3, $4, $5, $6, 'revoked', $7)
	`, org, actor, principalID, roleID, scopeType, scopeID, version); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
