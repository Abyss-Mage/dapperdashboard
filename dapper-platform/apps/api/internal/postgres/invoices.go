package postgres

import (
	"context"
	"crypto/rand"
	"dapper.local/platform/internal/paymentstate"
	"dapper.local/platform/internal/square"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvoiceNotFound       = errors.New("invoice not found in this scope")
	ErrInvoiceInput          = errors.New("valid customer, invoice number and at least one item required")
	ErrInvoiceStatusConflict = errors.New("invoice status does not allow this operation")
	ErrInvoiceAlreadyRaised  = errors.New("this booking already has an invoice")
	ErrPaymentInput          = errors.New("valid payment amount and method required")
	ErrPaymentProvider       = errors.New("provider payment requires verified provider confirmation")
	ErrPaymentRequest        = errors.New("payment request key conflicts with an earlier payment")
	ErrPaymentOverpay        = errors.New("payment amount exceeds invoice balance due")
)

type Invoice struct {
	Version          int                      `json:"version"`
	IssuerID         *string                  `json:"issuerId,omitempty"`
	IssuerSnapshot   *InvoiceIssuerSnapshot   `json:"issuerSnapshot,omitempty"`
	CustomerSnapshot *InvoiceCustomerSnapshot `json:"customerSnapshot,omitempty"`
	IssuedBy         *string                  `json:"issuedBy,omitempty"`
	ID               string                   `json:"id"`
	OrganizationID   string                   `json:"organizationId"`
	LocationID       string                   `json:"locationId"`
	CustomerID       string                   `json:"customerId"`
	BookingID        *string                  `json:"bookingId,omitempty"`
	JobID            *string                  `json:"jobId,omitempty"`
	InvoiceNumber    string                   `json:"invoiceNumber"`
	Status           string                   `json:"status"`
	Currency         string                   `json:"currency"`
	SubtotalCents    int64                    `json:"subtotalCents"`
	TaxCents         int64                    `json:"taxCents"`
	TotalCents       int64                    `json:"totalCents"`
	PaidCents        int64                    `json:"paidCents"`
	DueCents         int64                    `json:"dueCents"`
	Notes            string                   `json:"notes"`
	IssuedAt         *time.Time               `json:"issuedAt,omitempty"`
	DueAt            *time.Time               `json:"dueAt,omitempty"`
	PaidAt           *time.Time               `json:"paidAt,omitempty"`
	CreatedAt        time.Time                `json:"createdAt"`
	UpdatedAt        time.Time                `json:"updatedAt"`
	CustomerName     string                   `json:"customerName,omitempty"`
	CustomerEmail    string                   `json:"customerEmail,omitempty"`
	Items            []InvoiceItem            `json:"items,omitempty"`
	Payments         []Payment                `json:"payments,omitempty"`
	Events           []InvoiceEvent           `json:"events,omitempty"`
}

type InvoiceItem struct {
	ID              string    `json:"id"`
	InvoiceID       string    `json:"invoiceId"`
	Description     string    `json:"description"`
	Quantity        int       `json:"quantity"`
	UnitPriceCents  int64     `json:"unitPriceCents"`
	TotalPriceCents int64     `json:"totalPriceCents"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Payment struct {
	ID                string    `json:"id"`
	OrganizationID    string    `json:"organizationId"`
	LocationID        string    `json:"locationId"`
	InvoiceID         string    `json:"invoiceId"`
	CustomerID        string    `json:"customerId"`
	ActorID           string    `json:"actorId"`
	AmountCents       int64     `json:"amountCents"`
	Currency          string    `json:"currency"`
	Method            string    `json:"method"`
	Status            string    `json:"status"`
	ProviderReference string    `json:"providerReference"`
	IdempotencyKey    string    `json:"idempotencyKey"`
	Notes             string    `json:"notes"`
	RecordedAt        time.Time `json:"recordedAt"`
	CreatedAt         time.Time `json:"createdAt"`
}

type InvoiceEvent struct {
	ID             string    `json:"id"`
	InvoiceID      string    `json:"invoiceId"`
	OrganizationID string    `json:"organizationId"`
	LocationID     string    `json:"locationId"`
	ActorID        string    `json:"actorId"`
	FromStatus     *string   `json:"fromStatus,omitempty"`
	ToStatus       string    `json:"toStatus"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"createdAt"`
}

type CreateInvoiceItemInput struct {
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unitPriceCents"`
}

type CreateInvoiceInput struct {
	CustomerID    string                   `json:"customerId"`
	BookingID     *string                  `json:"bookingId,omitempty"`
	JobID         *string                  `json:"jobId,omitempty"`
	InvoiceNumber string                   `json:"invoiceNumber"`
	TaxCents      int64                    `json:"taxCents"`
	Notes         string                   `json:"notes"`
	Items         []CreateInvoiceItemInput `json:"items"`
}

type RecordPaymentInput struct {
	InvoiceID         string `json:"invoiceId"`
	AmountCents       int64  `json:"amountCents"`
	Method            string `json:"method"`
	ProviderReference string `json:"providerReference"`
	IdempotencyKey    string `json:"idempotencyKey"`
	Notes             string `json:"notes"`
}

func generateDefaultInvoiceNumber() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("DRAFT-%s-%s", time.Now().UTC().Format("20060102"), strings.ToUpper(hex.EncodeToString(b))), nil
}

