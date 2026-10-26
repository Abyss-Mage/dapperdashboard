package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

// Owners enter the Square application keys for the test and live applications here (organisation-level invoices.configure).
// The secret is write-only: it is never sent back to a browser.
func registerSquareAppCredentialRoutes(mux *http.ServeMux, store *postgres.Store, origin string) {
	base := "/api/v1/organizations/{organization}/square/app-credentials"
	org := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		id := r.PathValue("organization")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return "", false
		}
		return id, true
	}
	fail := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrSquareKeysInput):
			reply(w, 422, map[string]any{"errors": map[string]string{"keys": "Enter the Application ID and the application secret from your Square Developer dashboard"}})
		case errors.Is(err, postgres.ErrSquareOAuthUnavailable):
			reply(w, 503, map[string]string{"error": "The server key used to protect secrets is not set, so Square keys cannot be saved yet."})
		default:
			failure(w, err)
		}
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		items, err := store.SquareAppStatuses(r.Context(), requestSession(r).principalID, id)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "redirectUrl": origin + "/api/v1/integrations/square/oauth/callback"})
	})
	mux.HandleFunc("PUT "+base+"/{environment}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		var input struct {
			ApplicationID string `json:"applicationId"`
			Secret        string `json:"secret"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.SaveSquareAppCredentials(r.Context(), requestSession(r).principalID, id, r.PathValue("environment"), input.ApplicationID, input.Secret); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base+"/{environment}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		if err := store.ClearSquareAppCredentials(r.Context(), requestSession(r).principalID, id, r.PathValue("environment")); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
}
