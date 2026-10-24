package identity

import (
	"context"
	"crypto/sha256"
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type PasswordVerifier interface {
	SignIn(context.Context, string, string, time.Duration) (string, error)
}

// CodeVerifier is the optional second-step sign-in used when a person has authenticator-app MFA on.
type CodeVerifier interface {
	SignInWithCode(context.Context, string, string, string, time.Duration) (string, error)
}
type attemptBucket struct {
	count   int
	expires time.Time
}
type PasswordLogin struct {
	verifier PasswordVerifier
	origin   string
	lifetime time.Duration
	mu       sync.Mutex
	attempts map[[32]byte]attemptBucket
}

func NewPasswordLogin(verifier PasswordVerifier, origin string, lifetime time.Duration) (*PasswordLogin, error) {
	parsed, err := httpsURL(origin)
	if err != nil || parsed.Path != "" || lifetime < time.Second || lifetime > 24*time.Hour || verifier == nil {
		return nil, errors.New("explicit HTTPS origin and session lifetime required")
	}
	return &PasswordLogin{verifier: verifier, origin: origin, lifetime: lifetime, attempts: make(map[[32]byte]attemptBucket)}, nil
}

// Process-local abuse control is bounded, shared by all requests and does not
// trust forwarded client addresses. Distributed deployment needs a shared limit.
func (p *PasswordLogin) allow(ip, email string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for key, bucket := range p.attempts {
		if !now.Before(bucket.expires) {
			delete(p.attempts, key)
		}
	}
	keys := [][32]byte{sha256.Sum256([]byte("ip:" + ip)), sha256.Sum256([]byte("email:" + strings.ToLower(strings.TrimSpace(email))))}
	if len(p.attempts)+2 > 10000 {
		return false
	}
	for index, key := range keys {
		limit := 30
		if index == 1 {
			limit = 10
		}
		if p.attempts[key].count >= limit {
			return false
		}
	}
	for _, key := range keys {
		bucket := p.attempts[key]
		if bucket.count == 0 {
			bucket.expires = now.Add(15 * time.Minute)
		}
		bucket.count++
		p.attempts[key] = bucket
	}
	return true
}
func passwordReply(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func (p *PasswordLogin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path != "/api/v1/auth/login" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		passwordReply(w, 405, "Use POST to sign in")
		return
	}
	if r.Header.Get("Origin") != p.origin {
		passwordReply(w, 403, "Same-origin request required")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		passwordReply(w, 415, "Use application/json")
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		passwordReply(w, 400, "Send one valid sign-in object")
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !p.allow(ip, input.Email) {
		w.Header().Set("Retry-After", "900")
		passwordReply(w, 429, "Too many sign-in attempts. Try again later")
		return
	}
	var token string
	if coded, ok := p.verifier.(CodeVerifier); ok {
		token, err = coded.SignInWithCode(r.Context(), input.Email, input.Password, input.Code, p.lifetime)
	} else {
		token, err = p.verifier.SignIn(r.Context(), input.Email, input.Password, p.lifetime)
	}
	input.Password, input.Code = "", ""
	if errors.Is(err, postgres.ErrMFARequired) || errors.Is(err, postgres.ErrMFAInvalid) {
		message := "Enter the 6-digit code from your authenticator app, or a recovery code"
		if errors.Is(err, postgres.ErrMFAInvalid) {
			message = "That code is incorrect or was already used. Try the next code from your app"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": message, "mfaRequired": true})
		return
	}
	if errors.Is(err, postgres.ErrAuthenticationBusy) {
		w.Header().Set("Retry-After", "2")
		passwordReply(w, 503, "Sign-in is busy. Try again shortly")
		return
	}
	if errors.Is(err, postgres.ErrSession) {
		passwordReply(w, 401, "Email or password is incorrect")
		return
	}
	if err != nil {
		passwordReply(w, 503, "Sign-in is temporarily unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int(p.lifetime.Seconds())})
	w.WriteHeader(204)
}
