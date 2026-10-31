package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// loadEffectiveInvoiceTemplate is what a franchise prints with: its own template if it has one, else the organization's.
func loadEffectiveInvoiceTemplate(ctx context.Context, q rowQuerier, org, loc string) (InvoicePrintTemplateState, error) {
	out := InvoicePrintTemplateState{Template: DefaultInvoicePrintTemplate(), Scope: "franchise"}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT config,version FROM location_invoice_templates WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&raw, &out.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		base, e := loadInvoiceTemplate(ctx, q, org)
		base.Scope = "organization"
		return base, e
	}
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(raw, &out.Template)
}

func (s *Store) memberOf(ctx context.Context, actor, org string) error {
	var member bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_memberships m JOIN principals p ON p.id=m.principal_id WHERE m.principal_id=$1 AND m.organization_id=$2 AND p.active)`, actor, org).Scan(&member); err != nil {
		return err
	}
	if !member {
		return ErrForbidden
	}
	return nil
}

// LocationInvoicePrintTemplate returns the template a franchise prints with, to any member of the organization.
func (s *Store) LocationInvoicePrintTemplate(ctx context.Context, actor, org, loc string) (InvoicePrintTemplateState, error) {
	if err := s.memberOf(ctx, actor, org); err != nil {
		return InvoicePrintTemplateState{}, err
	}
	out, err := loadEffectiveInvoiceTemplate(ctx, s.pool, org, loc)
	if d := logoDigest(ctx, s.pool, org, loc); err == nil && d != "" {
		out.LogoURL = "/api/v1/organizations/" + org + "/locations/" + loc + "/logo?v=" + d
	}
	return out, err
}

// SaveLocationInvoicePrintTemplate creates or replaces a franchise's own template. It needs invoices.configure, like the
// organization template. version is the franchise template revision the editor loaded (0 when it had none).
func (s *Store) SaveLocationInvoicePrintTemplate(ctx context.Context, actor, org, loc string, t InvoicePrintTemplate, version int) (InvoicePrintTemplateState, error) {
	if err := t.normalize(); err != nil {
		return InvoicePrintTemplateState{}, err
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return InvoicePrintTemplateState{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InvoicePrintTemplateState{}, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return InvoicePrintTemplateState{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM locations WHERE organization_id=$1 AND id=$2)`, org, loc).Scan(&exists); err != nil {
		return InvoicePrintTemplateState{}, err
	}
	if !exists {
		return InvoicePrintTemplateState{}, ErrForbidden
	}
	out := InvoicePrintTemplateState{Template: t, Scope: "franchise"}
	if version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO location_invoice_templates(organization_id,location_id,config,updated_by) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING version`, org, loc, raw, actor).Scan(&out.Version)
	} else {
		err = tx.QueryRow(ctx, `UPDATE location_invoice_templates SET config=$3,version=version+1,updated_by=$4,updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND version=$5 RETURNING version`, org, loc, raw, actor, version).Scan(&out.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return InvoicePrintTemplateState{}, ErrInvoiceTemplateConflict
	}
	if err != nil {
		return InvoicePrintTemplateState{}, err
	}
	return out, tx.Commit(ctx)
}

// DeleteLocationInvoicePrintTemplate removes a franchise's own template so it goes back to the organization's.
func (s *Store) DeleteLocationInvoicePrintTemplate(ctx context.Context, actor, org, loc string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM location_invoice_templates WHERE organization_id=$1 AND location_id=$2`, org, loc); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
