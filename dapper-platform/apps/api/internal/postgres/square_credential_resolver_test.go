package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSquareCredentialResolver(t *testing.T) {
	if resolver, err := NewSquareCredentialResolver(nil); err != nil || resolver != nil {
		t.Fatal("unconfigured resolver enabled")
	}
	configuration := map[string]SquareCredentialConfig{"synthetic-franchise": {Environment: "sandbox", AccessToken: "synthetic-secret-not-real"}}
	resolver, err := NewSquareCredentialResolver(configuration)
	if err != nil {
		t.Fatal(err)
	}
	delete(configuration, "synthetic-franchise")
	if client, err := resolver(context.Background(), "sandbox", "synthetic-franchise"); err != nil || client == nil {
		t.Fatal("resolver did not preserve configuration snapshot")
	}
	for _, pair := range [][2]string{{"production", "synthetic-franchise"}, {"sandbox", "unknown"}, {"", "synthetic-franchise"}} {
		if _, err := resolver(context.Background(), pair[0], pair[1]); !errors.Is(err, ErrPaymentProvider) {
			t.Fatal("resolver used unbound/default credential")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolver(cancelled, "sandbox", "synthetic-franchise"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled credential lookup continued")
	}
	for _, invalid := range []map[string]SquareCredentialConfig{{"default": {Environment: "", AccessToken: "synthetic-secret-not-real"}}, {"default": {Environment: "production", AccessToken: ""}}, {"bad\nkey": {Environment: "sandbox", AccessToken: "synthetic-secret-not-real"}}} {
		if _, err := NewSquareCredentialResolver(invalid); !errors.Is(err, ErrPaymentProvider) {
			t.Fatal("invalid private configuration accepted")
		}
	}
	safe := SquareCredentialConfig{Environment: "sandbox", AccessToken: "synthetic-secret-not-real"}
	encoded, err := json.Marshal(safe)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), safe.AccessToken) || strings.Contains(fmt.Sprint(safe), safe.AccessToken) {
		t.Fatal("credential serialization/log representation leaked token")
	}
}