const invoiceSelectColumns = `i.id::text, i.organization_id::text, i.location_id::text, i.customer_id::text,
i.booking_id::text, i.job_id::text, i.invoice_number, i.status, i.currency, i.subtotal_cents,
i.tax_cents, i.total_cents, i.paid_cents, i.due_cents, i.notes, i.issued_at, i.due_at, i.paid_at,
i.created_at, i.updated_at, c.name, c.email, i.issuer_id::text,i.issuer_snapshot,i.customer_snapshot,i.issued_by::text,i.version`

func scanInvoice(row pgx.Row) (Invoice, error) {
	var inv Invoice
	err := row.Scan(
		&inv.ID, &inv.OrganizationID, &inv.LocationID, &inv.CustomerID,
		&inv.BookingID, &inv.JobID, &inv.InvoiceNumber, &inv.Status, &inv.Currency,
		&inv.SubtotalCents, &inv.TaxCents, &inv.TotalCents, &inv.PaidCents, &inv.DueCents,
		&inv.Notes, &inv.IssuedAt, &inv.DueAt, &inv.PaidAt,
		&inv.CreatedAt, &inv.UpdatedAt, &inv.CustomerName, &inv.CustomerEmail,
		&inv.IssuerID, &inv.IssuerSnapshot, &inv.CustomerSnapshot, &inv.IssuedBy, &inv.Version,
	)
	if inv.CustomerSnapshot != nil {
		inv.CustomerName = inv.CustomerSnapshot.Name
		inv.CustomerEmail = inv.CustomerSnapshot.Email
	}
	return inv, err
}

func (s *Store) CreateInvoice(ctx context.Context, scope Scope, input CreateInvoiceInput) (Invoice, error) {
	if err := s.authorize(ctx, scope, "invoices.create"); err != nil {
		return Invoice{}, err
	}
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	if input.CustomerID == "" || len(input.Items) == 0 {
		return Invoice{}, ErrInvoiceInput
	}
	if input.TaxCents < 0 {
		return Invoice{}, ErrInvoiceInput
	}
	invNumber := strings.TrimSpace(input.InvoiceNumber)
	if invNumber == "" {
		var err error
		invNumber, err = generateDefaultInvoiceNumber()
		if err != nil {
			return Invoice{}, err
		}
	} else if utf8.RuneCountInString(invNumber) < 3 || utf8.RuneCountInString(invNumber) > 60 {
		return Invoice{}, ErrInvoiceInput
	}

	subtotalCents, err := invoiceLineTotal(input.Items, input.TaxCents, input.Notes)
	if err != nil {
		return Invoice{}, err
	}
	totalCents := subtotalCents + input.TaxCents
	dueCents := totalCents

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)

	// Validate customer exists in this scope
	if err = authorizeTransaction(ctx, tx, scope, "invoices.create"); err != nil {
		return Invoice{}, err
	}
	var customerExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3)`,
		scope.OrganizationID, scope.LocationID, input.CustomerID).Scan(&customerExists); err != nil {
		return Invoice{}, err
	}
	if !customerExists {
		return Invoice{}, ErrInvoiceInput
	}

	if input.BookingID != nil {
		// Serialize invoice creation per booking, then refuse a second invoice while one that is not void exists.
		var locked string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, *input.BookingID).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Invoice{}, ErrInvoiceInput
			}
			return Invoice{}, err
		}
		var raised bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invoices WHERE organization_id=$1 AND location_id=$2 AND booking_id=$3 AND status<>'void')`, scope.OrganizationID, scope.LocationID, *input.BookingID).Scan(&raised); err != nil {
			return Invoice{}, err
		}
		if raised {
			return Invoice{}, ErrInvoiceAlreadyRaised
		}
	}

	var newID string
	err = tx.QueryRow(ctx, `
		INSERT INTO invoices(
			organization_id, location_id, customer_id, booking_id, job_id,
			invoice_number, status, currency, subtotal_cents, tax_cents,
			total_cents, paid_cents, due_cents, notes, issued_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, 'draft', 'CAD', $7, $8,
			$9, 0, $10, $11, NULL, now()
		)
		RETURNING id::text
	`, scope.OrganizationID, scope.LocationID, input.CustomerID, input.BookingID, input.JobID,
		invNumber, subtotalCents, input.TaxCents, totalCents, dueCents, strings.TrimSpace(input.Notes)).Scan(&newID)
	if err != nil {
		return Invoice{}, err
	}

	createdItems := make([]InvoiceItem, 0, len(input.Items))
	for _, it := range input.Items {
		desc := strings.TrimSpace(it.Description)
		lineTotal := int64(it.Quantity) * it.UnitPriceCents
		var item InvoiceItem
		err := tx.QueryRow(ctx, `
			INSERT INTO invoice_items(
				invoice_id, organization_id, location_id, description, quantity, unit_price_cents, total_price_cents
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id::text, invoice_id::text, description, quantity, unit_price_cents, total_price_cents, created_at
		`, newID, scope.OrganizationID, scope.LocationID, desc, it.Quantity, it.UnitPriceCents, lineTotal).Scan(
			&item.ID, &item.InvoiceID, &item.Description, &item.Quantity, &item.UnitPriceCents, &item.TotalPriceCents, &item.CreatedAt,
		)
		if err != nil {
			return Invoice{}, err
		}
		createdItems = append(createdItems, item)
	}

	// Audit event
	_, err = tx.Exec(ctx, `
		INSERT INTO invoice_events(
			invoice_id, organization_id, location_id, actor_id, from_status, to_status, notes
		) VALUES ($1, $2, $3, $4, NULL, 'draft', 'Invoice draft created')
	`, newID, scope.OrganizationID, scope.LocationID, scope.ActorID)
	if err != nil {
		return Invoice{}, err
	}

	row := tx.QueryRow(ctx, `
		SELECT `+invoiceSelectColumns+`
		FROM invoices i
		JOIN customers c ON c.organization_id=i.organization_id AND c.location_id=i.location_id AND c.id=i.customer_id
		WHERE i.id=$1
	`, newID)

	inv, err := scanInvoice(row)
	if err != nil {
		return Invoice{}, err
	}
	inv.Items = createdItems

	if err := tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	return inv, nil
}

