package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func registerAccessManagementRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/access"
	mux.HandleFunc("PUT "+base+"/organization-grants", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		var input struct {
			PrincipalID string `json:"principalId"`
			Action      string `json:"action"`
			Enabled     *bool  `json:"enabled"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.PrincipalID) || input.Enabled == nil {
			reply(w, 422, map[string]string{"error": "Explicit organization member and enabled state required"})
			return
		}
		err := store.SetOrganizationGrant(r.Context(), requestSession(r).principalID, org, input.PrincipalID, input.Action, *input.Enabled)
		if errors.Is(err, postgres.ErrLastAccessManager) {
			reply(w, 409, map[string]string{"error": "At least one active organization access manager must remain"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"enabled": *input.Enabled})
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		access, err := store.ManagedAccess(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, access)
	})
	mux.HandleFunc("GET "+base+"/people", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		people, err := store.PeopleRoster(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"people": people})
	})
	mux.HandleFunc("PUT "+base+"/location-grants", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		var input struct {
			PrincipalID string `json:"principalId"`
			LocationID  string `json:"locationId"`
			Action      string `json:"action"`
			Enabled     *bool  `json:"enabled"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.PrincipalID) || !uuid(input.LocationID) || input.Enabled == nil {
			reply(w, 422, map[string]string{"error": "Explicit member, location and enabled state required"})
			return
		}
		if err := store.SetLocationGrant(r.Context(), requestSession(r).principalID, org, input.PrincipalID, input.LocationID, input.Action, *input.Enabled); err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"enabled": *input.Enabled})
	})
}
