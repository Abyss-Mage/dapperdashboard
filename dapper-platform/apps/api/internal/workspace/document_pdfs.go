package workspace

import (
	"net/http"
	"strings"

	"dapper.local/platform/internal/postgres"
)

func replyPDF(w http.ResponseWriter, name string, pdf []byte) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

// Invoices and quotations as PDF files. Each route reads the document with the same check as its JSON route, so a PDF is visible to exactly
// the people who could already open that document.
func registerDocumentPDFRoutes(mux *http.ServeMux, store *postgres.Store) {
	locationBase := "/api/v1/organizations/{organization}/locations/{location}"
	scoped := func(w http.ResponseWriter, r *http.Request, action, id string) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return postgres.Scope{}, false
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, action)
		if err != nil {
			failure(w, err)
			return postgres.Scope{}, false
		}
		return scope, true
	}
	sendInvoice := func(w http.ResponseWriter, r *http.Request, inv postgres.Invoice, err error) {
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		name, pdf := store.InvoicePDF(r.Context(), inv)
		replyPDF(w, name, pdf)
	}
	mux.HandleFunc("GET "+locationBase+"/invoices/{invoice}/pdf", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("invoice")
		if scope, ok := scoped(w, r, "invoices.read", id); ok {
			inv, err := store.GetInvoice(r.Context(), scope, id)
			sendInvoice(w, r, inv, err)
		}
	})
	mux.HandleFunc("GET "+locationBase+"/legacy-invoices/{booking}/pdf", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("booking")
		if scope, ok := scoped(w, r, "invoices.read", id); ok {
			inv, err := store.LegacyInvoice(r.Context(), scope, id)
			sendInvoice(w, r, inv, err)
		}
	})
	mux.HandleFunc("GET "+locationBase+"/quotes/{quote}/pdf", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("quote")
		if scope, ok := scoped(w, r, "quotes.read", id); ok {
			quote, err := store.GetQuote(r.Context(), scope, id)
			if err != nil {
				quoteFailure(w, err)
				return
			}
			name, pdf := store.QuotePDF(r.Context(), quote)
			replyPDF(w, name, pdf)
		}
	})
	mux.HandleFunc("GET /api/v1/employee/bookings/{booking}/invoices/{invoice}/pdf", func(w http.ResponseWriter, r *http.Request) {
		booking, id := r.PathValue("booking"), r.PathValue("invoice")
		if !uuid(booking) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		items, err := store.EmployeeBookingInvoices(r.Context(), requestSession(r).principalID, booking)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		for _, inv := range items {
			if inv.ID == id {
				sendInvoice(w, r, inv, nil)
				return
			}
		}
		invoiceFailure(w, postgres.ErrInvoiceNotFound)
	})
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/invoices/{invoice}/pdf", func(w http.ResponseWriter, r *http.Request) {
		customer, id := r.PathValue("customer"), r.PathValue("invoice")
		actor := requestSession(r).principalID
		if booking, legacy := strings.CutPrefix(id, "legacy-"); legacy && uuid(customer) && uuid(booking) {
			inv, err := store.PortalLegacyInvoice(r.Context(), actor, customer, booking)
			sendInvoice(w, r, inv, err)
			return
		}
		if !uuid(customer) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		inv, err := store.PortalInvoice(r.Context(), actor, customer, id)
		sendInvoice(w, r, inv, err)
	})
}
