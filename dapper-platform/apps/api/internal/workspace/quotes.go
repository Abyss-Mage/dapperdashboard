package workspace

import (
	"context"
	"time"
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func quoteFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrQuoteNotFound) {
		reply(w, 404, map[string]string{"error": "Quotation not found in this scope"})
		return
	}
	if errors.Is(err, postgres.ErrQuoteInput) {
		reply(w, 422, map[string]string{"error": "Check the quotation inputs, line items, and quantities"})
		return
	}
	if errors.Is(err, postgres.ErrQuoteStatusConflict) {
		reply(w, 409, map[string]string{"error": "Quotation status does not allow this operation"})
		return
	}
	if errors.Is(err, postgres.ErrQuoteStale) {
		reply(w, 409, map[string]string{"error": "Quotation was modified by another request; reload and retry"})
		return
	}
	if errors.Is(err, postgres.ErrQuoteAlreadyFulfilled) {
		reply(w, 409, map[string]string{"error": "Quotation or booking request has already been fulfilled"})
		return
	}
	if errors.Is(err, postgres.ErrQuoteDepositRequired) {
		reply(w, 422, map[string]string{"error": "Verified deposit payment or explicit reviewed exemption configuration required"})
		return
	}
	if errors.Is(err, postgres.ErrQuoteExpired) {
		reply(w, 410, map[string]string{"error": "Quotation has expired and cannot be accepted or fulfilled"})
		return
	}
	if errors.Is(err, postgres.ErrQuoteAcceptanceConflict) {
		reply(w, 409, map[string]string{"error": "Acceptance key conflicts with an earlier submission"})
		return
	}
	if errors.Is(err, postgres.ErrReservationConflict) {
		reply(w, 409, map[string]string{"error": "Requested time slot is unavailable; check current availability"})
		return
	}
	failure(w, err)
}

