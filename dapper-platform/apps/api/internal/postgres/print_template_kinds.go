package postgres

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Invoices and quotations each have a printed template: one for the organization and, optionally, one per franchise. Both are
// stored the same way (quote_templates.sql mirrors the invoice tables); printKind names the tables.
type printKind struct {
	orgTable, locTable string
	quote              bool
}

var (
	invoicePrint = printKind{orgTable: "invoice_print_templates", locTable: "location_invoice_templates"}
	quotePrint   = printKind{orgTable: "quote_print_templates", locTable: "location_quote_templates", quote: true}
)

// effective is what a franchise prints with. A quotation template that was never saved starts from the invoice template the
// franchise prints with, titled with its quotation title, so quotes look the same until someone edits them.
func (k printKind) effective(ctx context.Context, q rowQuerier, org, loc string) (InvoicePrintTemplateState, error) {
	if !k.quote {
		return loadEffectivePrintTemplate(ctx, q, k, org, loc)
	}
	out, err := loadEffectivePrintTemplate(ctx, q, k, org, loc)
	if err != nil || out.Version > 0 {
		return out, err
	}
	base, err := loadEffectivePrintTemplate(ctx, q, invoicePrint, org, loc)
	if err != nil {
		return out, err
	}
	return InvoicePrintTemplateState{Template: quoteFromInvoice(base.Template), Scope: "organization"}, nil
}

func quoteFromInvoice(t InvoicePrintTemplate) InvoicePrintTemplate {
	t.Title = strings.TrimSpace(t.QuotationTitle)
	if t.Title == "" {
		t.Title = "Quotation"
	}
	t.PaymentInstructions = ""
	return t
}

// QuotePrintTemplate is the organization's quotation template (to any member), started from the invoice template until saved.
func (s *Store) QuotePrintTemplate(ctx context.Context, actor, org string) (InvoicePrintTemplateState, error) {
	if err := s.memberOf(ctx, actor, org); err != nil {
		return InvoicePrintTemplateState{}, err
	}
	out, err := loadOrgPrintTemplate(ctx, s.pool, quotePrint, org)
	if err != nil || out.Version > 0 {
		return out, err
	}
	base, err := loadInvoiceTemplate(ctx, s.pool, org)
	return InvoicePrintTemplateState{Template: quoteFromInvoice(base.Template)}, err
}

func (s *Store) SaveQuotePrintTemplate(ctx context.Context, actor, org string, t InvoicePrintTemplate, version int) (InvoicePrintTemplateState, error) {
	return s.saveOrgPrintTemplate(ctx, quotePrint, actor, org, t, version)
}

func (s *Store) LocationQuotePrintTemplate(ctx context.Context, actor, org, loc string) (InvoicePrintTemplateState, error) {
	return s.locationPrintTemplate(ctx, quotePrint, actor, org, loc)
}

func (s *Store) SaveLocationQuotePrintTemplate(ctx context.Context, actor, org, loc string, t InvoicePrintTemplate, version int) (InvoicePrintTemplateState, error) {
	return s.saveLocationPrintTemplate(ctx, quotePrint, actor, org, loc, t, version)
}

func (s *Store) DeleteLocationQuotePrintTemplate(ctx context.Context, actor, org, loc string) error {
	return s.deleteLocationPrintTemplate(ctx, quotePrint, actor, org, loc)
}

type partyQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// printParties adds what a franchise prints on every document: its business details and its active taxes (several taxes are
// named together and their rates added, e.g. "GST + PST" at 12 per cent).
func printParties(ctx context.Context, q partyQuerier, org, loc string, out *InvoicePrintTemplateState) error {
	var issuer QuoteIssuer
	err := q.QueryRow(ctx, `SELECT i.legal_name,i.address,i.tax_registration FROM location_invoice_issuers l JOIN invoice_issuers i ON i.id=l.issuer_id WHERE l.organization_id=$1 AND l.location_id=$2`, org, loc).Scan(&issuer.LegalName, &issuer.Address, &issuer.TaxRegistration)
	if err == nil {
		out.Issuer = &issuer
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	rows, err := q.Query(ctx, `SELECT name,rate_bp FROM franchise_tax_rates WHERE organization_id=$1 AND location_id=$2 AND active ORDER BY lower(name)`, org, loc)
	if err != nil {
		return err
	}
	defer rows.Close()
	names := []string{}
	total := 0
	for rows.Next() {
		var name string
		var bp int
		if err = rows.Scan(&name, &bp); err != nil {
			return err
		}
		names = append(names, name)
		total += bp
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(names) > 0 {
		out.TaxName = strings.Join(names, " + ")
		out.TaxRatePercent = math.Round(float64(total)) / 100
	}
	return nil
}
