package postgres

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Dealership (B2B) invoices: ordinary invoices for the dealership's customer record, tied to the dealership, with payment terms, the
// dealership number series (DLR-1 ...), VIN / stock number per line and the dealership template. They are issued, paid, voided and
// shown in the customer account like every other invoice; this file adds what is different.

var (
	ErrDealerInvoiceInput = errors.New("dealership invoice input invalid")
	ErrDealerNumberTaken  = errors.New("that invoice number is already used")
	vinPattern            = regexp.MustCompile(`^[A-Z0-9]{0,20}$`)
	manualNumberPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ./#-]{0,39}$`)
)

// DealerInvoiceLine is one line: the item, an optional description under it, quantity and rate. VIN / stock number are kept for
// invoices made before 2026-11-11 (the owner then dropped them from the form).
type DealerInvoiceLine struct {
	Description    string `json:"description"`
	Detail         string `json:"detail"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unitPriceCents"`
	VIN            string `json:"vin"`
	StockNumber    string `json:"stockNumber"`
}

type DealerInvoiceInput struct {
	DealershipID    string              `json:"dealershipId"`
	InvoiceDate     string              `json:"invoiceDate"`   // YYYY-MM-DD; empty: today
	DueDate         string              `json:"dueDate"`       // YYYY-MM-DD; empty: 30 days after the invoice date
	InvoiceNumber   string              `json:"invoiceNumber"` // typed by staff; empty: the next number in the dealership series
	PONumber        string              `json:"poNumber"`
	DiscountCents   int64               `json:"discountCents"`   // a fixed discount, or
	DiscountPercent float64             `json:"discountPercent"` // a percent of the subtotal; either is taken off before tax
	Notes           string              `json:"notes"`
	Lines           []DealerInvoiceLine `json:"lines"`
	Version         int                 `json:"version"`
	invoiceDate     time.Time
	dueDate         time.Time
}

