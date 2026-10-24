package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"time"
)

func registerOwnerOverviewRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/overview", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid location selectors required"})
			return
		}
		result, err := store.OwnerOverview(r.Context(), postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc})
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, result)
	})
	// Daily series and breakdowns for the overview charts. from/to are inclusive calendar days in tz.
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/overview/trends", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid location selectors required"})
			return
		}
		q := r.URL.Query()
		tz := q.Get("tz")
		if tz == "" {
			tz = "UTC"
		}
		from, to := q.Get("from"), q.Get("to")
		if from == "" || to == "" {
			now := time.Now().UTC()
			to, from = now.Format("2006-01-02"), now.AddDate(0, 0, -29).Format("2006-01-02")
		}
		result, err := store.OwnerTrends(r.Context(), postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc}, from, to, tz, q.Get("channel"))
		if errors.Is(err, postgres.ErrTrendsInput) {
			reply(w, 422, map[string]string{"error": "Choose a date range of at most 366 days, a valid timezone and a known channel"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, result)
	})
	// Extra figures for the Finance, Work Orders and Customers dashboards (section = finance | work | customers).
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/overview/insights", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid location selectors required"})
			return
		}
		q := r.URL.Query()
		tz := q.Get("tz")
		if tz == "" {
			tz = "UTC"
		}
		result, err := store.OwnerInsights(r.Context(), postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc}, q.Get("section"), q.Get("from"), q.Get("to"), tz)
		if errors.Is(err, postgres.ErrTrendsInput) {
			reply(w, 422, map[string]string{"error": "Choose finance, work or customers, a date range of at most 366 days and a valid timezone"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, result)
	})
}