func (s *Store) ListInvoices(ctx context.Context, scope Scope) ([]Invoice, error) {
	if err := s.authorize(ctx, scope, "invoices.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+invoiceSelectColumns+`
		FROM invoices i
		JOIN customers c ON c.organization_id=i.organization_id AND c.location_id=i.location_id AND c.id=i.customer_id
		WHERE i.organization_id=$1 AND i.location_id=$2
		ORDER BY i.created_at DESC
		LIMIT 100
	`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	if invoices == nil {
		invoices = []Invoice{}
	}
	return invoices, rows.Err()
}

func (s *Store) GetInvoice(ctx context.Context, scope Scope, invoiceID string) (Invoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)
	if err := authorizeTransaction(ctx, tx, scope, "invoices.read"); err != nil {
		return Invoice{}, err
	}
	row := tx.QueryRow(ctx, `
		SELECT `+invoiceSelectColumns+`
		FROM invoices i
		JOIN customers c ON c.organization_id=i.organization_id AND c.location_id=i.location_id AND c.id=i.customer_id
		WHERE i.id=$1 AND i.organization_id=$2 AND i.location_id=$3 FOR SHARE OF i
	`, invoiceID, scope.OrganizationID, scope.LocationID)

	inv, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}

	// Fetch items
	itemRows, err := tx.Query(ctx, `
		SELECT id::text, invoice_id::text, description, quantity, unit_price_cents, total_price_cents, created_at
		FROM invoice_items
		WHERE invoice_id=$1
		ORDER BY created_at ASC
	`, inv.ID)
	if err != nil {
		return Invoice{}, err
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var it InvoiceItem
		if err := itemRows.Scan(&it.ID, &it.InvoiceID, &it.Description, &it.Quantity, &it.UnitPriceCents, &it.TotalPriceCents, &it.CreatedAt); err != nil {
			return Invoice{}, err
		}
		inv.Items = append(inv.Items, it)
	}
	if inv.Items == nil {
		inv.Items = []InvoiceItem{}
	}
	if err := itemRows.Err(); err != nil {
		return Invoice{}, err
	}

	// Fetch payments
	pmtRows, err := tx.Query(ctx, `
		SELECT id::text, organization_id::text, location_id::text, invoice_id::text, customer_id::text,
			actor_id::text, amount_cents, currency, method, status, provider_reference, idempotency_key,
			notes, recorded_at, created_at
		FROM payments
		WHERE invoice_id=$1
		ORDER BY recorded_at ASC
	`, inv.ID)
	if err != nil {
		return Invoice{}, err
	}
	defer pmtRows.Close()

	for pmtRows.Next() {
		var p Payment
		if err := pmtRows.Scan(
			&p.ID, &p.OrganizationID, &p.LocationID, &p.InvoiceID, &p.CustomerID,
			&p.ActorID, &p.AmountCents, &p.Currency, &p.Method, &p.Status,
			&p.ProviderReference, &p.IdempotencyKey, &p.Notes, &p.RecordedAt, &p.CreatedAt,
		); err != nil {
			return Invoice{}, err
		}
		inv.Payments = append(inv.Payments, p)
	}
	if inv.Payments == nil {
		inv.Payments = []Payment{}
	}
	if err := pmtRows.Err(); err != nil {
		return Invoice{}, err
	}

	// Fetch events
	eventRows, err := tx.Query(ctx, `
		SELECT id::text, invoice_id::text, organization_id::text, location_id::text,
			actor_id::text, from_status, to_status, notes, created_at
		FROM invoice_events
		WHERE invoice_id=$1
		ORDER BY created_at ASC
	`, inv.ID)
	if err != nil {
		return Invoice{}, err
	}
	defer eventRows.Close()

	for eventRows.Next() {
		var ev InvoiceEvent
		if err := eventRows.Scan(
			&ev.ID, &ev.InvoiceID, &ev.OrganizationID, &ev.LocationID,
			&ev.ActorID, &ev.FromStatus, &ev.ToStatus, &ev.Notes, &ev.CreatedAt,
		); err != nil {
			return Invoice{}, err
		}
		inv.Events = append(inv.Events, ev)
	}
	if inv.Events == nil {
		inv.Events = []InvoiceEvent{}
	}

	if err = eventRows.Err(); err != nil {
		return Invoice{}, err
	}
	eventRows.Close()
	if err = tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	return inv, nil
}

func (s *Store) RecordPayment(ctx context.Context, scope Scope, input RecordPaymentInput) (Payment, Invoice, error) {
	if err := s.authorize(ctx, scope, "payments.record"); err != nil {
		return Payment{}, Invoice{}, err
	}
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	if input.InvoiceID == "" || input.AmountCents <= 0 {
		return Payment{}, Invoice{}, ErrPaymentInput
	}
	method := strings.TrimSpace(input.Method)
	switch method {
	case "cash", "bank_transfer":
		if strings.TrimSpace(input.ProviderReference) != "" {
			return Payment{}, Invoice{}, ErrPaymentInput
		}
	case "square_card", "square_terminal":
		return Payment{}, Invoice{}, ErrPaymentProvider
	default:
		return Payment{}, Invoice{}, ErrPaymentInput
	}

	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	input.Notes = strings.TrimSpace(input.Notes)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 128 || utf8.RuneCountInString(input.Notes) > 2000 {
		return Payment{}, Invoice{}, ErrPaymentInput
	}
	for _, c := range idempotencyKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return Payment{}, Invoice{}, ErrPaymentInput
		}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	defer tx.Rollback(ctx)

	// An explicit stable key is mandatory for every cash-receipt operation.
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return Payment{}, Invoice{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.OrganizationID+"/"+scope.LocationID+"/payment/"+idempotencyKey); err != nil {
		return Payment{}, Invoice{}, err
	}
	if idempotencyKey != "" {
		var existing Payment
		err := tx.QueryRow(ctx, `
			SELECT id::text, organization_id::text, location_id::text, invoice_id::text, customer_id::text,
				actor_id::text, amount_cents, currency, method, status, provider_reference, idempotency_key,
				notes, recorded_at, created_at
			FROM payments
			WHERE organization_id=$1 AND location_id=$2 AND idempotency_key=$3
		`, scope.OrganizationID, scope.LocationID, idempotencyKey).Scan(
			&existing.ID, &existing.OrganizationID, &existing.LocationID, &existing.InvoiceID, &existing.CustomerID,
			&existing.ActorID, &existing.AmountCents, &existing.Currency, &existing.Method, &existing.Status,
			&existing.ProviderReference, &existing.IdempotencyKey, &existing.Notes, &existing.RecordedAt, &existing.CreatedAt,
		)
		if err == nil {
			if existing.InvoiceID != input.InvoiceID || existing.AmountCents != input.AmountCents || existing.Method != method || existing.ActorID != scope.ActorID || existing.Notes != input.Notes {
				return Payment{}, Invoice{}, ErrPaymentRequest
			}
			inv, getErr := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3 FOR SHARE OF i`, scope.OrganizationID, scope.LocationID, existing.InvoiceID))
			if getErr != nil {
				return Payment{}, Invoice{}, getErr
			}
			if err = tx.Commit(ctx); err != nil {
				return Payment{}, Invoice{}, err
			}
			return existing, paymentInvoiceSummary(inv), nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, Invoice{}, err
		}
	}

	// Lock invoice
	row := tx.QueryRow(ctx, `
		SELECT `+invoiceSelectColumns+`
		FROM invoices i
		JOIN customers c ON c.organization_id=i.organization_id AND c.location_id=i.location_id AND c.id=i.customer_id
		WHERE i.id=$1 AND i.organization_id=$2 AND i.location_id=$3
		FOR UPDATE OF i
	`, input.InvoiceID, scope.OrganizationID, scope.LocationID)

	inv, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	if inv.Status != "issued" && inv.Status != "partially_paid" {
		return Payment{}, Invoice{}, ErrInvoiceStatusConflict
	}
	if input.AmountCents > inv.DueCents {
		return Payment{}, Invoice{}, ErrPaymentOverpay
	}

	newPaidCents := inv.PaidCents + input.AmountCents
	newDueCents := inv.TotalCents - newPaidCents
	newStatus := "partially_paid"
	if newDueCents == 0 {
		newStatus = "paid"
	}

	var pmt Payment
	pmtRow := tx.QueryRow(ctx, `
		INSERT INTO payments(
			organization_id, location_id, invoice_id, customer_id, actor_id,
			amount_cents, currency, method, status, provider_reference,
			idempotency_key, notes
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, 'CAD', $7, 'completed', $8,
			$9, $10
		)
		RETURNING id::text, organization_id::text, location_id::text, invoice_id::text, customer_id::text,
			actor_id::text, amount_cents, currency, method, status, provider_reference,
			idempotency_key, notes, recorded_at, created_at
	`, scope.OrganizationID, scope.LocationID, inv.ID, inv.CustomerID, scope.ActorID,
		input.AmountCents, method, strings.TrimSpace(input.ProviderReference),
		idempotencyKey, strings.TrimSpace(input.Notes))

	if err := pmtRow.Scan(
		&pmt.ID, &pmt.OrganizationID, &pmt.LocationID, &pmt.InvoiceID, &pmt.CustomerID,
		&pmt.ActorID, &pmt.AmountCents, &pmt.Currency, &pmt.Method, &pmt.Status,
		&pmt.ProviderReference, &pmt.IdempotencyKey, &pmt.Notes, &pmt.RecordedAt, &pmt.CreatedAt,
	); err != nil {
		return Payment{}, Invoice{}, err
	}

	// Update invoice totals and status
	_, err = tx.Exec(ctx, `
		UPDATE invoices
		SET paid_cents=$1, due_cents=$2, status=$3,
			paid_at=CASE WHEN $3='paid' THEN now() ELSE paid_at END,
			updated_at=now()
		WHERE id=$4
	`, newPaidCents, newDueCents, newStatus, inv.ID)
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	// Log audit event
	_, err = tx.Exec(ctx, `
		INSERT INTO invoice_events(
			invoice_id, organization_id, location_id, actor_id, from_status, to_status, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, inv.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, inv.Status, newStatus,
		fmt.Sprintf("Payment of %d cents recorded via %s", input.AmountCents, method))
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	updatedInv, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3`, scope.OrganizationID, scope.LocationID, inv.ID))
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Payment{}, Invoice{}, err
	}
	return pmt, paymentInvoiceSummary(updatedInv), nil
}

