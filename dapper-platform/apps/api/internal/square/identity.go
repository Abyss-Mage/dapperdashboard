package square

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var ErrSquareIdentity = errors.New("Square credential does not verify the configured merchant location")

type LocationBinding struct{ Environment, MerchantID, LocationID, Currency string }
type LocationVerifier interface {
	VerifyLocation(context.Context, LocationBinding) error
}

func (c *squareClient) VerifyLocation(ctx context.Context, expected LocationBinding) error {
	if expected.Environment != c.environment || strings.TrimSpace(expected.MerchantID) == "" || strings.TrimSpace(expected.LocationID) == "" || expected.Currency != "CAD" {
		return ErrSquareIdentity
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/locations/"+url.PathEscape(expected.LocationID), nil)
	if err != nil {
		return ErrSquareIdentity
	}
	request.Header.Set("Authorization", "Bearer "+c.accessToken)
	request.Header.Set("Square-Version", "2024-10-17")
	request.Header.Set("Accept", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return ErrSquareIdentity
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ErrSquareIdentity
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
	if err != nil || len(body) > 128*1024 {
		return ErrSquareIdentity
	}
	var decoded struct {
		Location struct {
			ID           string   `json:"id"`
			MerchantID   string   `json:"merchant_id"`
			Currency     string   `json:"currency"`
			Status       string   `json:"status"`
			Capabilities []string `json:"capabilities"`
		} `json:"location"`
	}
	if err = json.Unmarshal(body, &decoded); err != nil {
		return ErrSquareIdentity
	}
	location := decoded.Location
	if location.ID != expected.LocationID || location.MerchantID != expected.MerchantID || location.Currency != expected.Currency || location.Status != "ACTIVE" {
		return ErrSquareIdentity
	}
	for _, capability := range location.Capabilities {
		if capability == "CREDIT_CARD_PROCESSING" {
			return nil
		}
	}
	return ErrSquareIdentity
}
