package postgres

import (
	"context"
	"dapper.local/platform/internal/square"
)

type syntheticVerifiedSquareClient struct {
	square.MockClient
	identityErr error
	checks      int
	getPayment  func(context.Context, string) (square.PaymentResponse, error)
}

func (c *syntheticVerifiedSquareClient) VerifyLocation(_ context.Context, _ square.LocationBinding) error {
	c.checks++
	return c.identityErr
}

func (c *syntheticVerifiedSquareClient) GetPayment(ctx context.Context, id string) (square.PaymentResponse, error) {
	if c.getPayment == nil {
		return square.PaymentResponse{}, square.ErrSquarePaymentFailed
	}
	return c.getPayment(ctx, id)
}
