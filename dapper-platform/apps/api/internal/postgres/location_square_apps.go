package postgres

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"dapper.local/platform/internal/square"
	"github.com/jackc/pgx/v5"
)

// Per-franchise Square applications (error404 #50, owner 2026-11-13): a franchise may enter its own Square Application ID and secret
// for test and live. They are used for that franchise's Square sign-in, token renewal and card form; a franchise without its own
// keys uses the shared ones. The secret is sealed with the server key and never returned.

func locationSquareAppSealContext(org, loc, env string) string {
	return "square-app/" + org + "/" + loc + "/" + env
}

// squareAppFor is the Square application a franchise uses in an environment: its own if it has one, else the shared one.
func (s *Store) squareAppFor(ctx context.Context, org, loc, env string) (square.OAuthApp, bool) {
	if len(s.mfaKey) == 32 && org != "" && loc != "" {
		var id string
		var sealed []byte
		err := s.pool.QueryRow(ctx, `SELECT application_id,secret_sealed FROM location_square_apps WHERE organization_id::text=$1 AND location_id::text=$2 AND environment=$3`, org, loc, env).Scan(&id, &sealed)
		if err == nil {
			if secret, e := s.openSecret(locationSquareAppSealContext(org, loc, env), sealed); e == nil {
				return square.OAuthApp{Environment: env, ApplicationID: id, Secret: string(secret)}, true
			}
		}
	}
	return s.squareApp(env)
}

// SquareApplicationIDFor is the PUBLIC Application ID the browser card form uses for a franchise (its own keys; no shared fallback, owner 2026-11-13).
// Card tokens only work with the application whose account charges them, so this must follow the franchise's app.
func (s *Store) SquareApplicationIDFor(ctx context.Context, scope Scope, env string) string {
	var id string
	if err := s.pool.QueryRow(ctx, `SELECT application_id FROM location_square_apps WHERE organization_id::text=$1 AND location_id::text=$2 AND environment=$3`, scope.OrganizationID, scope.LocationID, env).Scan(&id); err == nil {
		return id
	}
	return ""
}

// SquareOAuthAvailableFor says whether a franchise can connect a Square account in an environment.
func (s *Store) SquareOAuthAvailableFor(ctx context.Context, org, loc, env string) bool {
	_, ok := s.squareAppFor(ctx, org, loc, env)
	return ok
}

// LocationSquareApps is, per environment, which Square application a franchise uses: its own ("franchise"), the shared one
// ("shared") or none (""). Never the secret.
func (s *Store) LocationSquareApps(ctx context.Context, actor, org, loc string) ([]SquareAppStatus, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return nil, err
	}
	out := []SquareAppStatus{}
	for _, env := range []string{"sandbox", "production"} {
		st := SquareAppStatus{Environment: env}
		var at time.Time
		err = tx.QueryRow(ctx, `SELECT application_id,updated_at FROM location_square_apps WHERE organization_id=$1 AND location_id=$2 AND environment=$3`, org, loc, env).Scan(&st.ApplicationID, &at)
		switch {
		case err == nil:
			st.Configured, st.Source, st.UpdatedAt = true, "franchise", &at
		case errors.Is(err, pgx.ErrNoRows):
			if app, ok := s.squareApp(env); ok {
				st.Configured, st.ApplicationID, st.Source = true, app.ApplicationID, "shared"
			}
		default:
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// SaveLocationSquareApp stores a franchise's own Square application keys for one environment (invoices.configure).
func (s *Store) SaveLocationSquareApp(ctx context.Context, actor, org, loc, env, applicationID, secret string) error {
	applicationID, secret = strings.TrimSpace(applicationID), strings.TrimSpace(secret)
	if !validSquareEnvironment(env) || utf8.RuneCountInString(applicationID) < 6 || utf8.RuneCountInString(applicationID) > 200 || len(secret) < 8 || len(secret) > 500 {
		return ErrSquareKeysInput
	}
	if len(s.mfaKey) != 32 {
		return ErrSquareOAuthUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	sealed, err := s.sealSecret(locationSquareAppSealContext(org, loc, env), []byte(secret))
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO location_square_apps(organization_id,location_id,environment,application_id,secret_sealed,updated_by) VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT (organization_id,location_id,environment) DO UPDATE SET application_id=excluded.application_id,secret_sealed=excluded.secret_sealed,updated_by=excluded.updated_by,updated_at=now()`, org, loc, env, applicationID, sealed, actor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4,$2)`, org, loc, actor, "square.franchise_app_saved."+env); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearLocationSquareApp removes a franchise's own keys, so it uses the shared ones again.
func (s *Store) ClearLocationSquareApp(ctx context.Context, actor, org, loc, env string) error {
	if !validSquareEnvironment(env) {
		return ErrSquareKeysInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM location_square_apps WHERE organization_id=$1 AND location_id=$2 AND environment=$3`, org, loc, env); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4,$2)`, org, loc, actor, "square.franchise_app_cleared."+env); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
