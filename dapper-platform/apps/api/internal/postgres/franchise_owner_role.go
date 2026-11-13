package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// FranchiseOwnerRoleName is the built-in role for someone who runs the franchises they are assigned to (owner request 2026-10-30, kept
// by error404 #89 on 2026-11-14 when a fresh install had lost it).
const FranchiseOwnerRoleName = "Franchise Owner"

const franchiseOwnerRoleDescription = "Runs the franchises it is assigned to: bookings, customers, work, staff time, invoices, quotes and marketing. No organization-wide settings, people/roles management, central catalogue or documents."

// FranchiseOwnerRoleCapabilities: every capability except organization settings, people and roles, central content and documents and
// invoice issuer setup (scripts/seed-owner-roles.ps1). The same list is written by migration 124 for existing businesses.
var FranchiseOwnerRoleCapabilities = []string{
	"customers.read", "customers.create", "customer_accounts.manage", "vehicles.read", "vehicles.create",
	"jobs.read", "jobs.create", "jobs.complete", "jobs.contribute",
	"catalogue.read", "catalogue.manage",
	"scheduling.manage", "scheduling.reserve", "scheduling.release", "scheduling.read",
	"attendance.record", "attendance.read", "attendance.manage",
	"bookings.read", "bookings.create", "bookings.manage",
	"invoices.read", "invoices.create", "invoices.issue", "invoices.manage",
	"payments.record", "payments.read",
	"sop.read", "sop.acknowledge",
	"quotes.read", "quotes.create", "quotes.manage", "quotes.issue",
	"dealership.read", "dealership.manage", "marketing.read", "marketing.manage",
}

// ensureFranchiseOwnerRole creates the Franchise Owner role for an organization that does not have one yet.
func ensureFranchiseOwnerRole(ctx context.Context, tx pgx.Tx, org string) error {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO roles(organization_id,name,description,version) VALUES($1,$2,$3,1) ON CONFLICT(organization_id,name) DO NOTHING RETURNING id::text`, org, FranchiseOwnerRoleName, franchiseOwnerRoleDescription).Scan(&id)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	for _, capability := range FranchiseOwnerRoleCapabilities {
		if _, err = tx.Exec(ctx, `INSERT INTO role_capabilities(role_id,action) VALUES($1,$2)`, id, capability); err != nil {
			return err
		}
	}
	return nil
}
