package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrInvoiceTemplateInput = errors.New("invoice template values are not valid")
var ErrInvoiceTemplateConflict = errors.New("invoice template changed since it was loaded")

// InvoicePrintTemplate is presentation only; the invoice supplies every amount, party and number.
type InvoicePrintTemplate struct {
	Title               string `json:"title"`
	AccentColor         string `json:"accentColor"`
	ShowLogo            bool   `json:"showLogo"`
	HeaderNote          string `json:"headerNote"`
	BillToLabel         string `json:"billToLabel"`
	ItemLabel           string `json:"itemLabel"`
	QuantityLabel       string `json:"quantityLabel"`
	UnitPriceLabel      string `json:"unitPriceLabel"`
	AmountLabel         string `json:"amountLabel"`
	ShowTaxRegistration bool   `json:"showTaxRegistration"`
	ShowCustomerContact bool   `json:"showCustomerContact"`
	PaymentInstructions string `json:"paymentInstructions"`
	FooterNote          string `json:"footerNote"`
	PaperSize           string `json:"paperSize"`
	// Optional owner-written layout. When CustomHTML is set it replaces the built-in layout; {{placeholders}} are filled from the
	// invoice when printing, and it is shown in a sandbox where scripts never run.
	CustomHTML string `json:"customHtml"`
	CustomCSS  string `json:"customCss"`
	// Quotations use the same look. These two are their own wording: the document title and the terms printed under the total.
	QuotationTitle string `json:"quotationTitle"`
	QuotationNote  string `json:"quotationNote"`
}

// InvoicePrintTemplateState is what the API returns: the template plus its revision (0 = built-in default, never saved).
type InvoicePrintTemplateState struct {
	Template InvoicePrintTemplate `json:"template"`
	Version  int                  `json:"version"`
	// Scope says which template this is: "organization" (shared) or "franchise" (one location's own).
	Scope string `json:"scope,omitempty"`
	// LogoURL is the franchise's own logo (empty when it has none and the default logo applies).
	LogoURL string `json:"logoUrl,omitempty"`
}

func DefaultInvoicePrintTemplate() InvoicePrintTemplate {
	return InvoicePrintTemplate{Title: "Invoice", AccentColor: "#111111", ShowLogo: true, BillToLabel: "Bill to", ItemLabel: "Item", QuantityLabel: "Qty", UnitPriceLabel: "Unit price", AmountLabel: "Amount", ShowTaxRegistration: true, ShowCustomerContact: true, PaperSize: "letter"}
}

var templateColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (t *InvoicePrintTemplate) normalize() error {
	for _, f := range []*string{&t.Title, &t.HeaderNote, &t.BillToLabel, &t.ItemLabel, &t.QuantityLabel, &t.UnitPriceLabel, &t.AmountLabel, &t.PaymentInstructions, &t.FooterNote} {
		*f = strings.TrimSpace(*f)
	}
	short := []struct {
		v   string
		max int
	}{{t.Title, 40}, {t.BillToLabel, 30}, {t.ItemLabel, 30}, {t.QuantityLabel, 20}, {t.UnitPriceLabel, 30}, {t.AmountLabel, 30}}
	for _, f := range short {
		if n := utf8.RuneCountInString(f.v); n > f.max || n == 0 {
			return ErrInvoiceTemplateInput
		}
	}
	if utf8.RuneCountInString(t.HeaderNote) > 500 || utf8.RuneCountInString(t.PaymentInstructions) > 1000 || utf8.RuneCountInString(t.FooterNote) > 1000 {
		return ErrInvoiceTemplateInput
	}
	if !templateColor.MatchString(t.AccentColor) || t.PaperSize != "letter" && t.PaperSize != "a4" {
		return ErrInvoiceTemplateInput
	}
	t.QuotationTitle, t.QuotationNote = strings.TrimSpace(t.QuotationTitle), strings.TrimSpace(t.QuotationNote)
	if t.QuotationTitle == "" {
		t.QuotationTitle = "Quotation"
	}
	if utf8.RuneCountInString(t.QuotationTitle) > 40 || utf8.RuneCountInString(t.QuotationNote) > 1000 {
		return ErrInvoiceTemplateInput
	}
	t.CustomHTML, t.CustomCSS = strings.TrimSpace(t.CustomHTML), strings.TrimSpace(t.CustomCSS)
	if len(t.CustomHTML) > 60000 || len(t.CustomCSS) > 60000 {
		return ErrInvoiceTemplateInput
	}
	// Layouts are HTML and CSS only: no scripts, frames or plug-ins.
	lower := strings.ToLower(t.CustomHTML + t.CustomCSS)
	for _, banned := range []string{"<script", "javascript:", "<iframe", "<object", "<embed", "<base", "<meta"} {
		if strings.Contains(lower, banned) {
			return ErrInvoiceTemplateInput
		}
	}
	return nil
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadInvoiceTemplate(ctx context.Context, q rowQuerier, org string) (InvoicePrintTemplateState, error) {
	out := InvoicePrintTemplateState{Template: DefaultInvoicePrintTemplate()}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT config,version FROM invoice_print_templates WHERE organization_id=$1`, org).Scan(&raw, &out.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	// Unmarshal over the defaults so a field added later still has a value for templates saved before it existed.
	return out, json.Unmarshal(raw, &out.Template)
}

// InvoicePrintTemplate returns the organization's template to any member (it is print wording, not private data).
func (s *Store) InvoicePrintTemplate(ctx context.Context, actor, org string) (InvoicePrintTemplateState, error) {
	var member bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_memberships m JOIN principals p ON p.id=m.principal_id WHERE m.principal_id=$1 AND m.organization_id=$2 AND p.active)`, actor, org).Scan(&member); err != nil {
		return InvoicePrintTemplateState{}, err
	}
	if !member {
		return InvoicePrintTemplateState{}, ErrForbidden
	}
	return loadInvoiceTemplate(ctx, s.pool, org)
}

// EmployeeInvoicePrintTemplate is the template of the organization the employee works for.
func (s *Store) EmployeeInvoicePrintTemplate(ctx context.Context, actor string) (InvoicePrintTemplateState, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InvoicePrintTemplateState{}, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return InvoicePrintTemplateState{}, err
	}
	out, err := loadEffectiveInvoiceTemplate(ctx, tx, a.OrganizationID, a.LocationID)
	if d := logoDigest(ctx, tx, a.OrganizationID, a.LocationID); err == nil && d != "" {
		out.LogoURL = "/api/v1/employee/logo?v=" + d
	}
	return out, err
}

// SaveInvoicePrintTemplate needs invoices.configure. version is the revision the editor loaded (0 when it was the unsaved default).
func (s *Store) SaveInvoicePrintTemplate(ctx context.Context, actor, org string, t InvoicePrintTemplate, version int) (InvoicePrintTemplateState, error) {
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
	out := InvoicePrintTemplateState{Template: t}
	if version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO invoice_print_templates(organization_id,config,updated_by) VALUES($1,$2,$3) ON CONFLICT DO NOTHING RETURNING version`, org, raw, actor).Scan(&out.Version)
	} else {
		err = tx.QueryRow(ctx, `UPDATE invoice_print_templates SET config=$2,version=version+1,updated_by=$3,updated_at=now() WHERE organization_id=$1 AND version=$4 RETURNING version`, org, raw, actor, version).Scan(&out.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return InvoicePrintTemplateState{}, ErrInvoiceTemplateConflict
	}
	if err != nil {
		return InvoicePrintTemplateState{}, err
	}
	return out, tx.Commit(ctx)
}