type CustomerPayInvoiceInput struct {
	SourceID       string `json:"sourceId"`
	AmountCents    int64  `json:"amountCents"`
	IdempotencyKey string `json:"idempotencyKey"`
	Notes          string `json:"notes,omitempty"`
}

// CustomerPayInvoice allows an authenticated customer to pay their own issued invoice
// via tokenized Square card payment.
func (s *Store) CustomerPayInvoice(ctx context.Context, actorID, customerID, invoiceID string, client square.Client, input CustomerPayInvoiceInput) (Payment, Invoice, error) {
	if client == nil {
		return Payment{}, Invoice{}, ErrPaymentProvider
	}
	if actorID == "" || customerID == "" || !uuid(invoiceID) {
		return Payment{}, Invoice{}, ErrForbidden
	}
	input.SourceID = strings.TrimSpace(input.SourceID)
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.SourceID == "" || input.AmountCents <= 0 || len(idempotencyKey) < 8 || len(idempotencyKey) > 128 || utf8.RuneCountInString(input.Notes) > 2000 {
		return Payment{}, Invoice{}, ErrPaymentInput
	}
	for _, c := range idempotencyKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return Payment{}, Invoice{}, ErrPaymentInput
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	defer tx.Rollback(ctx)

	var org, loc string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text, a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`,
		actorID, customerID).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, Invoice{}, ErrForbidden
	}
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, org+"/"+loc+"/cust-payment/"+idempotencyKey); err != nil {
		return Payment{}, Invoice{}, err
	}

	// Idempotency check: if already recorded for this key, return existing payment
	var existing Payment
	err = tx.QueryRow(ctx, `
		SELECT id::text, organization_id::text, location_id::text, invoice_id::text, customer_id::text,
			actor_id::text, amount_cents, currency, method, status, provider_reference, idempotency_key,
			notes, recorded_at, created_at
		FROM payments
		WHERE organization_id=$1 AND location_id=$2 AND idempotency_key=$3
	`, org, loc, idempotencyKey).Scan(
		&existing.ID, &existing.OrganizationID, &existing.LocationID, &existing.InvoiceID, &existing.CustomerID,
		&existing.ActorID, &existing.AmountCents, &existing.Currency, &existing.Method, &existing.Status,
		&existing.ProviderReference, &existing.IdempotencyKey, &existing.Notes, &existing.RecordedAt, &existing.CreatedAt,
	)
	if err == nil {
		if existing.InvoiceID != invoiceID || existing.AmountCents != input.AmountCents || existing.CustomerID != customerID || existing.ActorID != actorID {
			return Payment{}, Invoice{}, ErrPaymentRequest
		}
		inv, getErr := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3 FOR SHARE OF i`, org, loc, existing.InvoiceID))
		if getErr != nil {
			return Payment{}, Invoice{}, getErr
		}
		if err = tx.Commit(ctx); err != nil {
			return Payment{}, Invoice{}, err
		}
		return existing, paymentInvoiceSummary(inv), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, Invoice{}, err
	}

	// Lock invoice row
	row := tx.QueryRow(ctx, `
		SELECT `+invoiceSelectColumns+`
		FROM invoices i
		JOIN customers c ON c.organization_id=i.organization_id AND c.location_id=i.location_id AND c.id=i.customer_id
		WHERE i.id=$1 AND i.organization_id=$2 AND i.location_id=$3 AND i.customer_id=$4
		FOR UPDATE OF i
	`, invoiceID, org, loc, customerID)

	inv, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	if inv.Status != "issued" && inv.Status != "partially_paid" {
		return Payment{}, Invoice{}, ErrInvoiceStatusConflict
	}
	if input.AmountCents > inv.DueCents {
		return Payment{}, Invoice{}, ErrPaymentOverpay
	}

	// Call Square provider
	sqResp, err := client.CreatePayment(ctx, square.PaymentRequest{
		SourceID:       input.SourceID,
		IdempotencyKey: idempotencyKey,
		AmountCents:    input.AmountCents,
		Currency:       "CAD",
		ReferenceID:    inv.ID,
		Note:           fmt.Sprintf("Customer Online Payment Invoice %s", inv.InvoiceNumber),
	})
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	if sqResp.Status != "COMPLETED" {
		return Payment{}, Invoice{}, ErrPaymentProvider
	}
	if sqResp.AmountCents != input.AmountCents || sqResp.Currency != "CAD" || strings.TrimSpace(sqResp.ID) == "" {
		return Payment{}, Invoice{}, ErrPaymentProvider
	}

	newPaidCents := inv.PaidCents + input.AmountCents
	newDueCents := inv.TotalCents - newPaidCents
	newStatus := "partially_paid"
	if newDueCents == 0 {
		newStatus = "paid"
	}

	var pmt Payment
	pmtRow := tx.QueryRow(ctx, `
		INSERT INTO payments(
			organization_id, location_id, invoice_id, customer_id, actor_id,
			amount_cents, currency, method, status, provider_reference,
			idempotency_key, notes
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, 'CAD', 'square_card', 'completed', $7,
			$8, $9
		)
		RETURNING id::text, organization_id::text, location_id::text, invoice_id::text, customer_id::text,
			actor_id::text, amount_cents, currency, method, status, provider_reference,
			idempotency_key, notes, recorded_at, created_at
	`, org, loc, inv.ID, inv.CustomerID, actorID,
		input.AmountCents, strings.TrimSpace(sqResp.ID),
		idempotencyKey, strings.TrimSpace(input.Notes))

	if err := pmtRow.Scan(
		&pmt.ID, &pmt.OrganizationID, &pmt.LocationID, &pmt.InvoiceID, &pmt.CustomerID,
		&pmt.ActorID, &pmt.AmountCents, &pmt.Currency, &pmt.Method, &pmt.Status,
		&pmt.ProviderReference, &pmt.IdempotencyKey, &pmt.Notes, &pmt.RecordedAt, &pmt.CreatedAt,
	); err != nil {
		return Payment{}, Invoice{}, err
	}

	// Update invoice totals and status
	_, err = tx.Exec(ctx, `
		UPDATE invoices
		SET paid_cents=$1, due_cents=$2, status=$3,
			paid_at=CASE WHEN $3='paid' THEN now() ELSE paid_at END,
			updated_at=now()
		WHERE id=$4
	`, newPaidCents, newDueCents, newStatus, inv.ID)
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: actorID}
	eventNotes := fmt.Sprintf("Customer online payment of %d cents recorded via Square", input.AmountCents)
	if err = recordInvoiceEvent(ctx, tx, scope, inv.ID, &inv.Status, newStatus, eventNotes); err != nil {
		return Payment{}, Invoice{}, err
	}
	if err = jobAudit(ctx, tx, scope, "payment.customer_square_completed", pmt.ID); err != nil {
		return Payment{}, Invoice{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return Payment{}, Invoice{}, err
	}

	inv.PaidCents = newPaidCents
	inv.DueCents = newDueCents
	inv.Status = newStatus
	return pmt, paymentInvoiceSummary(inv), nil
}

