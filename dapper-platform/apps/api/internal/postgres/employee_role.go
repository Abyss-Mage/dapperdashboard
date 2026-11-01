package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// EmployeeRoleName is the built-in role for people who work in the employee workspace and nowhere else.
const EmployeeRoleName = "Employee"

const employeeRoleDescription = "Workspace staff. Signs in to the employee workspace only; the dashboard sends them there. Owners can adjust what the role may do under Team > Roles."

// EmployeeRoleCapabilities is what the employee workspace needs: clock in and out, the calendar, walk-ins and their invoices and
// payments, uploads, and reading and acknowledging SOP documents. The same list is written by migration 80 for existing businesses.
var EmployeeRoleCapabilities = []string{
	"attendance.record", "attendance.read",
	"bookings.read", "bookings.create", "bookings.manage",
	"customers.read", "customers.create", "vehicles.read", "vehicles.create",
	"invoices.read", "invoices.create", "invoices.issue", "payments.record", "payments.read",
	"scheduling.read", "scheduling.reserve", "catalogue.read",
	"jobs.read", "jobs.contribute",
	"sop.read", "sop.acknowledge",
}

// ensureEmployeeRole creates the Employee role for an organization that does not have one yet.
func ensureEmployeeRole(ctx context.Context, tx pgx.Tx, org string) error {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO roles(organization_id,name,description,version) VALUES($1,$2,$3,1) ON CONFLICT(organization_id,name) DO NOTHING RETURNING id::text`, org, EmployeeRoleName, employeeRoleDescription).Scan(&id)
	if err == pgx.ErrNoRows {
		return nil // already there
	}
	if err != nil {
		return err
	}
	for _, capability := range EmployeeRoleCapabilities {
		if _, err = tx.Exec(ctx, `INSERT INTO role_capabilities(role_id,action) VALUES($1,$2)`, id, capability); err != nil {
			return err
		}
	}
	return nil
}

// WorkspaceOnly is true for a signed-in person whose only role is Employee (and who holds no organization-wide grant): they work in
// the employee workspace, so the dashboard sends them there.
func (s *Store) WorkspaceOnly(ctx context.Context, principal string) (bool, error) {
	var employee, other bool
	err := s.pool.QueryRow(ctx, `SELECT
 COALESCE(bool_or(lower(r.name)=lower($2)),false),
 COALESCE(bool_or(lower(r.name)<>lower($2)),false) OR EXISTS(SELECT 1 FROM organization_grants g WHERE g.principal_id=$1)
FROM member_role_assignments a JOIN roles r ON r.id=a.role_id WHERE a.principal_id=$1`, principal, EmployeeRoleName).Scan(&employee, &other)
	if err != nil {
		return false, err
	}
	return employee && !other, nil
}