type DealerInvoiceSummary struct {
	ID             string     `json:"id"`
	InvoiceNumber  string     `json:"invoiceNumber"`
	DealershipID   string     `json:"dealershipId"`
	DealershipName string     `json:"dealershipName"`
	Status         string     `json:"status"`
	Overdue        bool       `json:"overdue"`
	ViewedAt       *time.Time `json:"viewedAt,omitempty"`
	InvoiceDate    string     `json:"invoiceDate,omitempty"`
	PONumber       string     `json:"poNumber"`
	IssuedAt       *time.Time `json:"issuedAt,omitempty"`
	DueAt          *time.Time `json:"dueAt,omitempty"`
	DueDate        string     `json:"dueDate,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	TotalCents     int64      `json:"totalCents"`
	DueCents       int64      `json:"dueCents"`
}

type DealerInvoice struct {
	Invoice
	DealershipID    string              `json:"dealershipId"`
	DealershipName  string              `json:"dealershipName"`
	TermsDays       int                 `json:"termsDays"`
	InvoiceDate     string              `json:"invoiceDate,omitempty"`
	DueDate         string              `json:"dueDate,omitempty"`
	PONumber        string              `json:"poNumber"`
	ManualNumber    string              `json:"manualNumber"`
	DiscountPercent float64             `json:"discountPercent"`
	ViewedAt        *time.Time          `json:"viewedAt,omitempty"`
	Overdue         bool                `json:"overdue"`
	Lines           []DealerInvoiceLine `json:"lines"`
}

type DealerInvoiceFilter struct {
	DealershipID string
	From, To     string // invoice dates (or, for drafts without one, creation dates), YYYY-MM-DD, inclusive
	Status       string // draft, issued (unpaid), partially_paid, paid, void, overdue, or empty for all
}

func dealerToday() time.Time {
	y, m, d := time.Now().In(documentZone()).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (in *DealerInvoiceInput) clean() ([]CreateInvoiceItemInput, error) {
	in.DealershipID, in.Notes, in.PONumber, in.InvoiceNumber = strings.TrimSpace(in.DealershipID), strings.TrimSpace(in.Notes), strings.TrimSpace(in.PONumber), strings.TrimSpace(in.InvoiceNumber)
	if in.DealershipID == "" || len(in.Lines) == 0 || len(in.Lines) > 100 || utf8.RuneCountInString(in.PONumber) > 40 || (in.InvoiceNumber != "" && !manualNumberPattern.MatchString(in.InvoiceNumber)) {
		return nil, ErrDealerInvoiceInput
	}
	if in.DiscountCents < 0 || in.DiscountPercent < 0 || in.DiscountPercent > 100 || (in.DiscountCents > 0 && in.DiscountPercent > 0) || math.Abs(math.Round(in.DiscountPercent*100)-in.DiscountPercent*100) > 1e-6 {
		return nil, ErrDealerInvoiceInput
	}
	in.invoiceDate = dealerToday()
	if in.InvoiceDate != "" {
		t, err := time.Parse("2006-01-02", in.InvoiceDate)
		if err != nil {
			return nil, ErrDealerInvoiceInput
		}
		in.invoiceDate = t
	}
	in.dueDate = in.invoiceDate.AddDate(0, 0, 30)
	if in.DueDate != "" {
		t, err := time.Parse("2006-01-02", in.DueDate)
		if err != nil {
			return nil, ErrDealerInvoiceInput
		}
		in.dueDate = t
	}
	if in.dueDate.Before(in.invoiceDate) || in.dueDate.After(in.invoiceDate.AddDate(1, 0, 0)) || in.invoiceDate.Year() < 2000 || in.invoiceDate.After(dealerToday().AddDate(1, 0, 0)) {
		return nil, ErrDealerInvoiceInput
	}
	items := make([]CreateInvoiceItemInput, len(in.Lines))
	for i := range in.Lines {
		l := &in.Lines[i]
		l.Description, l.Detail, l.StockNumber = strings.TrimSpace(l.Description), strings.TrimSpace(l.Detail), strings.TrimSpace(l.StockNumber)
		l.VIN = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(l.VIN), " ", ""))
		if !vinPattern.MatchString(l.VIN) || utf8.RuneCountInString(l.StockNumber) > 40 || utf8.RuneCountInString(l.Detail) > 500 {
			return nil, ErrDealerInvoiceInput
		}
		items[i] = CreateInvoiceItemInput{Description: l.Description, Quantity: l.Quantity, UnitPriceCents: l.UnitPriceCents}
	}
	return items, nil
}

// discountOn is the discount in cents on a subtotal; it cannot be more than the subtotal.
func (in DealerInvoiceInput) discountOn(subtotal int64) (int64, error) {
	d := in.DiscountCents
	if in.DiscountPercent > 0 {
		d = int64(math.Round(float64(subtotal) * in.DiscountPercent / 100))
	}
	if d > subtotal {
		return 0, ErrDealerInvoiceInput
	}
	return d, nil
}

// dealerAmounts is the discount and the franchise's tax on what is left after it.
func (s *Store) dealerAmounts(ctx context.Context, scope Scope, in DealerInvoiceInput, items []CreateInvoiceItemInput) (discount, tax int64, err error) {
	subtotal, err := invoiceLineTotal(items, 0, in.Notes)
	if err != nil {
		return 0, 0, err
	}
	if discount, err = in.discountOn(subtotal); err != nil {
		return 0, 0, err
	}
	tax, _, err = s.taxOnSubtotal(ctx, scope, subtotal-discount)
	return discount, tax, err
}

// dealerCustomer is the dealership's customer record in this franchise (the invoice is billed to it, so its account sees the invoice).
func (s *Store) dealerCustomer(ctx context.Context, scope Scope, dealership string) (string, error) {
	var customer string
	err := s.pool.QueryRow(ctx, `SELECT customer_id::text FROM dealership_accounts WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND active`, scope.OrganizationID, scope.LocationID, dealership).Scan(&customer)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrDealerInvoiceInput
	}
	return customer, err
}

// numberFree says whether a typed number is unused among dealership invoices (issued ones and other drafts' typed numbers); customer
// invoices are numbered apart (error404 #25).
func numberFree(ctx context.Context, tx pgx.Tx, org, id, number string) error {
	if number == "" {
		return nil
	}
	var taken bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invoices WHERE organization_id=$1 AND id<>$2 AND dealership_account_id IS NOT NULL AND (invoice_number=$3 OR (manual_number=$3 AND status='draft')))`, org, id, number).Scan(&taken)
	if err == nil && taken {
		err = ErrDealerNumberTaken
	}
	return err
}

