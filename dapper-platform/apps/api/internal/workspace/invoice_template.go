package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"io"
	"net/http"
)

func registerInvoiceTemplateRoutes(mux *http.ServeMux, store *postgres.Store) {
	templateFailure := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrInvoiceTemplateInput):
			reply(w, 422, map[string]string{"error": "Check the template: titles and labels are required, the colour must be a hex value like #111111, notes have length limits, and a custom layout must be HTML and CSS only (no scripts, frames or plug-ins, at most 60,000 characters each)."})
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
		out, err := store.OutletInvoicePrintTemplate(r.Context(), requestSession(r).principalID, org, r.URL.Query().Get("outlets"))
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
		out, err := store.SaveOutletInvoicePrintTemplate(r.Context(), requestSession(r).principalID, org, r.URL.Query().Get("outlets"), input.Template, input.Version)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	// Whether one outlet is brand-owned or a franchise decides which shared invoice template it prints with.
	mux.HandleFunc("PUT /api/v1/organizations/{organization}/locations/{location}/outlet-type", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		var input struct {
			Outlet string `json:"outlet"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.SetOutletType(r.Context(), requestSession(r).principalID, org, loc, input.Outlet); err != nil {
			if errors.Is(err, postgres.ErrOutletType) {
				reply(w, 422, map[string]string{"error": "An outlet is either brand-owned or a franchise."})
				return
			}
			templateFailure(w, err)
			return
		}
		reply(w, 200, map[string]string{"outlet": input.Outlet})
	})
	locBase := "/api/v1/organizations/{organization}/locations/{location}/invoice-template"
	mux.HandleFunc("GET "+locBase, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		out, err := store.LocationInvoicePrintTemplate(r.Context(), requestSession(r).principalID, org, loc)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("PUT "+locBase, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		var input struct {
			Template postgres.InvoicePrintTemplate `json:"template"`
			Version  int                           `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		out, err := store.SaveLocationInvoicePrintTemplate(r.Context(), requestSession(r).principalID, org, loc, input.Template, input.Version)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("DELETE "+locBase, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		if err := store.DeleteLocationInvoicePrintTemplate(r.Context(), requestSession(r).principalID, org, loc); err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	// The quotation template: the same shape and the same rules, stored apart so quotes have their own look and terms.
	quoteBase := "/api/v1/organizations/{organization}/quote-template"
	mux.HandleFunc("GET "+quoteBase, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		out, err := store.QuotePrintTemplate(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("PUT "+quoteBase, func(w http.ResponseWriter, r *http.Request) {
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
		out, err := store.SaveQuotePrintTemplate(r.Context(), requestSession(r).principalID, org, input.Template, input.Version)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	quoteLoc := "/api/v1/organizations/{organization}/locations/{location}/quote-template"
	mux.HandleFunc("GET "+quoteLoc, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		out, err := store.LocationQuotePrintTemplate(r.Context(), requestSession(r).principalID, org, loc)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("PUT "+quoteLoc, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		var input struct {
			Template postgres.InvoicePrintTemplate `json:"template"`
			Version  int                           `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		out, err := store.SaveLocationQuotePrintTemplate(r.Context(), requestSession(r).principalID, org, loc, input.Template, input.Version)
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("DELETE "+quoteLoc, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		if err := store.DeleteLocationQuotePrintTemplate(r.Context(), requestSession(r).principalID, org, loc); err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	logoBase := "/api/v1/organizations/{organization}/locations/{location}/logo"
	sendLogo := func(w http.ResponseWriter, contentType string, data []byte, digest string, found bool) {
		if !found {
			http.NotFound(w, nil)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "private, max-age=86400")
		w.Header().Set("ETag", "\""+digest+"\"")
		_, _ = w.Write(data)
	}
	mux.HandleFunc("GET "+logoBase, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		contentType, data, digest, found, err := store.LocationLogo(r.Context(), requestSession(r).principalID, org, loc)
		if err != nil {
			templateFailure(w, err)
			return
		}
		if !found {
			reply(w, 404, map[string]string{"error": "No logo set"})
			return
		}
		sendLogo(w, contentType, data, digest, true)
	})
	mux.HandleFunc("PUT "+logoBase, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 512*1024+1))
		if err != nil {
			reply(w, 413, map[string]string{"error": "The logo must be under 512 KB"})
			return
		}
		digest, err := store.SaveLocationLogo(r.Context(), requestSession(r).principalID, org, loc, body)
		if errors.Is(err, postgres.ErrLogoInput) {
			reply(w, 422, map[string]string{"error": "The logo must be a PNG, JPEG or WebP image under 512 KB"})
			return
		}
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, map[string]string{"logoUrl": "/api/v1/organizations/" + org + "/locations/" + loc + "/logo?v=" + digest})
	})
	mux.HandleFunc("DELETE "+logoBase, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		if err := store.DeleteLocationLogo(r.Context(), requestSession(r).principalID, org, loc); err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	mux.HandleFunc("GET /api/v1/employee/logo", func(w http.ResponseWriter, r *http.Request) {
		contentType, data, digest, found, err := store.EmployeeLocationLogo(r.Context(), requestSession(r).principalID)
		if err != nil {
			templateFailure(w, err)
			return
		}
		if !found {
			reply(w, 404, map[string]string{"error": "No logo set"})
			return
		}
		sendLogo(w, contentType, data, digest, true)
	})
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/invoice-template", func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("customer")) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		out, err := store.CustomerInvoicePrintTemplate(r.Context(), requestSession(r).principalID, r.PathValue("customer"))
		if err != nil {
			templateFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/logo", func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("customer")) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		contentType, data, digest, found, err := store.CustomerLocationLogo(r.Context(), requestSession(r).principalID, r.PathValue("customer"))
		if err != nil {
			templateFailure(w, err)
			return
		}
		sendLogo(w, contentType, data, digest, found)
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
