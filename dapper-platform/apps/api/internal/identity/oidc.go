// Package identity implements a configured OIDC login boundary. It does not
// provision accounts, assign grants or infer platform identity from WordPress.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const SessionCookieName = "__Host-dapper_workspace"
const flowCookieName = "__Host-dapper_login"
const flowLifetime = 5 * time.Minute

type Store interface {
	FindOIDCPrincipal(context.Context, string, string) (string, error)
	IssueSession(context.Context, string, time.Duration) (string, error)
}
type Config struct {
	Issuer, ClientID, ClientSecret, RedirectURL, SuccessURL string
	SessionLifetime                                         time.Duration
	HTTPClient                                              *http.Client
}
type flow struct {
	nonce, verifier string
	binding         [32]byte
	expires         time.Time
}
type Login struct {
	store    Store
	config   Config
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	client   *http.Client
	mu       sync.Mutex
	flows    map[[32]byte]flow
}

func httpsURL(raw string) (*url.URL, error) {
	value, err := url.Parse(raw)
	if err != nil || value.Scheme != "https" || value.Host == "" || value.User != nil || value.RawQuery != "" || value.Fragment != "" {
		return nil, errors.New("HTTPS identity configuration required")
	}
	return value, nil
}

func New(ctx context.Context, config Config, store Store) (*Login, error) {
	if _, err := httpsURL(config.Issuer); err != nil {
		return nil, err
	}
	redirect, err := httpsURL(config.RedirectURL)
	if err != nil {
		return nil, err
	}
	success, err := httpsURL(config.SuccessURL)
	if err != nil {
		return nil, err
	}
	if redirect.Path != "/api/v1/auth/callback" || redirect.Host != success.Host || strings.TrimSpace(config.ClientID) == "" || config.ClientSecret == "" || store == nil || config.SessionLifetime < time.Minute || config.SessionLifetime > 24*time.Hour {
		return nil, errors.New("complete identity configuration required")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if config.HTTPClient != nil {
		copy := *config.HTTPClient
		copy.Timeout = 10 * time.Second
		client = &copy
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	providerCtx := oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(providerCtx, config.Issuer)
	if err != nil {
		return nil, errors.New("identity provider discovery failed")
	}
	var metadata struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if provider.Claims(&metadata) != nil {
		return nil, errors.New("invalid provider metadata")
	}
	if _, err = httpsURL(metadata.JWKSURL); err != nil {
		return nil, errors.New("HTTPS provider key endpoint required")
	}
	endpoint := provider.Endpoint()
	if _, err = httpsURL(endpoint.AuthURL); err != nil {
		return nil, errors.New("HTTPS provider authorization endpoint required")
	}
	if _, err = httpsURL(endpoint.TokenURL); err != nil {
		return nil, errors.New("HTTPS provider token endpoint required")
	}
	return &Login{store: store, config: config, oauth: oauth2.Config{ClientID: config.ClientID, ClientSecret: config.ClientSecret, RedirectURL: config.RedirectURL, Endpoint: endpoint, Scopes: []string{oidc.ScopeOpenID}}, verifier: provider.VerifierContext(providerCtx, &oidc.Config{ClientID: config.ClientID, SupportedSigningAlgs: []string{oidc.RS256}}), client: client, flows: make(map[[32]byte]flow)}, nil
}

func randomValue() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func cookie(name, value string, lifetime time.Duration) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: int(lifetime.Seconds())}
}
func clearFlow(w http.ResponseWriter) { http.SetCookie(w, cookie(flowCookieName, "", -time.Second)) }
func denied(w http.ResponseWriter, status int) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Error(w, "Sign-in could not be completed. Start a new sign-in attempt.", status)
}

func (login *Login) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	switch r.URL.Path {
	case "/api/v1/auth/login":
		login.begin(w, r)
	case "/api/v1/auth/callback":
		login.callback(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (login *Login) begin(w http.ResponseWriter, r *http.Request) {
	state, err := randomValue()
	if err != nil {
		denied(w, 503)
		return
	}
	nonce, err := randomValue()
	if err != nil {
		denied(w, 503)
		return
	}
	binding, err := randomValue()
	if err != nil {
		denied(w, 503)
		return
	}
	verifier := oauth2.GenerateVerifier()
	now := time.Now()
	login.mu.Lock()
	for key, current := range login.flows {
		if !current.expires.After(now) {
			delete(login.flows, key)
		}
	}
	if len(login.flows) >= 1000 {
		login.mu.Unlock()
		denied(w, 429)
		return
	}
	login.flows[sha256.Sum256([]byte(state))] = flow{nonce: nonce, verifier: verifier, binding: sha256.Sum256([]byte(binding)), expires: now.Add(flowLifetime)}
	login.mu.Unlock()
	http.SetCookie(w, cookie(flowCookieName, binding, flowLifetime))
	http.Redirect(w, r, login.oauth.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce), oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

func (login *Login) consume(state, binding string) (flow, bool) {
	if len(state) != 43 || len(binding) != 43 {
		return flow{}, false
	}
	key := sha256.Sum256([]byte(state))
	hash := sha256.Sum256([]byte(binding))
	login.mu.Lock()
	defer login.mu.Unlock()
	current, exists := login.flows[key]
	if !exists || !current.expires.After(time.Now()) || subtle.ConstantTimeCompare(current.binding[:], hash[:]) != 1 {
		return flow{}, false
	}
	delete(login.flows, key)
	return current, true
}

func (login *Login) callback(w http.ResponseWriter, r *http.Request) {
	clearFlow(w)
	binding, err := r.Cookie(flowCookieName)
	if err != nil {
		denied(w, 400)
		return
	}
	current, ok := login.consume(r.URL.Query().Get("state"), binding.Value)
	if !ok || r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		denied(w, 400)
		return
	}
	ctx, cancel := context.WithTimeout(oidc.ClientContext(r.Context(), login.client), 15*time.Second)
	defer cancel()
	token, err := login.oauth.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(current.verifier))
	if err != nil {
		denied(w, 401)
		return
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || len(raw) > 32768 {
		denied(w, 401)
		return
	}
	id, err := login.verifier.Verify(ctx, raw)
	if err != nil || id.Subject == "" || subtle.ConstantTimeCompare([]byte(id.Nonce), []byte(current.nonce)) != 1 {
		denied(w, 401)
		return
	}
	principal, err := login.store.FindOIDCPrincipal(ctx, login.config.Issuer, id.Subject)
	if err != nil || principal == "" {
		denied(w, 403)
		return
	}
	session, err := login.store.IssueSession(ctx, principal, login.config.SessionLifetime)
	if err != nil {
		denied(w, 403)
		return
	}
	http.SetCookie(w, cookie(SessionCookieName, session, login.config.SessionLifetime))
	http.Redirect(w, r, login.config.SuccessURL, http.StatusSeeOther)
}
