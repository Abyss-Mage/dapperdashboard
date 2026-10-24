package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

// Invoices from the employee workspace: list, prepare from a booking, create a draft (taxes added by the server) and issue.
func registerEmployeeInvoiceRoutes(mux *http.ServeMux, store *postgres.Store) {
	fail := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrInvoiceInput):
			reply(w, 422, map[string]any{"errors": map[string]string{"invoice": "Add at least one line with a description, a quantity of 1 or more and a price"}})
		case errors.Is(err, postgres.ErrTaxInclusiveUnsupported):
			reply(w, 409, map[string]string{"error": "This franchise's taxes are set to be included in prices, which invoices do not support yet. Ask an administrator to change it in Master > Locations & Setup > Taxes."})
		case errors.Is(err, postgres.ErrBookingNotFound):
			reply(w, 404, map[string]string{"error": "That booking was not found at your franchise."})
		case errors.Is(err, postgres.ErrInvoiceIssuer):
			reply(w, 409, map[string]string{"error": "Invoices cannot be issued until an administrator sets up the invoice issuer for this franchise (Master > Finance & Invoices)."})
		default:
			invoiceFailure(w, err)
		}
	}
	mux.HandleFunc("GET /api/v1/employee/invoices", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.EmployeeInvoices(r.Context(), requestSession(r).principalID)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})
	mux.HandleFunc("GET /api/v1/employee/bookings/{booking}/invoice-draft", func(w http.ResponseWriter, r *http.Request) {
		b := r.PathValue("booking")
		if !uuid(b) {
			reply(w, 400, map[string]string{"error": "Valid booking required"})
			return
		}
		out, err := store.EmployeeInvoiceDraft(r.Context(), requestSession(r).principalID, b)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("POST /api/v1/employee/invoices", func(w http.ResponseWriter, r *http.Request) {
		var input postgres.EmployeeInvoiceInput
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.BookingID) {
			reply(w, 400, map[string]string{"error": "Valid booking required"})
			return
		}
		inv, assumed, err := store.CreateEmployeeInvoice(r.Context(), requestSession(r).principalID, input)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 201, map[string]any{"invoice": inv, "roundingAssumed": assumed})
	})
	mux.HandleFunc("POST /api/v1/employee/invoices/{invoice}/issue", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("invoice")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid invoice required"})
			return
		}
		inv, err := store.EmployeeIssueInvoice(r.Context(), requestSession(r).principalID, id)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, inv)
	})
}
