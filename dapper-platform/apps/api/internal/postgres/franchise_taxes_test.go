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

func verifyInvoiceTaxCalculation(t *testing.T, ctx context.Context, s *Store, a Scope) {
	t.Helper()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`DELETE FROM franchise_tax_settings WHERE organization_id=$1 AND location_id=$2`, a.OrganizationID, a.LocationID) // the earlier tax test leaves settings behind
	exec(`INSERT INTO franchise_tax_rates(organization_id,location_id,name,rate_bp) VALUES($1,$2,'Synthetic five',500),($1,$2,'Synthetic off',900)`, a.OrganizationID, a.LocationID)
	exec(`UPDATE franchise_tax_rates SET active=false WHERE organization_id=$1 AND location_id=$2 AND name='Synthetic off'`, a.OrganizationID, a.LocationID)
	defer exec(`DELETE FROM franchise_tax_rates WHERE organization_id=$1 AND location_id=$2`, a.OrganizationID, a.LocationID)
	defer exec(`DELETE FROM franchise_tax_settings WHERE organization_id=$1 AND location_id=$2`, a.OrganizationID, a.LocationID)
	// No rounding chosen yet: half-up is used and reported. Only the active 5% applies (10100 * 5% = 505).
	tax, assumed, err := s.taxOnSubtotal(ctx, a, 10100)
	if err != nil || tax != 505 || !assumed {
		t.Fatalf("tax on 10100 at 5%%: %d assumed=%v %v", tax, assumed, err)
	}
	// 1010 cents * 5% = 50.5 cents: half-up 51, half-even 50, down 50.
	for _, c := range []struct {
		mode string
		want int64
	}{{"half_up", 51}, {"half_even", 50}, {"down", 50}} {
		exec(`INSERT INTO franchise_tax_settings(organization_id,location_id,prices_include_tax,rounding) VALUES($1,$2,false,$3) ON CONFLICT(organization_id,location_id) DO UPDATE SET rounding=excluded.rounding`, a.OrganizationID, a.LocationID, c.mode)
		if got, assumed, err := s.taxOnSubtotal(ctx, a, 1010); err != nil || got != c.want || assumed {
			t.Fatalf("%s: got %d want %d assumed=%v %v", c.mode, got, c.want, assumed, err)
		}
	}
	exec(`UPDATE franchise_tax_settings SET prices_include_tax=true WHERE organization_id=$1 AND location_id=$2`, a.OrganizationID, a.LocationID)
	if _, _, err = s.taxOnSubtotal(ctx, a, 1000); !errors.Is(err, ErrTaxInclusiveUnsupported) {
		t.Fatal("tax-inclusive prices were accepted for invoices", err)
	}
}