// setDealerLines writes the dealership parts onto a draft whose lines were just (re)written in the same order, and its amounts with the
// discount taken off before tax.
func setDealerLines(ctx context.Context, tx pgx.Tx, scope Scope, id string, in DealerInvoiceInput, discount, tax int64) error {
	if err := numberFree(ctx, tx, scope.OrganizationID, id, in.InvoiceNumber); err != nil {
		return err
	}
	var pct *float64
	if in.DiscountPercent > 0 {
		pct = &in.DiscountPercent
	}
	tag, err := tx.Exec(ctx, `UPDATE invoices SET dealership_account_id=$4,terms_days=NULL,invoice_date=$5,due_date=$6,po_number=$7,manual_number=$8,discount_percent=$9,
 discount_cents=$10,tax_cents=$11,total_cents=subtotal_cents-$10+$11,due_cents=subtotal_cents-$10+$11,updated_at=now()
 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND status='draft' AND paid_cents=0`, scope.OrganizationID, scope.LocationID, id, in.DealershipID, in.invoiceDate, in.dueDate, in.PONumber, in.InvoiceNumber, pct, discount, tax)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrInvoiceStatusConflict
	}
	rows, err := tx.Query(ctx, `SELECT id::text FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var x string
		if err = rows.Scan(&x); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, x)
	}
	rows.Close()
	if len(ids) != len(in.Lines) {
		return ErrDealerInvoiceInput
	}
	for i, x := range ids {
		if _, err = tx.Exec(ctx, `UPDATE invoice_items SET vin=$2,stock_number=$3,detail=$4 WHERE id=$1`, x, in.Lines[i].VIN, in.Lines[i].StockNumber, in.Lines[i].Detail); err != nil {
			return err
		}
	}
	return nil
}

// CreateDealerInvoice drafts an invoice for a dealership; tax is the franchise's own tax on the lines after the discount.
func (s *Store) CreateDealerInvoice(ctx context.Context, scope Scope, in DealerInvoiceInput) (DealerInvoice, error) {
	items, err := in.clean()
	if err != nil {
		return DealerInvoice{}, err
	}
	customer, err := s.dealerCustomer(ctx, scope, in.DealershipID)
	if err != nil {
		return DealerInvoice{}, err
	}
	discount, tax, err := s.dealerAmounts(ctx, scope, in, items)
	if err != nil {
		return DealerInvoice{}, err
	}
	inv, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: customer, TaxCents: tax, Notes: in.Notes, Items: items})
	if err != nil {
		return DealerInvoice{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DealerInvoice{}, err
	}
	defer tx.Rollback(ctx)
	if err = setDealerLines(ctx, tx, scope, inv.ID, in, discount, tax); err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		// The draft has no dealership parts, so it must not stay as a plain customer invoice.
		_, _ = s.VoidInvoice(context.WithoutCancel(ctx), scope, inv.ID, "The dealership invoice could not be completed")
		return DealerInvoice{}, err
	}
	return s.GetDealerInvoice(ctx, scope, inv.ID)
}

// UpdateDealerInvoice rewrites a dealership draft; the draft's version guards against two people editing at once.
func (s *Store) UpdateDealerInvoice(ctx context.Context, scope Scope, id string, in DealerInvoiceInput) (DealerInvoice, error) {
	items, err := in.clean()
	if err != nil {
		return DealerInvoice{}, err
	}
	current, err := s.GetDealerInvoice(ctx, scope, id)
	if err != nil {
		return DealerInvoice{}, err
	}
	if current.DealershipID != in.DealershipID {
		return DealerInvoice{}, ErrDealerInvoiceInput
	}
	discount, tax, err := s.dealerAmounts(ctx, scope, in, items)
	if err != nil {
		return DealerInvoice{}, err
	}
	if _, err = s.UpdateInvoiceDraft(ctx, scope, id, UpdateInvoiceDraftInput{Version: in.Version, TaxCents: tax, Notes: in.Notes, Items: items}); err != nil {
		return DealerInvoice{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DealerInvoice{}, err
	}
	defer tx.Rollback(ctx)
	if err = setDealerLines(ctx, tx, scope, id, in, discount, tax); err != nil {
		return DealerInvoice{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return DealerInvoice{}, err
	}
	return s.GetDealerInvoice(ctx, scope, id)
}

// GetDealerInvoice is one dealership invoice with its lines (needs invoices.read).
func (s *Store) GetDealerInvoice(ctx context.Context, scope Scope, id string) (DealerInvoice, error) {
	inv, err := s.GetInvoice(ctx, scope, id)
	if err != nil {
		return DealerInvoice{}, err
	}
	out := DealerInvoice{Invoice: inv, Lines: []DealerInvoiceLine{}}
	var terms *int
	var pct *float64
	err = s.pool.QueryRow(ctx, `SELECT i.dealership_account_id::text,d.account_name,i.terms_days,coalesce(to_char(i.invoice_date,'YYYY-MM-DD'),''),coalesce(to_char(i.due_date,'YYYY-MM-DD'),''),i.po_number,i.manual_number,i.discount_percent::float8,i.viewed_at
 FROM invoices i JOIN dealership_accounts d ON d.id=i.dealership_account_id WHERE i.id=$1`, id).Scan(&out.DealershipID, &out.DealershipName, &terms, &out.InvoiceDate, &out.DueDate, &out.PONumber, &out.ManualNumber, &pct, &out.ViewedAt)
	if pct != nil {
		out.DiscountPercent = *pct
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return DealerInvoice{}, ErrInvoiceNotFound // a customer invoice, not a dealership one
	}
	if err != nil {
		return DealerInvoice{}, err
	}
	if terms != nil {
		out.TermsDays = *terms
	}
	out.Overdue = overdue(inv.Status, inv.DueAt)
	rows, err := s.pool.Query(ctx, `SELECT description,detail,quantity,unit_price_cents,vin,stock_number FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return DealerInvoice{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var l DealerInvoiceLine
		if err = rows.Scan(&l.Description, &l.Detail, &l.Quantity, &l.UnitPriceCents, &l.VIN, &l.StockNumber); err != nil {
			return DealerInvoice{}, err
		}
		out.Lines = append(out.Lines, l)
	}
	return out, rows.Err()
}

