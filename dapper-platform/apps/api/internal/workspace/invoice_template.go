package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func registerInvoiceTemplateRoutes(mux *http.ServeMux, store *postgres.Store) {
	templateFailure := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrInvoiceTemplateInput):
			reply(w, 422, map[string]string{"error": "Check the template: titles and labels are required, the colour must be a hex value like #111111, and notes have length limits."})
		case errors.Is(err, postgres.ErrInvoiceTemplateConflict):
			reply(w, 409, map[string]string{"error": "The template was changed by someone else. Reload it before saving."})
		default:
			invoiceFailure(w, err)
		}
	}
	base := "/api/v1/organizations/{organization}/invoice-template"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		out, err := store.InvoicePrintTemplate(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		var input struct {
			Template postgres.InvoicePrintTemplate `json:"template"`
			Version  int                           `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		out, err := store.SaveInvoicePrintTemplate(r.Context(), requestSession(r).principalID, org, input.Template, input.Version)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("GET /api/v1/employee/invoice-template", func(w http.ResponseWriter, r *http.Request) {
		out, err := store.EmployeeInvoicePrintTemplate(r.Context(), requestSession(r).principalID)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
}
