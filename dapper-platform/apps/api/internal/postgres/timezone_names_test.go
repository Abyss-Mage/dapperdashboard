package postgres

import (
	"testing"
	"time"
)

func TestLegacyZoneNamesAreMapped(t *testing.T) {
	if got := canonicalZone("Asia/Calcutta"); got != "Asia/Kolkata" {
		t.Fatalf("Asia/Calcutta maps to %q", got)
	}
	if got := canonicalZone("America/Toronto"); got != "America/Toronto" {
		t.Fatalf("a current name changed: %q", got)
	}
	for old, current := range legacyZones {
		if _, err := time.LoadLocation(current); err != nil {
			t.Fatalf("%s maps to %s, which is not a known zone: %v", old, current, err)
		}
	}
}
