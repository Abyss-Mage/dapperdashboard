package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

func serviceFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrServiceConflict) {
		reply(w, 409, map[string]string{"error": "Service changed, is unavailable, or still has active add-ons. Refresh before editing"})
		return
	}
	if errors.Is(err, postgres.ErrServiceInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"parentId": "Choose an active main service in this location"}})
		return
	}
	failure(w, err)
}
func registerServiceRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/services"
	for _, route := range []string{"GET " + base, "POST " + base, "PUT " + base + "/{service}", "POST " + base + "/{service}/archive"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			organization, location, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("service")
			if !uuid(organization) || !uuid(location) || id != "" && !uuid(id) {
				reply(w, 400, map[string]string{"error": "Valid location and service selectors required"})
				return
			}
			action := "catalogue.manage"
			if r.Method == "GET" {
				action = "catalogue.read"
			}
			scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, action)
			if err != nil {
				failure(w, err)
				return
			}
			if r.Method == "GET" {
				items, err := store.Services(r.Context(), scope)
				if err != nil {
					failure(w, err)
					return
				}
				reply(w, 200, map[string]any{"items": items, "limit": 1000})
				return
			}
			if strings.HasSuffix(r.URL.Path, "/archive") {
				var input struct {
					Version int `json:"version"`
				}
				if !decode(w, r, &input) {
					return
				}
				if input.Version < 1 {
					reply(w, 422, map[string]any{"errors": map[string]string{"version": "Refresh to obtain the current service version"}})
					return
				}
				service, err := store.ArchiveService(r.Context(), scope, id, input.Version)
				if err != nil {
					serviceFailure(w, err)
					return
				}
				reply(w, 200, service)
				return
			}
			var input struct {
				Name     string  `json:"name"`
				Kind     string  `json:"kind"`
				ParentID *string `json:"parentId"`
				Version  int     `json:"version"`
				// Time to complete, in minutes (null or absent = not set).
				DurationMinutes *int `json:"durationMinutes"`
			}
			if !decode(w, r, &input) {
				return
			}
			fields := map[string]string{}
			if input.DurationMinutes != nil && (*input.DurationMinutes < 1 || *input.DurationMinutes > 10080) {
				fields["durationMinutes"] = "Enter the time to complete in minutes (1 to 10080)"
			}
			if n := utf8.RuneCountInString(strings.TrimSpace(input.Name)); n < 2 || n > 120 {
				fields["name"] = "Enter a service name between 2 and 120 characters"
			}
			if input.Kind != "main" && input.Kind != "addon" {
				fields["kind"] = "Choose main or addon"
			}
			if input.ParentID != nil && !uuid(*input.ParentID) || input.Kind == "addon" && input.ParentID == nil || input.Kind == "main" && input.ParentID != nil {
				fields["parentId"] = "Add-ons require a valid main-service parent; main services have no parent"
			}
			if id != "" && input.Version < 1 {
				fields["version"] = "Refresh to obtain the current service version"
			}
			if len(fields) > 0 {
				reply(w, 422, map[string]any{"errors": fields})
				return
			}
			service, err := store.SaveService(r.Context(), scope, postgres.CatalogService{ID: id, Name: input.Name, Kind: input.Kind, ParentID: input.ParentID, Version: input.Version, DurationMinutes: input.DurationMinutes})
			if err != nil {
				serviceFailure(w, err)
				return
			}
			status := 200
			if id == "" {
				status = 201
			}
			reply(w, status, service)
		})
	}
	mux.HandleFunc("DELETE "+base+"/{service}", func(w http.ResponseWriter, r *http.Request) {
		organization, location, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("service")
		if !uuid(organization) || !uuid(location) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid location and service selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "catalogue.manage")
		if err != nil {
			failure(w, err)
			return
		}
		version := 0
		if vStr := r.URL.Query().Get("version"); vStr != "" {
			if v, err := strconv.Atoi(vStr); err == nil && v > 0 {
				version = v
			}
		}
		res, err := store.DeleteService(r.Context(), scope, id, version)
		if err != nil {
			serviceFailure(w, err)
			return
		}
		reply(w, 200, res)
	})
}
