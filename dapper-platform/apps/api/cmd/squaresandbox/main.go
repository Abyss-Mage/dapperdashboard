// Command squaresandbox is a manual provider-evidence tool. It exercises the
// real Square SANDBOX through the application's own client: location identity,
// a sandbox test-card payment, idempotent replay, payment retrieval and a
// declined test card. It never runs against production and never prints secrets.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"dapper.local/platform/internal/square"
)

type check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

func main() {
	token, merchant, location := os.Getenv("SQUARE_SANDBOX_TOKEN"), os.Getenv("SQUARE_SANDBOX_MERCHANT_ID"), os.Getenv("SQUARE_SANDBOX_LOCATION_ID")
	if token == "" || merchant == "" || location == "" {
		fmt.Fprintln(os.Stderr, "SQUARE_SANDBOX_TOKEN, SQUARE_SANDBOX_MERCHANT_ID and SQUARE_SANDBOX_LOCATION_ID are required")
		os.Exit(2)
	}
	raw, err := square.NewClient(square.Config{AccessToken: token, Environment: "sandbox"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "client configuration rejected")
		os.Exit(2)
	}
	client := raw.(interface {
		square.Client
		square.LocationVerifier
		square.PaymentReader
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var checks []check
	add := func(name string, ok bool, detail string) { checks = append(checks, check{name, ok, detail}) }

	err = client.VerifyLocation(ctx, square.LocationBinding{Environment: "sandbox", MerchantID: merchant, LocationID: location, Currency: "CAD"})
	add("location_identity_cad_card_processing", err == nil, errText(err))
	err = client.VerifyLocation(ctx, square.LocationBinding{Environment: "sandbox", MerchantID: merchant + "x", LocationID: location, Currency: "CAD"})
	add("wrong_merchant_rejected", errors.Is(err, square.ErrSquareIdentity), "")

	key, reference := randomHex(18), "sbx-"+randomHex(8)
	request := square.PaymentRequest{SourceID: "cnon:card-nonce-ok", IdempotencyKey: key, AmountCents: 150, Currency: "CAD", LocationID: location, ReferenceID: reference}
	first, err := client.CreatePayment(ctx, request)
	add("test_card_payment_completed", err == nil && first.Status == "COMPLETED" && first.AmountCents == 150 && first.Currency == "CAD" && first.LocationID == location && first.ReferenceID == reference, errText(err)+" status="+first.Status)
	replay, err := client.CreatePayment(ctx, request)
	add("idempotent_replay_same_payment", err == nil && replay.ID != "" && replay.ID == first.ID, errText(err))
	if first.ID != "" {
		got, err := client.GetPayment(ctx, first.ID)
		add("retrieve_matches_charge", err == nil && got.ID == first.ID && got.Status == "COMPLETED" && got.AmountCents == 150 && got.ReferenceID == reference, errText(err))
	}
	_, err = client.CreatePayment(ctx, square.PaymentRequest{SourceID: "cnon:card-nonce-declined", IdempotencyKey: randomHex(18), AmountCents: 150, Currency: "CAD", LocationID: location, ReferenceID: "sbx-" + randomHex(8)})
	add("declined_test_card_not_success", errors.Is(err, square.ErrSquareDeclined) || errors.Is(err, square.ErrSquarePaymentFailed), errText(err))

	passed := true
	for _, c := range checks {
		passed = passed && c.Passed
	}
	out, _ := json.MarshalIndent(map[string]any{"environment": "sandbox", "provider": "square", "checkedAt": time.Now().UTC().Format(time.RFC3339), "passed": passed, "checks": checks, "sandboxPaymentId": first.ID}, "", "  ")
	fmt.Println(string(out))
	if !passed {
		os.Exit(1)
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
