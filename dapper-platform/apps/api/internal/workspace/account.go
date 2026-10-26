package workspace

import (
	"errors"
	"net/http"
	"strings"

	"dapper.local/platform/internal/postgres"
)

// mfaSetupAllowed lists what an administrator who has not yet set up MFA may still call: the account/security routes
// themselves and sign-out. Everything else answers 403 "mfa_setup_required" until MFA is on.
func mfaSetupAllowed(path string) bool {
	return strings.HasPrefix(path, "/api/v1/account/") || path == "/api/v1/auth/logout" || path == "/api/v1/auth/me"
}

func accountFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrProfileInput):
		reply(w, 422, map[string]any{"errors": map[string]string{"profile": "Enter your full name and an international phone number such as +14165550123"}})
	case errors.Is(err, postgres.ErrMFAUnavailable):
		reply(w, 503, map[string]string{"error": "Two-factor authentication is not available yet. Ask your administrator."})
	case errors.Is(err, postgres.ErrMFAInvalid):
		reply(w, 422, map[string]any{"errors": map[string]string{"code": "That code is incorrect or expired. Enter the current 6-digit code from your app"}})
	case errors.Is(err, postgres.ErrMFARequired):
		reply(w, 422, map[string]any{"errors": map[string]string{"code": "Enter a code from your authenticator app"}})
	case errors.Is(err, postgres.ErrMFAState):
		reply(w, 409, map[string]string{"error": "Two-factor authentication is already set up, or setup has not been started"})
	case errors.Is(err, postgres.ErrMFAAdminRequired):
		reply(w, 403, map[string]string{"error": "Administrators must keep two-factor authentication on"})
	default:
		failure(w, err)
	}
}

func registerAccountRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/account/onboarding", func(w http.ResponseWriter, r *http.Request) {
		status, err := store.OnboardingStatus(r.Context(), requestSession(r).principalID)
		if err != nil {
			accountFailure(w, err)
			return
		}
		reply(w, 200, status)
	})
	mux.HandleFunc("GET /api/v1/account/profile", func(w http.ResponseWriter, r *http.Request) {
		p, err := store.GetProfile(r.Context(), requestSession(r).principalID)
		if err != nil {
			accountFailure(w, err)
			return
		}
		reply(w, 200, p)
	})
	mux.HandleFunc("PUT /api/v1/account/profile", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			FullName string `json:"fullName"`
			Phone    string `json:"phone"`
		}
		if !decode(w, r, &input) {
			return
		}
		p, err := store.SaveProfile(r.Context(), requestSession(r).principalID, input.FullName, input.Phone)
		if err != nil {
			accountFailure(w, err)
			return
		}
		reply(w, 200, p)
	})
	mux.HandleFunc("GET /api/v1/account/security", func(w http.ResponseWriter, r *http.Request) {
		st, err := store.MFAStatus(r.Context(), requestSession(r).principalID)
		if err != nil {
			accountFailure(w, err)
			return
		}
		reply(w, 200, st)
	})
	mux.HandleFunc("POST /api/v1/account/mfa/setup", func(w http.ResponseWriter, r *http.Request) {
		var input struct{}
		if !decode(w, r, &input) {
			return
		}
		secret, uri, err := store.BeginMFASetup(r.Context(), requestSession(r).principalID)
		if err != nil {
			accountFailure(w, err)
			return
		}
		reply(w, 200, map[string]string{"secret": secret, "uri": uri})
	})
	mux.HandleFunc("POST /api/v1/account/mfa/confirm", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Code string `json:"code"`
		}
		if !decode(w, r, &input) {
			return
		}
		codes, err := store.ConfirmMFASetup(r.Context(), requestSession(r).principalID, input.Code)
		if err != nil {
			accountFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"recoveryCodes": codes})
	})
	mux.HandleFunc("POST /api/v1/account/mfa/disable", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Code string `json:"code"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.DisableMFA(r.Context(), requestSession(r).principalID, input.Code); err != nil {
			accountFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"enabled": false})
	})
}
