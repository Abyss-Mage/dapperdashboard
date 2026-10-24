package identity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

type fixtureStore struct {
	issuer string
	issued int
}

func (s *fixtureStore) FindOIDCPrincipal(_ context.Context, issuer, subject string) (string, error) {
	if issuer != s.issuer || subject != "synthetic-approved-subject" {
		return "", errors.New("not provisioned")
	}
	return "synthetic-principal", nil
}
func (s *fixtureStore) IssueSession(_ context.Context, principal string, lifetime time.Duration) (string, error) {
	if principal != "synthetic-principal" || lifetime != time.Hour {
		return "", errors.New("invalid session context")
	}
	s.issued++
	return "synthetic-session-token", nil
}

type providerFixture struct {
	mu                     sync.Mutex
	mode, nonce, challenge string
	key, otherKey          *rsa.PrivateKey
	server                 *httptest.Server
	t                      *testing.T
}

func newProviderFixture(t *testing.T) *providerFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &providerFixture{key: key, otherKey: other, t: t}
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}
func jwt(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "fixture-key"})
	body, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
func (f *providerFixture) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/authorize", "token_endpoint": f.server.URL + "/token", "jwks_uri": f.server.URL + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
	case "/keys":
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "fixture-key", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(f.key.PublicKey.N.Bytes()), "e": "AQAB"}}})
	case "/token":
		if r.ParseForm() != nil {
			w.WriteHeader(400)
			return
		}
		user, password, ok := r.BasicAuth()
		f.mu.Lock()
		mode, nonce, challenge := f.mode, f.nonce, f.challenge
		f.mu.Unlock()
		hash := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || user != "synthetic-client" || password != "synthetic-secret" || r.Form.Get("code") != "synthetic-code" || base64.RawURLEncoding.EncodeToString(hash[:]) != challenge {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		claims := map[string]any{"iss": f.server.URL, "sub": "synthetic-approved-subject", "aud": "synthetic-client", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(), "nonce": nonce}
		switch mode {
		case "wrong_nonce":
			claims["nonce"] = "different"
		case "wrong_audience":
			claims["aud"] = "another-client"
		case "wrong_issuer":
			claims["iss"] = f.server.URL + "/other"
		case "expired":
			claims["exp"] = time.Now().Add(-time.Hour).Unix()
		case "unknown_subject":
			claims["sub"] = "unprovisioned"
		}
		signingKey := f.key
		if mode == "bad_signature" {
			signingKey = f.otherKey
		}
		response := map[string]any{"access_token": "synthetic-access-token", "token_type": "Bearer", "expires_in": 3600}
		if mode != "missing_id_token" {
			response["id_token"] = jwt(f.t, signingKey, claims)
		}
		_ = json.NewEncoder(w).Encode(response)
	default:
		http.NotFound(w, r)
	}
}

func (f *providerFixture) login(t *testing.T) (*Login, *fixtureStore) {
	t.Helper()
	store := &fixtureStore{issuer: f.server.URL}
	login, err := New(context.Background(), Config{Issuer: f.server.URL, ClientID: "synthetic-client", ClientSecret: "synthetic-secret", RedirectURL: "https://app.example.test/api/v1/auth/callback", SuccessURL: "https://app.example.test/workspace", SessionLifetime: time.Hour, HTTPClient: f.server.Client()}, store)
	if err != nil {
		t.Fatal("test provider configuration failed", err)
	}
	return login, store
}
func (f *providerFixture) begin(t *testing.T, login *Login) (string, *http.Cookie) {
	t.Helper()
	w := httptest.NewRecorder()
	login.ServeHTTP(w, httptest.NewRequest("GET", "https://app.example.test/api/v1/auth/login", nil))
	if w.Code != 302 {
		t.Fatal("login did not redirect")
	}
	authorization, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := authorization.Query()
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" || query.Get("nonce") == "" || query.Get("scope") != "openid" {
		t.Fatal("PKCE/nonce/minimal scope missing")
	}
	f.mu.Lock()
	f.nonce = query.Get("nonce")
	f.challenge = query.Get("code_challenge")
	f.mu.Unlock()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == flowCookieName {
			if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
				t.Fatal("flow cookie protections missing")
			}
			return query.Get("state"), cookie
		}
	}
	t.Fatal("flow cookie missing")
	return "", nil
}
func callback(login *Login, state string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "https://app.example.test/api/v1/auth/callback?state="+url.QueryEscape(state)+"&code=synthetic-code", nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	login.ServeHTTP(w, r)
	return w
}

