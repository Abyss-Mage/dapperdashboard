package paymentstate

import (
	"errors"
	"math"
	"testing"
)

func TestCumulativeRefundSequence(t *testing.T) {
	prior := int64(0)
	for _, step := range []struct{ observed, delta, net int64 }{{1000, 1000, 4000}, {2000, 1000, 3000}, {2000, 0, 3000}, {1000, 0, 3000}, {0, 0, 3000}, {5000, 3000, 0}, {2000, 0, 0}, {5000, 0, 0}} {
		result, err := ReconcileRefund(5000, prior, step.observed)
		if err != nil {
			t.Fatal(err)
		}
		if result.DeltaCents != step.delta || result.NetCapturedCents != step.net {
			t.Fatalf("prior%d observed%d: %+v", prior, step.observed, result)
		}
		prior = result.RefundedCents
	}
}
func TestRefundAmountBounds(t *testing.T) {
	for _, input := range [][3]int64{{0, 0, 0}, {-1, 0, 0}, {100, -1, 0}, {100, 0, -1}, {100, 101, 0}, {100, 0, 101}} {
		if _, err := ReconcileRefund(input[0], input[1], input[2]); !errors.Is(err, ErrRefundAmounts) {
			t.Fatalf("accepted invalid amounts%v", input)
		}
	}
	result, err := ReconcileRefund(math.MaxInt64, math.MaxInt64-1, math.MaxInt64)
	if err != nil || result.DeltaCents != 1 || result.NetCapturedCents != 0 {
		t.Fatalf("overflow boundary: %+v %v", result, err)
	}
}
func TestRefundAccountingConservation(t *testing.T) {
	for captured := int64(1); captured <= 60; captured++ {
		for prior := int64(0); prior <= captured; prior++ {
			for observed := int64(0); observed <= captured; observed++ {
				result, err := ReconcileRefund(captured, prior, observed)
				if err != nil {
					t.Fatal(err)
				}
				if result.RefundedCents+result.NetCapturedCents != captured || result.DeltaCents < 0 || prior+result.DeltaCents != result.RefundedCents {
					t.Fatalf("conservation failed: %+v", result)
				}
				replay, err := ReconcileRefund(captured, result.RefundedCents, observed)
				if err != nil || replay.DeltaCents != 0 || replay.NetCapturedCents != result.NetCapturedCents {
					t.Fatal("replay changed net amount")
				}
			}
		}
	}
}
