package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"dapper.local/platform/internal/square"
	"github.com/jackc/pgx/v5"
)

// Each franchise connects its own Square account through Square OAuth. The tokens are sealed with the server key and
// never leave the server. A binding with credential reference "oauth:<location id>" charges with that connection.

var ErrSquareOAuthUnavailable = errors.New("Square account connection is not configured on the server")
var ErrSquareNotConnected = errors.New("this franchise has not connected a Square account")

const squareOAuthPrefix = "oauth:"

// SetSquareOAuthApps installs the Square application (ID and secret) per environment used to connect franchise accounts.
func (s *Store) SetSquareOAuthApps(apps map[string]square.OAuthApp) {
	s.squareMu.Lock()
	defer s.squareMu.Unlock()
	s.squareOAuth = apps
}

func (s *Store) squareApp(environment string) (square.OAuthApp, bool) {
	s.squareMu.RLock()
	app, ok := s.squareOAuth[environment]
	s.squareMu.RUnlock()
	return app, ok && app.ApplicationID != "" && app.Secret != "" && len(s.mfaKey) == 32
}

// SquareOAuthAvailable says whether owners can connect a Square account in this environment.
func (s *Store) SquareOAuthAvailable(environment string) bool { _, ok := s.squareApp(environment); return ok }

func squareSealContext(org, loc, env string) string { return "square-oauth/" + org + "/" + loc + "/" + env }

// StartSquareOAuth records a single-use state and returns Square's approval page for this franchise.
func (s *Store) StartSquareOAuth(ctx context.Context, actor, org, loc, environment string) (string, error) {
	app, ok := s.squareAppFor(ctx, org, loc, environment)
	if !ok {
		return "", ErrSquareOAuthUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return "", err
	}
	var known bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM locations WHERE organization_id=$1 AND id=$2)`, org, loc).Scan(&known); err != nil || !known {
		return "", ErrForbidden
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(state))
	if _, err = tx.Exec(ctx, `DELETE FROM square_oauth_states WHERE expires_at<now()`); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO square_oauth_states(state_hash,organization_id,location_id,environment,actor_id,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '15 minutes')`, digest[:], org, loc, environment, actor); err != nil {
		return "", err
	}
	return app.AuthorizeURL(state), tx.Commit(ctx)
}

// FinishSquareOAuth checks the returned state (unguessable, single use, 15 minutes), exchanges the code and stores the tokens
// for the person who started the connection, whose invoices.configure grant is checked again. Square's redirect is a
// cross-site navigation, so the state (not the browser session) identifies who started it.
func (s *Store) FinishSquareOAuth(ctx context.Context, state, code string) (string, error) {
	digest := sha256.Sum256([]byte(state))
	var org, loc, env, startedBy string
	err := s.pool.QueryRow(ctx, `DELETE FROM square_oauth_states WHERE state_hash=$1 AND expires_at>now() RETURNING organization_id::text,location_id::text,environment,actor_id::text`, digest[:]).Scan(&org, &loc, &env, &startedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", square.ErrOAuth
	}
	if err != nil {
		return "", err
	}
	app, ok := s.squareAppFor(ctx, org, loc, env)
	if !ok {
		return "", ErrSquareOAuthUnavailable
	}
	token, err := app.ExchangeCode(ctx, code)
	if err != nil {
		return "", err
	}
	return loc, s.storeSquareToken(ctx, startedBy, org, loc, env, token)
}

func (s *Store) storeSquareToken(ctx context.Context, actor, org, loc, env string, token square.OAuthToken) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	sealedAccess, err := s.sealSecret(squareSealContext(org, loc, env), []byte(token.AccessToken))
	if err != nil {
		return err
	}
	sealedRefresh, err := s.sealSecret(squareSealContext(org, loc, env), []byte(token.RefreshToken))
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO square_oauth_connections(organization_id,location_id,environment,merchant_id,access_token_sealed,refresh_token_sealed,expires_at,connected_by)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (organization_id,location_id,environment) DO UPDATE SET merchant_id=EXCLUDED.merchant_id,
 access_token_sealed=EXCLUDED.access_token_sealed,refresh_token_sealed=EXCLUDED.refresh_token_sealed,expires_at=EXCLUDED.expires_at,refreshed_at=now(),connected_by=EXCLUDED.connected_by,connected_at=now()`,
		org, loc, env, token.MerchantID, sealedAccess, sealedRefresh, token.ExpiresAt, actor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'square.account_connected',$2)`, org, loc, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConnectSquareKeys connects a franchise's own Square account, test or live, with the two keys from its app in the Square Developer
