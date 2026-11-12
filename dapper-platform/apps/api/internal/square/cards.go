package square

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Cards on file (error404 #27/#32): a Square customer for the dealership and the cards it saves. Square keeps the card; Dapper keeps only
// its id, brand, last four digits and expiry.

// ErrSquareScope: the business's Square connection does not allow this (cards on file need the customers permission).
var ErrSquareScope = errors.New("square connection lacks the permission for this")

type SavedCard struct {
	ID       string
	Brand    string
	Last4    string
	ExpMonth int
	ExpYear  int
}

type CardVault interface {
	CreateCustomer(ctx context.Context, idempotencyKey, referenceID, companyName, email string) (string, error)
	CreateCard(ctx context.Context, idempotencyKey, sourceID, customerID string) (SavedCard, error)
	DisableCard(ctx context.Context, cardID string) error
}

func (c *squareClient) call(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Square-Version", "2024-10-17")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSquarePaymentFailed, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Errors []struct{ Category, Code, Detail string } `json:"errors"`
		}
		_ = json.Unmarshal(data, &e)
		if len(e.Errors) > 0 {
			if e.Errors[0].Code == "INSUFFICIENT_SCOPES" {
				return ErrSquareScope
			}
			if e.Errors[0].Category == "PAYMENT_METHOD_ERROR" || strings.HasPrefix(e.Errors[0].Code, "CARD_") || e.Errors[0].Code == "INVALID_CARD" {
				return fmt.Errorf("%w: %s", ErrSquareDeclined, e.Errors[0].Code)
			}
			return fmt.Errorf("%w: %s (%s)", ErrSquarePaymentFailed, e.Errors[0].Detail, e.Errors[0].Code)
		}
		return fmt.Errorf("%w: status %d", ErrSquarePaymentFailed, resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *squareClient) CreateCustomer(ctx context.Context, idempotencyKey, referenceID, companyName, email string) (string, error) {
	var out struct {
		Customer struct {
			ID string `json:"id"`
		} `json:"customer"`
	}
	body := map[string]any{"idempotency_key": idempotencyKey, "reference_id": referenceID, "company_name": companyName}
	if email != "" {
		body["email_address"] = email
	}
	if err := c.call(ctx, "POST", "/v2/customers", body, &out); err != nil {
		return "", err
	}
	if out.Customer.ID == "" {
		return "", ErrSquarePaymentFailed
	}
	return out.Customer.ID, nil
}

func (c *squareClient) CreateCard(ctx context.Context, idempotencyKey, sourceID, customerID string) (SavedCard, error) {
	if strings.TrimSpace(sourceID) == "" || customerID == "" {
		return SavedCard{}, ErrSquareBadRequest
	}
	var out struct {
		Card struct {
			ID       string `json:"id"`
			Brand    string `json:"card_brand"`
			Last4    string `json:"last_4"`
			ExpMonth int    `json:"exp_month"`
			ExpYear  int    `json:"exp_year"`
		} `json:"card"`
	}
	body := map[string]any{"idempotency_key": idempotencyKey, "source_id": sourceID, "card": map[string]any{"customer_id": customerID}}
	if err := c.call(ctx, "POST", "/v2/cards", body, &out); err != nil {
		return SavedCard{}, err
	}
	if out.Card.ID == "" {
		return SavedCard{}, ErrSquarePaymentFailed
	}
	return SavedCard{ID: out.Card.ID, Brand: out.Card.Brand, Last4: out.Card.Last4, ExpMonth: out.Card.ExpMonth, ExpYear: out.Card.ExpYear}, nil
}

func (c *squareClient) DisableCard(ctx context.Context, cardID string) error {
	return c.call(ctx, "POST", "/v2/cards/"+url.PathEscape(cardID)+"/disable", map[string]any{}, nil)
}
