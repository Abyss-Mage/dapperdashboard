package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

// Per-franchise tax rates and settings. Read: invoices.read. Change: invoices.configure (organization-level authority).
func registerFranchiseTaxRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}"
	scope := func(w http.ResponseWriter, r *http.Request) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid franchise selectors required"})
			return postgres.Scope{}, false
		}
		return postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc}, true
	}
	fail := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrTaxInput):
			reply(w, 422, map[string]any{"errors": map[string]string{"tax": "Enter a name (2-60 characters) and a rate between 0% and 50%"}})
		case errors.Is(err, postgres.ErrTaxConflict):
			reply(w, 409, map[string]string{"error": "That tax already exists, was not found, or was changed by someone else. Refresh and try again."})
		default:
			failure(w, err)
		}
	}
	type rateInput struct {
		Name    string `json:"name"`
		RateBP  int    `json:"rateBasisPoints"`
		Active  *bool  `json:"active"`
		Version int    `json:"version"`
	}
	mux.HandleFunc("GET "+base+"/taxes", func(w http.ResponseWriter, r *http.Request) {
		sc, ok := scope(w, r)
		if !ok {
			return
		}
		out, err := store.FranchiseTaxes(r.Context(), sc)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("POST "+base+"/taxes", func(w http.ResponseWriter, r *http.Request) {
		sc, ok := scope(w, r)
		if !ok {
			return
		}
		var in rateInput
		if !decode(w, r, &in) {
			return
		}
		out, err := store.CreateTaxRate(r.Context(), sc, in.Name, in.RateBP)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 201, out)
	})
	mux.HandleFunc("PUT "+base+"/taxes/{rate}", func(w http.ResponseWriter, r *http.Request) {
		sc, ok := scope(w, r)
		if !ok {
			return
		}
		if !uuid(r.PathValue("rate")) {
			reply(w, 400, map[string]string{"error": "Valid tax selector required"})
			return
		}
		var in rateInput
		if !decode(w, r, &in) {
			return
		}
		active := true
		if in.Active != nil {
			active = *in.Active
		}
		out, err := store.UpdateTaxRate(r.Context(), sc, r.PathValue("rate"), in.Name, in.RateBP, active, in.Version)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("DELETE "+base+"/taxes/{rate}", func(w http.ResponseWriter, r *http.Request) {
		sc, ok := scope(w, r)
		if !ok {
			return
		}
		if !uuid(r.PathValue("rate")) {
			reply(w, 400, map[string]string{"error": "Valid tax selector required"})
			return
		}
		if err := store.DeleteTaxRate(r.Context(), sc, r.PathValue("rate")); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"deleted": true})
	})
	mux.HandleFunc("PUT "+base+"/tax-settings", func(w http.ResponseWriter, r *http.Request) {
		sc, ok := scope(w, r)
		if !ok {
			return
		}
		var in struct {
			PricesIncludeTax bool   `json:"pricesIncludeTax"`
			Rounding         string `json:"rounding"`
			Version          int    `json:"version"`
		}
		if !decode(w, r, &in) {
			return
		}
		out, err := store.SaveTaxSettings(r.Context(), sc, in.PricesIncludeTax, in.Rounding, in.Version)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
}
