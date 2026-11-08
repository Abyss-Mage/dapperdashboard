package workspace

import (
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// waitOnSquare gives a request that calls Square (two calls, up to 15 seconds each) longer than the server's usual 10-second write limit.
func waitOnSquare(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(60 * time.Second))
}

type squareWebhookEvent struct {
	MerchantID string `json:"merchant_id"`
	Type       string `json:"type"`
	EventID    string `json:"event_id"`
	CreatedAt  string `json:"created_at"`
	Data       struct {
		Type   string          `json:"type"`
		ID     string          `json:"id"`
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type squarePaymentObject struct {
	Payment struct {
		ID          string `json:"id"`
		ReferenceID string `json:"reference_id"` // our payment attempt, when the charge came through one
		Status      string `json:"status"`       // "COMPLETED", "CANCELED", "FAILED"
		AmountMoney struct {
			Amount   int64  `json:"amount"`
			Currency string `json:"currency"`
		} `json:"amount_money"`
		RefundedMoney *struct {
			Amount   int64  `json:"amount"`
			Currency string `json:"currency"`
		} `json:"refunded_money,omitempty"`
	} `json:"payment"`
}

func registerSquareWebhookRoutes(mux *http.ServeMux, store *postgres.Store, sqClient square.Client, webhookURL, signatureKey string) {
	mux.HandleFunc("POST /api/v1/integrations/square/webhooks", func(w http.ResponseWriter, r *http.Request) {
		sigKey := strings.TrimSpace(signatureKey)
		url := strings.TrimSpace(webhookURL)
		if sigKey == "" || url == "" {
			reply(w, 503, map[string]string{"error": "Square webhook ingress is not configured"})
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 128*1024))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				reply(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Webhook payload exceeds limit"})
				return
			}
			reply(w, 400, map[string]string{"error": "Failed to read webhook payload"})
			return
		}

		sig := r.Header.Get("x-square-hmacsha256-signature")
		// Franchises connected by OAuth need no server-wide Square client.
		verify := square.VerifyWebhookSignature
		if sqClient != nil {
			verify = sqClient.VerifyWebhookSignature
		}
		if !verify(body, sig, url, sigKey) {
			reply(w, 401, map[string]string{"error": "Invalid webhook signature"})
			return
		}

		var event squareWebhookEvent
		if err := json.Unmarshal(body, &event); err != nil {
			reply(w, 400, map[string]string{"error": "Invalid webhook JSON structure"})
			return
		}

		if event.Type == "payment.updated" {
			var obj squarePaymentObject
			if err := json.Unmarshal(event.Data.Object, &obj); err != nil || strings.TrimSpace(obj.Payment.ID) == "" || strings.TrimSpace(obj.Payment.Status) == "" {
				reply(w, http.StatusBadRequest, map[string]string{"error": "Invalid payment notification"})
				return
			}
			{
				var refundAmt int64
				if obj.Payment.RefundedMoney != nil {
					refundAmt = obj.Payment.RefundedMoney.Amount
				}
				if err := store.HandleSquareWebhookPayment(r.Context(), obj.Payment.ID, obj.Payment.Status, refundAmt); err != nil {
					reply(w, http.StatusServiceUnavailable, map[string]string{"error": "Payment notification persistence unavailable"})
					return
				}
				if obj.Payment.Status == "COMPLETED" {
					waitOnSquare(w)
					if err := store.SettleSquareIntentFromWebhook(r.Context(), obj.Payment.ReferenceID, obj.Payment.ID); err != nil {
						// A failed reply makes Square send this again later.
						log.Printf("square webhook: payment attempt %s could not be settled: %v", obj.Payment.ReferenceID, err)
						reply(w, http.StatusServiceUnavailable, map[string]string{"error": "Payment could not be settled yet"})
						return
					}
				}
			}
		}

		reply(w, 200, map[string]any{"received": true})
	})
}
