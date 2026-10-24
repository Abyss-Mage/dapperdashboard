package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func registerInvoiceIssuanceRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/invoices/{invoice}/issuance-preview", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("invoice")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid invoice selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "invoices.issue")
		if err != nil {
			failure(w, err)
			return
		}
		preview, err := store.InvoiceIssuancePreview(r.Context(), scope, id)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, preview)
	})
	configBase := "/api/v1/organizations/{organization}/invoice-issuers"
	mux.HandleFunc("GET "+configBase, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		config, err := store.InvoiceConfiguration(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, config)
	})
	for _, route := range []string{"POST " + configBase, "PUT " + configBase + "/{issuer}"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			org := r.PathValue("organization")
			id := r.PathValue("issuer")
			if !uuid(org) || id != "" && !uuid(id) {
				reply(w, 400, map[string]string{"error": "Valid organization and issuer selectors required"})
				return
			}
			var input postgres.InvoiceIssuerInput
			if !decode(w, r, &input) {
				return
			}
			if input.ID != "" {
				reply(w, 400, map[string]string{"error": "Issuer identity belongs in the route"})
				return
			}
			input.ID = id
			issuer, err := store.SaveInvoiceIssuer(r.Context(), requestSession(r).principalID, org, input)
			if errors.Is(err, postgres.ErrInvoiceIssuer) {
				reply(w, 422, map[string]string{"error": "Provide actual legal name, address, currency CAD, unique uppercase prefix and explicit starting number for a new issuer; edits require current version and cannot reset numbering"})
				return
			}
			if err != nil {
				invoiceFailure(w, err)
				return
			}
			reply(w, 200, issuer)
		})
	}
	mux.HandleFunc("PUT "+configBase+"/locations/{location}", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid scope selectors required"})
			return
		}
		var input struct {
			IssuerID string `json:"issuerId"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.IssuerID) {
			reply(w, 422, map[string]string{"error": "Select an actual configured issuer"})
			return
		}
		if err := store.BindInvoiceIssuer(r.Context(), requestSession(r).principalID, org, loc, input.IssuerID); err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, map[string]string{"issuerId": input.IssuerID})
	})
	mux.HandleFunc("POST /api/v1/organizations/{organization}/locations/{location}/invoices/{invoice}/issue", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("invoice")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid invoice scope selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "invoices.issue")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.InvoiceIssuanceReview
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.IssuerID) || input.IssuerVersion < 1 || input.InvoiceVersion < 1 {
			reply(w, 422, map[string]string{"error": "Review the configured issuer and invoice; provide their current revisions before issuing"})
			return
		}
		invoice, err := store.IssueInvoice(r.Context(), scope, id, input)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, invoice)
	})
}
