package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

// registerEmployeeSquareRoutes lets an assigned employee take a card payment for an invoice on a booking
// at their own dealership. Scope, merchant and credentials are all server-derived; the browser sends only
// the Square single-use token, the invoice, the amount and an idempotency key.
func registerEmployeeSquareRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/employee/square", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Query()) != 0 {
			reply(w, 400, map[string]string{"error": "Employee scope is assigned by your administrator"})
			return
		}
		scope, err := store.EmployeeSquareScope(r.Context(), requestSession(r).principalID)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		b, err := store.CurrentSquareMerchantBinding(r.Context(), scope, "payments.record")
		if errors.Is(err, postgres.ErrSquareBinding) {
			reply(w, 200, map[string]any{"configured": false})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		// Employees need only what the card form requires; the merchant ID stays with administrators.
		reply(w, 200, map[string]any{"configured": true, "credentialsConfigured": store.SquareCredentialResolver() != nil, "applicationId": store.SquareApplicationIDFor(r.Context(), scope, b.Environment), "binding": map[string]any{"version": b.Version, "environment": b.Environment, "squareLocationId": b.SquareLocationID}})
	})
	mux.HandleFunc("POST /api/v1/employee/bookings/{booking}/square-reader", func(w http.ResponseWriter, r *http.Request) {
		booking := r.PathValue("booking")
		var input struct {
			InvoiceID   string `json:"invoiceId"`
			RequestKey  string `json:"requestKey"`
			AmountCents int64  `json:"amountCents"`
		}
		if !uuid(booking) || !decode(w, r, &input) || !uuid(input.InvoiceID) {
			reply(w, 422, map[string]string{"error": "Choose a reviewed invoice"})
			return
		}
		scope, err := store.EmployeeCardInvoiceScope(r.Context(), requestSession(r).principalID, booking, input.InvoiceID)
		if err != nil {
			if errors.Is(err, postgres.ErrBookingNotFound) || errors.Is(err, postgres.ErrInvoiceNotFound) {
				reply(w, 404, map[string]string{"error": "Invoice not found for this booking"})
				return
			}
			employeeFailure(w, err)
			return
		}
		prepareSquareReader(w, r, store, scope, input.InvoiceID, input.RequestKey, input.AmountCents)
	})
	mux.HandleFunc("POST /api/v1/employee/bookings/{booking}/square-reader/{intent}/complete", func(w http.ResponseWriter, r *http.Request) {
		booking, intent := r.PathValue("booking"), r.PathValue("intent")
		var input struct {
			InvoiceID string `json:"invoiceId"`
			PaymentID string `json:"paymentId"`
		}
		if !uuid(booking) || !uuid(intent) || !decode(w, r, &input) || !uuid(input.InvoiceID) {
			reply(w, 422, map[string]string{"error": "Choose a reviewed invoice"})
			return
		}
		scope, err := store.EmployeeCardInvoiceScope(r.Context(), requestSession(r).principalID, booking, input.InvoiceID)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		completeSquareReader(w, r, store, scope, intent, input.PaymentID)
	})
	mux.HandleFunc("POST /api/v1/employee/bookings/{booking}/square-payments", func(w http.ResponseWriter, r *http.Request) {
		booking := r.PathValue("booking")
		if !uuid(booking) {
			reply(w, 400, map[string]string{"error": "Valid booking required"})
			return
		}
		var input struct {
			InvoiceID   string `json:"invoiceId"`
			RequestKey  string `json:"requestKey"`
			SourceID    string `json:"sourceId"`
			AmountCents int64  `json:"amountCents"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.InvoiceID) {
			reply(w, 422, map[string]string{"error": "Choose a reviewed invoice"})
			return
		}
		scope, err := store.EmployeeCardInvoiceScope(r.Context(), requestSession(r).principalID, booking, input.InvoiceID)
		if err != nil {
			if errors.Is(err, postgres.ErrBookingNotFound) || errors.Is(err, postgres.ErrInvoiceNotFound) {
				reply(w, 404, map[string]string{"error": "Invoice not found for this booking"})
				return
			}
			employeeFailure(w, err)
			return
		}
		chargeSquareInvoice(w, r, store, scope, input.InvoiceID, input.RequestKey, input.SourceID, input.AmountCents)
	})
}
