package workspace

import (
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

func invoiceFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, square.ErrSquareDeclined) {
		reply(w, 422, map[string]string{"error": "Payment card was declined by the card issuer"})
		return
	}
	if errors.Is(err, square.ErrSquarePaymentFailed) {
		reply(w, 502, map[string]string{"error": "Payment processing failed upstream with Square"})
		return
	}
	if errors.Is(err, postgres.ErrInvoiceVersion) {
		reply(w, 409, map[string]string{"error": "Invoice changed since review. Reload the invoice and review the current draft before saving or issuing."})
		return
	}
	if errors.Is(err, postgres.ErrInvoiceIssuer) {
		reply(w, 409, map[string]string{"error": "An explicit invoice issuer, location binding and available numbering sequence are required before issuance"})
		return
	}
	if errors.Is(err, postgres.ErrInvoiceIssuerConflict) {
		reply(w, 409, map[string]string{"error": "Issuer version, location binding or invoice number conflicts; refresh before retrying"})
		return
	}
	if errors.Is(err, postgres.ErrPaymentRequest) {
		reply(w, 409, map[string]string{"error": "Payment request key is already used with different details"})
		return
	}
	if errors.Is(err, postgres.ErrPaymentProvider) {
		reply(w, 409, map[string]string{"error": "Square payment must be confirmed by the connected provider; manual references cannot settle an invoice"})
		return
	}
	if errors.Is(err, postgres.ErrInvoiceNotFound) {
		reply(w, 404, map[string]string{"error": "Invoice not found in this scope"})
		return
	}
	if errors.Is(err, postgres.ErrInvoiceStatusConflict) {
		reply(w, 409, map[string]string{"error": "Invoice status does not allow this operation"})
		return
	}
	if errors.Is(err, postgres.ErrPaymentOverpay) {
		reply(w, 422, map[string]any{"errors": map[string]string{"amountCents": "Payment amount exceeds balance due"}})
		return
	}
	if errors.Is(err, postgres.ErrInvoiceInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"invoice": "Provide valid customer and line items"}})
		return
	}
	if errors.Is(err, postgres.ErrPaymentInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"payment": "Provide valid cash amount, no provider reference, and a stable 8–128 character request key"}})
		return
	}
	failure(w, err)
}