func TestOIDCVerifiedIdentityAndNegativeBoundaries(t *testing.T) {
	fixture := newProviderFixture(t)
	for _, mode := range []string{"valid", "wrong_nonce", "wrong_audience", "wrong_issuer", "expired", "bad_signature", "missing_id_token", "unknown_subject"} {
		t.Run(mode, func(t *testing.T) {
			login, store := fixture.login(t)
			state, cookie := fixture.begin(t, login)
			fixture.mu.Lock()
			fixture.mode = mode
			fixture.mu.Unlock()
			result := callback(login, state, cookie)
			if mode == "valid" {
				if result.Code != 303 || store.issued != 1 || result.Header().Get("Location") != "https://app.example.test/workspace" {
					t.Fatal("verified sign-in failed")
				}
				protected := false
				for _, cookie := range result.Result().Cookies() {
					if cookie.Name == SessionCookieName {
						protected = cookie.Secure && cookie.HttpOnly && cookie.Path == "/" && cookie.Domain == "" && cookie.MaxAge == 3600
					}
				}
				if !protected {
					t.Fatal("protected session cookie missing")
				}
				if callback(login, state, cookie).Code != 400 {
					t.Fatal("callback replay accepted")
				}
			} else if result.Code < 400 || store.issued != 0 {
				t.Fatal("unverified identity issued a session")
			}
		})
	}
	t.Run("browser_binding", func(t *testing.T) {
		login, store := fixture.login(t)
		state, _ := fixture.begin(t, login)
		if callback(login, state, nil).Code != 400 || store.issued != 0 {
			t.Fatal("unbound callback accepted")
		}
	})
	t.Run("expired_state", func(t *testing.T) {
		login, store := fixture.login(t)
		state, cookie := fixture.begin(t, login)
		key := sha256.Sum256([]byte(state))
		login.mu.Lock()
		current := login.flows[key]
		current.expires = time.Now().Add(-time.Minute)
		login.flows[key] = current
		login.mu.Unlock()
		if callback(login, state, cookie).Code != 400 || store.issued != 0 {
			t.Fatal("expired login flow accepted")
		}
	})
	t.Run("flow_bound", func(t *testing.T) {
		login, _ := fixture.login(t)
		for i := 0; i < 1000; i++ {
			key := sha256.Sum256([]byte{byte(i), byte(i >> 8)})
			login.flows[key] = flow{expires: time.Now().Add(time.Minute)}
		}
		w := httptest.NewRecorder()
		login.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/auth/login", nil))
		if w.Code != 429 {
			t.Fatal("unbounded pending login flows")
		}
	})
}

func TestOIDCConfigurationFailsClosed(t *testing.T) {
	if _, err := New(context.Background(), Config{Issuer: "http://insecure.example"}, &fixtureStore{}); err == nil {
		t.Fatal("insecure issuer accepted")
	}
	if _, err := New(context.Background(), Config{Issuer: "https://issuer.example", RedirectURL: "https://app.example/api/v1/auth/callback", SuccessURL: "https://other.example/", ClientID: "test", ClientSecret: "synthetic", SessionLifetime: time.Hour}, &fixtureStore{}); err == nil {
		t.Fatal("cross-origin success redirect accepted")
	}
}