func registerQuoteRoutes(mux *http.ServeMux, store *postgres.Store, emailSender postgres.EmailSender) {
	base := "/api/v1/organizations/{organization}/locations/{location}/quotes"

	// The business details printed on a quotation (the same ones invoices use).
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/quote-issuer", func(w http.ResponseWriter, r *http.Request) {
		organization, location := r.PathValue("organization"), r.PathValue("location")
		if !uuid(organization) || !uuid(location) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "quotes.read")
		if err != nil {
			failure(w, err)
			return
		}
		issuer, err := store.QuoteIssuerFor(r.Context(), scope)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, issuer)
	})
	// GET list quotes
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		organization, location := r.PathValue("organization"), r.PathValue("location")
		if !uuid(organization) || !uuid(location) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors are required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "quotes.read")
		if err != nil {
			failure(w, err)
			return
		}
		items, err := store.ListQuotes(r.Context(), scope)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 200})
	})

	// POST create draft quote
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		organization, location := r.PathValue("organization"), r.PathValue("location")
		if !uuid(organization) || !uuid(location) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors are required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "quotes.create")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.CreateQuoteInput
		if !decodeLimited(w, r, &input, 32*1024) {
			return
		}
		quote, err := store.CreateQuote(r.Context(), scope, input)
		if err != nil {
			quoteFailure(w, err)
			return
		}
		reply(w, 201, quote)
	})

	// GET single quote
	mux.HandleFunc("GET "+base+"/{quote}", func(w http.ResponseWriter, r *http.Request) {
		organization, location, quoteID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("quote")
		if !uuid(organization) || !uuid(location) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors are required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "quotes.read")
		if err != nil {
			failure(w, err)
			return
		}
		quote, err := store.GetQuote(r.Context(), scope, quoteID)
		if err != nil {
			quoteFailure(w, err)
			return
		}
		reply(w, 200, quote)
	})

	// PUT edit draft quote
	mux.HandleFunc("PUT "+base+"/{quote}", func(w http.ResponseWriter, r *http.Request) {
		organization, location, quoteID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("quote")
		if !uuid(organization) || !uuid(location) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors are required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "quotes.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.EditQuoteInput
		if !decodeLimited(w, r, &input, 32*1024) {
			return
		}
		quote, err := store.EditQuote(r.Context(), scope, quoteID, input)
		if err != nil {
			quoteFailure(w, err)
			return
		}
		reply(w, 200, quote)
	})

	mux.HandleFunc("DELETE "+base+"/{quote}", func(w http.ResponseWriter, r *http.Request) {
		organization, location, quoteID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("quote")
		if !uuid(organization) || !uuid(location) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors are required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "quotes.manage")
		if err != nil {
			failure(w, err)
			return
		}
		if err = store.DeleteQuote(r.Context(), scope, quoteID); err != nil {
			quoteFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"deleted": true})
	})

	// POST issue quote
	mux.HandleFunc("POST "+base+"/{quote}/issue", func(w http.ResponseWriter, r *http.Request) {
		organization, location, quoteID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("quote")
		if !uuid(organization) || !uuid(location) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors are required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "quotes.issue")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.IssueQuoteInput
		if !decodeLimited(w, r, &input, 8192) {
			return
		}
		quote, err := store.IssueQuote(r.Context(), scope, quoteID, input)
		if err != nil {
			quoteFailure(w, err)
			return
		}
		// Not tied to the request: staff leaving the page must not cut the email off half way.
		mailCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 40*time.Second)
		defer cancel()
		status, to := emailIssuedQuote(mailCtx, store, emailSender, quote)
		reply(w, 200, struct {
			postgres.Quote
			EmailStatus string `json:"emailStatus"`
			EmailedTo   string `json:"emailedTo,omitempty"`
		}{quote, status, to})
	})

	// POST fulfill quote
	mux.HandleFunc("POST "+base+"/{quote}/fulfill", func(w http.ResponseWriter, r *http.Request) {
		organization, location, quoteID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("quote")
		if !uuid(organization) || !uuid(location) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors are required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "quotes.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.FulfillQuoteInput
		if !decodeLimited(w, r, &input, 16*1024) {
			return
		}
		input.QuoteID = quoteID
		result, err := store.FulfillQuote(r.Context(), scope, input)
		if err != nil {
			quoteFailure(w, err)
			return
		}
		reply(w, 200, result)
	})

	// Customer portal quote routes
	portalBase := "/api/v1/portal/accounts/{customer}/quotes"

	mux.HandleFunc("GET "+portalBase, func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		items, err := store.CustomerQuotes(r.Context(), requestSession(r).principalID, customer)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})

	mux.HandleFunc("GET "+portalBase+"/{quote}", func(w http.ResponseWriter, r *http.Request) {
		customer, quoteID := r.PathValue("customer"), r.PathValue("quote")
		if !uuid(customer) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		quote, err := store.GetCustomerQuote(r.Context(), requestSession(r).principalID, customer, quoteID)
		if err != nil {
			quoteFailure(w, err)
			return
		}
		reply(w, 200, quote)
	})

	mux.HandleFunc("POST "+portalBase+"/{quote}/accept", func(w http.ResponseWriter, r *http.Request) {
		customer, quoteID := r.PathValue("customer"), r.PathValue("quote")
		if !uuid(customer) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		var input postgres.AcceptQuoteInput
		if !decodeLimited(w, r, &input, 8192) {
			return
		}
		quote, err := store.AcceptQuote(r.Context(), requestSession(r).principalID, customer, quoteID, input)
		if err != nil {
			quoteFailure(w, err)
			return
		}
		reply(w, 200, quote)
	})

	mux.HandleFunc("POST "+portalBase+"/{quote}/decline", func(w http.ResponseWriter, r *http.Request) {
		customer, quoteID := r.PathValue("customer"), r.PathValue("quote")
		if !uuid(customer) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		var input postgres.DeclineQuoteInput
		if !decodeLimited(w, r, &input, 8192) {
			return
		}
		quote, err := store.DeclineQuote(r.Context(), requestSession(r).principalID, customer, quoteID, input)
		if err != nil {
			quoteFailure(w, err)
			return
		}
		reply(w, 200, quote)
	})

	mux.HandleFunc("POST "+portalBase+"/{quote}/pay-deposit", func(w http.ResponseWriter, r *http.Request) {
		customer, quoteID := r.PathValue("customer"), r.PathValue("quote")
		if !uuid(customer) || !uuid(quoteID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		var input postgres.CustomerPayQuoteDepositInput
		if !decodeLimited(w, r, &input, 8192) {
			return
		}
		waitOnSquare(w)
		principal := requestSession(r).principalID
		client, locationID, err := portalSquareClient(r.Context(), store, principal, customer)
		if errors.Is(err, postgres.ErrSquareBinding) {
			reply(w, 503, map[string]string{"error": "Square payment provider is not configured for this location"})
			return
		}
		if err == nil {
			input.SquareLocationID = locationID
			var res postgres.CustomerQuoteDepositResult
			if res, err = store.CustomerPayQuoteDeposit(r.Context(), principal, customer, quoteID, client, input); err == nil {
				reply(w, 201, res)
				return
			}
		}
		if err != nil {
			switch {
			case errors.Is(err, postgres.ErrForbidden):
				reply(w, 403, map[string]string{"error": "Access denied to customer quote"})
			case errors.Is(err, postgres.ErrQuoteNotFound):
				reply(w, 404, map[string]string{"error": "Quote not found"})
			case errors.Is(err, postgres.ErrQuoteAlreadyFulfilled):
				reply(w, 409, map[string]string{"error": "Quote has already been fulfilled"})
			case errors.Is(err, postgres.ErrQuoteStatusConflict):
				reply(w, 409, map[string]string{"error": "Quote must be accepted and not expired to pay deposit"})
			case errors.Is(err, postgres.ErrPaymentInput):
				reply(w, 422, map[string]string{"error": "Invalid payment input or idempotency key"})
			case errors.Is(err, postgres.ErrPaymentProvider):
				reply(w, 502, map[string]string{"error": "Payment processor error"})
			default:
				quoteFailure(w, err)
			}
			return
		}
	})
}
