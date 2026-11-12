package workspace

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"dapper.local/platform/internal/postgres"
)

// The private pay page behind the link in a dealership invoice email (error404 #18): the one invoice, its PDF and a card payment, with
// no sign-in. The link's secret is the only key, so wrong links are rate limited per visitor.
func registerInvoicePayLinkRoutes(mux *http.ServeMux, store *postgres.Store) {
	wrong := newVisitorLimiter(20, 15*time.Minute)
	base := "/api/v1/public/invoice-pay/{token}"
	open := func(w http.ResponseWriter, r *http.Request) (postgres.Scope, postgres.DealerInvoice, bool) {
		key := "paylink:" + visitor(r)
		if wrong.full(key) {
			reply(w, 429, map[string]string{"error": "Too many tries. Wait 15 minutes and open the link from your email again."})
			return postgres.Scope{}, postgres.DealerInvoice{}, false
		}
		scope, inv, err := store.PayLinkInvoice(r.Context(), r.PathValue("token"))
		if errors.Is(err, postgres.ErrPayLink) || errors.Is(err, postgres.ErrInvoiceNotFound) {
			wrong.allow(key)
			reply(w, 404, map[string]string{"error": "This invoice link is not valid or has expired. Ask the business to send it again."})
			return scope, inv, false
		}
		if err != nil {
			failure(w, err)
			return scope, inv, false
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		return scope, inv, true
	}
	view := func(inv postgres.DealerInvoice) map[string]any {
		lines := make([]map[string]any, len(inv.Lines))
		for i, l := range inv.Lines {
			lines[i] = map[string]any{"description": l.Description, "detail": l.Detail, "quantity": l.Quantity, "unitPriceCents": l.UnitPriceCents}
		}
		business := ""
		if inv.IssuerSnapshot != nil {
			business = inv.IssuerSnapshot.LegalName
		}
		return map[string]any{"number": inv.InvoiceNumber, "status": inv.Status, "overdue": inv.Overdue, "dealershipName": inv.DealershipName, "businessName": business,
			"invoiceDate": inv.InvoiceDate, "dueDate": inv.DueDate, "poNumber": inv.PONumber, "lines": lines, "notes": inv.Notes, "currency": inv.Currency,
			"subtotalCents": inv.SubtotalCents, "discountCents": inv.DiscountCents, "discountPercent": inv.DiscountPercent, "taxCents": inv.TaxCents,
			"totalCents": inv.TotalCents, "paidCents": inv.PaidCents, "dueCents": inv.DueCents}
	}

	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		if _, inv, ok := open(w, r); ok {
			reply(w, 200, view(inv))
		}
	})
	mux.HandleFunc("GET "+base+"/pdf", func(w http.ResponseWriter, r *http.Request) {
		if _, inv, ok := open(w, r); ok {
			name, pdf := store.InvoicePDF(r.Context(), inv.Invoice)
			replyPDF(w, name, pdf)
		}
	})
	mux.HandleFunc("GET "+base+"/square", func(w http.ResponseWriter, r *http.Request) {
		scope, _, ok := open(w, r)
		if !ok {
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
		reply(w, 200, map[string]any{"configured": true, "credentialsConfigured": store.SquareCredentialResolver() != nil, "applicationId": store.SquareApplicationIDFor(r.Context(), scope, b.Environment), "binding": map[string]any{"environment": b.Environment, "squareLocationId": b.SquareLocationID}})
	})
	mux.HandleFunc("POST "+base+"/pay", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			SourceID       string `json:"sourceId"`
			AmountCents    int64  `json:"amountCents"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if !decodeLimited(w, r, &input, 16*1024) {
			return
		}
		scope, inv, ok := open(w, r)
		if !ok {
			return
		}
		waitOnSquare(w)
		var err error
		switch {
		case store.SquareCredentialResolver() == nil:
			err = postgres.ErrSquareBinding
		case strings.TrimSpace(input.SourceID) == "" || len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 64:
			err = postgres.ErrPaymentInput
		default:
			err = takeSquareDeposit(r, store, scope, inv.ID, "paylink-invoice-"+input.IdempotencyKey, input.SourceID, input.AmountCents)
		}
		if err == nil {
			_, inv, err = store.PayLinkInvoice(r.Context(), r.PathValue("token"))
		}
		switch {
		case err == nil:
			reply(w, 201, map[string]any{"payment": map[string]any{"amountCents": input.AmountCents}, "invoice": view(inv)})
		case errors.Is(err, postgres.ErrSquareBinding):
			reply(w, 503, map[string]string{"error": "Card payments are not set up for this business yet. Contact them to pay another way."})
		case errors.Is(err, postgres.ErrPaymentDeclined):
			reply(w, 402, map[string]string{"error": "Your card was declined. Check the details or try another card."})
		case errors.Is(err, postgres.ErrSquareIntentState):
			reply(w, 409, map[string]string{"error": "This payment is already being processed."})
		case errors.Is(err, postgres.ErrInvoiceStatusConflict):
			reply(w, 409, map[string]string{"error": "This invoice cannot be paid in its current status."})
		case errors.Is(err, postgres.ErrPaymentOverpay):
			reply(w, 422, map[string]string{"error": "The amount is more than the balance owing."})
		case errors.Is(err, postgres.ErrPaymentInput):
			reply(w, 422, map[string]string{"error": "The payment details are not valid. Reload the page and try again."})
		case errors.Is(err, postgres.ErrPaymentRequest):
			reply(w, 409, map[string]string{"error": "This payment was already sent. Reload the page to see the balance."})
		case errors.Is(err, postgres.ErrPaymentProvider):
			reply(w, 502, map[string]string{"error": "The card processor did not answer. Try again in a minute."})
		default:
			failure(w, err)
		}
	})
}
