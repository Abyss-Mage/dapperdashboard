package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrTaxInclusiveUnsupported = errors.New("franchise prices include tax; invoices cannot use that setting yet")

// InvoiceDraftPrefill is what the Workspace shows before an invoice is created from a booking: the lines to bill (with the catalogue
// price when one is set; the employee can change it) and the franchise taxes that will be added.
type InvoiceDraftPrefill struct {
	BookingID    string         `json:"bookingId"`
	CustomerName string         `json:"customerName"`
	Lines        []PrefillLine  `json:"lines"`
	Taxes        FranchiseTaxes `json:"taxes"`
	Existing     []InvoiceBrief `json:"existing"`
}
type PrefillLine struct {
	Description string `json:"description"`
	// PriceCents is the catalogue price before tax when one is set; the employee can still change it.
	PriceCents *int64 `json:"priceCents"`
}
type InvoiceBrief struct {
	ID            string `json:"id"`
	InvoiceNumber string `json:"invoiceNumber"`
	Status        string `json:"status"`
	TotalCents    int64  `json:"totalCents"`
}

// taxOnSubtotal adds up the franchise's active taxes on a tax-exclusive subtotal. Each tax is rounded to the cent with the franchise's
// rounding rule; when nobody has chosen one yet it uses nearest-cent half-up and reports that it did.
func (s *Store) taxOnSubtotal(ctx context.Context, scope Scope, subtotal int64) (tax int64, roundingAssumed bool, err error) {
	var includes bool
	var rounding *string
	err = s.pool.QueryRow(ctx, `SELECT prices_include_tax,rounding FROM franchise_tax_settings WHERE organization_id=$1 AND location_id=$2`, scope.OrganizationID, scope.LocationID).Scan(&includes, &rounding)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}
	if includes {
		return 0, false, ErrTaxInclusiveUnsupported
	}
	mode := "half_up"
	if rounding != nil {
		mode = *rounding
	} else {
		roundingAssumed = true
	}
	rows, err := s.pool.Query(ctx, `SELECT rate_bp FROM franchise_tax_rates WHERE organization_id=$1 AND location_id=$2 AND active`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var bp int64
		if err = rows.Scan(&bp); err != nil {
			return 0, false, err
		}
		num := subtotal * bp // cents * basis points
		q, r := num/10000, num%10000
		switch mode {
		case "down":
		case "half_even":
			if r > 5000 || r == 5000 && q%2 == 1 {
				q++
			}
		default:
			if r >= 5000 {
				q++
			}
		}
		tax += q
	}
	return tax, roundingAssumed, rows.Err()
}

