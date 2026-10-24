// Package paymentstate contains exact, provider-independent payment arithmetic.
package paymentstate

import "errors"

var ErrRefundAmounts = errors.New("invalid captured or cumulative refunded amount")

// RefundProjection applies an authoritative cumulative refunded total to a
// previously persisted cumulative total. Refunds cannot be undone by duplicate
// notifications or older snapshots. Persist this projection and its audit event
// in the same transaction under the payment row lock.
type RefundProjection struct {
	RefundedCents    int64
	DeltaCents       int64
	NetCapturedCents int64
}

func ReconcileRefund(capturedCents, persistedRefundedCents, observedRefundedCents int64) (RefundProjection, error) {
	if capturedCents <= 0 || persistedRefundedCents < 0 || observedRefundedCents < 0 || persistedRefundedCents > capturedCents || observedRefundedCents > capturedCents {
		return RefundProjection{}, ErrRefundAmounts
	}
	next := persistedRefundedCents
	if observedRefundedCents > next {
		next = observedRefundedCents
	}
	return RefundProjection{RefundedCents: next, DeltaCents: next - persistedRefundedCents, NetCapturedCents: capturedCents - next}, nil
}
