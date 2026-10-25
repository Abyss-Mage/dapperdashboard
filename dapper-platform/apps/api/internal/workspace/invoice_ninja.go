package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

// Invoice Ninja for dealership (B2B) orders: connection settings (invoices.configure) and sending one invoice (invoices.manage).
func registerInvoiceNinjaRoutes(mux *http.ServeMux, store *postgres.Store) {
	fail := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrInvoiceNinjaInput):
			reply(w, 422, map[string]string{"error": "Enter the https address of your Invoice Ninja and an API token."})
		case errors.Is(err, postgres.ErrInvoiceNinjaRejected):
			reply(w, 502, map[string]string{"error": "Invoice Ninja did not accept the request. Check the address and the API token."})
		case errors.Is(err, postgres.ErrInvoiceNinjaNotSet):
			reply(w, 409, map[string]string{"error": "Connect Invoice Ninja first (Dealerships → Invoice Ninja)."})
		case errors.Is(err, postgres.ErrInvoiceNinjaNotDealership):
			reply(w, 422, map[string]string{"error": "Only issued invoices of a dealership customer can be sent to Invoice Ninja."})
		case errors.Is(err, postgres.ErrInvoiceNinjaAlreadySent):
			reply(w, 409, map[string]string{"error": "This invoice was already sent to Invoice Ninja."})
		case errors.Is(err, postgres.ErrInvoiceTemplateConflict):
			reply(w, 409, map[string]string{"error": "The settings were changed by someone else. Reload before saving."})
		default:
			failure(w, err)
		}
	}
	org := "/api/v1/organizations/{organization}/invoice-ninja"
	mux.HandleFunc("GET "+org, func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("organization")) {
			reply(w, 400, map[string]string{"error": "Valid organization required"})
			return
		}
		out, err := store.InvoiceNinjaSettings(r.Context(), requestSession(r).principalID, r.PathValue("organization"))
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("PUT "+org, func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("organization")) {
			reply(w, 400, map[string]string{"error": "Valid organization required"})
			return
		}
		var input struct {
			BaseURL string `json:"baseUrl"`
			Token   string `json:"token"`
			Version int    `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		out, err := store.SaveInvoiceNinjaSettings(r.Context(), requestSession(r).principalID, r.PathValue("organization"), input.BaseURL, input.Token, input.Version)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("DELETE "+org, func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("organization")) {
			reply(w, 400, map[string]string{"error": "Valid organization required"})
			return
		}
		if err := store.DisconnectInvoiceNinja(r.Context(), requestSession(r).principalID, r.PathValue("organization")); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(204)
	})
	loc := "/api/v1/organizations/{organization}/locations/{location}"
	scopeFor := func(w http.ResponseWriter, r *http.Request, action string) (postgres.Scope, bool) {
		o, l := r.PathValue("organization"), r.PathValue("location")
		if !uuid(o) || !uuid(l) {
			reply(w, 400, map[string]string{"error": "Valid franchise selectors required"})
			return postgres.Scope{}, false
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, o, l, action)
		if err != nil {
			failure(w, err)
			return postgres.Scope{}, false
		}
		return scope, true
	}
	mux.HandleFunc("GET "+loc+"/invoice-ninja/exports", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.read")
		if !ok {
			return
		}
		items, err := store.InvoiceNinjaExports(r.Context(), scope)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+loc+"/invoices/{invoice}/invoice-ninja", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "invoices.manage")
		if !ok {
			return
		}
		if !uuid(r.PathValue("invoice")) {
			reply(w, 400, map[string]string{"error": "Valid invoice required"})
			return
		}
		out, err := store.SendInvoiceToNinja(r.Context(), scope, r.PathValue("invoice"))
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
}
