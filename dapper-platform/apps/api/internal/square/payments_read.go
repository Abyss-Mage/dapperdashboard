package square

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type PaymentReader interface {
	GetPayment(context.Context, string) (PaymentResponse, error)
}

func (c *squareClient) GetPayment(ctx context.Context, id string) (PaymentResponse, error) {
	if strings.TrimSpace(id) == "" || len(id) > 192 {
		return PaymentResponse{}, ErrSquareBadRequest
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/payments/"+url.PathEscape(id), nil)
	if err != nil {
		return PaymentResponse{}, ErrSquarePaymentFailed
	}
	request.Header.Set("Authorization", "Bearer "+c.accessToken)
	request.Header.Set("Square-Version", "2024-10-17")
	request.Header.Set("Accept", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return PaymentResponse{}, ErrSquarePaymentFailed
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return PaymentResponse{}, ErrSquarePaymentFailed
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
	if err != nil || len(body) > 128*1024 {
		return PaymentResponse{}, ErrSquarePaymentFailed
	}
	var decoded struct {
		Payment struct {
			ID          string `json:"id"`
			ReferenceID string `json:"reference_id"`
			LocationID  string `json:"location_id"`
			Status      string `json:"status"`
			Amount      struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			} `json:"amount_money"`
			Refunded struct {
				Amount int64 `json:"amount"`
			} `json:"refunded_money"`
		} `json:"payment"`
	}
	if err = json.Unmarshal(body, &decoded); err != nil {
		return PaymentResponse{}, ErrSquarePaymentFailed
	}
	payment := decoded.Payment
	// Already-refunded captures require dedicated net-refund reconciliation before
	// allocation. Do not replay their original full amount onto an invoice.
	if payment.ID != id || payment.Amount.Amount <= 0 || payment.Status == "" || payment.Refunded.Amount != 0 {
		return PaymentResponse{}, ErrSquarePaymentFailed
	}
	return PaymentResponse{ID: payment.ID, ReferenceID: payment.ReferenceID, LocationID: payment.LocationID, Status: payment.Status, AmountCents: payment.Amount.Amount, Currency: payment.Amount.Currency}, nil
}
