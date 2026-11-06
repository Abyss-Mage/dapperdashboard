package workspace

import (
	"context"
	"crypto/sha256"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"dapper.local/platform/internal/postgres"
)

// resetLimiter keeps a person (or an address) from asking for reset emails over and over. It is process-local and bounded.
type resetLimiter struct {
	mu      sync.Mutex
	hits    map[[32]byte][]time.Time
	max     int
	horizon time.Duration
}

func (l *resetLimiter) allow(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.hits) > 10000 {
		l.hits = map[[32]byte][]time.Time{}
	}
	ok := true
	hashed := make([][32]byte, len(keys))
	for i, k := range keys {
		hashed[i] = sha256.Sum256([]byte(k))
		recent := l.hits[hashed[i]][:0]
		for _, t := range l.hits[hashed[i]] {
			if now.Sub(t) < l.horizon {
				recent = append(recent, t)
			}
		}
		l.hits[hashed[i]] = recent
		if len(recent) >= l.max {
			ok = false
		}
	}
	if ok {
		for _, h := range hashed {
			l.hits[h] = append(l.hits[h], now)
		}
	}
	return ok
}

func credentialFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrPasswordWrong):
		reply(w, 422, map[string]any{"errors": map[string]string{"password": "That password is not correct"}})
	case errors.Is(err, postgres.ErrPasswordPolicy):
		reply(w, 422, map[string]any{"errors": map[string]string{"newPassword": "Use at least 15 characters"}})
	case errors.Is(err, postgres.ErrEmailInvalid):
		reply(w, 422, map[string]any{"errors": map[string]string{"email": "Enter a different, valid email address"}})
	case errors.Is(err, postgres.ErrEmailTaken):
		reply(w, 409, map[string]string{"error": "That email address is already used by another account"})
	case errors.Is(err, postgres.ErrResetTokenBad):
		reply(w, 410, map[string]string{"error": "This reset link is invalid or has expired. Ask for a new one."})
	case errors.Is(err, postgres.ErrAuthenticationBusy):
		reply(w, 503, map[string]string{"error": "Please try again in a moment"})
	default:
		failure(w, err)
	}
}

// registerAccountCredentialRoutes: change password and email while signed in, and the emailed forgot-password link (public).
func registerAccountCredentialRoutes(mux *http.ServeMux, store *postgres.Store, sender postgres.EmailSender, origin string) {
	auth, err := postgres.NewPasswordAuthenticator(store)
	if err != nil {
		log.Print("Account credential routes are unavailable")
		return
	}
	limiter := &resetLimiter{hits: map[[32]byte][]time.Time{}, max: 5, horizon: time.Hour}

	mux.HandleFunc("PUT /api/v1/account/password", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Current  string `json:"current"`
			Password string `json:"newPassword"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := auth.ChangePassword(r.Context(), requestSession(r).principalID, input.Current, input.Password); err != nil {
			credentialFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"signedOut": true})
	})
	mux.HandleFunc("PUT /api/v1/account/email", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := auth.ChangeEmail(r.Context(), requestSession(r).principalID, input.Password, input.Email); err != nil {
			credentialFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"signedOut": true})
	})

	mux.HandleFunc("POST /api/v1/public/auth/password-reset/request", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Email string `json:"email"`
		}
		if !decode(w, r, &input) {
			return
		}
		same := map[string]string{"message": "If that address has an account, a reset link is on its way. It works for one hour."}
		ip, _, splitErr := net.SplitHostPort(r.RemoteAddr)
		if splitErr != nil {
			ip = r.RemoteAddr
		}
		email := strings.ToLower(strings.TrimSpace(input.Email))
		if !limiter.allow("ip:"+ip, "email:"+email) {
			reply(w, 429, map[string]string{"error": "Too many requests. Try again later."})
			return
		}
		token, err := auth.RequestPasswordReset(r.Context(), email)
		if err != nil {
			credentialFailure(w, err)
			return
		}
		if token != "" {
			link := strings.TrimRight(origin, "/") + "/reset-password?token=" + token
			if !emailReady(sender) {
				log.Print("A password reset was requested but email delivery is not configured (DAPPER_SMTP_HOST)")
			} else {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()
					subject, body, text := store.RenderEmail(ctx, "password_reset", map[string]string{"link": link})
					if _, err := sender.SendEmail(ctx, email, subject, body, text); err != nil {
						log.Print("The password reset email could not be sent")
					}
				}()
			}
		}
		reply(w, 200, same)
	})
	// Staff hand a customer their reset link directly (for example while outgoing email is not working). The link works once, for one hour.
	mux.HandleFunc("POST /api/v1/organizations/{organization}/locations/{location}/customer-accounts/{principal}/reset-link", func(w http.ResponseWriter, r *http.Request) {
		org, loc, principal := r.PathValue("organization"), r.PathValue("location"), r.PathValue("principal")
		if !uuid(org) || !uuid(loc) || !uuid(principal) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "customer_accounts.manage")
		if err != nil {
			failure(w, err)
			return
		}
		email, err := store.LinkedAccountEmail(r.Context(), scope, principal)
		if errors.Is(err, postgres.ErrCustomerNotFound) {
			reply(w, 404, map[string]string{"error": "That account is not linked to a customer here."})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		token, err := auth.RequestPasswordReset(r.Context(), email)
		if err != nil {
			credentialFailure(w, err)
			return
		}
		if token == "" {
			reply(w, 429, map[string]string{"error": "A reset link was just made for this account. Wait two minutes and try again."})
			return
		}
		reply(w, 200, map[string]string{"link": strings.TrimRight(origin, "/") + "/reset-password?token=" + token, "email": email})
	})
	mux.HandleFunc("POST /api/v1/public/auth/password-reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Token    string `json:"token"`
			Password string `json:"newPassword"`
		}
		if !decode(w, r, &input) {
			return
		}
		ip, _, splitErr := net.SplitHostPort(r.RemoteAddr)
		if splitErr != nil {
			ip = r.RemoteAddr
		}
		if !limiter.allow("confirm-ip:" + ip) {
			reply(w, 429, map[string]string{"error": "Too many attempts. Try again later."})
			return
		}
		if err := auth.ConfirmPasswordReset(r.Context(), input.Token, input.Password); err != nil {
			credentialFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"reset": true})
	})
}
