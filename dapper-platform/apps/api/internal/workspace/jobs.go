package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

func registerJobRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/jobs"
	for _, route := range []string{"GET " + base, "POST " + base, "POST " + base + "/{job}/complete", "POST " + base + "/{job}/contribute"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			organization, location := r.PathValue("organization"), r.PathValue("location")
			if !uuid(organization) || !uuid(location) {
				reply(w, 400, map[string]string{"error": "Valid organization and location selectors are required"})
				return
			}
			action := "jobs.read"
			if r.Method == "POST" {
				action = "jobs.create"
			}
			if r.PathValue("job") != "" {
				action = "jobs.complete"
			}
			if r.Pattern == "POST "+base+"/{job}/contribute" {
				action = "jobs.contribute"
			}
			scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, action)
			if err != nil {
				failure(w, err)
				return
			}
			if action == "jobs.read" {
				jobs, err := store.Jobs(r.Context(), scope)
				if err != nil {
					failure(w, err)
					return
				}
				reply(w, 200, map[string]any{"items": jobs, "limit": 100})
				return
			}
			if action == "jobs.contribute" {
				if !uuid(r.PathValue("job")) {
					reply(w, 400, map[string]string{"error": "A valid job selector is required"})
					return
				}
				var input struct{}
				if !decode(w, r, &input) {
					return
				}
				item, err := store.ContributeJob(r.Context(), scope, r.PathValue("job"))
				if errors.Is(err, postgres.ErrJobUnavailable) {
					reply(w, 409, map[string]string{"error": "Job unavailable or already completed"})
					return
				}
				if err != nil {
					failure(w, err)
					return
				}
				reply(w, 200, item)
				return
			}
			if action == "jobs.complete" {
				if !uuid(r.PathValue("job")) {
					reply(w, 400, map[string]string{"error": "A valid job selector is required"})
					return
				}
				var input struct{}
				if !decode(w, r, &input) {
					return
				}
				job, err := store.CompleteJob(r.Context(), scope, r.PathValue("job"))
				if errors.Is(err, postgres.ErrJobUnavailable) {
					reply(w, 409, map[string]string{"error": "Job is unavailable or already completed"})
					return
				}
				if err != nil {
					failure(w, err)
					return
				}
				reply(w, 200, job)
				return
			}
			var input struct {
				CustomerID    string   `json:"customerId"`
				ServiceName   string   `json:"serviceName"`
				ServiceID     *string  `json:"serviceId"`
				Description   string   `json:"description"`
				AddonIDs      []string `json:"addonIds"`
				ReservationID *string  `json:"reservationId"`
			}
			if !decode(w, r, &input) {
				return
			}
			fields := map[string]string{}
			seen := map[string]bool{}
			if len(input.AddonIDs) > 100 || len(input.AddonIDs) > 0 && input.ServiceID == nil {
				fields["addonIds"] = "Choose up to 100 distinct add-ons linked to a main service"
			}
			for _, id := range input.AddonIDs {
				if !uuid(id) || seen[id] {
					fields["addonIds"] = "Choose distinct valid add-ons for this main service"
				}
				seen[id] = true
			}
			if !uuid(input.CustomerID) {
				fields["customerId"] = "Choose a customer in the selected location"
			}
			if input.ReservationID != nil && !uuid(*input.ReservationID) {
				fields["reservationId"] = "Choose a valid reservation"
			}
			if n := utf8.RuneCountInString(strings.TrimSpace(input.ServiceName)); input.ServiceID == nil && (n < 2 || n > 120) {
				fields["serviceName"] = "Enter a service name between 2 and 120 characters"
			}
			if input.ServiceID != nil && (!uuid(*input.ServiceID) || input.ServiceName != "") {
				fields["serviceId"] = "Choose a main service; its name is resolved by the server"
			}
			if utf8.RuneCountInString(input.Description) > 2000 {
				fields["description"] = "Use 2000 characters or fewer"
			}
			if len(fields) > 0 {
				reply(w, 422, map[string]any{"errors": fields})
				return
			}
			job, err := store.CreateJob(r.Context(), scope, postgres.Job{CustomerID: input.CustomerID, ServiceName: input.ServiceName, Description: input.Description, ServiceID: input.ServiceID, AddonIDs: input.AddonIDs, ReservationID: input.ReservationID})
			if errors.Is(err, postgres.ErrJobAddons) {
				reply(w, 422, map[string]any{"errors": map[string]string{"addonIds": "Choose active add-ons linked to the selected main service in this location"}})
				return
			}
			if errors.Is(err, postgres.ErrJobInput) {
				reply(w, 422, map[string]any{"errors": map[string]string{"serviceId": "Choose an active main service or valid uncancelled reservation in this location"}})
				return
			}
			if err != nil {
				failure(w, err)
				return
			}
			reply(w, 201, job)
		})
	}
}
