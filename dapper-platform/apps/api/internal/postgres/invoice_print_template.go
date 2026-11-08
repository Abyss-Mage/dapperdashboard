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
	// ValidDays is the quotation template's default "valid until" (days after the quote is made); 0 leaves it empty.
	ValidDays int `json:"validDays"`
}

// InvoicePrintTemplateState is what the API returns: the template plus its revision (0 = built-in default, never saved).
type InvoicePrintTemplateState struct {
	Template InvoicePrintTemplate `json:"template"`
	Version  int                  `json:"version"`
	// Scope says which template this is: "organization" (shared) or "franchise" (one location's own).
	Scope string `json:"scope,omitempty"`
	// Outlet is, for one location, whether it is "brand" owned or a "franchise" (which invoice template it prints with).
	Outlet string `json:"outlet,omitempty"`
	// LogoURL is the franchise's own logo (empty when it has none and the default logo applies).
	LogoURL string `json:"logoUrl,omitempty"`
	// For one franchise: the business details its invoices and quotations print (so drafts and quotes show them too) and its
	// active tax, e.g. "HST" at 13 per cent, so the tax line can name the rate.
	Issuer         *QuoteIssuer `json:"issuer,omitempty"`
	TaxName        string       `json:"taxName,omitempty"`
	TaxRatePercent float64      `json:"taxRatePercent,omitempty"`
}

func DefaultInvoicePrintTemplate() InvoicePrintTemplate {
	return InvoicePrintTemplate{Title: "Invoice", AccentColor: "#e4202e", ShowLogo: true, BillToLabel: "Bill to", ItemLabel: "Item", QuantityLabel: "Qty", UnitPriceLabel: "Unit price", AmountLabel: "Amount", ShowTaxRegistration: true, ShowCustomerContact: true, FooterNote: "Thank you for choosing Dapper Detailing Pros.", PaperSize: "letter", QuotationTitle: "Quotation"}
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
	if utf8.RuneCountInString(t.QuotationTitle) > 40 || utf8.RuneCountInString(t.QuotationNote) > 1000 || t.ValidDays < 0 || t.ValidDays > 365 {
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
	return loadOrgPrintTemplate(ctx, q, invoicePrint, org)
}

func loadOrgPrintTemplate(ctx context.Context, q rowQuerier, kind printKind, org string) (InvoicePrintTemplateState, error) {
	out := InvoicePrintTemplateState{Template: DefaultInvoicePrintTemplate()}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT config,version FROM `+kind.orgTable+` WHERE organization_id=$1`, org).Scan(&raw, &out.Version)
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
	if err == nil {
		out.Outlet, err = outletOf(ctx, tx, a.OrganizationID, a.LocationID)
	}
	if err == nil {
		err = printParties(ctx, tx, a.OrganizationID, a.LocationID, &out)
	}
	return out, err
}

// customerLocation is where a signed-in customer's account lives.
func (s *Store) customerLocation(ctx context.Context, actor, customer string) (org, loc string, err error) {
	err = s.pool.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2`, actor, customer).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrForbidden
	}
	return org, loc, err
}

// CustomerInvoicePrintTemplate is the look of the invoices a customer opens from their account. A custom HTML layout is left out: it is
// written for staff screens and has no place running in a customer's signed-in session, so customers get the built-in layout with its wording.
func (s *Store) CustomerInvoicePrintTemplate(ctx context.Context, actor, customer string) (InvoicePrintTemplateState, error) {
	org, loc, err := s.customerLocation(ctx, actor, customer)
	if err != nil {
		return InvoicePrintTemplateState{}, err
	}
	out, err := loadEffectiveInvoiceTemplate(ctx, s.pool, org, loc)
	if err != nil {
		return out, err
	}
	out.Template.CustomHTML, out.Template.CustomCSS = "", ""
	if out.Outlet, err = outletOf(ctx, s.pool, org, loc); err != nil {
		return out, err
	}
	if d := logoDigest(ctx, s.pool, org, loc); d != "" {
		out.LogoURL = "/api/v1/portal/accounts/" + customer + "/logo?v=" + d
	}
	return out, printParties(ctx, s.pool, org, loc, &out)
}

// CustomerLocationLogo is the logo of the franchise a customer's account belongs to.
func (s *Store) CustomerLocationLogo(ctx context.Context, actor, customer string) (contentType string, data []byte, digest string, found bool, err error) {
	org, loc, err := s.customerLocation(ctx, actor, customer)
	if err != nil {
		return
	}
	return s.readLocationLogo(ctx, org, loc)
}

// SaveInvoicePrintTemplate needs invoices.configure. version is the revision the editor loaded (0 when it was the unsaved default).
func (s *Store) SaveInvoicePrintTemplate(ctx context.Context, actor, org string, t InvoicePrintTemplate, version int) (InvoicePrintTemplateState, error) {
	return s.saveOrgPrintTemplate(ctx, invoicePrint, actor, org, t, version)
}

func (s *Store) saveOrgPrintTemplate(ctx context.Context, kind printKind, actor, org string, t InvoicePrintTemplate, version int) (InvoicePrintTemplateState, error) {
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
		err = tx.QueryRow(ctx, `INSERT INTO `+kind.orgTable+`(organization_id,config,updated_by) VALUES($1,$2,$3) ON CONFLICT DO NOTHING RETURNING version`, org, raw, actor).Scan(&out.Version)
	} else {
		err = tx.QueryRow(ctx, `UPDATE `+kind.orgTable+` SET config=$2,version=version+1,updated_by=$3,updated_at=now() WHERE organization_id=$1 AND version=$4 RETURNING version`, org, raw, actor, version).Scan(&out.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return InvoicePrintTemplateState{}, ErrInvoiceTemplateConflict
	}
	if err != nil {
		return InvoicePrintTemplateState{}, err
	}
	return out, tx.Commit(ctx)
}
