package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func registerCustomerAccountManagementRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/customer-accounts"
	scopeFor := func(w http.ResponseWriter, r *http.Request) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization/location selectors required"})
			return postgres.Scope{}, false
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "customer_accounts.manage")
		if err != nil {
			failure(w, err)
			return postgres.Scope{}, false
		}
		return scope, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		data, err := store.ManagedCustomerAccounts(r.Context(), scope)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		var input struct {
			CustomerID        string `json:"customerId"`
			PrincipalID       string `json:"principalId"`
			Enabled           *bool  `json:"enabled"`
			OwnershipReviewed bool   `json:"ownershipReviewed"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.CustomerID) || !uuid(input.PrincipalID) || input.Enabled == nil {
			reply(w, 422, map[string]string{"error": "Explicit customer, enrolled account and enabled state required"})
			return
		}
		err := store.SetCustomerAccountLink(r.Context(), scope, input.CustomerID, input.PrincipalID, *input.Enabled, input.OwnershipReviewed)
		if errors.Is(err, postgres.ErrCustomerAccountReview) {
			reply(w, 422, map[string]string{"error": "Confirm that you reviewed the account's authorization for this customer record before granting access"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"enabled": *input.Enabled})
	})
}
