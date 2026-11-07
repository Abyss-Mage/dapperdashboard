package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Brand-owned outlets and franchisee outlets print invoices with different templates. The brand-owned one is the organization's
// invoice template; the franchisee one is stored beside it (outlet_templates.sql). Either can still be replaced by one outlet's own
// template, and quotations have one template for everybody.
var franchisePrint = printKind{orgTable: "franchise_print_templates", locTable: "location_invoice_templates"}

var ErrOutletType = errors.New("an outlet is either brand-owned or a franchise")

// outletOf says whether a location is "brand" owned or a "franchise" (every location starts as brand-owned).
func outletOf(ctx context.Context, q rowQuerier, org, loc string) (string, error) {
	var o string
	err := q.QueryRow(ctx, `SELECT ownership FROM locations WHERE organization_id=$1 AND id=$2`, org, loc).Scan(&o)
	if errors.Is(err, pgx.ErrNoRows) {
		return "brand", nil
	}
	return o, err
}

// OutletInvoicePrintTemplate is the shared invoice template of one kind of outlet ("brand" or "franchise"), to any member. A
// franchisee template that was never saved starts from the brand-owned one.
func (s *Store) OutletInvoicePrintTemplate(ctx context.Context, actor, org, outlet string) (InvoicePrintTemplateState, error) {
	if err := s.memberOf(ctx, actor, org); err != nil {
		return InvoicePrintTemplateState{}, err
	}
	if outlet != "franchise" {
		return loadInvoiceTemplate(ctx, s.pool, org)
	}
	out, err := loadOrgPrintTemplate(ctx, s.pool, franchisePrint, org)
	if err != nil || out.Version > 0 {
		return out, err
	}
	base, err := loadInvoiceTemplate(ctx, s.pool, org)
	return InvoicePrintTemplateState{Template: base.Template}, err
}

func (s *Store) SaveOutletInvoicePrintTemplate(ctx context.Context, actor, org, outlet string, t InvoicePrintTemplate, version int) (InvoicePrintTemplateState, error) {
	if outlet == "franchise" {
		return s.saveOrgPrintTemplate(ctx, franchisePrint, actor, org, t, version)
	}
	return s.saveOrgPrintTemplate(ctx, invoicePrint, actor, org, t, version)
}

// SetOutletType records whether an outlet is brand-owned or a franchise, so it prints with the matching invoice template. It
// needs invoices.configure, like the templates themselves.
func (s *Store) SetOutletType(ctx context.Context, actor, org, loc, outlet string) error {
	if outlet != "brand" && outlet != "franchise" {
		return ErrOutletType
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE locations SET ownership=$3 WHERE organization_id=$1 AND id=$2`, org, loc, outlet)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOutletType
	}
	return tx.Commit(ctx)
}
