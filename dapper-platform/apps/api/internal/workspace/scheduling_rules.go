package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func registerSchedulingRuleRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/scheduling/resources/{resource}/rules"
	for _, route := range []string{"GET " + base, "POST " + base, "PUT " + base + "/{rule}"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			org, location, resource, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("resource"), r.PathValue("rule")
			if !uuid(org) || !uuid(location) || !uuid(resource) || id != "" && !uuid(id) {
				reply(w, 400, map[string]string{"error": "Valid scope, resource and rule selectors required"})
				return
			}
			scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, location, "scheduling.manage")
			if err != nil {
				failure(w, err)
				return
			}
			if r.Method == "GET" {
				items, err := store.SchedulingRules(r.Context(), scope, resource)
				if err != nil {
					failure(w, err)
					return
				}
				reply(w, 200, map[string]any{"items": items, "limit": 100})
				return
			}
			var input struct {
				ServiceID string   `json:"serviceId"`
				AddonIDs  []string `json:"addonIds"`
				Duration  *int     `json:"durationMinutes"`
				WorkingTime bool   `json:"workingTime"`
				Before    *int     `json:"beforeMinutes"`
				After     *int     `json:"afterMinutes"`
				Units     int      `json:"units"`
				Version   int      `json:"version"`
			}
			if !decode(w, r, &input) {
				return
			}
			fields := map[string]string{}
			if !uuid(input.ServiceID) {
				fields["serviceId"] = "Choose an active main service in this location"
			}
			seen := map[string]bool{}
			if len(input.AddonIDs) > 100 {
				fields["addonIds"] = "Choose at most 100 distinct add-ons"
			}
			for _, addon := range input.AddonIDs {
				if !uuid(addon) || seen[addon] {
					fields["addonIds"] = "Choose distinct valid add-ons linked to this main service"
				}
				seen[addon] = true
			}
			for _, field := range []struct {
				name    string
				value   *int
				minimum int
			}{{"durationMinutes", input.Duration, 1}, {"beforeMinutes", input.Before, 0}, {"afterMinutes", input.After, 0}} {
				if field.value == nil || *field.value < field.minimum || *field.value > 10080 {
					fields[field.name] = "Provide explicit whole minutes within the supported request range"
				}
			}
			if input.Units < 1 || input.Units > 1000000 {
				fields["units"] = "Provide explicit positive resource capacity units"
			}
			if id != "" && input.Version < 1 || id == "" && input.Version != 0 {
				fields["version"] = "Use current version when editing; omit it when creating"
			}
			if len(fields) > 0 {
				reply(w, 422, map[string]any{"errors": fields})
				return
			}
			item, err := store.SaveSchedulingRule(r.Context(), scope, postgres.SchedulingRule{ID: id, ResourceID: resource, ServiceID: input.ServiceID, AddonIDs: input.AddonIDs, DurationMinutes: *input.Duration, BeforeMinutes: *input.Before, AfterMinutes: *input.After, Units: input.Units, WorkingTime: input.WorkingTime, Version: input.Version})
			if errors.Is(err, postgres.ErrSchedulingRule) {
				reply(w, 422, map[string]any{"errors": map[string]string{"schedulingRule": "Choose an active scoped main/add-on combination and sufficient resource capacity"}})
				return
			}
			if errors.Is(err, postgres.ErrSchedulingRuleConflict) {
				reply(w, 409, map[string]string{"error": "Rule duplicated, changed or unavailable; reload before editing"})
				return
			}
			if err != nil {
				failure(w, err)
				return
			}
			status := 201
			if id != "" {
				status = 200
			}
			reply(w, status, item)
		})
	}
}
