package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"strconv"
)

// Which days and times a configured service can be booked, for the guided booking screens (Workspace and Master).
func registerRuleAvailabilityRoutes(mux *http.ServeMux, store *postgres.Store) {
	parse := func(r *http.Request) (resource, rule string, version int, from string, days int, ok bool) {
		q := r.URL.Query()
		resource, rule, from = q.Get("resourceId"), q.Get("ruleId"), q.Get("from")
		var e1, e2 error
		version, e1 = strconv.Atoi(q.Get("version"))
		days, e2 = strconv.Atoi(q.Get("days"))
		ok = uuid(resource) && uuid(rule) && e1 == nil && e2 == nil && version >= 1 && days >= 1 && days <= 31 && len(from) == 10
		return
	}
	fail := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrAvailabilityInput), errors.Is(err, postgres.ErrSchedulingRule), errors.Is(err, postgres.ErrSchedulingRuleConflict):
			reply(w, 422, map[string]string{"error": "Choose a service that is set up for booking, then try again."})
		case errors.Is(err, postgres.ErrAvailabilityLimit):
			reply(w, 422, map[string]string{"error": "That period has too many bookings to show at once. Pick a shorter period."})
		default:
			employeeFailure(w, err)
		}
	}
	mux.HandleFunc("GET /api/v1/employee/walk-in/availability", func(w http.ResponseWriter, r *http.Request) {
		resource, rule, version, from, days, ok := parse(r)
		if !ok {
			reply(w, 400, map[string]string{"error": "Valid service, resource, start day and number of days (1-31) required"})
			return
		}
		out, err := store.EmployeeRuleAvailability(r.Context(), requestSession(r).principalID, resource, rule, version, from, days)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("GET /api/v1/employee/walk-in/taxes", func(w http.ResponseWriter, r *http.Request) {
		out, err := store.EmployeeBookingTaxes(r.Context(), requestSession(r).principalID)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/bookings/availability", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		resource, rule, version, from, days, ok := parse(r)
		if !uuid(org) || !uuid(loc) || !ok {
			reply(w, 400, map[string]string{"error": "Valid service, resource, start day and number of days (1-31) required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.create")
		if err != nil {
			failure(w, err)
			return
		}
		// Administrators who manage bookings can also see earlier days, to enter a booking that already happened.
		past := false
		if r.URL.Query().Get("past") == "1" {
			_, perr := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.manage")
			past = perr == nil
		}
		out, err := store.RuleAvailability(r.Context(), scope, resource, rule, version, from, days, past)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
}