func overdue(status string, due *time.Time) bool {
	return (status == "issued" || status == "partially_paid") && due != nil && due.Before(time.Now())
}

// ListDealerInvoices lists the franchise's dealership invoices with the filters the owner picks (newest first, at most 1000).
func (s *Store) ListDealerInvoices(ctx context.Context, scope Scope, f DealerInvoiceFilter) ([]DealerInvoiceSummary, error) {
	if err := s.authorize(ctx, scope, "invoices.read"); err != nil {
		return nil, err
	}
	switch f.Status {
	case "", "draft", "issued", "partially_paid", "paid", "void", "overdue", "sent", "viewed":
	default:
		return nil, ErrDealerInvoiceInput
	}
	for _, d := range []string{f.From, f.To} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return nil, ErrDealerInvoiceInput
			}
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT i.id::text,CASE WHEN i.status='draft' THEN i.manual_number ELSE i.invoice_number END,i.dealership_account_id::text,d.account_name,i.status,coalesce(to_char(i.invoice_date,'YYYY-MM-DD'),''),i.po_number,i.issued_at,i.due_at,coalesce(to_char(i.due_date,'YYYY-MM-DD'),''),i.created_at,i.total_cents,i.due_cents,i.viewed_at
FROM invoices i JOIN dealership_accounts d ON d.id=i.dealership_account_id
WHERE i.organization_id=$1 AND i.location_id=$2
 AND ($3='' OR i.dealership_account_id::text=$3)
 AND ($4='' OR coalesce(i.invoice_date,(coalesce(i.issued_at,i.created_at) AT TIME ZONE 'America/Toronto')::date) >= $4::date)
 AND ($5='' OR coalesce(i.invoice_date,(coalesce(i.issued_at,i.created_at) AT TIME ZONE 'America/Toronto')::date) <= $5::date)
 AND ($6='' OR ($6='overdue' AND i.status IN ('issued','partially_paid') AND i.due_at < now()) OR ($6='sent' AND i.status='issued' AND i.viewed_at IS NULL) OR ($6='viewed' AND i.status IN ('issued','partially_paid') AND i.viewed_at IS NOT NULL) OR i.status=$6)
