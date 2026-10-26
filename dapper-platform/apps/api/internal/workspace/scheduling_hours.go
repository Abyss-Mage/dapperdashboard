package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"time"
)

// Weekly opening hours and whole-day closures for one scheduling resource (grant: scheduling.manage).
func registerSchedulingHoursRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/scheduling/resources/{resource}"
	scoped := func(w http.ResponseWriter, r *http.Request) (postgres.Scope, string, bool) {
		org, loc, resource := r.PathValue("organization"), r.PathValue("location"), r.PathValue("resource")
		if !uuid(org) || !uuid(loc) || !uuid(resource) {
			reply(w, 400, map[string]string{"error": "Valid scope and resource selectors required"})
			return postgres.Scope{}, "", false
		}
		return postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc}, resource, true
	}
	done := func(w http.ResponseWriter, err error) {
		if errors.Is(err, postgres.ErrReservationInput) {
			reply(w, 422, map[string]any{"errors": map[string]string{"hours": "Use weekdays, HH:MM times with the end after the start, no overlaps, at most 4 windows a day"}})
			return
		}
		schedulingFailure(w, err)
	}
	// Slots: how many bookings the franchise can take at the same time. Creates the franchise's booking resource on first use.
	mux.HandleFunc("PUT /api/v1/organizations/{organization}/locations/{location}/scheduling/slots", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid scope selectors required"})
			return
		}
		var input struct {
			Slots    int    `json:"slots"`
			Timezone string `json:"timezone"`
		}
		if !decode(w, r, &input) {
			return
		}
		scope := postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc}
		if err := store.SaveBookingSlots(r.Context(), scope, input.Slots, input.Timezone); err != nil {
			if errors.Is(err, postgres.ErrReservationInput) {
				reply(w, 422, map[string]any{"errors": map[string]string{"slots": "Enter a whole number of slots from 1 to 1000 and a valid time zone"}})
				return
			}
			schedulingFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"saved": true, "slots": input.Slots})
	})
	mux.HandleFunc("PUT "+base+"/hours", func(w http.ResponseWriter, r *http.Request) {
		scope, resource, ok := scoped(w, r)
		if !ok {
			return
		}
		var input struct {
			Weekly []postgres.HourWindow `json:"weekly"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.SaveWeeklyHours(r.Context(), scope, resource, input.Weekly); err != nil {
			done(w, err)
			return
		}
		reply(w, 200, map[string]any{"saved": true, "bookingConfirmed": false})
	})
	mux.HandleFunc("POST "+base+"/closures", func(w http.ResponseWriter, r *http.Request) {
		scope, resource, ok := scoped(w, r)
		if !ok {
			return
		}
		var input struct {
			Date string `json:"date"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.AddClosure(r.Context(), scope, resource, input.Date); err != nil {
			done(w, err)
			return
		}
		reply(w, 201, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base+"/closures", func(w http.ResponseWriter, r *http.Request) {
		scope, resource, ok := scoped(w, r)
		if !ok {
			return
		}
		start, err := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
		if err != nil {
			reply(w, 422, map[string]any{"errors": map[string]string{"start": "Provide the closure start time"}})
			return
		}
		if err := store.RemoveClosure(r.Context(), scope, resource, start); err != nil {
			done(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
}
