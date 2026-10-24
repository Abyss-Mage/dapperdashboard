package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrInvoiceVersion = errors.New("invoice changed since review")

type UpdateInvoiceDraftInput struct {
	Version  int                      `json:"version"`
	TaxCents int64                    `json:"taxCents"`
	Notes    string                   `json:"notes"`
	Items    []CreateInvoiceItemInput `json:"items"`
}

func invoiceLineTotal(items []CreateInvoiceItemInput, tax int64, notes string) (int64, error) {
	if len(items) < 1 || len(items) > 100 || tax < 0 || utf8.RuneCountInString(strings.TrimSpace(notes)) > 2000 {
		return 0, ErrInvoiceInput
	}
	var subtotal int64
	for _, item := range items {
		n := utf8.RuneCountInString(strings.TrimSpace(item.Description))
		if n < 2 || n > 255 || item.Quantity < 1 || int64(item.Quantity) > math.MaxInt32 || item.UnitPriceCents < 0 || item.UnitPriceCents > math.MaxInt64/int64(item.Quantity) {
			return 0, ErrInvoiceInput
		}
		line := int64(item.Quantity) * item.UnitPriceCents
		if subtotal > math.MaxInt64-line {
			return 0, ErrInvoiceInput
		}
		subtotal += line
	}
	if subtotal > math.MaxInt64-tax {
		return 0, ErrInvoiceInput
	}
	return subtotal, nil
}

func invoiceItemsInTransaction(ctx context.Context, tx pgx.Tx, id string) ([]InvoiceItem, error) {
	rows, err := tx.Query(ctx, `SELECT id::text,invoice_id::text,description,quantity,unit_price_cents,total_price_cents,created_at FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []InvoiceItem{}
	for rows.Next() {
		var item InvoiceItem
		if err = rows.Scan(&item.ID, &item.InvoiceID, &item.Description, &item.Quantity, &item.UnitPriceCents, &item.TotalPriceCents, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateInvoiceDraft(ctx context.Context, scope Scope, id string, input UpdateInvoiceDraftInput) (Invoice, error) {
	if err := s.authorize(ctx, scope, "invoices.manage"); err != nil {
		return Invoice{}, err
	}
	subtotal, err := invoiceLineTotal(input.Items, input.TaxCents, input.Notes)
	if err != nil || input.Version < 1 {
		return Invoice{}, ErrInvoiceInput
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.manage"); err != nil {
		return Invoice{}, err
	}
	before, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3 FOR UPDATE OF i`, scope.OrganizationID, scope.LocationID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}
	if before.Status != "draft" || before.PaidCents != 0 {
		return Invoice{}, ErrInvoiceStatusConflict
	}
	if before.Version != input.Version {
		return Invoice{}, ErrInvoiceVersion
	}
	before.Items, err = invoiceItemsInTransaction(ctx, tx, id)
	if err != nil {
		return Invoice{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM invoice_items WHERE invoice_id=$1`, id); err != nil {
		return Invoice{}, err
	}
	for _, item := range input.Items {
		if _, err = tx.Exec(ctx, `INSERT INTO invoice_items(invoice_id,organization_id,location_id,description,quantity,unit_price_cents,total_price_cents) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, scope.OrganizationID, scope.LocationID, strings.TrimSpace(item.Description), item.Quantity, item.UnitPriceCents, int64(item.Quantity)*item.UnitPriceCents); err != nil {
			return Invoice{}, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE invoices SET subtotal_cents=$4,tax_cents=$5,total_cents=$4::bigint+$5::bigint,due_cents=$4::bigint+$5::bigint,notes=$6,updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id, subtotal, input.TaxCents, strings.TrimSpace(input.Notes)); err != nil {
		return Invoice{}, err
	}
	after, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.id=$1`, id))
	if err != nil {
		return Invoice{}, err
	}
	after.Items, err = invoiceItemsInTransaction(ctx, tx, id)
	if err != nil {
		return Invoice{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_draft_revisions(organization_id,location_id,invoice_id,actor_id,from_version,to_version,before_snapshot,after_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.OrganizationID, scope.LocationID, id, scope.ActorID, before.Version, after.Version, before, after); err != nil {
		return Invoice{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_events(organization_id,location_id,invoice_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,'draft','draft',$5)`, scope.OrganizationID, scope.LocationID, id, scope.ActorID, fmt.Sprintf("Invoice draft revised from version %d to %d", before.Version, after.Version)); err != nil {
		return Invoice{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	return paymentInvoiceSummary(after), nil
}

type InvoiceDraftRevision struct {
	CreatedAt   time.Time `json:"createdAt"`
	ID          int64     `json:"id"`
	ActorID     string    `json:"actorId"`
	FromVersion int       `json:"fromVersion"`
	ToVersion   int       `json:"toVersion"`
	Before      Invoice   `json:"before"`
	After       Invoice   `json:"after"`
}

func (s *Store) InvoiceDraftHistory(ctx context.Context, scope Scope, id string) ([]InvoiceDraftRevision, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.read"); err != nil {
		return nil, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invoices WHERE organization_id=$1 AND location_id=$2 AND id=$3)`, scope.OrganizationID, scope.LocationID, id).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrInvoiceNotFound
	}
	rows, err := tx.Query(ctx, `SELECT id,actor_id::text,from_version,to_version,before_snapshot,after_snapshot,created_at FROM invoice_draft_revisions WHERE organization_id=$1 AND location_id=$2 AND invoice_id=$3 ORDER BY to_version DESC LIMIT 100`, scope.OrganizationID, scope.LocationID, id)
	if err != nil {
		return nil, err
	}
	history := []InvoiceDraftRevision{}
	for rows.Next() {
		var revision InvoiceDraftRevision
		if err = rows.Scan(&revision.ID, &revision.ActorID, &revision.FromVersion, &revision.ToVersion, &revision.Before, &revision.After, &revision.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		history = append(history, revision)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return history, nil
}
