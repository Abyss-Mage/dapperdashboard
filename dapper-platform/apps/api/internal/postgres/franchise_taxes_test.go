package postgres

import (
	"context"
	"errors"
	"testing"
)

func verifyFranchiseTaxes(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	// a holds invoices.read and invoices.configure (granted by the caller); b holds nothing.
	rate, err := s.CreateTaxRate(ctx, a, "Synthetic HST", 1300)
	if err != nil || rate.RateBP != 1300 || !rate.Active || rate.Version != 1 {
		t.Fatal("tax rate create failed", err)
	}
	if _, err = s.CreateTaxRate(ctx, a, "synthetic hst ", 500); !errors.Is(err, ErrTaxConflict) {
		t.Fatal("duplicate tax name accepted", err)
	}
	if _, err = s.CreateTaxRate(ctx, a, "X", 500); !errors.Is(err, ErrTaxInput) {
		t.Fatal("short name accepted", err)
	}
	if _, err = s.CreateTaxRate(ctx, a, "Too high", 5001); !errors.Is(err, ErrTaxInput) {
		t.Fatal("rate above 50% accepted", err)
	}
	if _, err = s.CreateTaxRate(ctx, b, "Foreign", 100); !errors.Is(err, ErrForbidden) {
		t.Fatal("tax created without invoices.configure", err)
	}
	if _, err = s.FranchiseTaxes(ctx, b); !errors.Is(err, ErrForbidden) {
		t.Fatal("taxes readable without invoices.read", err)
	}
	up, err := s.UpdateTaxRate(ctx, a, rate.ID, "Synthetic HST", 1500, true, 1)
	if err != nil || up.Version != 2 || up.RateBP != 1500 {
		t.Fatal("tax update failed", err)
	}
	if _, err = s.UpdateTaxRate(ctx, a, rate.ID, "Synthetic HST", 1300, true, 1); !errors.Is(err, ErrTaxConflict) {
		t.Fatal("stale tax revision accepted", err)
	}
	st, err := s.SaveTaxSettings(ctx, a, true, "half_up", 0)
	if err != nil || st.Version != 1 || !st.PricesIncludeTax || st.Rounding == nil {
		t.Fatal("tax settings failed", err)
	}
	if _, err = s.SaveTaxSettings(ctx, a, false, "half_up", 0); !errors.Is(err, ErrTaxConflict) {
		t.Fatal("stale settings accepted", err)
	}
	if _, err = s.SaveTaxSettings(ctx, a, false, "bogus", 1); !errors.Is(err, ErrTaxInput) {
		t.Fatal("unknown rounding accepted", err)
	}
	got, err := s.FranchiseTaxes(ctx, a)
	if err != nil || len(got.Rates) != 1 || got.Rates[0].RateBP != 1500 || got.Settings.Version != 1 {
		t.Fatal("tax read wrong", err)
	}
	var events int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM franchise_tax_events WHERE location_id=$1`, a.LocationID).Scan(&events); err != nil || events != 3 {
		t.Fatalf("tax history incomplete: %d %v", events, err)
	}
	if err = s.DeleteTaxRate(ctx, a, rate.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteTaxRate(ctx, a, rate.ID); !errors.Is(err, ErrTaxConflict) {
		t.Fatal("double delete accepted", err)
	}
}
