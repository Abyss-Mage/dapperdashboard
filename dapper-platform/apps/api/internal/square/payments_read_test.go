package square

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticatedPaymentRead(t *testing.T) {
	valid := `{"payment":{"id":"synthetic-payment","reference_id":"synthetic-intent","location_id":"synthetic-location","status":"COMPLETED","amount_money":{"amount":500,"currency":"CAD"}}}`
	for _, probe := range []struct {
		name, body string
		status     int
		ok         bool
	}{{"valid", valid, 200, true}, {"wrong id", strings.Replace(valid, "synthetic-payment", "other", 1), 200, false}, {"refunded", strings.Replace(valid, `"status":"COMPLETED"`, `"status":"COMPLETED","refunded_money":{"amount":100}`, 1), 200, false}, {"malformed", "not-json", 200, false}, {"not found", "{}", 404, false}, {"too large", strings.Repeat("x", 128*1024+1), 200, false}} {
		t.Run(probe.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/v2/payments/synthetic-payment" || r.Header.Get("Authorization") != "Bearer synthetic-token" {
					t.Error("unexpected authenticated read request")
				}
				w.WriteHeader(probe.status)
				_, _ = w.Write([]byte(probe.body))
			}))
			defer server.Close()
			client, err := NewClient(Config{AccessToken: "synthetic-token", BaseURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.(PaymentReader).GetPayment(context.Background(), "synthetic-payment")
			if (err == nil) != probe.ok {
				t.Fatalf("read result: %+v %v", result, err)
			}
			if probe.ok && (result.ReferenceID != "synthetic-intent" || result.LocationID != "synthetic-location" || result.AmountCents != 500) {
				t.Fatal("provider identity fields lost")
			}
		})
	}
}
