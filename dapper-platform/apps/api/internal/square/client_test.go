package square

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSquareClientConfiguration(t *testing.T) {
	_, err := NewClient(Config{})
	if !errors.Is(err, ErrSquareConfig) {
		t.Fatalf("expected ErrSquareConfig for empty access token, got %v", err)
	}

	sandboxClient, err := NewClient(Config{AccessToken: "sandbox-token", Environment: "sandbox"})
	if err != nil {
		t.Fatalf("unexpected error creating sandbox client: %v", err)
	}
	sc, ok := sandboxClient.(*squareClient)
	if !ok || sc.baseURL != "https://connect.squareupsandbox.com" {
		t.Fatalf("expected sandbox base URL, got %s", sc.baseURL)
	}

	prodClient, err := NewClient(Config{AccessToken: "prod-token", Environment: "production"})
	if err != nil {
		t.Fatalf("unexpected error creating production client: %v", err)
	}
	pc, ok := prodClient.(*squareClient)
	if !ok || pc.baseURL != "https://connect.squareup.com" {
		t.Fatalf("expected production base URL, got %s", pc.baseURL)
	}
}

func TestSquareCreatePayment(t *testing.T) {
	ctx := context.Background()

	t.Run("Validation errors", func(t *testing.T) {
		client, _ := NewClient(Config{AccessToken: "token"})
		_, err := client.CreatePayment(ctx, PaymentRequest{})
		if !errors.Is(err, ErrSquareBadRequest) {
			t.Fatalf("expected ErrSquareBadRequest for empty request, got %v", err)
		}

		_, err = client.CreatePayment(ctx, PaymentRequest{
			SourceID:       "cnon:card-nonce-ok",
			IdempotencyKey: "idem-001",
			AmountCents:    -100,
		})
		if !errors.Is(err, ErrSquareBadRequest) {
			t.Fatalf("expected ErrSquareBadRequest for negative amount, got %v", err)
		}
	})

	t.Run("Successful payment", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" || r.URL.Path != "/v2/payments" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Square-Version") != "2024-10-17" {
				t.Fatalf("unexpected square version header: %s", r.Header.Get("Square-Version"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{
				"payment": {
					"id": "sq_pay_test_12345",
					"status": "COMPLETED",
					"amount_money": {
						"amount": 5000,
						"currency": "CAD"
					},
					"location_id": "LOC_123",
					"receipt_url": "https://squareup.com/r/test",
					"card_details": {
						"card": {
							"card_brand": "VISA",
							"last_4": "4242"
						}
					},
					"created_at": "2026-10-23T12:00:00Z"
				}
			}`))
		}))
		defer server.Close()

		client, err := NewClient(Config{AccessToken: "test-token", BaseURL: server.URL})
		if err != nil {
			t.Fatal(err)
		}

		resp, err := client.CreatePayment(ctx, PaymentRequest{
			SourceID:       "cnon:card-nonce-ok",
			IdempotencyKey: "idem-test-001",
			AmountCents:    5000,
			Currency:       "CAD",
			LocationID:     "LOC_123",
		})
		if err != nil {
			t.Fatalf("CreatePayment failed: %v", err)
		}
		if resp.ID != "sq_pay_test_12345" {
			t.Errorf("expected ID sq_pay_test_12345, got %s", resp.ID)
		}
		if resp.Status != "COMPLETED" {
			t.Errorf("expected status COMPLETED, got %s", resp.Status)
		}
		if resp.AmountCents != 5000 {
			t.Errorf("expected amount 5000, got %d", resp.AmountCents)
		}
		if resp.Currency != "CAD" {
			t.Errorf("expected currency CAD, got %s", resp.Currency)
		}
		if resp.CardBrand != "VISA" || resp.Last4 != "4242" {
			t.Errorf("expected VISA / 4242, got %s / %s", resp.CardBrand, resp.Last4)
		}
	})

	t.Run("Card declined", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{
				"errors": [
					{
						"category": "PAYMENT_METHOD_ERROR",
						"code": "CARD_DECLINED",
						"detail": "Card was declined by the issuer."
					}
				]
			}`))
		}))
		defer server.Close()

		client, err := NewClient(Config{AccessToken: "test-token", BaseURL: server.URL})
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.CreatePayment(ctx, PaymentRequest{
			SourceID:       "cnon:card-nonce-declined",
			IdempotencyKey: "idem-declined-001",
			AmountCents:    5000,
		})
		if !errors.Is(err, ErrSquareDeclined) {
			t.Fatalf("expected ErrSquareDeclined, got %v", err)
		}
	})
}

func TestSquareWebhookSignatureVerification(t *testing.T) {
	client, err := NewClient(Config{AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}

	webhookURL := "https://api.dapperdash.ca/api/v1/integrations/square/webhooks"
	signatureKey := "sig-key-secret-12345"
	payload := []byte(`{"type":"payment.updated","data":{"object":{"payment":{"id":"sq_pay_01"}}}}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, []byte(signatureKey))
	mac.Write([]byte(webhookURL))
	mac.Write(payload)
	validSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Valid signature passes
	if !client.VerifyWebhookSignature(payload, validSig, webhookURL, signatureKey) {
		t.Error("expected valid signature to pass verification")
	}

	// Tampered payload fails
	tamperedPayload := []byte(`{"type":"payment.updated","data":{"object":{"payment":{"id":"sq_pay_02"}}}}`)
	if client.VerifyWebhookSignature(tamperedPayload, validSig, webhookURL, signatureKey) {
		t.Error("expected tampered payload to fail verification")
	}

	// Wrong signature key fails
	if client.VerifyWebhookSignature(payload, validSig, webhookURL, "wrong-key") {
		t.Error("expected wrong signature key to fail verification")
	}

	// Empty inputs fail
	if client.VerifyWebhookSignature(payload, "", webhookURL, signatureKey) {
		t.Error("expected empty signature to fail")
	}
	if client.VerifyWebhookSignature(payload, validSig, "", signatureKey) {
		t.Error("expected empty webhookURL to fail")
	}
}

func TestMockSquareClient(t *testing.T) {
	mock := &MockClient{}
	resp, err := mock.CreatePayment(context.Background(), PaymentRequest{
		SourceID:       "cnon:mock",
		IdempotencyKey: "mock-001",
		AmountCents:    2500,
		Currency:       "CAD",
	})
	if err != nil {
		t.Fatalf("mock payment failed: %v", err)
	}
	if resp.ID != "sq_mock_pay_mock-001" || resp.Status != "COMPLETED" || resp.AmountCents != 2500 {
		t.Fatalf("unexpected mock response: %+v", resp)
	}

	if !mock.VerifyWebhookSignature(nil, "valid-mock-signature", "", "") {
		t.Error("expected mock signature to match")
	}
}
