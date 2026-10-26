package postgres

import (
	"context"
	"dapper.local/platform/internal/square"
	"strings"
)

// SquareCredentialConfig is private process configuration, never a request DTO.
// Tokens must be supplied from secure runtime configuration, not source/Obsidian.
type SquareCredentialConfig struct {
	Environment string
	AccessToken string `json:"-"`
}

func (SquareCredentialConfig) String() string { return "Square credential configuration (redacted)" }
func NewSquareCredentialResolver(configuration map[string]SquareCredentialConfig) (SquareCredentialResolver, error) {
	if len(configuration) == 0 {
		return nil, nil
	}
	clients := make(map[string]VerifiedSquareClient, len(configuration))
	environments := make(map[string]string, len(configuration))
	for reference, config := range configuration {
		if len(reference) < 1 || len(reference) > 128 || strings.TrimSpace(reference) != reference {
			return nil, ErrPaymentProvider
		}
		for _, c := range reference {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
				return nil, ErrPaymentProvider
			}
		}
		if (config.Environment != "sandbox" && config.Environment != "production") || strings.TrimSpace(config.AccessToken) == "" {
			return nil, ErrPaymentProvider
		}
		client, err := square.NewClient(square.Config{Environment: config.Environment, AccessToken: config.AccessToken})
		if err != nil {
			return nil, ErrPaymentProvider
		}
		verified, ok := client.(VerifiedSquareClient)
		if !ok {
			return nil, ErrPaymentProvider
		}
		clients[reference] = verified
		environments[reference] = config.Environment
	}
	return func(ctx context.Context, environment, reference string) (VerifiedSquareClient, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		client, ok := clients[reference]
		if !ok || environments[reference] != environment {
			return nil, ErrPaymentProvider
		}
		return client, nil
	}, nil
}
