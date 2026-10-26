package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"time"
)

func registerServiceReservationRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "POST /api/v1/organizations/{organization}/locations/{location}/scheduling/resources/{resource}/rules/{rule}/reservations"
	for _, route := range []string{base, base + "/{reservation}/reschedule"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			org, location, resource, rule := r.PathValue("organization"), r.PathValue("location"), r.PathValue("resource"), r.PathValue("rule")
			id := r.PathValue("reservation")
			if !uuid(org) || !uuid(location) || !uuid(resource) || !uuid(rule) || id != "" && !uuid(id) {
				reply(w, 400, map[string]string{"error": "Valid scope, resource and rule selectors required"})
				return
			}
			scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, location, "scheduling.reserve")
			if err != nil {
				failure(w, err)
				return
			}
			var input struct {
				Start       time.Time `json:"start"`
				RuleVersion int       `json:"ruleVersion"`
				Version     int       `json:"version"`
			}
			if !decode(w, r, &input) {
				return
			}
			fields := map[string]string{}
			if input.Start.IsZero() {
				fields["start"] = "Provide a timestamp with an explicit UTC offset"
			}
			if input.RuleVersion < 1 {
				fields["ruleVersion"] = "Select the current scheduling rule version"
			}
			if id != "" && input.Version < 1 || id == "" && input.Version != 0 {
				fields["version"] = "Use the current reservation version for a move; omit it for creation"
			}
			if len(fields) > 0 {
				reply(w, 422, map[string]any{"errors": fields})
				return
			}
			var item postgres.CapacityReservation
			if id == "" {
				item, err = store.ReserveServiceCapacity(r.Context(), scope, resource, rule, input.RuleVersion, input.Start)
			} else {
				item, err = store.RescheduleServiceCapacity(r.Context(), scope, resource, rule, id, input.RuleVersion, input.Version, input.Start)
			}
			if errors.Is(err, postgres.ErrSchedulingRuleConflict) {
				reply(w, 409, map[string]string{"error": "Scheduling rule changed or is unavailable; reload before reserving"})
				return
			}
			if errors.Is(err, postgres.ErrSchedulingRule) {
				reply(w, 422, map[string]any{"errors": map[string]string{"ruleVersion": "This service combination is no longer active; choose a current rule"}})
				return
			}
			if err != nil {
				schedulingFailure(w, err)
				return
			}
			status := 201
			if id != "" {
				status = 200
			}
			reply(w, status, reservationView(item))
		})
	}
}
