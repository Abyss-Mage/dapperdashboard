package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"regexp"
)

type SyntheticFixture struct {
	Schema         string `json:"schema"`
	Email          string `json:"email"`
	PrincipalID    string `json:"principalId"`
	OrganizationID string `json:"organizationId"`
	LocationID     string `json:"locationId"`
}

// CreateSyntheticFixture is an explicit development operator action. It accepts
// only a fresh dedicated schema and never modifies an existing schema/account.
func (s *Store) CreateSyntheticFixture(ctx context.Context, password string) (SyntheticFixture, error) {
	if len(password) < 15 || len(password) > 1024 {
		return SyntheticFixture{}, ErrSession
	}
	schema := s.pool.Config().ConnConfig.RuntimeParams["search_path"]
	if !regexp.MustCompile(`^dapper_workspace_demo_[a-z0-9_]{1,40}$`).MatchString(schema) {
		return SyntheticFixture{}, errors.New("fresh dedicated synthetic schema required")
	}
	if _, err := s.pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		return SyntheticFixture{}, errors.New("could not create fresh synthetic schema; existing schemas are never reused")
	}
	if err := s.Migrate(ctx); err != nil {
		return SyntheticFixture{}, err
	}
	result := SyntheticFixture{Schema: schema, Email: "operator@example.invalid"}
	if err := s.pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Dapper Operations') RETURNING id::text`).Scan(&result.OrganizationID); err != nil {
		return SyntheticFixture{}, err
	}
	if err := s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:workspace-operator') RETURNING id::text`).Scan(&result.PrincipalID); err != nil {
		return SyntheticFixture{}, err
	}
	for _, action := range []string{
		"structure.read", "structure.manage", "catalogue.read", "catalogue.manage",
		"attendance.read", "attendance.manage",
		"bookings.read", "bookings.manage",
		"invoices.read", "invoices.manage", "payments.read",
		"sop.read", "sop.manage",
		"quotes.read", "quotes.manage",
	} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,$3)`, result.PrincipalID, result.OrganizationID, action); err != nil {
			return SyntheticFixture{}, err
		}
	}
	franchise, err := s.CreateFranchise(ctx, result.PrincipalID, result.OrganizationID, FranchiseInput{Code: "SYNTH", Name: "Synthetic Training Franchise", Address: Address{Street: "1 Synthetic Way", City: "Synthetic City", Region: "ON", PostalCode: "A1A 1A1", Country: "Canada"}})
	if err != nil {
		return SyntheticFixture{}, err
	}
	result.LocationID = franchise.ID
	for _, action := range []string{
		"customers.read", "customers.create", "vehicles.read", "vehicles.create",
		"jobs.read", "jobs.create", "jobs.complete", "jobs.contribute",
		"catalogue.read", "catalogue.manage",
		"scheduling.manage", "scheduling.reserve", "scheduling.release", "scheduling.read",
		"attendance.record", "attendance.read", "attendance.manage",
		"bookings.read", "bookings.create", "bookings.manage",
		"invoices.read", "invoices.create", "invoices.issue", "invoices.manage", "payments.record", "payments.read",
		"sop.read", "sop.acknowledge", "sop.manage",
		"quotes.read", "quotes.create", "quotes.manage", "quotes.issue",
	} {
		if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, result.PrincipalID, result.OrganizationID, result.LocationID, action); err != nil {
			return SyntheticFixture{}, err
		}
	}
	scope := Scope{OrganizationID: result.OrganizationID, LocationID: result.LocationID, ActorID: result.PrincipalID}
	customer, err := s.CreateCustomer(ctx, scope, "Synthetic Training Customer", "training@example.invalid", "")
	if err != nil {
		return SyntheticFixture{}, err
	}
	main, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic Interior Detail", Kind: "main"})
	if err != nil {
		return SyntheticFixture{}, err
	}
	if _, err = s.SaveService(ctx, scope, CatalogService{Name: "Synthetic Fabric Protection", Kind: "addon", ParentID: &main.ID}); err != nil {
		return SyntheticFixture{}, err
	}
	if _, err = s.CreateJob(ctx, scope, Job{CustomerID: customer.ID, ServiceName: main.Name, Description: "Synthetic training work order. No payment or live booking."}); err != nil {
		return SyntheticFixture{}, err
	}

	// Seed canonical SOP documents from source evidence
	for _, sop := range []SOPDocument{
		{Title: "Interior Detail SOP", Category: "procedure", FileName: "interior_detail_sop.pdf", Sha256Hash: "db5732a72dba4bfcbf455a846c30467a2c06910abf4e601a7d90efc9d377d587", Version: 1, PageCount: 7},
		{Title: "Full Body Detail SOP", Category: "procedure", FileName: "full_body_detail_sop.pdf", Sha256Hash: "dbe16870f4a2d6ffe3e0169ed25b65484cf7edd4e099e0d27e38a8fe5d4fd70d", Version: 1, PageCount: 6},
		{Title: "Ceramic Coating SOP", Category: "procedure", FileName: "ceramic_coating_sop.pdf", Sha256Hash: "776a28a2e444c8fbb2850279a4488570968d0cac111488b399e04e93374102e5", Version: 1, PageCount: 6},
		{Title: "Add-on Services SOP", Category: "procedure", FileName: "addon_services_sop.pdf", Sha256Hash: "5208b6c0bbfcf9ba9df1efa5e40646a04e048915f1c84bd678fe26d739a9074b", Version: 1, PageCount: 6},
		{Title: "Employee Onboarding & Safety Policy", Category: "onboarding", FileName: "onboarding_policy.pdf", Sha256Hash: "3bd0ccbf8a3834d9e5abdccb3911f98c6d3f4fe07f7848eb428a5de1e7ba2999", Version: 1, PageCount: 6},
	} {
		if _, err := s.RegisterSOPDocument(ctx, result.PrincipalID, result.OrganizationID, sop); err != nil {
			return SyntheticFixture{}, err
		}
	}
	auth, err := NewPasswordAuthenticator(s)
	if err != nil {
		return SyntheticFixture{}, err
	}
	if err = auth.SetPassword(ctx, result.PrincipalID, result.Email, password); err != nil {
		return SyntheticFixture{}, err
	}
	return result, nil
}