// Payment authority does not implicitly grant invoice history or private notes.
func paymentInvoiceSummary(inv Invoice) Invoice {
	return Invoice{Version: inv.Version, ID: inv.ID, OrganizationID: inv.OrganizationID, LocationID: inv.LocationID, CustomerID: inv.CustomerID, InvoiceNumber: inv.InvoiceNumber, Status: inv.Status, Currency: inv.Currency, SubtotalCents: inv.SubtotalCents, TaxCents: inv.TaxCents, TotalCents: inv.TotalCents, PaidCents: inv.PaidCents, DueCents: inv.DueCents, PaidAt: inv.PaidAt, UpdatedAt: inv.UpdatedAt}
}

type RecordSquarePaymentInput struct {
	InvoiceID        string `json:"invoiceId"`
	AmountCents      int64  `json:"amountCents"`
	SourceID         string `json:"sourceId"`
	SquareLocationID string `json:"squareLocationId,omitempty"`
	IdempotencyKey   string `json:"idempotencyKey"`
	Method           string `json:"method"` // "square_card" or "square_terminal"
	Notes            string `json:"notes,omitempty"`
}

// RecordSquarePayment executes a verified charge with the Square payment provider,
// then atomically records the payment and updates the invoice in the database.
func (s *Store) RecordSquarePayment(ctx context.Context, scope Scope, client square.Client, input RecordSquarePaymentInput) (Payment, Invoice, error) {
	if client == nil {
		return Payment{}, Invoice{}, ErrPaymentProvider
	}
	if err := s.authorize(ctx, scope, "payments.record"); err != nil {
		return Payment{}, Invoice{}, err
	}
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.SourceID = strings.TrimSpace(input.SourceID)
	if input.InvoiceID == "" || input.SourceID == "" || input.AmountCents <= 0 {
		return Payment{}, Invoice{}, ErrPaymentInput
	}
	method := strings.TrimSpace(input.Method)
	if method != "square_card" && method != "square_terminal" {
		return Payment{}, Invoice{}, ErrPaymentInput
	}

	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	input.Notes = strings.TrimSpace(input.Notes)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 128 || utf8.RuneCountInString(input.Notes) > 2000 {
		return Payment{}, Invoice{}, ErrPaymentInput
	}
	for _, c := range idempotencyKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return Payment{}, Invoice{}, ErrPaymentInput
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return Payment{}, Invoice{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.OrganizationID+"/"+scope.LocationID+"/payment/"+idempotencyKey); err != nil {
		return Payment{}, Invoice{}, err
	}

	// Idempotency check: if already recorded for this key, return the existing payment
	var existing Payment
	err = tx.QueryRow(ctx, `
		SELECT id::text, organization_id::text, location_id::text, invoice_id::text, customer_id::text,
			actor_id::text, amount_cents, currency, method, status, provider_reference, idempotency_key,
			notes, recorded_at, created_at
		FROM payments
		WHERE organization_id=$1 AND location_id=$2 AND idempotency_key=$3
	`, scope.OrganizationID, scope.LocationID, idempotencyKey).Scan(
		&existing.ID, &existing.OrganizationID, &existing.LocationID, &existing.InvoiceID, &existing.CustomerID,
		&existing.ActorID, &existing.AmountCents, &existing.Currency, &existing.Method, &existing.Status,
		&existing.ProviderReference, &existing.IdempotencyKey, &existing.Notes, &existing.RecordedAt, &existing.CreatedAt,
	)
	if err == nil {
		if existing.InvoiceID != input.InvoiceID || existing.AmountCents != input.AmountCents || existing.Method != method || existing.ActorID != scope.ActorID || existing.Notes != input.Notes {
			return Payment{}, Invoice{}, ErrPaymentRequest
		}
		inv, getErr := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3 FOR SHARE OF i`, scope.OrganizationID, scope.LocationID, existing.InvoiceID))
		if getErr != nil {
			return Payment{}, Invoice{}, getErr
		}
		if err = tx.Commit(ctx); err != nil {
			return Payment{}, Invoice{}, err
		}
		return existing, paymentInvoiceSummary(inv), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, Invoice{}, err
	}

	// Lock invoice row
	row := tx.QueryRow(ctx, `
		SELECT `+invoiceSelectColumns+`
		FROM invoices i
		JOIN customers c ON c.organization_id=i.organization_id AND c.location_id=i.location_id AND c.id=i.customer_id
		WHERE i.id=$1 AND i.organization_id=$2 AND i.location_id=$3
		FOR UPDATE OF i
	`, input.InvoiceID, scope.OrganizationID, scope.LocationID)

	inv, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	if inv.Status != "issued" && inv.Status != "partially_paid" {
		return Payment{}, Invoice{}, ErrInvoiceStatusConflict
	}
	if input.AmountCents > inv.DueCents {
		return Payment{}, Invoice{}, ErrPaymentOverpay
	}

	// Call Square payment provider to charge the source
	sqResp, err := client.CreatePayment(ctx, square.PaymentRequest{
		SourceID:       input.SourceID,
		IdempotencyKey: idempotencyKey,
		AmountCents:    input.AmountCents,
		Currency:       "CAD",
		LocationID:     input.SquareLocationID,
		ReferenceID:    inv.ID,
		Note:           fmt.Sprintf("Dapper Invoice %s", inv.InvoiceNumber),
	})
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	if sqResp.Status != "COMPLETED" {
		return Payment{}, Invoice{}, ErrPaymentProvider
	}
	if sqResp.AmountCents != input.AmountCents || sqResp.Currency != "CAD" || strings.TrimSpace(sqResp.ID) == "" {
		return Payment{}, Invoice{}, ErrPaymentProvider
	}

	newPaidCents := inv.PaidCents + input.AmountCents
	newDueCents := inv.TotalCents - newPaidCents
	newStatus := "partially_paid"
	if newDueCents == 0 {
		newStatus = "paid"
	}

	var pmt Payment
	pmtRow := tx.QueryRow(ctx, `
		INSERT INTO payments(
			organization_id, location_id, invoice_id, customer_id, actor_id,
			amount_cents, currency, method, status, provider_reference,
			idempotency_key, notes
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, 'CAD', $7, 'completed', $8,
			$9, $10
		)
		RETURNING id::text, organization_id::text, location_id::text, invoice_id::text, customer_id::text,
			actor_id::text, amount_cents, currency, method, status, provider_reference,
			idempotency_key, notes, recorded_at, created_at
	`, scope.OrganizationID, scope.LocationID, inv.ID, inv.CustomerID, scope.ActorID,
		input.AmountCents, method, strings.TrimSpace(sqResp.ID),
		idempotencyKey, strings.TrimSpace(input.Notes))

	if err := pmtRow.Scan(
		&pmt.ID, &pmt.OrganizationID, &pmt.LocationID, &pmt.InvoiceID, &pmt.CustomerID,
		&pmt.ActorID, &pmt.AmountCents, &pmt.Currency, &pmt.Method, &pmt.Status,
		&pmt.ProviderReference, &pmt.IdempotencyKey, &pmt.Notes, &pmt.RecordedAt, &pmt.CreatedAt,
	); err != nil {
		return Payment{}, Invoice{}, err
	}

	// Update invoice totals and status
	_, err = tx.Exec(ctx, `
		UPDATE invoices
		SET paid_cents=$1, due_cents=$2, status=$3,
			paid_at=CASE WHEN $3='paid' THEN now() ELSE paid_at END,
			updated_at=now()
		WHERE id=$4 AND organization_id=$5 AND location_id=$6
	`, newPaidCents, newDueCents, newStatus, inv.ID, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	eventNotes := fmt.Sprintf("Payment of %d cents recorded via %s (%s)", input.AmountCents, method, sqResp.ID)
	if strings.TrimSpace(input.Notes) != "" {
		eventNotes += " - " + input.Notes
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO invoice_events(
			invoice_id, organization_id, location_id, actor_id, from_status, to_status, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, inv.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, inv.Status, newStatus, eventNotes)
	if err != nil {
		return Payment{}, Invoice{}, err
	}

	if err = jobAudit(ctx, tx, scope, "payment.recorded", pmt.ID); err != nil {
		return Payment{}, Invoice{}, err
	}

	updatedInv, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3`, scope.OrganizationID, scope.LocationID, inv.ID))
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Payment{}, Invoice{}, err
	}
	return pmt, paymentInvoiceSummary(updatedInv), nil
}

