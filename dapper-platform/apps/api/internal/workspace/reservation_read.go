package workspace

import (
	"dapper.local/platform/internal/postgres"
	"net/http"
	"time"
)

func registerReservationReadRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/scheduling/reservations", func(w http.ResponseWriter, r *http.Request) {
		org, location := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(location) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, location, "scheduling.read")
		if err != nil {
			failure(w, err)
			return
		}
		from, fromErr := time.Parse(time.RFC3339Nano, r.URL.Query().Get("from"))
		to, toErr := time.Parse(time.RFC3339Nano, r.URL.Query().Get("to"))
		if fromErr != nil || toErr != nil {
			reply(w, 422, map[string]any{"errors": map[string]string{"range": "Provide from/to timestamps with explicit UTC offsets"}})
			return
		}
		page, err := store.Reservations(r.Context(), scope, from, to, r.URL.Query().Get("cursor"), 100)
		if err != nil {
			schedulingFailure(w, err)
			return
		}
		items := []map[string]any{}
		for _, item := range page.Items {
			items = append(items, reservationView(item))
		}
		reply(w, 200, map[string]any{"items": items, "nextCursor": page.NextCursor, "limit": 100, "bookingConfirmed": false})
	})
}
