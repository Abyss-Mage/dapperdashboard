package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

func customScriptFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrCustomScriptInput):
		reply(w, 422, map[string]string{"error": "Give the script a name, choose where it goes, paste its code (under 20,000 characters) and start any page paths with /"})
	case errors.Is(err, postgres.ErrCustomScriptLimit):
		reply(w, 422, map[string]string{"error": "You can keep up to 50 custom scripts. Delete one you no longer use."})
	case errors.Is(err, postgres.ErrCustomScriptNotFound):
		reply(w, 404, map[string]string{"error": "Script not found"})
	default:
		failure(w, err)
	}
}

// registerCustomScriptRoutes: Super Admin and Owner roles manage extra website code; the public site reads the active
// snippets together with the tracking settings.
func registerCustomScriptRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/custom-scripts"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		items, err := store.CustomScripts(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			customScriptFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	save := func(w http.ResponseWriter, r *http.Request, id string) {
		org := r.PathValue("organization")
		if !uuid(org) || (id != "" && !uuid(id)) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		var input postgres.CustomScript
		if !decodeLimited(w, r, &input, 64*1024) {
			return
		}
		input.ID = id
		saved, err := store.SaveCustomScript(r.Context(), requestSession(r).principalID, org, input)
		if err != nil {
			customScriptFailure(w, err)
			return
		}
		reply(w, 200, saved)
	}
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) { save(w, r, "") })
	mux.HandleFunc("PUT "+base+"/{script}", func(w http.ResponseWriter, r *http.Request) { save(w, r, r.PathValue("script")) })
	mux.HandleFunc("DELETE "+base+"/{script}", func(w http.ResponseWriter, r *http.Request) {
		org, id := r.PathValue("organization"), r.PathValue("script")
		if !uuid(org) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		if err := store.DeleteCustomScript(r.Context(), requestSession(r).principalID, org, id); err != nil {
			customScriptFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
}
