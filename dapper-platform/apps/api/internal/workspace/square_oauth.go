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
		reply(w, 200, map[string]any{"connections": items, "available": map[string]bool{"sandbox": store.SquareOAuthAvailable("sandbox"), "production": store.SquareOAuthAvailable("production")}})
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
