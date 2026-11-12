package workspace

import (
	"errors"
	"net/http"
	"net/url"

	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
)

// Connecting a franchise's own Square account. Owners with invoices.configure start it from Master; Square sends the
// owner back to /api/v1/integrations/square/oauth/callback, which stores the tokens and returns to the Card payments page.
func registerSquareOAuthRoutes(mux *http.ServeMux, store *postgres.Store, origin string) {
	base := "/api/v1/organizations/{organization}/locations/{location}/square/connection"
	selectors := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return "", "", false
		}
		return org, loc, true
	}
	env := func(r *http.Request) string {
		if r.URL.Query().Get("environment") == "production" {
			return "production"
		}
		return "sandbox"
	}
	fail := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrSquareOAuthUnavailable):
			reply(w, 503, map[string]string{"error": "Connecting Square accounts is not set up on the server yet (the Square application secret is missing)."})
		case errors.Is(err, postgres.ErrSquareNotConnected):
			reply(w, 404, map[string]string{"error": "This franchise has not connected a Square account yet."})
		case errors.Is(err, square.ErrOAuth):
			reply(w, 502, map[string]string{"error": "Square did not answer as expected. Try connecting again."})
		default:
			failure(w, err)
		}
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		items, err := store.SquareConnections(r.Context(), requestSession(r).principalID, org, loc)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"connections": items, "available": map[string]bool{"sandbox": store.SquareOAuthAvailableFor(r.Context(), org, loc, "sandbox"), "production": store.SquareOAuthAvailableFor(r.Context(), org, loc, "production")}})
	})
	mux.HandleFunc("POST "+base+"/start", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		link, err := store.StartSquareOAuth(r.Context(), requestSession(r).principalID, org, loc, env(r))
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]string{"authorizeUrl": link})
	})
	// This franchise's own Square application keys (error404 #50): used instead of the shared ones for its sign-in and card form.
	mux.HandleFunc("GET "+base+"/app-keys", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		items, err := store.LocationSquareApps(r.Context(), requestSession(r).principalID, org, loc)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("PUT "+base+"/app-keys", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		var in struct {
			Environment   string `json:"environment"`
			ApplicationID string `json:"applicationId"`
			Secret        string `json:"secret"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := store.SaveLocationSquareApp(r.Context(), requestSession(r).principalID, org, loc, in.Environment, in.ApplicationID, in.Secret); err != nil {
			if errors.Is(err, postgres.ErrSquareKeysInput) {
				reply(w, 422, map[string]string{"error": "Enter the Square Application ID and the Application secret from the Square Developer Console."})
				return
			}
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base+"/app-keys", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		if err := store.ClearLocationSquareApp(r.Context(), requestSession(r).principalID, org, loc, env(r)); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	// Connect this franchise's own Square account with its Application ID and Access token (test or live), owner 2026-11-13.
	mux.HandleFunc("POST "+base+"/keys", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		var in struct {
			Environment   string `json:"environment"`
			ApplicationID string `json:"applicationId"`
			AccessToken   string `json:"accessToken"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := store.ConnectSquareKeys(r.Context(), requestSession(r).principalID, org, loc, in.Environment, in.ApplicationID, in.AccessToken); err != nil {
			if errors.Is(err, postgres.ErrSquareKeysInput) {
				reply(w, 422, map[string]string{"error": "Enter the Application ID and Access token from the same Square app. Test ones start with sandbox-sq0idb- and EAAA; live ones with sq0idp- and EAAA."})
				return
			}
			if errors.Is(err, square.ErrOAuth) {
				reply(w, 422, map[string]string{"error": "Square did not accept that Access token. Copy it again from the Square Developer Console (Credentials page, the same Test or Live switch as here)."})
				return
			}
			fail(w, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET "+base+"/locations", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		merchant, items, err := store.SquareMerchantLocations(r.Context(), requestSession(r).principalID, org, loc, env(r))
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"merchantId": merchant, "locations": items})
	})
	mux.HandleFunc("POST "+base+"/disconnect", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		if err := store.DisconnectSquare(r.Context(), requestSession(r).principalID, org, loc, env(r)); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(204)
	})
	// Square's redirect after the owner approves or declines. Always ends on the dashboard with a short result.
	mux.HandleFunc("GET /api/v1/integrations/square/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		back := func(result string) {
			http.Redirect(w, r, origin+"/dashboard?square="+url.QueryEscape(result)+"#payments", http.StatusSeeOther)
		}
		if q.Get("error") != "" || q.Get("code") == "" || q.Get("state") == "" {
			back("declined")
			return
		}
		if _, err := store.FinishSquareOAuth(r.Context(), q.Get("state"), q.Get("code")); err != nil {
			back("failed")
			return
		}
		back("connected")
	})
}
