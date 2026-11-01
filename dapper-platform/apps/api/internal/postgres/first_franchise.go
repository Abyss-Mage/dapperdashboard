package postgres

import (
	"context"
	"strings"
)

// FirstFranchiseInput is the first franchise created during setup. Phone is stored as the franchise's main contact.
type FirstFranchiseInput struct {
	Name    string  `json:"name"`
	Code    string  `json:"code"`
	Address Address `json:"address"`
	Phone   string  `json:"phone"`
}

// SuperAdminRoleName is the built-in role the first owner receives at setup. It holds every capability the server knows.
const SuperAdminRoleName = "Super Admin"

// ProvisionFirstFranchise runs once at setup, right after InitializeAdministrator. It creates the first franchise (with its
// public website entry, so portfolio photos can be published), and creates the Super Admin role with every capability and
// assigns it to the initial administrator for the whole organization. It refuses to run if a role or franchise already exists.
func (s *Store) ProvisionFirstFranchise(ctx context.Context, admin InitialAdministrator, in FirstFranchiseInput) (Franchise, error) {
	contacts := []FranchiseContact{}
	if phone := strings.TrimSpace(in.Phone); phone != "" {
		contacts = append(contacts, FranchiseContact{Name: "Main line", Phone: phone})
	}
	franchise, err := s.CreateFranchise(ctx, admin.PrincipalID, admin.OrganizationID, FranchiseInput{Code: in.Code, Name: in.Name, Address: in.Address, Contacts: contacts})
	if err != nil {
		return Franchise{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Franchise{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, admin.OrganizationID+"/access-authority"); err != nil {
		return Franchise{}, err
	}
	capabilities := ServerKnownCapabilities()
	var roleID string
	if err = tx.QueryRow(ctx, `INSERT INTO roles(organization_id, name, description, version) VALUES($1,$2,$3,1) RETURNING id::text`,
		admin.OrganizationID, SuperAdminRoleName, "Built in at setup. Can use and change everything in the system.").Scan(&roleID); err != nil {
		return Franchise{}, err
	}
	for _, capability := range capabilities {
		if _, err = tx.Exec(ctx, `INSERT INTO role_capabilities(role_id, action) VALUES($1,$2)`, roleID, capability); err != nil {
			return Franchise{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO role_definition_events(organization_id, actor_id, role_id, role_name, action, version, capabilities) VALUES($1,$2,$3,$4,'created',1,$5)`,
		admin.OrganizationID, admin.PrincipalID, roleID, SuperAdminRoleName, capabilities); err != nil {
		return Franchise{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_role_assignments(organization_id, principal_id, role_id, scope_type, scope_id, version, assigned_by) VALUES($1,$2,$3,'organization',$1,1,$2)`,
		admin.OrganizationID, admin.PrincipalID, roleID); err != nil {
		return Franchise{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO role_assignment_events(organization_id, actor_id, principal_id, role_id, scope_type, scope_id, action, version) VALUES($1,$2,$2,$3,'organization',$1,'assigned',1)`,
		admin.OrganizationID, admin.PrincipalID, roleID); err != nil {
		return Franchise{}, err
	}
	if err = ensureEmployeeRole(ctx, tx, admin.OrganizationID); err != nil {
		return Franchise{}, err
	}
	// The franchise's public website entry: published, so portfolio photos added later appear on the website.
	slug := strings.ToLower(franchise.Code)
	if _, err = tx.Exec(ctx, `INSERT INTO public_sites(organization_id, location_id, slug, name, published) VALUES($1,$2,$3,$4,true) ON CONFLICT DO NOTHING`,
		admin.OrganizationID, franchise.ID, slug, franchise.Name); err != nil {
		return Franchise{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Franchise{}, err
	}
	return franchise, nil
}
