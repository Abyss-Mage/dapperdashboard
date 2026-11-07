package square

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuth lets each franchise connect its own Square account (Square OAuth code flow, confidential client):
// the owner approves Dapper on Square's page, Square redirects back with a one-time code (valid 5 minutes),
// and the server exchanges it for an access token (30 days) and a refresh token (code flow: does not expire).
// Square asks for access tokens to be renewed every 7 days or less.

const OAuthVersion = "2025-12-17"

// Scopes: take card payments, read them back, and read the merchant's locations to pick and verify one.
var OAuthScopes = []string{"PAYMENTS_WRITE", "PAYMENTS_READ", "MERCHANT_PROFILE_READ"}

var ErrOAuth = errors.New("Square did not complete the account connection")

type OAuthApp struct {
	Environment   string
	ApplicationID string
	Secret        string `json:"-"`
	BaseURL       string // optional override (tests)
	HTTPClient    *http.Client
}

func (a OAuthApp) String() string { return "Square OAuth application (secret redacted)" }

func (a OAuthApp) base() string {
	if a.BaseURL != "" {
		return a.BaseURL
	}
	if a.Environment == "production" {
		return "https://connect.squareup.com"
	}
	return "https://connect.squareupsandbox.com"
}
func (a OAuthApp) client() *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// AuthorizeURL is where the owner approves the connection. state is the single-use CSRF value checked on return.
func (a OAuthApp) AuthorizeURL(state string) string {
	q := url.Values{"client_id": {a.ApplicationID}, "scope": {strings.Join(OAuthScopes, " ")}, "state": {state}}
	if a.Environment == "production" {
		// Ask Square to show the sign-in page even if someone is already signed in, so the right account is connected.
		q.Set("session", "false")
	}
	return a.base() + "/oauth2/authorize?" + q.Encode()
}

type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	MerchantID   string    `json:"merchant_id"`
}

func (a OAuthApp) token(ctx context.Context, body map[string]string) (OAuthToken, error) {
	var out OAuthToken
	if a.ApplicationID == "" || a.Secret == "" {
		return out, ErrOAuth
	}
	body["client_id"], body["client_secret"] = a.ApplicationID, a.Secret
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base()+"/oauth2/token", bytes.NewReader(raw))
	if err != nil {
		return out, ErrOAuth
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Square-Version", OAuthVersion)
	res, err := a.client().Do(req)
	if err != nil {
		return out, ErrOAuth
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 64*1024))
	if err != nil || res.StatusCode != http.StatusOK || json.Unmarshal(data, &out) != nil || out.AccessToken == "" || out.MerchantID == "" {
		return OAuthToken{}, ErrOAuth
	}
	return out, nil
}

// ExchangeCode turns the one-time authorization code into tokens.
func (a OAuthApp) ExchangeCode(ctx context.Context, code string) (OAuthToken, error) {
	return a.token(ctx, map[string]string{"grant_type": "authorization_code", "code": code})
}

// Refresh gets a new access token. A code-flow refresh token keeps working, so it is kept when Square does not send a new one.
func (a OAuthApp) Refresh(ctx context.Context, refreshToken string) (OAuthToken, error) {
	t, err := a.token(ctx, map[string]string{"grant_type": "refresh_token", "refresh_token": refreshToken})
	if err == nil && t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, err
}

// Revoke ends Dapper's access to the merchant's account (all of this application's tokens for that merchant).
func (a OAuthApp) Revoke(ctx context.Context, merchantID string) error {
	raw, _ := json.Marshal(map[string]string{"client_id": a.ApplicationID, "merchant_id": merchantID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base()+"/oauth2/revoke", bytes.NewReader(raw))
	if err != nil {
		return ErrOAuth
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Square-Version", OAuthVersion)
	req.Header.Set("Authorization", "Client "+a.Secret)
	res, err := a.client().Do(req)
	if err != nil {
		return ErrOAuth
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ErrOAuth
	}
	return nil
}

type MerchantLocation struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Cards    bool   `json:"cards"`
}

// MerchantMe returns the merchant id a Square access token belongs to; it also proves the token works.
func MerchantMe(ctx context.Context, environment, accessToken string, hc *http.Client) (string, error) {
	app := OAuthApp{Environment: environment, HTTPClient: hc}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, app.base()+"/v2/merchants/me", nil)
	if err != nil {
		return "", ErrOAuth
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Square-Version", OAuthVersion)
	req.Header.Set("Accept", "application/json")
	res, err := app.client().Do(req)
	if err != nil {
		return "", ErrOAuth
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 64*1024))
	var decoded struct {
		Merchant struct {
			ID string `json:"id"`
		} `json:"merchant"`
	}
	if err != nil || res.StatusCode != http.StatusOK || json.Unmarshal(data, &decoded) != nil || decoded.Merchant.ID == "" {
		return "", ErrOAuth
	}
	return decoded.Merchant.ID, nil
}

// ListLocations reads the connected merchant's locations so the owner can pick the one this franchise charges to.
func ListLocations(ctx context.Context, environment, accessToken string, hc *http.Client) ([]MerchantLocation, error) {
	app := OAuthApp{Environment: environment, HTTPClient: hc}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, app.base()+"/v2/locations", nil)
	if err != nil {
		return nil, ErrOAuth
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Square-Version", OAuthVersion)
	req.Header.Set("Accept", "application/json")
	res, err := app.client().Do(req)
	if err != nil {
		return nil, ErrOAuth
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 512*1024))
	if err != nil || res.StatusCode != http.StatusOK {
		return nil, ErrOAuth
	}
	var decoded struct {
		Locations []struct {
			ID           string   `json:"id"`
			Name         string   `json:"name"`
			Currency     string   `json:"currency"`
			Status       string   `json:"status"`
			Capabilities []string `json:"capabilities"`
		} `json:"locations"`
	}
	if json.Unmarshal(data, &decoded) != nil {
		return nil, ErrOAuth
	}
	out := []MerchantLocation{}
	for _, l := range decoded.Locations {
		m := MerchantLocation{ID: l.ID, Name: l.Name, Currency: l.Currency, Status: l.Status}
		for _, c := range l.Capabilities {
			m.Cards = m.Cards || c == "CREDIT_CARD_PROCESSING"
		}
		out = append(out, m)
	}
	return out, nil
}
