package workspace

import (
	"strings"
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
)

// registerSquareIntentRoutes exposes the durable, merchant-bound card payment flow. Card data never
// reaches this server: the browser/device sends a Square single-use source token (sourceId). The
// merchant, location, currency and credentials always come from the saved binding.
func registerSquareIntentRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}"
	selectors := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return "", "", false
		}
		return org, loc, true
	}
	mux.HandleFunc("GET "+base+"/square/binding", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "payments.read")
		if err != nil {
			failure(w, err)
			return
		}
		b, err := store.CurrentSquareMerchantBinding(r.Context(), scope, "payments.read")
		if errors.Is(err, postgres.ErrSquareBinding) {
			reply(w, 200, map[string]any{"configured": false})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"configured": true, "credentialsConfigured": store.SquareCredentialResolver() != nil, "applicationId": store.SquareApplicationID(b.Environment), "binding": map[string]any{"version": b.Version, "environment": b.Environment, "merchantId": b.MerchantID, "squareLocationId": b.SquareLocationID, "reviewedAt": b.ReviewedAt}})
	})
	mux.HandleFunc("PUT "+base+"/square/binding", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		var input struct {
			Environment         string `json:"environment"`
			MerchantID          string `json:"merchantId"`
			SquareLocationID    string `json:"squareLocationId"`
			CredentialReference string `json:"credentialReference"`
			ExpectedVersion     int    `json:"expectedVersion"`
		}
		if !decode(w, r, &input) {
			return
		}
		if strings.HasPrefix(input.CredentialReference, "oauth:") && input.CredentialReference != "oauth:"+loc {
			reply(w, 422, map[string]string{"error": "A franchise can only use its own connected Square account"})
			return
		}
		resolve := store.SquareCredentialResolver()
		if resolve == nil {
			reply(w, 503, map[string]string{"error": "No Square credentials are configured on the server"})
			return
		}
		client, err := resolve(r.Context(), input.Environment, input.CredentialReference)
		if err != nil || client == nil {
			reply(w, 422, map[string]string{"error": "That credential reference is not configured for this environment"})
			return
		}
		// The saved binding must be the one the provider itself confirms for this credential.
		if err = client.VerifyLocation(r.Context(), square.LocationBinding{Environment: input.Environment, MerchantID: input.MerchantID, LocationID: input.SquareLocationID, Currency: "CAD"}); err != nil {
			reply(w, 422, map[string]string{"error": "Square did not confirm this merchant and location for a CAD card-processing account"})
			return
		}
		saved, err := store.SaveSquareMerchantBinding(r.Context(), requestSession(r).principalID, org, loc, input.ExpectedVersion, postgres.SquareMerchantBinding{Environment: input.Environment, MerchantID: input.MerchantID, SquareLocationID: input.SquareLocationID, CredentialReference: input.CredentialReference})
		if err != nil {
			if errors.Is(err, postgres.ErrPaymentRequest) {
				reply(w, 409, map[string]string{"error": "The binding changed. Refresh and review before saving again"})
				return
			}
			if errors.Is(err, postgres.ErrPaymentInput) {
				reply(w, 422, map[string]string{"error": "Environment, merchant, location and credential reference are required"})
				return
			}
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"version": saved.Version, "environment": saved.Environment, "merchantId": saved.MerchantID, "squareLocationId": saved.SquareLocationID})
	})
	mux.HandleFunc("POST "+base+"/invoices/{invoice}/square-payments", func(w http.ResponseWriter, r *http.Request) {
		org, loc, ok := selectors(w, r)
		if !ok {
			return
		}
		invoice := r.PathValue("invoice")
		if !uuid(invoice) {
			reply(w, 400, map[string]string{"error": "Valid invoice required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "payments.record")
		if err != nil {
			failure(w, err)
			return
		}
		var input struct {
			RequestKey  string `json:"requestKey"`
			SourceID    string `json:"sourceId"`
			AmountCents int64  `json:"amountCents"`
		}
		if !decode(w, r, &input) {
			return
		}
		chargeSquareInvoice(w, r, store, scope, invoice, input.RequestKey, input.SourceID, input.AmountCents)
	})
}

// chargeSquareInvoice runs the shared prepare + execute flow for both the location-scoped and the
// employee-scoped routes. The scope is always resolved by the caller from server-side authority.
func chargeSquareInvoice(w http.ResponseWriter, r *http.Request, store *postgres.Store, scope postgres.Scope, invoice, requestKey, sourceID string, amountCents int64) {
	resolve := store.SquareCredentialResolver()
	if resolve == nil {
		reply(w, 503, map[string]string{"error": "Square payments are not configured on the server"})
		return
	}
	binding, err := store.CurrentSquareMerchantBinding(r.Context(), scope, "payments.record")
	if err != nil {
		if errors.Is(err, postgres.ErrSquareBinding) {
			reply(w, 409, map[string]string{"error": "An administrator must review and save the Square merchant for this location first"})
			return
		}
		failure(w, err)
		return
	}
	attempt, err := store.PrepareSquareInvoiceIntent(r.Context(), scope, postgres.PrepareSquareInvoiceIntentInput{InvoiceID: invoice, BindingVersion: binding.Version, RequestKey: requestKey, AmountCents: amountCents})
	if err != nil {
		squareIntentFailure(w, err, "")
		return
	}
	allocation, err := store.ExecuteSquareInvoiceIntent(r.Context(), scope, attempt.ID, sourceID, resolve)
	if err != nil {
		squareIntentFailure(w, err, attempt.ID)
		return
	}
	reply(w, 201, map[string]any{"intentId": allocation.IntentID, "paymentId": allocation.PaymentID, "invoiceId": allocation.InvoiceID, "status": "settled"})
}

func squareIntentFailure(w http.ResponseWriter, err error, intent string) {
	switch {
	case errors.Is(err, postgres.ErrPaymentInput):
		reply(w, 422, map[string]string{"error": "Check the amount, request key and card details"})
	case errors.Is(err, postgres.ErrPaymentRequest):
		reply(w, 409, map[string]string{"error": "This payment request key was already used with different details"})
	case errors.Is(err, postgres.ErrInvoiceNotFound):
		reply(w, 404, map[string]string{"error": "Invoice not found in this scope"})
	case errors.Is(err, postgres.ErrSquareIntentState):
		reply(w, 409, map[string]string{"error": "This payment attempt is already being processed", "intentId": intent})
	case errors.Is(err, postgres.ErrPaymentProvider):
		// The outcome is not confirmed; the same request key retries the SAME provider charge identity.
		reply(w, 502, map[string]string{"error": "The payment could not be confirmed. Do not use a different card. Retry with the same request key or ask an administrator to reconcile it", "intentId": intent})
	default:
		failure(w, err)
	}
}