func (s *Store) VoidInvoice(ctx context.Context, scope Scope, invoiceID string, notes string) (Invoice, error) {
	if err := s.authorize(ctx, scope, "invoices.manage"); err != nil {
		return Invoice{}, err
	}
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return Invoice{}, ErrInvoiceNotFound
	}
	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > 2000 {
		return Invoice{}, ErrInvoiceInput
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.manage"); err != nil {
		return Invoice{}, err
	}

	row := tx.QueryRow(ctx, `
		SELECT `+invoiceSelectColumns+`
		FROM invoices i
		JOIN customers c ON c.organization_id=i.organization_id AND c.location_id=i.location_id AND c.id=i.customer_id
		WHERE i.id=$1 AND i.organization_id=$2 AND i.location_id=$3
		FOR UPDATE OF i
	`, invoiceID, scope.OrganizationID, scope.LocationID)

	inv, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}

	if inv.Status == "void" {
		if err = tx.Commit(ctx); err != nil {
			return Invoice{}, err
		}
		return paymentInvoiceSummary(inv), nil
	}
	if inv.PaidCents > 0 || (inv.Status != "draft" && inv.Status != "issued") {
		return Invoice{}, ErrInvoiceStatusConflict
	}

	err = tx.QueryRow(ctx, `
		UPDATE invoices
		SET status='void', updated_at=now()
		WHERE id=$1
		RETURNING updated_at,version
	`, inv.ID).Scan(&inv.UpdatedAt, &inv.Version)
	if err != nil {
		return Invoice{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO invoice_events(
			invoice_id, organization_id, location_id, actor_id, from_status, to_status, notes
		) VALUES ($1, $2, $3, $4, $5, 'void', $6)
	`, inv.ID, scope.OrganizationID, scope.LocationID, scope.ActorID, inv.Status, strings.TrimSpace(notes))
	if err != nil {
		return Invoice{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}

	inv.Status = "void"
	return paymentInvoiceSummary(inv), nil
}

func recordInvoiceEvent(ctx context.Context, tx pgx.Tx, scope Scope, invoiceID string, fromStatus *string, toStatus, notes string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO invoice_events(
			invoice_id, organization_id, location_id, actor_id, from_status, to_status, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, invoiceID, scope.OrganizationID, scope.LocationID, scope.ActorID, fromStatus, toStatus, strings.TrimSpace(notes))
	return err
}

// HandleSquareWebhookPayment updates payment and invoice state asynchronously upon
// receiving a verified Square webhook event (e.g. payment.updated).
func (s *Store) HandleSquareWebhookPayment(ctx context.Context, reference, status string, observedRefund int64) error {
	reference = strings.TrimSpace(reference)
	if reference == "" || observedRefund < 0 {
		return ErrPaymentInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Resolve identity without a row lock, then take invoice before payment locks,
	// matching payment recording and avoiding the opposite-order deadlock.
	var id, invoiceID string
	rows, err := tx.Query(ctx, `SELECT id::text,invoice_id::text FROM payments WHERE provider_reference=$1 AND method IN ('square_card','square_terminal')`, reference)
	if err != nil {
		return err
	}
	count := 0
	for rows.Next() {
		count++
		if err = rows.Scan(&id, &invoiceID); err != nil {
			rows.Close()
			return err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if count != 1 {
		return ErrPaymentProvider
	}
	inv, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.id=$1 FOR UPDATE OF i`, invoiceID))
	if err != nil {
		return err
	}
	var amount, priorRefund int64
	var paymentStatus, org, loc, actor string
	err = tx.QueryRow(ctx, `SELECT amount_cents,refunded_cents,status,organization_id::text,location_id::text,actor_id::text FROM payments WHERE id=$1 AND invoice_id=$2 AND provider_reference=$3 FOR UPDATE`, id, invoiceID, reference).Scan(&amount, &priorRefund, &paymentStatus, &org, &loc, &actor)
	if err != nil {
		return err
	}
	projection, err := paymentstate.ReconcileRefund(amount, priorRefund, observedRefund)
	if err != nil {
		return ErrPaymentInput
	}
	wasCaptured := paymentStatus == "completed" || paymentStatus == "refunded"
	captured := wasCaptured || status == "COMPLETED"
	if status == "REFUNDED" && (!wasCaptured || observedRefund == 0) {
		return ErrPaymentProvider
	}
	if !captured && projection.RefundedCents > 0 {
		return ErrPaymentProvider
	}
	previousNet := int64(0)
	if wasCaptured {
		previousNet = amount - priorRefund
	}
	nextNet := int64(0)
	if captured {
		nextNet = projection.NetCapturedCents
	}
	change := nextNet - previousNet
	if change > inv.DueCents || change < -inv.PaidCents {
		return ErrPaymentProvider
	}
	nextStatus := paymentStatus
	if captured {
		nextStatus = "completed"
		if projection.RefundedCents == amount {
			nextStatus = "refunded"
		}
	} else if status == "FAILED" || status == "CANCELED" {
		nextStatus = "failed"
	}
	if change == 0 && projection.DeltaCents == 0 && nextStatus == paymentStatus {
		return tx.Commit(ctx)
	}
	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: actor}
	if _, err = tx.Exec(ctx, `UPDATE payments SET refunded_cents=$1,status=$2 WHERE id=$3`, projection.RefundedCents, nextStatus, id); err != nil {
		return err
	}
	if projection.DeltaCents > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO payment_refund_adjustments(organization_id,location_id,payment_id,captured_cents,previous_refunded_cents,refunded_cents,delta_cents) VALUES($1,$2,$3,$4,$5,$6,$7)`, org, loc, id, amount, priorRefund, projection.RefundedCents, projection.DeltaCents); err != nil {
			return err
		}
	}
	if change != 0 {
		if inv.Status != "issued" && inv.Status != "partially_paid" && inv.Status != "paid" {
			return ErrInvoiceStatusConflict
		}
		paid := inv.PaidCents + change
		due := inv.TotalCents - paid
		invoiceStatus := "partially_paid"
		if paid == 0 {
			invoiceStatus = "issued"
		} else if due == 0 {
			invoiceStatus = "paid"
		}
		if _, err = tx.Exec(ctx, `UPDATE invoices SET paid_cents=$1,due_cents=$2,status=$3,paid_at=CASE WHEN $3='paid' THEN COALESCE(paid_at,now()) ELSE NULL END,updated_at=now() WHERE id=$4`, paid, due, invoiceStatus, invoiceID); err != nil {
			return err
		}
		if err = recordInvoiceEvent(ctx, tx, scope, invoiceID, &inv.Status, invoiceStatus, fmt.Sprintf("Verified Square payment reconciliation; net adjustment %d cents", change)); err != nil {
			return err
		}
	}
	if err = jobAudit(ctx, tx, scope, "payment.provider_reconciled", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
