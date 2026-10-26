package postgres

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"dapper.local/platform/internal/square"
)

// The Square application (its Application ID and secret) is what lets each franchise connect its own Square account. Owners can
// enter the keys for the test (sandbox) and live (production) applications here instead of setting them on the server. The secret
// is sealed with the server key, never returned, and takes effect immediately. Keys set on the server stay in use until replaced.

var ErrSquareKeysInput = errors.New("enter the Square Application ID and secret")

type SquareAppStatus struct {
	Environment   string     `json:"environment"`
	Configured    bool       `json:"configured"`
	ApplicationID string     `json:"applicationId"`
	Source        string     `json:"source"` // "saved" (entered here), "server" (set on the server) or ""
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

func squareAppSealContext(env string) string { return "square-app/" + env }
func validSquareEnvironment(env string) bool { return env == "sandbox" || env == "production" }

func (s *Store) setSquareApp(env, applicationID, secret string) {
	s.squareMu.Lock()
	defer s.squareMu.Unlock()
	next := map[string]square.OAuthApp{}
	for k, v := range s.squareOAuth {
		next[k] = v
	}
	ids := map[string]string{}
	for k, v := range s.squareApplicationIDs {
		ids[k] = v
	}
	if applicationID == "" {
		delete(next, env)
		delete(ids, env)
	} else {
		next[env] = square.OAuthApp{Environment: env, ApplicationID: applicationID, Secret: secret}
		ids[env] = applicationID
	}
	s.squareOAuth, s.squareApplicationIDs = next, ids
}

// SaveSquareAppCredentials stores the keys for one environment (invoices.configure at organisation level).
func (s *Store) SaveSquareAppCredentials(ctx context.Context, actor, org, env, applicationID, secret string) error {
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
	sealed, err := s.sealSecret(squareAppSealContext(env), []byte(secret))
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO square_app_credentials(environment,application_id,secret_sealed,updated_by) VALUES($1,$2,$3,$4)
ON CONFLICT(environment) DO UPDATE SET application_id=excluded.application_id,secret_sealed=excluded.secret_sealed,updated_by=excluded.updated_by,updated_at=now()`, env, applicationID, sealed, actor); err != nil {
		return err
	}
	if err = structureAudit(ctx, tx, actor, org, "square.app_credentials_saved."+env, org); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.setSquareApp(env, applicationID, secret)
	return nil
}

// ClearSquareAppCredentials removes the keys entered here (keys set on the server are not affected after the next restart).
func (s *Store) ClearSquareAppCredentials(ctx context.Context, actor, org, env string) error {
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
	if _, err = tx.Exec(ctx, `DELETE FROM square_app_credentials WHERE environment=$1`, env); err != nil {
		return err
	}
	if err = structureAudit(ctx, tx, actor, org, "square.app_credentials_cleared."+env, org); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.setSquareApp(env, "", "")
	return nil
}

// SquareAppStatuses reports, for both environments, whether keys exist and which Application ID they use. Never the secret.
func (s *Store) SquareAppStatuses(ctx context.Context, actor, org string) ([]SquareAppStatus, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return nil, err
	}
	saved := map[string]SquareAppStatus{}
	rows, err := tx.Query(ctx, `SELECT environment,application_id,updated_at FROM square_app_credentials`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var st SquareAppStatus
		var at time.Time
		if err = rows.Scan(&st.Environment, &st.ApplicationID, &at); err != nil {
			rows.Close()
			return nil, err
		}
		st.Configured, st.Source, st.UpdatedAt = true, "saved", &at
		saved[st.Environment] = st
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []SquareAppStatus{}
	for _, env := range []string{"sandbox", "production"} {
		if st, ok := saved[env]; ok {
			out = append(out, st)
			continue
		}
		st := SquareAppStatus{Environment: env}
		if app, ok := s.squareApp(env); ok {
			st.Configured, st.ApplicationID, st.Source = true, app.ApplicationID, "server"
		}
		out = append(out, st)
	}
	return out, nil
}

// LoadSquareAppCredentials reads the saved keys at startup (after the server key is set) and puts them in use.
func (s *Store) LoadSquareAppCredentials(ctx context.Context) error {
	if len(s.mfaKey) != 32 {
		return nil
	}
	rows, err := s.pool.Query(ctx, `SELECT environment,application_id,secret_sealed FROM square_app_credentials`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var env, id string
		var sealed []byte
		if err = rows.Scan(&env, &id, &sealed); err != nil {
			return err
		}
		secret, e := s.openSecret(squareAppSealContext(env), sealed)
		if e != nil {
			continue
		}
		s.setSquareApp(env, id, string(secret))
	}
	return rows.Err()
}