ORDER BY coalesce(i.invoice_date,(coalesce(i.issued_at,i.created_at) AT TIME ZONE 'America/Toronto')::date) DESC, i.created_at DESC, i.id LIMIT 1000`, scope.OrganizationID, scope.LocationID, f.DealershipID, f.From, f.To, f.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DealerInvoiceSummary{}
	for rows.Next() {
		var x DealerInvoiceSummary
		if err = rows.Scan(&x.ID, &x.InvoiceNumber, &x.DealershipID, &x.DealershipName, &x.Status, &x.InvoiceDate, &x.PONumber, &x.IssuedAt, &x.DueAt, &x.DueDate, &x.CreatedAt, &x.TotalCents, &x.DueCents, &x.ViewedAt); err != nil {
			return nil, err
		}
		x.Overdue = overdue(x.Status, x.DueAt)
		out = append(out, x)
	}
	return out, rows.Err()
}

type DealerInvoiceSeries struct {
	Prefix     string `json:"prefix"`
	NextNumber int64  `json:"nextNumber"`
	Version    int    `json:"version"`
}

// DealerSeries is the dealership number series (defaults until the owner saves one).
func (s *Store) DealerSeries(ctx context.Context, actor, org string) (DealerInvoiceSeries, error) {
	if err := s.memberOf(ctx, actor, org); err != nil {
		return DealerInvoiceSeries{}, err
	}
	out := DealerInvoiceSeries{Prefix: "DLR-", NextNumber: 1}
	err := s.pool.QueryRow(ctx, `SELECT prefix,next_number,version FROM dealer_invoice_series WHERE organization_id=$1`, org).Scan(&out.Prefix, &out.NextNumber, &out.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	return out, err
}

var seriesPrefix = regexp.MustCompile(`^[A-Z0-9-]{1,24}$`)

// SaveDealerSeries sets the prefix and next number (finance configuration rights, like the invoice issuer).
func (s *Store) SaveDealerSeries(ctx context.Context, actor, org string, in DealerInvoiceSeries) (DealerInvoiceSeries, error) {
	in.Prefix = strings.ToUpper(strings.TrimSpace(in.Prefix))
	if !seriesPrefix.MatchString(in.Prefix) || in.NextNumber < 1 || in.NextNumber > 1_000_000_000 || in.Version < 0 {
		return DealerInvoiceSeries{}, ErrDealerInvoiceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DealerInvoiceSeries{}, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return DealerInvoiceSeries{}, err
	}
	var out DealerInvoiceSeries
	if in.Version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO dealer_invoice_series(organization_id,prefix,next_number,updated_by) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING prefix,next_number,version`, org, in.Prefix, in.NextNumber, actor).Scan(&out.Prefix, &out.NextNumber, &out.Version)
	} else {
		err = tx.QueryRow(ctx, `UPDATE dealer_invoice_series SET prefix=$2,next_number=$3,version=version+1,updated_by=$4,updated_at=now() WHERE organization_id=$1 AND version=$5 RETURNING prefix,next_number,version`, org, in.Prefix, in.NextNumber, actor, in.Version).Scan(&out.Prefix, &out.NextNumber, &out.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return DealerInvoiceSeries{}, ErrInvoiceVersion
	}
	if err != nil {
		return DealerInvoiceSeries{}, err
	}
	return out, tx.Commit(ctx)
}