// Console: the Application ID (for the card form) and the Access token (for charging). No Square sign-in page and no shared keys
// (owner, 2026-11-13). Square checks the token before anything is saved. The token does not expire or refresh, so its refresh time
// is pushed far ahead to keep the renewal job away from it.
func (s *Store) ConnectSquareKeys(ctx context.Context, actor, org, loc, env, applicationID, accessToken string) error {
	applicationID, accessToken = strings.TrimSpace(applicationID), strings.TrimSpace(accessToken)
	if len(s.mfaKey) != 32 {
		return ErrSquareOAuthUnavailable
	}
	wantApp := map[string]string{"sandbox": "sandbox-sq0idb-", "production": "sq0idp-"}[env]
	if wantApp == "" || !strings.HasPrefix(applicationID, wantApp) || len(applicationID) > 200 || len(accessToken) < 20 || len(accessToken) > 400 {
		return ErrSquareKeysInput
	}
	merchant, err := square.MerchantMe(ctx, env, accessToken, nil)
	if err != nil {
		return err
	}
	if err = s.storeSquareToken(ctx, actor, org, loc, env, square.OAuthToken{AccessToken: accessToken, MerchantID: merchant, ExpiresAt: time.Now().AddDate(10, 0, 0)}); err != nil {
		return err
	}
	if _, err = s.pool.Exec(ctx, `UPDATE square_oauth_connections SET refreshed_at=now()+interval '10 years' WHERE organization_id=$1 AND location_id=$2 AND environment=$3`, org, loc, env); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO location_square_apps(organization_id,location_id,environment,application_id,secret_sealed,updated_by) VALUES($1,$2,$3,$4,NULL,$5)
ON CONFLICT (organization_id,location_id,environment) DO UPDATE SET application_id=excluded.application_id,secret_sealed=NULL,updated_by=excluded.updated_by,updated_at=now()`, org, loc, env, applicationID, actor)
	return err
}

// SquareConnection is what the dashboard shows about a franchise's Square account (never the tokens).
type SquareConnection struct {
	Environment string    `json:"environment"`
	MerchantID  string    `json:"merchantId"`
	ConnectedAt time.Time `json:"connectedAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	RefreshedAt time.Time `json:"refreshedAt"`
}

func (s *Store) SquareConnections(ctx context.Context, actor, org, loc string) ([]SquareConnection, error) {
	out := []SquareConnection{}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT environment,merchant_id,connected_at,expires_at,refreshed_at FROM square_oauth_connections WHERE organization_id=$1 AND location_id=$2 ORDER BY environment`, org, loc)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c SquareConnection
		if err = rows.Scan(&c.Environment, &c.MerchantID, &c.ConnectedAt, &c.ExpiresAt, &c.RefreshedAt); err != nil {
			return out, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// squareAccessToken returns the franchise's access token, renewing it first when it is 7 days old or close to expiring.
func (s *Store) squareAccessToken(ctx context.Context, org, loc, env string) (string, string, error) {
	var merchant string
	var access, refresh []byte
	var expires, refreshed time.Time
	err := s.pool.QueryRow(ctx, `SELECT merchant_id,access_token_sealed,refresh_token_sealed,expires_at,refreshed_at FROM square_oauth_connections WHERE organization_id=$1 AND location_id=$2 AND environment=$3`, org, loc, env).Scan(&merchant, &access, &refresh, &expires, &refreshed)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrSquareNotConnected
	}
	if err != nil {
		return "", "", err
	}
	if time.Since(refreshed) < 7*24*time.Hour && time.Until(expires) > 24*time.Hour {
		plain, e := s.openSecret(squareSealContext(org, loc, env), access)
		return string(plain), merchant, e
	}
	app, ok := s.squareAppFor(ctx, org, loc, env)
	if !ok {
		return "", "", ErrSquareOAuthUnavailable
	}
	plainRefresh, err := s.openSecret(squareSealContext(org, loc, env), refresh)
	if err != nil {
		return "", "", err
	}
	token, err := app.Refresh(ctx, string(plainRefresh))
	if err != nil {
		return "", "", err
	}
	sealedAccess, err := s.sealSecret(squareSealContext(org, loc, env), []byte(token.AccessToken))
	if err != nil {
		return "", "", err
	}
	sealedRefresh, err := s.sealSecret(squareSealContext(org, loc, env), []byte(token.RefreshToken))
	if err != nil {
		return "", "", err
	}
	if _, err = s.pool.Exec(ctx, `UPDATE square_oauth_connections SET access_token_sealed=$4,refresh_token_sealed=$5,expires_at=$6,refreshed_at=now() WHERE organization_id=$1 AND location_id=$2 AND environment=$3`, org, loc, env, sealedAccess, sealedRefresh, token.ExpiresAt); err != nil {
		return "", "", err
	}
	return token.AccessToken, merchant, nil
}

// RefreshSquareConnections renews every connection that is due (run every few hours so tokens never lapse).
func (s *Store) RefreshSquareConnections(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT organization_id::text,location_id::text,environment FROM square_oauth_connections WHERE refreshed_at < now()-interval '6 days' OR expires_at < now()+interval '2 days'`)
	if err != nil {
		return 0, err
	}
	type key struct{ org, loc, env string }
	due := []key{}
	for rows.Next() {
		var k key
		if err = rows.Scan(&k.org, &k.loc, &k.env); err != nil {
			rows.Close()
			return 0, err
		}
		due = append(due, k)
	}
	rows.Close()
	n := 0
	for _, k := range due {
		if _, _, err := s.squareAccessToken(ctx, k.org, k.loc, k.env); err == nil {
			n++
		}
	}
	return n, nil
}

