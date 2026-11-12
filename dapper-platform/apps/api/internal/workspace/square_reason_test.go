package workspace

import (
	"fmt"
	"strings"
	"testing"

	"dapper.local/platform/internal/square"
)

// The portal shows what Square said when a card cannot be saved, not a generic message.
func TestSquareReason(t *testing.T) {
	err := fmt.Errorf("%w: %s (%s)", square.ErrSquarePaymentFailed, "Invalid customer reference", "INVALID_VALUE")
	if got := squareReason(err); got != "Invalid customer reference (INVALID_VALUE)" {
		t.Fatalf("reason %q", got)
	}
	wrapped := fmt.Errorf("%w: %w", fmt.Errorf("payment provider"), fmt.Errorf("%w: %s (%s)", square.ErrSquareRejected, "Card not found", "NOT_FOUND"))
	if got := squareReason(wrapped); got != "Card not found (NOT_FOUND)" {
		t.Fatalf("refusal reason %q", got)
	}
	if got := squareReason(square.ErrSquareScope); !strings.Contains(got, "reconnects Square") {
		t.Fatalf("scope reason %q", got)
	}
}