func (s *Store) bookingForEmployee(ctx context.Context, tx pgx.Tx, a EmployeeWorksite, booking string) (Booking, error) {
	if err := employeeBookingLock(ctx, tx, a, booking); err != nil {
		return Booking{}, err
	}
	return scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.id=$1`, booking))
}

// EmployeeInvoices lists the franchise's most recent invoices (invoices.read).
func (s *Store) EmployeeInvoices(ctx context.Context, actor string) ([]Invoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	a, err := employeeWorksite(ctx, tx, actor)
	tx.Rollback(ctx)
	if err != nil {
		return nil, err
	}
	return s.ListInvoices(ctx, employeeScope(a))
}

// EmployeeInvoiceDraft prepares an invoice for one booking: lines from the service and its add-ons, the customer, existing invoices.
func (s *Store) EmployeeInvoiceDraft(ctx context.Context, actor, booking string) (InvoiceDraftPrefill, error) {
	out := InvoiceDraftPrefill{BookingID: booking, Lines: []PrefillLine{}, Existing: []InvoiceBrief{}}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return out, err
	}
	scope := employeeScope(a)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.create"); err != nil {
		return out, err
	}
	b, err := s.bookingForEmployee(ctx, tx, a, booking)
	if err != nil {
		return out, err
	}
	out.CustomerName = b.CustomerName
	price := func(id *string) *int64 {
		var p *int64
		if id != nil {
			_ = tx.QueryRow(ctx, `SELECT price_cents FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3`, a.OrganizationID, a.LocationID, *id).Scan(&p)
		}
		return p
	}
	out.Lines = append(out.Lines, PrefillLine{Description: b.ServiceName, PriceCents: price(b.ServiceID)})
	for _, addon := range b.Addons {
		id := addon.ServiceID
		out.Lines = append(out.Lines, PrefillLine{Description: addon.Name, PriceCents: price(&id)})
	}
	rows, err := tx.Query(ctx, `SELECT id::text,invoice_number,status,total_cents FROM invoices WHERE organization_id=$1 AND location_id=$2 AND booking_id=$3 ORDER BY created_at DESC LIMIT 20`, a.OrganizationID, a.LocationID, booking)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var x InvoiceBrief
		if err = rows.Scan(&x.ID, &x.InvoiceNumber, &x.Status, &x.TotalCents); err != nil {
			rows.Close()
			return out, err
		}
		out.Existing = append(out.Existing, x)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	out.Taxes, err = s.franchiseTaxesUnchecked(ctx, scope)
	return out, err
}

// franchiseTaxesUnchecked reads the franchise's taxes for invoice preparation (the caller was already authorized to invoice).
func (s *Store) franchiseTaxesUnchecked(ctx context.Context, scope Scope) (FranchiseTaxes, error) {
	out := FranchiseTaxes{Rates: []TaxRate{}}
	rows, err := s.pool.Query(ctx, `SELECT id::text,name,rate_bp,active,version FROM franchise_tax_rates WHERE organization_id=$1 AND location_id=$2 ORDER BY lower(name)`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var r TaxRate
		if err = rows.Scan(&r.ID, &r.Name, &r.RateBP, &r.Active, &r.Version); err != nil {
			rows.Close()
			return out, err
		}
		out.Rates = append(out.Rates, r)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	err = s.pool.QueryRow(ctx, `SELECT prices_include_tax,rounding,version FROM franchise_tax_settings WHERE organization_id=$1 AND location_id=$2`, scope.OrganizationID, scope.LocationID).Scan(&out.Settings.PricesIncludeTax, &out.Settings.Rounding, &out.Settings.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return out, err
}

type EmployeeInvoiceInput struct {
	BookingID string                   `json:"bookingId"`
	Notes     string                   `json:"notes"`
	Items     []CreateInvoiceItemInput `json:"items"`
}

// CreateEmployeeInvoice creates a draft invoice for a booking at the employee's franchise. The server adds the franchise's active
// taxes to the subtotal; the employee only enters what is billed.
func (s *Store) CreateEmployeeInvoice(ctx context.Context, actor string, input EmployeeInvoiceInput) (Invoice, bool, error) {
	if input.BookingID == "" || len(input.Items) == 0 {
		return Invoice{}, false, ErrInvoiceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, false, err
	}
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		tx.Rollback(ctx)
		return Invoice{}, false, err
	}
	scope := employeeScope(a)
	b, err := s.bookingForEmployee(ctx, tx, a, input.BookingID)
	tx.Rollback(ctx)
	if err != nil {
		return Invoice{}, false, err
	}
	subtotal, err := invoiceLineTotal(input.Items, 0, input.Notes)
	if err != nil {
		return Invoice{}, false, err
	}
	tax, assumed, err := s.taxOnSubtotal(ctx, scope, subtotal)
	if err != nil {
		return Invoice{}, false, err
	}
	booking := input.BookingID
	inv, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: b.CustomerID, BookingID: &booking, TaxCents: tax, Notes: input.Notes, Items: input.Items})
	return inv, assumed, err
}

// EmployeeIssueInvoice issues a draft invoice (invoices.issue) under the franchise's configured issuer.
func (s *Store) EmployeeIssueInvoice(ctx context.Context, actor, id string) (Invoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	a, err := employeeWorksite(ctx, tx, actor)
	tx.Rollback(ctx)
	if err != nil {
		return Invoice{}, err
	}
	return s.IssueInvoice(ctx, employeeScope(a), id)
}