// SquareMerchantLocations lists the connected account's locations for the owner to pick from.
func (s *Store) SquareMerchantLocations(ctx context.Context, actor, org, loc, env string) (string, []square.MerchantLocation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", nil, err
	}
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		tx.Rollback(ctx)
		return "", nil, err
	}
	tx.Rollback(ctx)
	token, merchant, err := s.squareAccessToken(ctx, org, loc, env)
	if err != nil {
		return "", nil, err
	}
	list, err := square.ListLocations(ctx, env, token, nil)
	return merchant, list, err
}

// DisconnectSquare revokes Dapper's access at Square (best effort) and deletes the stored tokens.
func (s *Store) DisconnectSquare(ctx context.Context, actor, org, loc, env string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	var merchant string
	err = tx.QueryRow(ctx, `DELETE FROM square_oauth_connections WHERE organization_id=$1 AND location_id=$2 AND environment=$3 RETURNING merchant_id`, org, loc, env).Scan(&merchant)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSquareNotConnected
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'square.account_disconnected',$2)`, org, loc, actor); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if app, ok := s.squareAppFor(ctx, org, loc, env); ok {
		_ = app.Revoke(ctx, merchant)
	}
	return nil
}

// SquareCredentialResolver resolves both server-held references ("sandbox-default", "production-default") and franchise
// connections ("oauth:<location id>"). The location in the reference must belong to an organization the binding is for,
// which the binding itself guarantees (references are only written by SaveSquareMerchantBinding for that location).
func (s *Store) SquareCredentialResolver() SquareCredentialResolver {
	// Always on: each franchise's own keys are enough, no shared Square keys are needed (owner, 2026-11-13).
	return func(ctx context.Context, environment, reference string) (VerifiedSquareClient, error) {
		if loc, ok := strings.CutPrefix(reference, squareOAuthPrefix); ok {
			var org string
			if err := s.pool.QueryRow(ctx, `SELECT organization_id::text FROM locations WHERE id=$1::uuid`, loc).Scan(&org); err != nil {
				return nil, ErrPaymentProvider
			}
			token, _, err := s.squareAccessToken(ctx, org, loc, environment)
			if err != nil {
				return nil, ErrPaymentProvider
			}
			client, err := square.NewClient(square.Config{Environment: environment, AccessToken: token})
			if err != nil {
				return nil, ErrPaymentProvider
			}
			verified, ok := client.(VerifiedSquareClient)
			if !ok {
				return nil, ErrPaymentProvider
			}
			return verified, nil
		}
		if s.squareResolver == nil {
			return nil, ErrPaymentProvider
		}
		return s.squareResolver(ctx, environment, reference)
	}
}
