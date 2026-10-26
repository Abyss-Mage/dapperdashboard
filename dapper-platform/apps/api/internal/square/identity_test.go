package square

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCredentialLocationBinding(t *testing.T) {
	valid := `{"location":{"id":"synthetic-location","merchant_id":"synthetic-merchant","currency":"CAD","status":"ACTIVE","capabilities":["CREDIT_CARD_PROCESSING"]}}`
	cases := []struct {
		name, body string
		status     int
		accept     bool
	}{{"verified", valid, 200, true}, {"wrong merchant", strings.Replace(valid, "synthetic-merchant", "other-merchant", 1), 200, false}, {"wrong location", strings.Replace(valid, "synthetic-location", "other-location", 1), 200, false}, {"wrong currency", strings.Replace(valid, "CAD", "USD", 1), 200, false}, {"inactive", strings.Replace(valid, "ACTIVE", "INACTIVE", 1), 200, false}, {"missing card capability", strings.Replace(valid, "CREDIT_CARD_PROCESSING", "OTHER", 1), 200, false}, {"malformed", "not-json", 200, false}, {"missing location", "{}", 200, false}, {"unauthorized", valid, 401, false}, {"oversized", strings.Repeat("x", 128*1024+1), 200, false}}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/v2/locations/synthetic-location" || r.Header.Get("Authorization") != "Bearer synthetic-token" {
					t.Error("unexpected provider identity request")
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := NewClient(Config{AccessToken: "synthetic-token", Environment: "sandbox", BaseURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			verifier := client.(LocationVerifier)
			expected := LocationBinding{Environment: "sandbox", MerchantID: "synthetic-merchant", LocationID: "synthetic-location", Currency: "CAD"}
			err = verifier.VerifyLocation(context.Background(), expected)
			if (err == nil) != test.accept {
				t.Fatalf("identity verification: %v", err)
			}
			if err != nil && !errors.Is(err, ErrSquareIdentity) {
				t.Fatal("raw provider error leaked")
			}
			before := calls
			expected.Environment = "production"
			if verifier.VerifyLocation(context.Background(), expected) == nil || calls != before {
				t.Fatal("environment mismatch not rejected locally")
			}
		})
	}
	normalized, err := NewClient(Config{AccessToken: "synthetic-token", Environment: " Production "})
	if err != nil || normalized.(*squareClient).baseURL != "https://connect.squareup.com" {
		t.Fatal("normalized environment routed to wrong host")
	}
	if _, err := NewClient(Config{AccessToken: "synthetic-token", Environment: "typo"}); !errors.Is(err, ErrSquareConfig) {
		t.Fatal("unknown environment accepted")
	}
}
