package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
	"errors"
	"io"
	"net/http"
	"strings"
)

// portalSquareClient is the Square account saved for the signed-in customer's location, and that account's location id.
func portalSquareClient(ctx context.Context, store *postgres.Store, principal, customer string) (square.Client, string, error) {
	scope, err := store.CustomerPaymentScope(ctx, principal, customer)
	if err != nil {
		return nil, "", err
	}
	resolve := store.SquareCredentialResolver()
	if resolve == nil {
		return nil, "", postgres.ErrSquareBinding
	}
	binding, err := store.CurrentSquareMerchantBinding(ctx, scope, "payments.record")
	if err != nil {
		return nil, "", err
	}
	client, err := resolve(ctx, binding.Environment, binding.CredentialReference)
	if err != nil || client == nil {
		return nil, "", postgres.ErrSquareBinding
	}
	if err = client.VerifyLocation(ctx, square.LocationBinding{Environment: binding.Environment, MerchantID: binding.MerchantID, LocationID: binding.SquareLocationID, Currency: "CAD"}); err != nil {
		return nil, "", postgres.ErrPaymentProvider
	}
	return client, binding.SquareLocationID, nil
}

func registerCustomerPortalRoutes(mux *http.ServeMux, store *postgres.Store) {
	// Only what the card form needs: public identifiers, never credentials.
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/square", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		scope, err := store.CustomerPaymentScope(r.Context(), requestSession(r).principalID, customer)
		if err != nil {
			failure(w, err)
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
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, map[string]any{"configured": true, "credentialsConfigured": store.SquareCredentialResolver() != nil, "applicationId": store.SquareApplicationIDFor(r.Context(), scope, b.Environment), "binding": map[string]any{"environment": b.Environment, "squareLocationId": b.SquareLocationID}})
	})
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
	// Saved addresses and the profile picture of the customer's own record.
	mux.HandleFunc("PUT /api/v1/portal/accounts/{customer}/addresses", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		var input postgres.PortalAddress
		if !decode(w, r, &input) {
			return
		}
		if input.ID != "" && !uuid(input.ID) {
			reply(w, 400, map[string]string{"error": "Valid address selector required"})
			return
		}
		saved, err := store.SavePortalAddress(r.Context(), requestSession(r).principalID, customer, input)
		switch {
		case errors.Is(err, postgres.ErrPortalAddress):
			reply(w, 422, map[string]string{"error": "Give the address a name (up to 40 characters) and enter the full address"})
		case errors.Is(err, postgres.ErrPortalAddressLimit):
			reply(w, 409, map[string]string{"error": "You can save up to 10 addresses. Remove one first"})
		case errors.Is(err, postgres.ErrCustomerNotFound):
			reply(w, 404, map[string]string{"error": "Address not found"})
		case err != nil:
			failure(w, err)
		default:
			reply(w, 200, saved)
		}
	})
	mux.HandleFunc("DELETE /api/v1/portal/accounts/{customer}/addresses/{address}", func(w http.ResponseWriter, r *http.Request) {
		customer, address := r.PathValue("customer"), r.PathValue("address")
		if !uuid(customer) || !uuid(address) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		err := store.DeletePortalAddress(r.Context(), requestSession(r).principalID, customer, address)
		switch {
		case errors.Is(err, postgres.ErrCustomerNotFound):
			reply(w, 404, map[string]string{"error": "Address not found"})
		case err != nil:
			failure(w, err)
		default:
			reply(w, 200, map[string]bool{"removed": true})
		}
	})
	mux.HandleFunc("PUT /api/v1/portal/accounts/{customer}/avatar", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 3*1024*1024)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			reply(w, 413, map[string]string{"error": "Upload a JPEG or PNG picture of 2 MB or less"})
			return
		}
		switch err = store.SetPortalAvatar(r.Context(), requestSession(r).principalID, customer, data); {
		case errors.Is(err, postgres.ErrPortalAvatar):
			reply(w, 422, map[string]string{"error": "Upload a JPEG or PNG picture of 2 MB or less"})
		case err != nil:
			failure(w, err)
		default:
			reply(w, 200, map[string]bool{"saved": true})
		}
	})
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/avatar", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		data, mime, err := store.PortalAvatar(r.Context(), requestSession(r).principalID, customer)
		switch {
		case errors.Is(err, postgres.ErrCustomerNotFound):
			reply(w, 404, map[string]string{"error": "No picture"})
		case err != nil:
			failure(w, err)
		default:
			w.Header().Set("Cache-Control", "private, no-cache")
			w.Header().Set("Content-Type", mime)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Write(data)
		}
	})
	mux.HandleFunc("DELETE /api/v1/portal/accounts/{customer}/avatar", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		if err := store.DeletePortalAvatar(r.Context(), requestSession(r).principalID, customer); err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	mux.HandleFunc("GET /api/v1/portal/accounts", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.CustomerAccounts(r.Context(), requestSession(r).principalID)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})
	mux.HandleFunc("PUT /api/v1/portal/accounts/{customer}/details", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		var input struct {
			Name    string `json:"name"`
			Phone   string `json:"phone"`
			Address string `json:"address"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.UpdatePortalDetails(r.Context(), requestSession(r).principalID, customer, input.Name, input.Phone, input.Address); err != nil {
			if errors.Is(err, postgres.ErrCustomerInput) {
				reply(w, 422, map[string]string{"error": "Enter your name (2 to 100 characters), a phone number and an address under 300 characters"})
				return
			}
			if errors.Is(err, postgres.ErrCustomerDuplicate) {
				reply(w, 409, map[string]string{"error": "Another customer already uses that phone number"})
				return
			}
			failure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/invoices/{invoice}", func(w http.ResponseWriter, r *http.Request) {
		customer, invoice := r.PathValue("customer"), r.PathValue("invoice")
		if booking, legacy := strings.CutPrefix(invoice, "legacy-"); legacy && uuid(customer) && uuid(booking) {
			inv, err := store.PortalLegacyInvoice(r.Context(), requestSession(r).principalID, customer, booking)
			if err != nil {
				invoiceFailure(w, err)
				return
			}
			reply(w, 200, inv)
			return
		}
		if !uuid(customer) || !uuid(invoice) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		inv, err := store.PortalInvoice(r.Context(), requestSession(r).principalID, customer, invoice)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, inv)
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
		var input struct {
			SourceID       string `json:"sourceId"`
			AmountCents    int64  `json:"amountCents"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if !decodeLimited(w, r, &input, 16*1024) {
			return
		}
		waitOnSquare(w)
		principal := requestSession(r).principalID
		// The customer proves the invoice is theirs; the charge then goes to the Square account saved for the invoice's own location.
		scope, err := store.CustomerInvoicePaymentScope(r.Context(), principal, customer, invoice)
		if err == nil {
			if store.SquareCredentialResolver() == nil {
				reply(w, 503, map[string]string{"error": "Square payment provider is not configured for this location"})
				return
			}
			if strings.TrimSpace(input.SourceID) == "" {
				err = postgres.ErrPaymentInput
			} else {
				err = takeSquareDeposit(r, store, scope, invoice, "portal-invoice-"+input.IdempotencyKey, input.SourceID, input.AmountCents)
			}
		}
		if errors.Is(err, postgres.ErrSquareBinding) {
			reply(w, 503, map[string]string{"error": "Square payment provider is not configured for this location"})
			return
		}
		var updatedInv postgres.Invoice
		if err == nil {
			updatedInv, err = store.PortalInvoice(r.Context(), principal, customer, invoice)
		}
		if err != nil {
			switch {
			case errors.Is(err, postgres.ErrPaymentDeclined):
				reply(w, 402, map[string]string{"error": "Your card was declined. Please check the details or try another card."})
			case errors.Is(err, postgres.ErrSquareIntentState):
				reply(w, 409, map[string]string{"error": "This payment is already being processed"})
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
			"payment": map[string]any{"amountCents": input.AmountCents},
			"invoice": updatedInv,
		})
	})
}
