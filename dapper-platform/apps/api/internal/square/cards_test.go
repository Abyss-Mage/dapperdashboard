package square

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A Square connection without the customers permission is told apart from a card problem (error404 #42).
func TestCardVaultMissingScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errors":[{"category":"AUTHENTICATION_ERROR","code":"INSUFFICIENT_SCOPES","detail":"no"}]}`))
	}))
	defer srv.Close()
	c, err := NewClient(Config{AccessToken: "test", Environment: "sandbox", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.(CardVault).CreateCustomer(context.Background(), "dc-key", "ref", "Dealer", ""); !errors.Is(err, ErrSquareScope) {
		t.Fatal("missing scope not recognised", err)
	}
}

// A payment Square refuses with an error (4xx) is definite, so the attempt can fail and free the balance; a 5xx stays unknown.
func TestPaymentRefusalIsDefinite(t *testing.T) {
	status := 400
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"NOT_FOUND","detail":"Card not found"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Config{AccessToken: "test", Environment: "sandbox", BaseURL: srv.URL})
	req := PaymentRequest{SourceID: "ccof:x", IdempotencyKey: "k", AmountCents: 100}
	if _, err := c.CreatePayment(context.Background(), req); !errors.Is(err, ErrSquareRejected) || !errors.Is(err, ErrSquarePaymentFailed) {
		t.Fatal("a 400 was not a definite refusal", err)
	}
	status = 503
	if _, err := c.CreatePayment(context.Background(), req); errors.Is(err, ErrSquareRejected) || !errors.Is(err, ErrSquarePaymentFailed) {
		t.Fatal("a 503 was treated as definite", err)
	}
}

// A payment attempt's reference is looked up among the location's payments.
func TestFindPaymentByReference(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"payments":[{"id":"p1","reference_id":"other","status":"COMPLETED"},{"id":"p2","reference_id":"mine","status":"COMPLETED"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Config{AccessToken: "test", Environment: "sandbox", BaseURL: srv.URL})
	f := c.(interface {
		FindPaymentByReference(context.Context, string, string, time.Time) (PaymentResponse, bool, error)
	})
	if p, found, err := f.FindPaymentByReference(context.Background(), "L1", "mine", time.Now()); err != nil || !found || p.ID != "p2" {
		t.Fatal("payment not found", p, found, err)
	}
	if _, found, err := f.FindPaymentByReference(context.Background(), "L1", "missing", time.Now()); err != nil || found {
		t.Fatal("a missing payment was found", found, err)
	}
}
