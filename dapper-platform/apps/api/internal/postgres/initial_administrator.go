package postgres

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
)

var ErrAdministratorInitialized = errors.New("initial administrator provisioning requires an empty application")
var ErrAdministratorInput = errors.New("valid organization, administrator email and password required")

type InitialAdministrator struct {
	OrganizationID string `json:"organizationId"`
	PrincipalID    string `json:"principalId"`
	Email          string `json:"email"`
}

// InitializeAdministrator is a trusted operator boundary, never a public route.
// One transaction creates the organization, explicit administrative capabilities,
// membership, credential and audit. No location, finance grant or data is seeded.
func (s *Store) InitializeAdministrator(ctx context.Context, organization, email, password string) (InitialAdministrator, error) {
	organization = strings.TrimSpace(organization)
	email, valid := normalizedEmail(email)
	if n := utf8.RuneCountInString(organization); n < 2 || n > 120 || !valid || len(password) < 15 || len(password) > 1024 {
		return InitialAdministrator{}, ErrAdministratorInput
	}
	hash, err := argon2id.CreateHash(password, passwordParams())
	if err != nil {
		return InitialAdministrator{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return InitialAdministrator{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(current_database()||'/'||current_schema()||'/initial-administrator',0))`); err != nil {
		return InitialAdministrator{}, err
	}
	var occupied bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organizations) OR EXISTS(SELECT 1 FROM principals)`).Scan(&occupied); err != nil {
		return InitialAdministrator{}, err
	}
	if occupied {
		return InitialAdministrator{}, ErrAdministratorInitialized
	}
	result := InitialAdministrator{Email: email}
	if err = tx.QueryRow(ctx, `INSERT INTO organizations(name) VALUES($1) RETURNING id::text`, organization).Scan(&result.OrganizationID); err != nil {
		return InitialAdministrator{}, err
	}
	if err = tx.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('managed-initial-administrator') RETURNING id::text`).Scan(&result.PrincipalID); err != nil {
		return InitialAdministrator{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO password_credentials(principal_id,email,password_hash) VALUES($1,$2,$3)`, result.PrincipalID, email, hash); err != nil {
		return InitialAdministrator{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, result.OrganizationID, result.PrincipalID); err != nil {
		return InitialAdministrator{}, err
	}
	// Exact capabilities, not an implicit universal role or location bypass.
	for _, action := range []string{"structure.read", "structure.manage", "catalogue.read", "catalogue.manage", "access.manage"} {
		if _, err = tx.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,$3)`, result.PrincipalID, result.OrganizationID, action); err != nil {
			return InitialAdministrator{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,actor_id,action,entity_id) VALUES($1,$2,'administrator.initialized',$2)`, result.OrganizationID, result.PrincipalID); err != nil {
		return InitialAdministrator{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InitialAdministrator{}, err
	}
	return result, nil
}