// dealerNumbering is used while issuing: a dealership invoice takes the number staff typed, or the dealership series. It returns ok=false
// for a customer invoice. The series row is locked until the issuing transaction ends.
func dealerNumbering(ctx context.Context, tx pgx.Tx, org, id string) (prefix string, next int64, manual string, ok bool, err error) {
	var dealer *string
	if err = tx.QueryRow(ctx, `SELECT dealership_account_id::text,manual_number FROM invoices WHERE id=$1`, id).Scan(&dealer, &manual); err != nil || dealer == nil {
		return "", 0, "", false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO dealer_invoice_series(organization_id,prefix,next_number) VALUES($1,'DLR-',1) ON CONFLICT DO NOTHING`, org); err != nil {
		return "", 0, "", false, err
	}
	err = tx.QueryRow(ctx, `SELECT prefix,next_number FROM dealer_invoice_series WHERE organization_id=$1 FOR UPDATE`, org).Scan(&prefix, &next)
	return prefix, next, manual, err == nil, err
}

// DealerContact is where a dealership invoice email goes: the dealership's primary contact (needs invoices.read in the franchise).
func (s *Store) DealerContact(ctx context.Context, scope Scope, dealership string) (email, name string, err error) {
	if err = s.authorize(ctx, scope, "invoices.read"); err != nil {
		return "", "", err
	}
	err = s.pool.QueryRow(ctx, `SELECT btrim(contact_email),btrim(contact_name) FROM dealership_accounts WHERE organization_id=$1 AND location_id=$2 AND id::text=$3`, scope.OrganizationID, scope.LocationID, dealership).Scan(&email, &name)
	return email, name, err
}

// ReopenDealerInvoice puts an issued dealership invoice that nothing has been paid on back to a draft, keeping its number, so it can be
// edited and issued again (owner, error404 #23). The change is written to the invoice's history. A pay link stops working until it is
// issued again.
func (s *Store) ReopenDealerInvoice(ctx context.Context, scope Scope, id string) (DealerInvoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DealerInvoice{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.manage"); err != nil {
		return DealerInvoice{}, err
	}
	var status, number string
	var paid int64
	var dealer *string
	err = tx.QueryRow(ctx, `SELECT status,invoice_number,paid_cents,dealership_account_id::text FROM invoices WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id).Scan(&status, &number, &paid, &dealer)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && dealer == nil) {
		return DealerInvoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return DealerInvoice{}, err
	}
	var paying bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM square_payment_intents WHERE invoice_id=$1 AND state IN ('prepared','submitting','uncertain','captured'))`, id).Scan(&paying); err != nil {
		return DealerInvoice{}, err
	}
	if status != "issued" || paid != 0 || paying {
		return DealerInvoice{}, ErrInvoiceStatusConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE invoices SET status='draft',manual_number=invoice_number,due_at=NULL,updated_at=now() WHERE id=$1`, id); err != nil {
		return DealerInvoice{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_events(organization_id,location_id,invoice_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,'issued','draft',$5)`, scope.OrganizationID, scope.LocationID, id, scope.ActorID, "Reopened for editing; keeps number "+number); err != nil {
		return DealerInvoice{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return DealerInvoice{}, err
	}
	return s.GetDealerInvoice(ctx, scope, id)
}

type DealerPreview struct {
	SubtotalCents int64 `json:"subtotalCents"`
	DiscountCents int64 `json:"discountCents"`
	TaxCents      int64 `json:"taxCents"`
	TotalCents    int64 `json:"totalCents"`
}

// PreviewDealerInvoice works out the amounts the editor shows while typing (error404 #24), with the same tax as saving.
func (s *Store) PreviewDealerInvoice(ctx context.Context, scope Scope, in DealerInvoiceInput) (DealerPreview, error) {
	if err := s.authorize(ctx, scope, "invoices.read"); err != nil {
		return DealerPreview{}, err
	}
	var out DealerPreview
	if len(in.Lines) > 100 || in.DiscountCents < 0 || in.DiscountPercent < 0 || in.DiscountPercent > 100 {
		return out, ErrDealerInvoiceInput
	}
	for _, l := range in.Lines {
		if l.Quantity < 0 || l.Quantity > 100000 || l.UnitPriceCents < 0 || l.UnitPriceCents > 1_000_000_000 {
			return out, ErrDealerInvoiceInput
		}
		out.SubtotalCents += int64(l.Quantity) * l.UnitPriceCents
	}
	d, err := in.discountOn(out.SubtotalCents)
	if err != nil {
		d = out.SubtotalCents
	}
	out.DiscountCents = d
	if out.TaxCents, _, err = s.taxOnSubtotal(ctx, scope, out.SubtotalCents-d); err != nil {
		return out, err
	}
	out.TotalCents = out.SubtotalCents - d + out.TaxCents
	return out, nil
}
