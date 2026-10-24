package workspace

import (
	"dapper.local/platform/internal/postgres"
	"net/http"
)

func registerCentralServiceLinkRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("POST /api/v1/organizations/{organization}/locations/{location}/services/from-central", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location required"})
			return
		}
		var input struct {
			CentralServiceID string `json:"centralServiceId"`
			Version          int    `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.CentralServiceID) || input.Version < 1 {
			reply(w, 422, map[string]string{"error": "Choose a current central service revision"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "catalogue.manage")
		if err != nil {
			failure(w, err)
			return
		}
		service, err := store.ImportCentralService(r.Context(), scope, input.CentralServiceID, input.Version)
		if err != nil {
			serviceFailure(w, err)
			return
		}
		reply(w, 201, service)
	})
}
