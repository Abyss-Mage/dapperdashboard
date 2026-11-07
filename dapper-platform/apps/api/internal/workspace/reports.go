package workspace

import (
	"errors"
	"net/http"
	"time"

	"dapper.local/platform/internal/postgres"
)

// registerReportRoutes: the appointments, revenue and customers reports with date, service and status filters.
func registerReportRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/reports/{kind}", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		q := r.URL.Query()
		to, from := q.Get("to"), q.Get("from")
		if to == "" || from == "" {
			now := time.Now().UTC()
			to, from = now.Format("2006-01-02"), now.AddDate(0, 0, -29).Format("2006-01-02")
		}
		report, err := store.RunReport(r.Context(), postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc}, r.PathValue("kind"),
			postgres.ReportFilter{From: from, To: to, TZ: q.Get("tz"), Service: q.Get("service"), Status: q.Get("status")})
		if errors.Is(err, postgres.ErrReportInput) {
			reply(w, 422, map[string]string{"error": "Choose a report, a date range of at most two years and a valid time zone."})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, report)
	})
}
