package square

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrSquareConfig        = errors.New("square client configuration invalid")
	ErrSquarePaymentFailed = errors.New("square payment processing failed")
	ErrSquareDeclined      = errors.New("square payment card was declined")
	ErrSquareBadRequest    = errors.New("invalid square payment request")
	ErrSquareSignature     = errors.New("invalid square webhook signature")
)

type Client interface {
	CreatePayment(ctx context.Context, req PaymentRequest) (PaymentResponse, error)
	VerifyWebhookSignature(body []byte, signature, webhookURL, signatureKey string) bool
}

type Config struct {
	AccessToken string
	Environment string // "sandbox" or "production"
	BaseURL     string // Optional override, defaults according to Environment
	HTTPClient  *http.Client
}

type PaymentRequest struct {
	SourceID       string `json:"sourceId"`
	IdempotencyKey string `json:"idempotencyKey"`
	AmountCents    int64  `json:"amountCents"`
	Currency       string `json:"currency"`
	LocationID     string `json:"locationId"`
	ReferenceID    string `json:"referenceId,omitempty"`
	Note           string `json:"note,omitempty"`
}

type PaymentResponse struct {
	ReferenceID string    `json:"referenceId"`
	ID          string    `json:"id"`
	Status      string    `json:"status"` // "COMPLETED", "APPROVED", "PENDING", "FAILED", "CANCELED"
	AmountCents int64     `json:"amountCents"`
	Currency    string    `json:"currency"`
	LocationID  string    `json:"locationId"`
	ReceiptURL  string    `json:"receiptUrl,omitempty"`
	CardBrand   string    `json:"cardBrand,omitempty"`
	Last4       string    `json:"last4,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	Note        string    `json:"note,omitempty"`
}

type squareClient struct {
	environment string
	accessToken string
	baseURL     string
	client      *http.Client
}

func NewClient(cfg Config) (Client, error) {
	token := strings.TrimSpace(cfg.AccessToken)
	if token == "" {
		return nil, ErrSquareConfig
	}
	environment := strings.ToLower(strings.TrimSpace(cfg.Environment))
	if environment == "" {
		environment = "sandbox"
	}
	if environment != "sandbox" && environment != "production" {
		return nil, ErrSquareConfig
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		if environment == "production" {
			baseURL = "https://connect.squareup.com"
		} else {
			baseURL = "https://connect.squareupsandbox.com"
		}
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &squareClient{
		environment: environment,
		accessToken: token,
		baseURL:     strings.TrimRight(baseURL, "/"),
		client:      hc,
	}, nil
}

func (c *squareClient) CreatePayment(ctx context.Context, req PaymentRequest) (PaymentResponse, error) {
	if strings.TrimSpace(req.SourceID) == "" || strings.TrimSpace(req.IdempotencyKey) == "" || req.AmountCents <= 0 {
		return PaymentResponse{}, ErrSquareBadRequest
	}
	curr := strings.ToUpper(strings.TrimSpace(req.Currency))
	if curr == "" {
		curr = "CAD"
	}

	bodyMap := map[string]any{
		"source_id":       req.SourceID,
		"idempotency_key": req.IdempotencyKey,
		"amount_money": map[string]any{
			"amount":   req.AmountCents,
			"currency": curr,
		},
		"autocomplete": true,
	}
	if req.LocationID != "" {
		bodyMap["location_id"] = req.LocationID
	}
	if req.ReferenceID != "" {
		bodyMap["reference_id"] = req.ReferenceID
	}
	if req.Note != "" {
		bodyMap["note"] = req.Note
	}

	jsonBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return PaymentResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v2/payments", bytes.NewReader(jsonBytes))
	if err != nil {
		return PaymentResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.accessToken)
	httpReq.Header.Set("Square-Version", "2024-10-17")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return PaymentResponse{}, fmt.Errorf("%w: %v", ErrSquarePaymentFailed, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return PaymentResponse{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Errors []struct {
				Category string `json:"category"`
				Code     string `json:"code"`
				Detail   string `json:"detail"`
			} `json:"errors"`
		}
		_ = json.Unmarshal(bodyBytes, &errResp)
		if len(errResp.Errors) > 0 {
			first := errResp.Errors[0]
			if first.Category == "PAYMENT_METHOD_ERROR" || first.Code == "CARD_DECLINED" || first.Code == "CARD_EXPIRED" || first.Code == "INSUFFICIENT_FUNDS" {
				return PaymentResponse{}, fmt.Errorf("%w: %s - %s", ErrSquareDeclined, first.Code, first.Detail)
			}
			return PaymentResponse{}, fmt.Errorf("%w: %s (%s)", ErrSquarePaymentFailed, first.Detail, first.Code)
		}
		return PaymentResponse{}, fmt.Errorf("%w: status %d", ErrSquarePaymentFailed, resp.StatusCode)
	}

	var sqResp struct {
		Payment struct {
			ID          string `json:"id"`
			ReferenceID string `json:"reference_id"`
			Status      string `json:"status"`
			ReceiptURL  string `json:"receipt_url"`
			LocationID  string `json:"location_id"`
			AmountMoney struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			} `json:"amount_money"`
			CardDetails struct {
				Card struct {
					CardBrand string `json:"card_brand"`
					Last4     string `json:"last_4"`
				} `json:"card"`
			} `json:"card_details"`
			CreatedAt string `json:"created_at"`
		} `json:"payment"`
	}
	if err = json.Unmarshal(bodyBytes, &sqResp); err != nil {
		return PaymentResponse{}, fmt.Errorf("%w: failed to parse payment response", ErrSquarePaymentFailed)
	}

	t, _ := time.Parse(time.RFC3339, sqResp.Payment.CreatedAt)
	if t.IsZero() {
		t = time.Now().UTC()
	}

	return PaymentResponse{
		ID:          sqResp.Payment.ID,
		ReferenceID: sqResp.Payment.ReferenceID,
		Status:      sqResp.Payment.Status,
		AmountCents: sqResp.Payment.AmountMoney.Amount,
		Currency:    sqResp.Payment.AmountMoney.Currency,
		LocationID:  sqResp.Payment.LocationID,
		ReceiptURL:  sqResp.Payment.ReceiptURL,
		CardBrand:   sqResp.Payment.CardDetails.Card.CardBrand,
		Last4:       sqResp.Payment.CardDetails.Card.Last4,
		CreatedAt:   t,
	}, nil
}

func (c *squareClient) VerifyWebhookSignature(body []byte, signature, webhookURL, signatureKey string) bool {
	sig := strings.TrimSpace(signature)
	key := strings.TrimSpace(signatureKey)
	url := strings.TrimSpace(webhookURL)
	if sig == "" || key == "" || url == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(url))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)

	decodedSig, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return false
	}
	return hmac.Equal(expectedMAC, decodedSig)
}

type MockClient struct {
	CreatePaymentFunc          func(ctx context.Context, req PaymentRequest) (PaymentResponse, error)
	VerifyWebhookSignatureFunc func(body []byte, signature, webhookURL, signatureKey string) bool
}

func (m *MockClient) CreatePayment(ctx context.Context, req PaymentRequest) (PaymentResponse, error) {
	if m.CreatePaymentFunc != nil {
		return m.CreatePaymentFunc(ctx, req)
	}
	return PaymentResponse{
		ID:          "sq_mock_pay_" + req.IdempotencyKey,
		ReferenceID: req.ReferenceID,
		Status:      "COMPLETED",
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		LocationID:  req.LocationID,
		ReceiptURL:  "https://squareup.com/receipt/preview/mock",
		CardBrand:   "VISA",
		Last4:       "1111",
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (m *MockClient) VerifyWebhookSignature(body []byte, signature, webhookURL, signatureKey string) bool {
	if m.VerifyWebhookSignatureFunc != nil {
		return m.VerifyWebhookSignatureFunc(body, signature, webhookURL, signatureKey)
	}
	return signature == "valid-mock-signature"
}
