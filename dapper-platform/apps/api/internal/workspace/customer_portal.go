package workspace

import (
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
	"errors"
	"net/http"
)

func registerCustomerPortalRoutes(mux *http.ServeMux, store *postgres.Store, sqClient square.Client) {
	mux.HandleFunc("POST /api/v1/portal/accounts/{customer}/vehicles", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		var input struct {
			Description string `json:"description"`
			Category    string `json:"categoryDescription"`
			Identifier  string `json:"identifier"`
		}
		if !decode(w, r, &input) {
			return
		}
		vehicle, err := store.CreateCustomerVehicle(r.Context(), requestSession(r).principalID, customer, postgres.Vehicle{Description: input.Description, CategoryDescription: input.Category, Identifier: input.Identifier})
		if errors.Is(err, postgres.ErrVehicle) {
			reply(w, 422, map[string]string{"error": "Description must contain 2–120 characters; category and identifier at most 60 each"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 201, vehicle)
	})
	mux.HandleFunc("GET /api/v1/portal/accounts", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.CustomerAccounts(r.Context(), requestSession(r).principalID)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/records", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		records, err := store.CustomerRecords(r.Context(), requestSession(r).principalID, customer)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, records)
	})
	mux.HandleFunc("POST /api/v1/portal/accounts/{customer}/invoices/{invoice}/pay", func(w http.ResponseWriter, r *http.Request) {
		customer, invoice := r.PathValue("customer"), r.PathValue("invoice")
		if !uuid(customer) || !uuid(invoice) {
			reply(w, 400, map[string]string{"error": "Valid customer and invoice selectors are required"})
			return
		}
		if sqClient == nil {
			reply(w, 503, map[string]string{"error": "Square payment provider is not configured for this location"})
			return
		}
		var input postgres.CustomerPayInvoiceInput
		if !decodeLimited(w, r, &input, 16*1024) {
			return
		}
		pmt, updatedInv, err := store.CustomerPayInvoice(r.Context(), requestSession(r).principalID, customer, invoice, sqClient, input)
		if err != nil {
			switch {
			case errors.Is(err, postgres.ErrForbidden):
				reply(w, 403, map[string]string{"error": "Access denied to customer account"})
			case errors.Is(err, postgres.ErrInvoiceNotFound):
				reply(w, 404, map[string]string{"error": "Invoice not found"})
			case errors.Is(err, postgres.ErrInvoiceStatusConflict):
				reply(w, 409, map[string]string{"error": "Invoice cannot be paid in its current status"})
			case errors.Is(err, postgres.ErrPaymentOverpay):
				reply(w, 422, map[string]string{"error": "Payment amount exceeds invoice balance due"})
			case errors.Is(err, postgres.ErrPaymentInput):
				reply(w, 422, map[string]string{"error": "Invalid payment input or idempotency key"})
			case errors.Is(err, postgres.ErrPaymentRequest):
				reply(w, 409, map[string]string{"error": "Idempotency key already used for a different payment"})
			case errors.Is(err, postgres.ErrPaymentProvider):
				reply(w, 502, map[string]string{"error": "Payment processor error"})
			default:
				failure(w, err)
			}
			return
		}
		reply(w, 201, map[string]any{
			"payment": pmt,
			"invoice": updatedInv,
		})
	})
}
