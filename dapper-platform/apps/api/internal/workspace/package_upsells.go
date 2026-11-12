package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

// registerPackageUpsellRoutes: the organization's upgrade offers for the booking flow (catalogue rights; the website reads them with its content).
func registerPackageUpsellRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/package-upsells"
	orgOf := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return "", false
		}
		return org, true
	}
	fail := func(w http.ResponseWriter, err error) {
		if errors.Is(err, postgres.ErrUpsellInput) {
			reply(w, 422, map[string]string{"error": "Choose two different packages. The pitch can be up to 200 characters, with up to 6 points of 120 characters each."})
			return
		}
		failure(w, err)
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org, ok := orgOf(w, r)
		if !ok {
			return
		}
		items, err := store.PackageUpsells(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		org, ok := orgOf(w, r)
		if !ok {
			return
		}
		var input postgres.PackageUpsell
		if !decode(w, r, &input) {
			return
		}
		saved, err := store.SavePackageUpsell(r.Context(), requestSession(r).principalID, org, input)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, saved)
	})
	mux.HandleFunc("DELETE "+base, func(w http.ResponseWriter, r *http.Request) {
		org, ok := orgOf(w, r)
		if !ok {
			return
		}
		if err := store.DeletePackageUpsell(r.Context(), requestSession(r).principalID, org, r.URL.Query().Get("fromPack")); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
}