func registerInvoiceRoutes(mux *http.ServeMux, store *postgres.Store, sqClient square.Client) {
	base := "/api/v1/organizations/{organization}/locations/{location}/invoices"
	mux.HandleFunc("PUT "+base+"/{invoiceId}/draft", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("invoiceId")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid invoice selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "invoices.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.UpdateInvoiceDraftInput
		if !decode(w, r, &input) {
			return
		}
		invoice, err := store.UpdateInvoiceDraft(r.Context(), scope, id, input)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, invoice)
	})
	mux.HandleFunc("GET "+base+"/{invoiceId}/draft-history", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("invoiceId")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid invoice selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "invoices.read")
		if err != nil {
			failure(w, err)
			return
		}
		items, err := store.InvoiceDraftHistory(r.Context(), scope, id)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})

	// List Invoices
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "invoices.read")
		if err != nil {
			failure(w, err)
			return
		}
		items, err := store.ListInvoices(r.Context(), scope)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})

	// Create Invoice
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "invoices.create")
		if err != nil {
			failure(w, err)
			return
		}

		var input struct {
			CustomerID    string  `json:"customerId"`
			BookingID     *string `json:"bookingId"`
			JobID         *string `json:"jobId"`
			InvoiceNumber string  `json:"invoiceNumber"`
			TaxCents      int64   `json:"taxCents"`
			Notes         string  `json:"notes"`
			Items         []struct {
				Description    string `json:"description"`
				Quantity       int    `json:"quantity"`
				UnitPriceCents int64  `json:"unitPriceCents"`
			} `json:"items"`
		}
		if !decode(w, r, &input) {
			return
		}

		if !uuid(input.CustomerID) {
			reply(w, 422, map[string]any{"errors": map[string]string{"customerId": "Valid customer required"}})
			return
		}
		if input.BookingID != nil && !uuid(*input.BookingID) {
			reply(w, 422, map[string]any{"errors": map[string]string{"bookingId": "Valid booking ID required"}})
			return
		}
		if input.JobID != nil && !uuid(*input.JobID) {
			reply(w, 422, map[string]any{"errors": map[string]string{"jobId": "Valid job ID required"}})
			return
		}
		if len(input.Items) == 0 {
			reply(w, 422, map[string]any{"errors": map[string]string{"items": "At least one line item is required"}})
			return
		}
		if input.TaxCents < 0 {
			reply(w, 422, map[string]any{"errors": map[string]string{"taxCents": "Tax cannot be negative"}})
			return
		}

		items := make([]postgres.CreateInvoiceItemInput, 0, len(input.Items))
		for _, it := range input.Items {
			desc := strings.TrimSpace(it.Description)
			if utf8.RuneCountInString(desc) < 2 || utf8.RuneCountInString(desc) > 255 {
				reply(w, 422, map[string]any{"errors": map[string]string{"items": "Line item description must be between 2 and 255 characters"}})
				return
			}
			if it.Quantity <= 0 || it.UnitPriceCents < 0 {
				reply(w, 422, map[string]any{"errors": map[string]string{"items": "Line item quantity must be positive and price non-negative"}})
				return
			}
			items = append(items, postgres.CreateInvoiceItemInput{
				Description:    desc,
				Quantity:       it.Quantity,
				UnitPriceCents: it.UnitPriceCents,
			})
		}

		inv, err := store.CreateInvoice(r.Context(), scope, postgres.CreateInvoiceInput{
			CustomerID:    input.CustomerID,
			BookingID:     input.BookingID,
			JobID:         input.JobID,
			InvoiceNumber: input.InvoiceNumber,
			TaxCents:      input.TaxCents,
			Notes:         input.Notes,
			Items:         items,
		})
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 201, inv)
	})

	// Get Invoice by ID
	mux.HandleFunc("GET "+base+"/{invoiceId}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, invID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("invoiceId")
		if !uuid(org) || !uuid(loc) || !uuid(invID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "invoices.read")
		if err != nil {
			failure(w, err)
			return
		}
		inv, err := store.GetInvoice(r.Context(), scope, invID)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, inv)
	})

	// Record Payment
	mux.HandleFunc("POST "+base+"/{invoiceId}/payments", func(w http.ResponseWriter, r *http.Request) {
		org, loc, invID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("invoiceId")
		if !uuid(org) || !uuid(loc) || !uuid(invID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "payments.record")
		if err != nil {
			failure(w, err)
			return
		}

		var input struct {
			AmountCents       int64  `json:"amountCents"`
			Method            string `json:"method"`
			SourceID          string `json:"sourceId"`
			SquareLocationID  string `json:"squareLocationId"`
			ProviderReference string `json:"providerReference"`
			IdempotencyKey    string `json:"idempotencyKey"`
			Notes             string `json:"notes"`
		}
		if !decode(w, r, &input) {
			return
		}

		if input.AmountCents <= 0 {
			reply(w, 422, map[string]any{"errors": map[string]string{"amountCents": "Payment amount must be greater than zero"}})
			return
		}
		method := strings.TrimSpace(input.Method)
		switch method {
		case "cash":
			pmt, updatedInv, err := store.RecordPayment(r.Context(), scope, postgres.RecordPaymentInput{
				InvoiceID:         invID,
				AmountCents:       input.AmountCents,
				Method:            method,
				ProviderReference: input.ProviderReference,
				IdempotencyKey:    input.IdempotencyKey,
				Notes:             input.Notes,
			})
			if err != nil {
				invoiceFailure(w, err)
				return
			}
			reply(w, 201, map[string]any{
				"payment": pmt,
				"invoice": updatedInv,
			})
		case "square_card", "square_terminal":
			if strings.TrimSpace(input.SourceID) == "" {
				invoiceFailure(w, postgres.ErrPaymentProvider)
				return
			}
			if sqClient == nil {
				reply(w, 503, map[string]string{"error": "Square payment provider is not configured for this location"})
				return
			}
			pmt, updatedInv, err := store.RecordSquarePayment(r.Context(), scope, sqClient, postgres.RecordSquarePaymentInput{
				InvoiceID:        invID,
				AmountCents:      input.AmountCents,
				SourceID:         input.SourceID,
				SquareLocationID: input.SquareLocationID,
				IdempotencyKey:   input.IdempotencyKey,
				Method:           method,
				Notes:            input.Notes,
			})
			if err != nil {
				invoiceFailure(w, err)
				return
			}
			reply(w, 201, map[string]any{
				"payment": pmt,
				"invoice": updatedInv,
			})
		default:
			reply(w, 422, map[string]any{"errors": map[string]string{"method": "Use cash recording or Square payment"}})
			return
		}
	})

	// Void Invoice
	mux.HandleFunc("POST "+base+"/{invoiceId}/void", func(w http.ResponseWriter, r *http.Request) {
		org, loc, invID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("invoiceId")
		if !uuid(org) || !uuid(loc) || !uuid(invID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "invoices.manage")
		if err != nil {
			failure(w, err)
			return
		}

		var input struct {
			Notes string `json:"notes"`
		}
		if !decode(w, r, &input) {
			return
		}

		inv, err := store.VoidInvoice(r.Context(), scope, invID, input.Notes)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, inv)
	})
}
