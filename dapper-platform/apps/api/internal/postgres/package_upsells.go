package postgres

import (
	"context"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// PackageUpsell offers a bigger package when a smaller one is chosen. Packages are matched by name without the vehicle, so one offer covers
// every vehicle and the booking flow prices the difference from that vehicle's own packages.
type PackageUpsell struct {
	FromPack string   `json:"fromPack"`
	ToPack   string   `json:"toPack"`
	Pitch    string   `json:"pitch"`
	Perks    []string `json:"perks"`
	Active   bool     `json:"active"`
	// A discount off the upgraded package when the offer is taken online: a percent or an amount (error404 #34). Zero: none.
	DiscountPercent float64 `json:"discountPercent"`
	DiscountCents   int64   `json:"discountCents"`
}

var ErrUpsellInput = errors.New("upgrade offer input invalid")

func (u *PackageUpsell) clean() error {
	u.FromPack, u.ToPack, u.Pitch = strings.TrimSpace(u.FromPack), strings.TrimSpace(u.ToPack), strings.TrimSpace(u.Pitch)
	perks := []string{}
	for _, p := range u.Perks {
		if p = strings.TrimSpace(p); p != "" {
			if utf8.RuneCountInString(p) > 120 {
				return ErrUpsellInput
			}
			perks = append(perks, p)
		}
	}
	u.Perks = perks
	if u.DiscountPercent < 0 || u.DiscountPercent > 100 || u.DiscountCents < 0 || u.DiscountCents > 100_000_000 || (u.DiscountPercent > 0 && u.DiscountCents > 0) || math.Abs(math.Round(u.DiscountPercent*100)-u.DiscountPercent*100) > 1e-6 {
		return ErrUpsellInput
	}
	if !validContentText(u.FromPack, 2, 120) || !validContentText(u.ToPack, 2, 120) || strings.EqualFold(u.FromPack, u.ToPack) || utf8.RuneCountInString(u.Pitch) > 200 || len(perks) > 6 {
		return ErrUpsellInput
	}
	return nil
}

func readUpsells(ctx context.Context, tx pgx.Tx, org string, activeOnly bool) ([]PackageUpsell, error) {
	rows, err := tx.Query(ctx, `SELECT from_pack,to_pack,pitch,perks,active,coalesce(discount_percent,0)::float8,discount_cents FROM package_upsells WHERE organization_id=$1 AND (active OR NOT $2) ORDER BY lower(from_pack) LIMIT 300`, org, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PackageUpsell{}
	for rows.Next() {
		var u PackageUpsell
		if err = rows.Scan(&u.FromPack, &u.ToPack, &u.Pitch, &u.Perks, &u.Active, &u.DiscountPercent, &u.DiscountCents); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// PackageUpsells lists the organization's upgrade offers (staff who can read the catalogue).
func (s *Store) PackageUpsells(ctx context.Context, actor, org string) ([]PackageUpsell, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = centralCatalogueGrant(ctx, tx, actor, org, "catalogue.read"); err != nil {
		return nil, err
	}
	return readUpsells(ctx, tx, org, false)
}

// SavePackageUpsell adds or replaces the offer for one package.
func (s *Store) SavePackageUpsell(ctx context.Context, actor, org string, u PackageUpsell) (PackageUpsell, error) {
	if err := u.clean(); err != nil {
		return PackageUpsell{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PackageUpsell{}, err
	}
	defer tx.Rollback(ctx)
	if err = centralCatalogueGrant(ctx, tx, actor, org, "catalogue.manage"); err != nil {
		return PackageUpsell{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM package_upsells WHERE organization_id=$1 AND lower(btrim(from_pack))=lower($2)`, org, u.FromPack); err != nil {
		return PackageUpsell{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO package_upsells(organization_id,from_pack,to_pack,pitch,perks,active,discount_percent,discount_cents) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7::numeric,0),$8)`, org, u.FromPack, u.ToPack, u.Pitch, u.Perks, u.Active, u.DiscountPercent, u.DiscountCents); err != nil {
		return PackageUpsell{}, err
	}
	return u, tx.Commit(ctx)
}

// DeletePackageUpsell removes the offer for one package.
func (s *Store) DeletePackageUpsell(ctx context.Context, actor, org, fromPack string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = centralCatalogueGrant(ctx, tx, actor, org, "catalogue.manage"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM package_upsells WHERE organization_id=$1 AND lower(btrim(from_pack))=lower(btrim($2))`, org, fromPack); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// upsellDiscount is what an upgrade offer takes off when a customer upgraded from fromPack and booked serviceName (whose package price is
// packCents): zero when there is no switched-on offer from that package to this one.
func upsellDiscount(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, org, fromPack, serviceName string, packCents int64) (int64, error) {
	fromPack = strings.TrimSpace(fromPack)
	if fromPack == "" || packCents <= 0 {
		return 0, nil
	}
	var toPack string
	var pct float64
	var cents int64
	err := q.QueryRow(ctx, `SELECT to_pack,coalesce(discount_percent,0)::float8,discount_cents FROM package_upsells WHERE organization_id=$1 AND lower(btrim(from_pack))=lower($2) AND active`, org, fromPack).Scan(&toPack, &pct, &cents)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	// The booked package must be the offer's target, for any vehicle ("Sedan - Luxury Full Body Detail" ends with "Luxury Full Body Detail").
	name := strings.ToLower(strings.TrimSpace(serviceName))
	target := strings.ToLower(strings.TrimSpace(toPack))
	if name != target && !strings.HasSuffix(name, " "+target) && !strings.HasSuffix(name, "-"+target) {
		return 0, nil
	}
	d := cents
	if pct > 0 {
		d = int64(math.Round(float64(packCents) * pct / 100))
	}
	return min(d, packCents), nil
}

// applyUpgradeDiscount takes an upgrade offer's discount off a draft booking invoice, with the tax worked out again on what is left.
func (s *Store) applyUpgradeDiscount(ctx context.Context, scope Scope, invoiceID string, off int64) error {
	var subtotal int64
	if err := s.pool.QueryRow(ctx, `SELECT subtotal_cents FROM invoices WHERE id::text=$1 AND organization_id=$2 AND location_id=$3 AND status='draft'`, invoiceID, scope.OrganizationID, scope.LocationID).Scan(&subtotal); err != nil {
		return err
	}
	off = min(off, subtotal)
	tax, _, err := s.taxOnSubtotal(ctx, scope, subtotal-off)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE invoices SET discount_cents=$2,tax_cents=$3,total_cents=subtotal_cents-$2+$3,due_cents=subtotal_cents-$2+$3-paid_cents,updated_at=now() WHERE id::text=$1 AND status='draft'`, invoiceID, off, tax); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_events(invoice_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1::uuid,$2,$3,$4,'draft','draft','Upgrade offer discount applied')`, invoiceID, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
