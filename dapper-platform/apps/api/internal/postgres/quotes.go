package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"dapper.local/platform/internal/scheduling"
	"dapper.local/platform/internal/square"
	"github.com/jackc/pgx/v5"
)

var (
	ErrQuoteNotFound           = errors.New("quotation not found in this scope")
	ErrQuoteInput              = errors.New("valid customer, request and at least one line item required")
	ErrQuoteStatusConflict     = errors.New("quotation status does not allow this operation")
	ErrQuoteStale              = errors.New("quotation was modified by another request; reload and retry")
	ErrQuoteAcceptanceConflict = errors.New("acceptance key conflicts with an earlier submission")
	ErrQuoteAlreadyFulfilled   = errors.New("quotation or booking request has already been fulfilled")
	ErrQuoteDepositRequired    = errors.New("verified server-side deposit payment required")
	ErrQuoteExpired            = errors.New("quotation has expired and cannot be accepted or fulfilled")
)

type QuoteItem struct {
	ID             string    `json:"id"`
	SortOrder      int       `json:"sortOrder"`
	Description    string    `json:"description"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unitPriceCents"`
	LineTotalCents int64     `json:"lineTotalCents"`
	ServiceID      *string   `json:"serviceId,omitempty"`
	ServiceName    *string   `json:"serviceName,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type QuoteItemInput struct {
	Description    string  `json:"description"`
	Quantity       int     `json:"quantity"`
	UnitPriceCents int64   `json:"unitPriceCents"`
	ServiceID      *string `json:"serviceId,omitempty"`
	SortOrder      int     `json:"sortOrder"`
}

type QuoteEvent struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	ActorID   string    `json:"actorId"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type QuoteAcceptance struct {
	ID               string    `json:"id"`
	QuotationID      string    `json:"quotationId"`
	QuotationVersion int       `json:"quotationVersion"`
	CustomerID       string    `json:"customerId"`
	ActorID          string    `json:"actorId"`
	ContentSHA256    string    `json:"contentSha256"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Quote struct {
	ID              string           `json:"id"`
	OrganizationID  string           `json:"organizationId"`
	LocationID      string           `json:"locationId"`
	RequestID       string           `json:"requestId"`
	CustomerID      string           `json:"customerId"`
	VehicleID       string           `json:"vehicleId"`
	ReferenceNumber string           `json:"referenceNumber"`
	Status          string           `json:"status"`
	Version         int              `json:"version"`
	Currency        string           `json:"currency"`
	SubtotalCents   int64            `json:"subtotalCents"`
	TaxCents        int64            `json:"taxCents"`
	DepositCents    int64            `json:"depositCents"`
	TotalCents      int64            `json:"totalCents"`
	InternalNotes   string           `json:"internalNotes,omitempty"`
	CustomerNote    string           `json:"customerNote,omitempty"`
	ExpiresAt       *time.Time       `json:"expiresAt,omitempty"`
	IssuedAt        *time.Time       `json:"issuedAt,omitempty"`
	IssuedBy        *string          `json:"issuedBy,omitempty"`
	DecidedAt       *time.Time       `json:"decidedAt,omitempty"`
	BookingID       *string          `json:"bookingId,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	CustomerName    string           `json:"customerName"`
	Items           []QuoteItem      `json:"items"`
	Events          []QuoteEvent     `json:"events,omitempty"`
	Acceptance      *QuoteAcceptance `json:"acceptance,omitempty"`
}

type CreateQuoteInput struct {
	RequestID  string `json:"requestId"`
	CustomerID string `json:"customerId"`
	VehicleID  string `json:"vehicleId"`
	// Staff can quote a client by name instead of a booking request: the name (with optional phone and email) finds or adds the customer.
	ClientName    string           `json:"clientName"`
	ClientPhone   string           `json:"clientPhone"`
	ClientEmail   string           `json:"clientEmail"`
	Items         []QuoteItemInput `json:"items"`
	TaxCents      int64            `json:"taxCents"`
	DepositCents  int64            `json:"depositCents"`
	InternalNotes string           `json:"internalNotes"`
	CustomerNote  string           `json:"customerNote"`
	ExpiresAt     *time.Time       `json:"expiresAt,omitempty"`
}

type EditQuoteInput struct {
	Version       int              `json:"version"`
	Items         []QuoteItemInput `json:"items"`
	TaxCents      int64            `json:"taxCents"`
	DepositCents  int64            `json:"depositCents"`
	InternalNotes string           `json:"internalNotes"`
	CustomerNote  string           `json:"customerNote"`
	ExpiresAt     *time.Time       `json:"expiresAt,omitempty"`
}

type IssueQuoteInput struct {
	Version int    `json:"version"`
	Note    string `json:"note"`
}

type AcceptQuoteInput struct {
	AcceptanceKey string `json:"acceptanceKey"`
	TermsReviewed bool   `json:"termsReviewed"`
}

type DeclineQuoteInput struct {
	AcceptanceKey string `json:"acceptanceKey"`
	Note          string `json:"note"`
}

type FulfillQuoteInput struct {
	RequestKey              string  `json:"requestKey"`
	QuoteID                 string  `json:"quoteId"`
	DepositPaymentID        *string `json:"depositPaymentId,omitempty"`
	ApprovedNoDepositConfig bool    `json:"approvedNoDepositConfig,omitempty"`
	Channel                 string  `json:"channel,omitempty"`
	Notes                   string  `json:"notes,omitempty"`
}

type FulfillQuoteResult struct {
	Quote   Quote   `json:"quote"`
	Booking Booking `json:"booking"`
}

const quoteColumns = `q.id::text, q.organization_id::text, q.location_id::text, COALESCE(q.request_id::text,''), q.customer_id::text, COALESCE(q.vehicle_id::text,''), q.reference_number, q.status, q.version, q.currency, q.subtotal_cents, q.tax_cents, q.deposit_cents, q.total_cents, q.internal_notes, q.customer_note, q.expires_at, q.issued_at, q.issued_by::text, q.decided_at, q.booking_id::text, q.created_at, q.updated_at, COALESCE((SELECT c.name FROM customers c WHERE c.organization_id=q.organization_id AND c.location_id=q.location_id AND c.id=q.customer_id),'')`

func scanQuote(row pgx.Row) (Quote, error) {
	var q Quote
	var issuedBy *string
	var bookingID *string
	err := row.Scan(
		&q.ID, &q.OrganizationID, &q.LocationID, &q.RequestID,
		&q.CustomerID, &q.VehicleID, &q.ReferenceNumber,
		&q.Status, &q.Version, &q.Currency,
		&q.SubtotalCents, &q.TaxCents, &q.DepositCents, &q.TotalCents,
		&q.InternalNotes, &q.CustomerNote,
		&q.ExpiresAt, &q.IssuedAt, &issuedBy, &q.DecidedAt,
		&bookingID, &q.CreatedAt, &q.UpdatedAt, &q.CustomerName,
	)
	if err != nil {
		return Quote{}, err
	}
	q.IssuedBy = issuedBy
	q.BookingID = bookingID
	q.Items = []QuoteItem{}
	q.Events = []QuoteEvent{}
	if q.Status == "issued" && q.ExpiresAt != nil && time.Now().UTC().After(*q.ExpiresAt) {
		q.Status = "expired"
	}
	return q, nil
}

func generateReferenceNumber() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("QTE-%08X", b), nil
}

func computeSubtotal(items []QuoteItemInput) (int64, error) {
	var total int64
	for _, item := range items {
		if item.Quantity < 1 || item.Quantity > 999 {
			return 0, ErrQuoteInput
		}
		if item.UnitPriceCents < 0 || item.UnitPriceCents > 9_000_000_000_000 {
			return 0, ErrQuoteInput
		}
		line := int64(item.Quantity) * item.UnitPriceCents
		total += line
	}
	return total, nil
}

func validateQuoteItems(items []QuoteItemInput) error {
	if len(items) == 0 || len(items) > 50 {
		return ErrQuoteInput
	}
	for _, item := range items {
		if utf8.RuneCountInString(strings.TrimSpace(item.Description)) < 1 {
			return ErrQuoteInput
		}
		if utf8.RuneCountInString(item.Description) > 500 {
			return ErrQuoteInput
		}
		if item.Quantity < 1 || item.Quantity > 999 {
			return ErrQuoteInput
		}
		if item.UnitPriceCents < 0 || item.UnitPriceCents > 9_000_000_000_000 {
			return ErrQuoteInput
		}
	}
	return nil
}

func (s *Store) CreateQuote(ctx context.Context, scope Scope, input CreateQuoteInput) (Quote, error) {
	input.InternalNotes = strings.TrimSpace(input.InternalNotes)
	input.CustomerNote = strings.TrimSpace(input.CustomerNote)
	input.ClientName, input.ClientPhone, input.ClientEmail = strings.TrimSpace(input.ClientName), strings.TrimSpace(input.ClientPhone), strings.TrimSpace(input.ClientEmail)
	withRequest := input.RequestID != "" || input.VehicleID != ""
	if withRequest && (!uuid(input.RequestID) || !uuid(input.CustomerID) || !uuid(input.VehicleID)) {
		return Quote{}, ErrQuoteInput
	}
	if !withRequest && !uuid(input.CustomerID) && utf8.RuneCountInString(input.ClientName) < 2 {
		return Quote{}, ErrQuoteInput
	}
	if utf8.RuneCountInString(input.ClientName) > 100 || len(input.ClientEmail) > 254 || utf8.RuneCountInString(input.ClientPhone) > 30 {
		return Quote{}, ErrQuoteInput
	}
	if err := validateQuoteItems(input.Items); err != nil {
		return Quote{}, err
	}
	if input.TaxCents < 0 || input.TaxCents > 9_000_000_000_000 {
		return Quote{}, ErrQuoteInput
	}
	if utf8.RuneCountInString(input.InternalNotes) > 2000 || utf8.RuneCountInString(input.CustomerNote) > 2000 {
		return Quote{}, ErrQuoteInput
	}
	if input.ExpiresAt != nil {
		t := input.ExpiresAt.UTC()
		input.ExpiresAt = &t
		if t.Before(time.Now().UTC().Add(5 * time.Minute)) {
			return Quote{}, ErrQuoteInput
		}
	}
	subtotal, err := computeSubtotal(input.Items)
	if err != nil {
		return Quote{}, err
	}
	if input.DepositCents < 0 || input.DepositCents > subtotal+input.TaxCents {
		return Quote{}, ErrQuoteInput
	}

	refNum, err := generateReferenceNumber()
	if err != nil {
		return Quote{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "quotes.create"); err != nil {
		return Quote{}, err
	}

	if !withRequest && !uuid(input.CustomerID) {
		if err = authorizeTransaction(ctx, tx, scope, "customers.create"); err != nil {
			return Quote{}, err
		}
		found, ferr := existingCustomer(ctx, tx, scope.OrganizationID, scope.LocationID, input.ClientEmail, input.ClientPhone)
		if ferr != nil {
			return Quote{}, ferr
		}
		if found == "" {
			if err = tx.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, scope.OrganizationID, scope.LocationID, input.ClientName, input.ClientEmail, input.ClientPhone).Scan(&found); err != nil {
				return Quote{}, customerUniqueness(err)
			}
		}
		input.CustomerID = found
	} else if !withRequest {
		var known bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND removed_at IS NULL)`, scope.OrganizationID, scope.LocationID, input.CustomerID).Scan(&known); err != nil || !known {
			return Quote{}, ErrQuoteInput
		}
	}

	var reqOrg, reqLoc string
	if withRequest {
		err = tx.QueryRow(ctx, `SELECT organization_id::text, location_id::text FROM customer_booking_requests WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND customer_id=$4 AND vehicle_id=$5 FOR SHARE`,
			scope.OrganizationID, scope.LocationID, input.RequestID, input.CustomerID, input.VehicleID).Scan(&reqOrg, &reqLoc)
		if errors.Is(err, pgx.ErrNoRows) {
			return Quote{}, ErrQuoteInput
		}
		if err != nil {
			return Quote{}, err
		}
	}

	var quoteID string
	err = tx.QueryRow(ctx, `INSERT INTO scoped_quotations(organization_id,location_id,request_id,customer_id,vehicle_id,currency,status,subtotal_cents,tax_cents,deposit_cents,reference_number,internal_notes,customer_note,expires_at) VALUES($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,'')::uuid,'CAD','draft',$6,$7,$8,$9,$10,$11,$12) RETURNING id::text`,
		scope.OrganizationID, scope.LocationID, input.RequestID, input.CustomerID, input.VehicleID,
		subtotal, input.TaxCents, input.DepositCents, refNum,
		input.InternalNotes, input.CustomerNote, input.ExpiresAt,
	).Scan(&quoteID)
	if err != nil {
		return Quote{}, err
	}

	if err = insertQuoteItems(ctx, tx, quoteID, scope, input.Items); err != nil {
		return Quote{}, err
	}

	if _, err = tx.Exec(ctx, `INSERT INTO scoped_quotation_events(quotation_id,organization_id,location_id,actor_id,action) VALUES($1,$2,$3,$4,'created')`,
		quoteID, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return Quote{}, err
	}

	if err = jobAudit(ctx, tx, scope, "quotation.created", quoteID); err != nil {
		return Quote{}, err
	}

	quote, err := loadQuoteFull(ctx, tx, scope.OrganizationID, scope.LocationID, quoteID)
	if err != nil {
		return Quote{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Quote{}, err
	}
	return quote, nil
}

func insertQuoteItems(ctx context.Context, tx pgx.Tx, quoteID string, scope Scope, items []QuoteItemInput) error {
	for i, item := range items {
		sortOrder := item.SortOrder
		if sortOrder == 0 {
			sortOrder = i + 1
		}
		desc := strings.TrimSpace(item.Description)
		if item.ServiceID != nil {
			var svcName string
			err := tx.QueryRow(ctx, `SELECT name FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND NOT archived`,
				scope.OrganizationID, scope.LocationID, *item.ServiceID).Scan(&svcName)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrQuoteInput
			}
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO scoped_quotation_items(quotation_id,sort_order,description,quantity,unit_price_cents,service_id,service_name_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7)`,
				quoteID, sortOrder, desc, item.Quantity, item.UnitPriceCents, *item.ServiceID, svcName); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx, `INSERT INTO scoped_quotation_items(quotation_id,sort_order,description,quantity,unit_price_cents) VALUES($1,$2,$3,$4,$5)`,
				quoteID, sortOrder, desc, item.Quantity, item.UnitPriceCents); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadQuoteItems(ctx context.Context, tx pgx.Tx, quoteID string) ([]QuoteItem, error) {
	rows, err := tx.Query(ctx, `SELECT id::text,sort_order,description,quantity,unit_price_cents,(quantity::bigint*unit_price_cents),service_id::text,service_name_snapshot,created_at FROM scoped_quotation_items WHERE quotation_id=$1 ORDER BY sort_order,id`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []QuoteItem{}
	for rows.Next() {
		var item QuoteItem
		if err = rows.Scan(&item.ID, &item.SortOrder, &item.Description, &item.Quantity, &item.UnitPriceCents, &item.LineTotalCents, &item.ServiceID, &item.ServiceName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadQuoteEvents(ctx context.Context, tx pgx.Tx, quoteID string) ([]QuoteEvent, error) {
	rows, err := tx.Query(ctx, `SELECT id::text,action,actor_id::text,note,created_at FROM scoped_quotation_events WHERE quotation_id=$1 ORDER BY created_at,id`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []QuoteEvent{}
	for rows.Next() {
		var ev QuoteEvent
		if err = rows.Scan(&ev.ID, &ev.Action, &ev.ActorID, &ev.Note, &ev.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

func loadQuoteAcceptance(ctx context.Context, tx pgx.Tx, quoteID string) (*QuoteAcceptance, error) {
	var acc QuoteAcceptance
	var sha []byte
	err := tx.QueryRow(ctx, `SELECT id::text,quotation_id::text,quotation_version,customer_id::text,actor_id::text,content_sha256,created_at FROM scoped_quotation_acceptances WHERE quotation_id=$1 LIMIT 1`, quoteID).
		Scan(&acc.ID, &acc.QuotationID, &acc.QuotationVersion, &acc.CustomerID, &acc.ActorID, &sha, &acc.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	acc.ContentSHA256 = fmt.Sprintf("%x", sha)
	return &acc, nil
}

func loadQuoteFull(ctx context.Context, tx pgx.Tx, org, loc, quoteID string) (Quote, error) {
	q, err := scanQuote(tx.QueryRow(ctx, `SELECT `+quoteColumns+` FROM scoped_quotations q WHERE q.organization_id=$1 AND q.location_id=$2 AND q.id=$3`, org, loc, quoteID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrQuoteNotFound
	}
	if err != nil {
		return Quote{}, err
	}
	q.Items, err = loadQuoteItems(ctx, tx, quoteID)
	if err != nil {
		return Quote{}, err
	}
	q.Events, err = loadQuoteEvents(ctx, tx, quoteID)
	if err != nil {
		return Quote{}, err
	}
	q.Acceptance, err = loadQuoteAcceptance(ctx, tx, quoteID)
	if err != nil {
		return Quote{}, err
	}
	return q, nil
}

func (s *Store) GetQuote(ctx context.Context, scope Scope, quoteID string) (Quote, error) {
	if err := s.authorize(ctx, scope, "quotes.read"); err != nil {
		return Quote{}, err
	}
	if !uuid(quoteID) {
		return Quote{}, ErrQuoteNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)
	q, err := loadQuoteFull(ctx, tx, scope.OrganizationID, scope.LocationID, quoteID)
	if err != nil {
		return Quote{}, err
	}
	return q, tx.Commit(ctx)
}

func (s *Store) ListQuotes(ctx context.Context, scope Scope) ([]Quote, error) {
	if err := s.authorize(ctx, scope, "quotes.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+quoteColumns+` FROM scoped_quotations q WHERE q.organization_id=$1 AND q.location_id=$2 ORDER BY q.created_at DESC,q.id LIMIT 200`,
		scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	quotes := []Quote{}
	for rows.Next() {
		q, e := scanQuote(rows)
		if e != nil {
			return nil, e
		}
		quotes = append(quotes, q)
	}
	return quotes, rows.Err()
}

func (s *Store) EditQuote(ctx context.Context, scope Scope, quoteID string, input EditQuoteInput) (Quote, error) {
	if !uuid(quoteID) {
		return Quote{}, ErrQuoteNotFound
	}
	input.InternalNotes = strings.TrimSpace(input.InternalNotes)
	input.CustomerNote = strings.TrimSpace(input.CustomerNote)
	if err := validateQuoteItems(input.Items); err != nil {
		return Quote{}, err
	}
	if input.TaxCents < 0 || input.TaxCents > 9_000_000_000_000 {
		return Quote{}, ErrQuoteInput
	}
	if utf8.RuneCountInString(input.InternalNotes) > 2000 || utf8.RuneCountInString(input.CustomerNote) > 2000 {
		return Quote{}, ErrQuoteInput
	}
	if input.ExpiresAt != nil {
		t := input.ExpiresAt.UTC()
		input.ExpiresAt = &t
		if t.Before(time.Now().UTC().Add(5 * time.Minute)) {
			return Quote{}, ErrQuoteInput
		}
	}
	subtotal, err := computeSubtotal(input.Items)
	if err != nil {
		return Quote{}, err
	}
	if input.DepositCents < 0 || input.DepositCents > subtotal+input.TaxCents {
		return Quote{}, ErrQuoteInput
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "quotes.manage"); err != nil {
		return Quote{}, err
	}

	var currentVersion int
	var currentStatus string
	err = tx.QueryRow(ctx, `SELECT version, status FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, quoteID).Scan(&currentVersion, &currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrQuoteNotFound
	}
	if err != nil {
		return Quote{}, err
	}
	if currentStatus != "draft" {
		return Quote{}, ErrQuoteStatusConflict
	}
	if currentVersion != input.Version {
		return Quote{}, ErrQuoteStale
	}

	if _, err = tx.Exec(ctx, `DELETE FROM scoped_quotation_items WHERE quotation_id=$1`, quoteID); err != nil {
		return Quote{}, err
	}
	if err = insertQuoteItems(ctx, tx, quoteID, scope, input.Items); err != nil {
		return Quote{}, err
	}

	var newVersion int
	err = tx.QueryRow(ctx, `UPDATE scoped_quotations SET version=version+1, subtotal_cents=$1, tax_cents=$2, deposit_cents=$3, internal_notes=$4, customer_note=$5, expires_at=$6, updated_at=now() WHERE organization_id=$7 AND location_id=$8 AND id=$9 RETURNING version`,
		subtotal, input.TaxCents, input.DepositCents, input.InternalNotes, input.CustomerNote, input.ExpiresAt,
		scope.OrganizationID, scope.LocationID, quoteID,
	).Scan(&newVersion)
	if err != nil {
		return Quote{}, err
	}

	if _, err = tx.Exec(ctx, `INSERT INTO scoped_quotation_events(quotation_id,organization_id,location_id,actor_id,action) VALUES($1,$2,$3,$4,'edited')`,
		quoteID, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return Quote{}, err
	}

	if err = jobAudit(ctx, tx, scope, "quotation.edited", quoteID); err != nil {
		return Quote{}, err
	}

	q, err := loadQuoteFull(ctx, tx, scope.OrganizationID, scope.LocationID, quoteID)
	if err != nil {
		return Quote{}, err
	}
	return q, tx.Commit(ctx)
}

func (s *Store) IssueQuote(ctx context.Context, scope Scope, quoteID string, input IssueQuoteInput) (Quote, error) {
	if !uuid(quoteID) {
		return Quote{}, ErrQuoteNotFound
	}
	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > 500 {
		return Quote{}, ErrQuoteInput
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "quotes.issue"); err != nil {
		return Quote{}, err
	}

	var currentVersion int
	var currentStatus string
	err = tx.QueryRow(ctx, `SELECT version, status FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, quoteID).Scan(&currentVersion, &currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrQuoteNotFound
	}
	if err != nil {
		return Quote{}, err
	}
	if currentStatus != "draft" {
		return Quote{}, ErrQuoteStatusConflict
	}
	if currentVersion != input.Version {
		return Quote{}, ErrQuoteStale
	}

	if _, err = tx.Exec(ctx, `UPDATE scoped_quotations SET status='issued', issued_at=now(), issued_by=$1, version=version+1, updated_at=now() WHERE organization_id=$2 AND location_id=$3 AND id=$4`,
		scope.ActorID, scope.OrganizationID, scope.LocationID, quoteID); err != nil {
		return Quote{}, err
	}

	if _, err = tx.Exec(ctx, `INSERT INTO scoped_quotation_events(quotation_id,organization_id,location_id,actor_id,action,note) VALUES($1,$2,$3,$4,'issued',$5)`,
		quoteID, scope.OrganizationID, scope.LocationID, scope.ActorID, note); err != nil {
		return Quote{}, err
	}

	if err = jobAudit(ctx, tx, scope, "quotation.issued", quoteID); err != nil {
		return Quote{}, err
	}

	q, err := loadQuoteFull(ctx, tx, scope.OrganizationID, scope.LocationID, quoteID)
	if err != nil {
		return Quote{}, err
	}
	return q, tx.Commit(ctx)
}

func (s *Store) AcceptQuote(ctx context.Context, actorID, customerID, quoteID string, input AcceptQuoteInput) (Quote, error) {
	if actorID == "" || customerID == "" || !uuid(quoteID) {
		return Quote{}, ErrForbidden
	}
	if !input.TermsReviewed {
		return Quote{}, ErrQuoteInput
	}
	if len(input.AcceptanceKey) < 16 || len(input.AcceptanceKey) > 128 {
		return Quote{}, ErrQuoteInput
	}
	for _, c := range input.AcceptanceKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return Quote{}, ErrQuoteInput
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)

	var org, loc string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text, a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`,
		actorID, customerID).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrForbidden
	}
	if err != nil {
		return Quote{}, err
	}

	var currentStatus string
	var currentVersion int
	var expiresAt *time.Time
	err = tx.QueryRow(ctx, `SELECT status, version, expires_at FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND customer_id=$4 FOR UPDATE`,
		org, loc, quoteID, customerID).Scan(&currentStatus, &currentVersion, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrQuoteNotFound
	}
	if err != nil {
		return Quote{}, err
	}

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, actorID+"/quote-accept/"+input.AcceptanceKey); err != nil {
		return Quote{}, err
	}

	var existingAccID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM scoped_quotation_acceptances WHERE actor_id=$1 AND acceptance_key=$2`, actorID, input.AcceptanceKey).Scan(&existingAccID)
	if err == nil {
		var accQuoteID string
		if err2 := tx.QueryRow(ctx, `SELECT quotation_id::text FROM scoped_quotation_acceptances WHERE id=$1`, existingAccID).Scan(&accQuoteID); err2 != nil {
			return Quote{}, err2
		}
		if accQuoteID != quoteID {
			return Quote{}, ErrQuoteAcceptanceConflict
		}
		q, err2 := loadQuoteFull(ctx, tx, org, loc, quoteID)
		if err2 != nil {
			return Quote{}, err2
		}
		return q, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, err
	}

	if currentStatus == "accepted" || currentStatus == "fulfilled" {
		q, e := loadQuoteFull(ctx, tx, org, loc, quoteID)
		if e != nil {
			return Quote{}, e
		}
		return q, tx.Commit(ctx)
	}
	if currentStatus != "issued" {
		return Quote{}, ErrQuoteStatusConflict
	}
	if expiresAt != nil && time.Now().UTC().After(*expiresAt) {
		return Quote{}, ErrQuoteExpired
	}

	contentJSON, err := buildQuoteContentJSON(ctx, tx, org, loc, quoteID)
	if err != nil {
		return Quote{}, err
	}
	digest := sha256.Sum256(contentJSON)

	var accID string
	err = tx.QueryRow(ctx, `INSERT INTO scoped_quotation_acceptances(quotation_id,quotation_version,customer_id,content_sha256,actor_id,acceptance_key,terms_reviewed) VALUES($1,$2,$3,$4,$5,$6,true) RETURNING id::text`,
		quoteID, currentVersion, customerID, digest[:], actorID, input.AcceptanceKey).Scan(&accID)
	if err != nil {
		return Quote{}, err
	}

	if _, err = tx.Exec(ctx, `UPDATE scoped_quotations SET status='accepted', decided_at=now(), version=version+1, updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3`,
		org, loc, quoteID); err != nil {
		return Quote{}, err
	}

	if _, err = tx.Exec(ctx, `INSERT INTO scoped_quotation_events(quotation_id,organization_id,location_id,actor_id,action) VALUES($1,$2,$3,$4,'accepted')`,
		quoteID, org, loc, actorID); err != nil {
		return Quote{}, err
	}

	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: actorID}
	if err = jobAudit(ctx, tx, scope, "quotation.accepted", quoteID); err != nil {
		return Quote{}, err
	}

	q, err := loadQuoteFull(ctx, tx, org, loc, quoteID)
	if err != nil {
		return Quote{}, err
	}
	return q, tx.Commit(ctx)
}

func (s *Store) DeclineQuote(ctx context.Context, actorID, customerID, quoteID string, input DeclineQuoteInput) (Quote, error) {
	if actorID == "" || customerID == "" || !uuid(quoteID) {
		return Quote{}, ErrForbidden
	}
	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > 500 {
		return Quote{}, ErrQuoteInput
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)

	var org, loc string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text, a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`,
		actorID, customerID).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrForbidden
	}
	if err != nil {
		return Quote{}, err
	}

	var currentStatus string
	var expiresAt *time.Time
	err = tx.QueryRow(ctx, `SELECT status, expires_at FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND customer_id=$4 FOR UPDATE`,
		org, loc, quoteID, customerID).Scan(&currentStatus, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrQuoteNotFound
	}
	if err != nil {
		return Quote{}, err
	}
	if currentStatus != "issued" {
		return Quote{}, ErrQuoteStatusConflict
	}
	if expiresAt != nil && time.Now().UTC().After(*expiresAt) {
		return Quote{}, ErrQuoteExpired
	}

	if _, err = tx.Exec(ctx, `UPDATE scoped_quotations SET status='declined', decided_at=now(), version=version+1, updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3`,
		org, loc, quoteID); err != nil {
		return Quote{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO scoped_quotation_events(quotation_id,organization_id,location_id,actor_id,action,note) VALUES($1,$2,$3,$4,'declined',$5)`,
		quoteID, org, loc, actorID, note); err != nil {
		return Quote{}, err
	}

	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: actorID}
	if err = jobAudit(ctx, tx, scope, "quotation.declined", quoteID); err != nil {
		return Quote{}, err
	}

	q, err := loadQuoteFull(ctx, tx, org, loc, quoteID)
	if err != nil {
		return Quote{}, err
	}
	return q, tx.Commit(ctx)
}

func (s *Store) CustomerQuotes(ctx context.Context, actorID, customerID string) ([]Quote, error) {
	if actorID == "" || customerID == "" {
		return nil, ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var org, loc string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text, a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`,
		actorID, customerID).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `SELECT `+quoteColumns+` FROM scoped_quotations q WHERE q.organization_id=$1 AND q.location_id=$2 AND q.customer_id=$3 AND q.status NOT IN ('draft','superseded') ORDER BY q.created_at DESC,q.id LIMIT 100`,
		org, loc, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	quotes := []Quote{}
	for rows.Next() {
		q, e := scanQuote(rows)
		if e != nil {
			return nil, e
		}
		quotes = append(quotes, q)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return quotes, tx.Commit(ctx)
}

func (s *Store) GetCustomerQuote(ctx context.Context, actorID, customerID, quoteID string) (Quote, error) {
	if actorID == "" || customerID == "" || !uuid(quoteID) {
		return Quote{}, ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)

	var org, loc string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text, a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`,
		actorID, customerID).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrForbidden
	}
	if err != nil {
		return Quote{}, err
	}

	var currentStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND customer_id=$4`,
		org, loc, quoteID, customerID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, ErrQuoteNotFound
	}
	if err != nil {
		return Quote{}, err
	}
	if currentStatus == "draft" || currentStatus == "superseded" {
		return Quote{}, ErrQuoteNotFound
	}

	q, err := loadQuoteFull(ctx, tx, org, loc, quoteID)
	if err != nil {
		return Quote{}, err
	}
	return q, tx.Commit(ctx)
}

// FulfillQuote converts an accepted quotation into a confirmed capacity reservation
// and booking atomically. Locks resources in sorted order before reservation.
// Enforces exactly one conversion receipt per request and per quotation.
func (s *Store) FulfillQuote(ctx context.Context, scope Scope, input FulfillQuoteInput) (FulfillQuoteResult, error) {
	input.Notes = strings.TrimSpace(input.Notes)
	if !uuid(input.QuoteID) || len(input.RequestKey) < 16 || len(input.RequestKey) > 128 || utf8.RuneCountInString(input.Notes) > 2000 {
		return FulfillQuoteResult{}, ErrQuoteInput
	}
	for _, c := range input.RequestKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return FulfillQuoteResult{}, ErrQuoteInput
		}
	}
	channel := input.Channel
	if channel == "" {
		channel = "online"
	}
	switch channel {
	case "online", "internal", "phone", "walk_in", "dealership":
	default:
		return FulfillQuoteResult{}, ErrBookingInput
	}

	encoded, err := json.Marshal(struct {
		Scope Scope
		Input FulfillQuoteInput
	}{scope, input})
	if err != nil {
		return FulfillQuoteResult{}, err
	}
	digest := sha256.Sum256(encoded)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return FulfillQuoteResult{}, err
	}
	defer tx.Rollback(ctx)

	for _, action := range []string{"quotes.manage", "bookings.create", "scheduling.reserve"} {
		if err = authorizeTransaction(ctx, tx, scope, action); err != nil {
			return FulfillQuoteResult{}, err
		}
	}

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.OrganizationID+"/"+scope.LocationID+"/quote-fulfill/"+input.RequestKey); err != nil {
		return FulfillQuoteResult{}, err
	}

	// Idempotent retry check on exact requestKey
	var existingBookingID, existingQuoteID string
	var existingHash []byte
	err = tx.QueryRow(ctx, `SELECT booking_id::text, quotation_id::text, request_hash FROM quote_fulfillment_receipts WHERE actor_id=$1 AND request_key=$2`,
		scope.ActorID, input.RequestKey).Scan(&existingBookingID, &existingQuoteID, &existingHash)
	if err == nil {
		if string(existingHash) != string(digest[:]) {
			return FulfillQuoteResult{}, ErrCustomerBookingRequestConflict
		}
		booking, e := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3 FOR SHARE OF b`, scope.OrganizationID, scope.LocationID, existingBookingID))
		if e != nil {
			return FulfillQuoteResult{}, e
		}
		q, e := loadQuoteFull(ctx, tx, scope.OrganizationID, scope.LocationID, existingQuoteID)
		if e != nil {
			return FulfillQuoteResult{}, e
		}
		if err = tx.Commit(ctx); err != nil {
			return FulfillQuoteResult{}, bookingWriteError(err)
		}
		return FulfillQuoteResult{Quote: q, Booking: bookingMutationSummary(booking)}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return FulfillQuoteResult{}, err
	}

	// Lock the quotation row for conversion
	var quoteID, requestID, customerID, vehicleID, status, currency string
	var version int
	var depositCents, totalCents int64
	var expiresAt *time.Time
	err = tx.QueryRow(ctx, `SELECT id::text, COALESCE(request_id::text,''), customer_id::text, COALESCE(vehicle_id::text,''), status, version, deposit_cents, total_cents, currency, expires_at FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, input.QuoteID).
		Scan(&quoteID, &requestID, &customerID, &vehicleID, &status, &version, &depositCents, &totalCents, &currency, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return FulfillQuoteResult{}, ErrQuoteNotFound
	}
	if err != nil {
		return FulfillQuoteResult{}, err
	}

	if status == "fulfilled" {
		return FulfillQuoteResult{}, ErrQuoteAlreadyFulfilled
	}
	if status != "accepted" {
		return FulfillQuoteResult{}, ErrQuoteStatusConflict
	}
	if expiresAt != nil && time.Now().UTC().After(*expiresAt) {
		return FulfillQuoteResult{}, ErrQuoteExpired
	}

	// Verify one conversion receipt per request and per quotation
	var receiptExists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM quote_fulfillment_receipts WHERE organization_id=$1 AND location_id=$2 AND (quotation_id=$3 OR request_id=$4))`,
		scope.OrganizationID, scope.LocationID, quoteID, requestID).Scan(&receiptExists)
	if err != nil {
		return FulfillQuoteResult{}, err
	}
	if receiptExists {
		return FulfillQuoteResult{}, ErrQuoteAlreadyFulfilled
	}

	// Deposit verification: verified server-side payment or approved configuration
	if depositCents > 0 {
		if input.DepositPaymentID == nil || *input.DepositPaymentID == "" {
			var autoPayID string
			var autoAmt int64
			var autoCurr, autoStatus string
			_ = tx.QueryRow(ctx, `
				SELECT p.id::text, p.amount_cents, p.currency, p.status
				FROM payments p
				JOIN invoices i ON (i.organization_id, i.location_id, i.customer_id, i.id) = (p.organization_id, p.location_id, p.customer_id, p.invoice_id)
				WHERE p.organization_id=$1 AND p.location_id=$2 AND p.customer_id=$3 AND p.status='completed'
				  AND i.notes LIKE '%' || (SELECT reference_number FROM scoped_quotations WHERE id=$4) || '%'
				ORDER BY p.created_at DESC LIMIT 1
			`, scope.OrganizationID, scope.LocationID, customerID, quoteID).Scan(&autoPayID, &autoAmt, &autoCurr, &autoStatus)
			if autoPayID != "" && autoStatus == "completed" && autoCurr == currency && autoAmt >= depositCents {
				input.DepositPaymentID = &autoPayID
			}
		}

		if input.DepositPaymentID != nil && *input.DepositPaymentID != "" {
			var payAmount int64
			var payCurr, payStatus string
			err = tx.QueryRow(ctx, `SELECT amount_cents, currency, status FROM payments WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND customer_id=$4`,
				scope.OrganizationID, scope.LocationID, *input.DepositPaymentID, customerID).
				Scan(&payAmount, &payCurr, &payStatus)
			if err != nil || payStatus != "completed" || payCurr != currency || payAmount < depositCents {
				return FulfillQuoteResult{}, ErrQuoteDepositRequired
			}
		} else if input.ApprovedNoDepositConfig {
			// Explicit reviewed exemption configuration allowed
		} else {
			return FulfillQuoteResult{}, ErrQuoteDepositRequired
		}
	}

	// Load request snapshot to obtain scheduling rule and requested start time
	var requestedStart time.Time
	var offerSnapshot, ruleSnapshot []byte
	err = tx.QueryRow(ctx, `SELECT requested_start, offer_snapshot, rule_snapshot FROM customer_booking_requests WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`,
		scope.OrganizationID, scope.LocationID, requestID).Scan(&requestedStart, &offerSnapshot, &ruleSnapshot)
	if err != nil {
		return FulfillQuoteResult{}, err
	}

	var offerWrapper struct {
		Configuration struct {
			ResourceID string `json:"resourceId"`
		} `json:"configuration"`
	}
	if err = json.Unmarshal(offerSnapshot, &offerWrapper); err != nil || offerWrapper.Configuration.ResourceID == "" {
		return FulfillQuoteResult{}, ErrQuoteInput
	}
	resourceID := offerWrapper.Configuration.ResourceID

	var rule SchedulingRule
	if err = json.Unmarshal(ruleSnapshot, &rule); err != nil || rule.ID == "" {
		return FulfillQuoteResult{}, ErrQuoteInput
	}

	// Sorted resource locking to prevent deadlocks under concurrency
	resources := []string{resourceID}
	sort.Strings(resources)
	for _, res := range resources {
		var locked string
		err = tx.QueryRow(ctx, `SELECT id::text FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`,
			scope.OrganizationID, scope.LocationID, res).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return FulfillQuoteResult{}, ErrReservationConflict
		}
		if err != nil {
			return FulfillQuoteResult{}, err
		}
	}

	// Re-evaluate rule/capacity and write capacity reservation atomically
	reservation, err := s.writeReservationInTransaction(ctx, tx, scope, resourceID, "", 0, scheduling.Request{Start: requestedStart}, &reservationRuleChoice{ID: rule.ID, Version: rule.Version})
	if err != nil {
		return FulfillQuoteResult{}, reservationCommitError(err)
	}

	bookingInput := Booking{
		CustomerID:    customerID,
		VehicleID:     &vehicleID,
		ServiceID:     &rule.ServiceID,
		AddonIDs:      rule.AddonIDs,
		ReservationID: &reservation.ID,
		Channel:       channel,
		Status:        "confirmed",
		BookingTime:   reservation.Request.Start,
		Notes:         input.Notes,
	}
	booking, err := s.createBookingInTransaction(ctx, tx, scope, bookingInput)
	if err != nil {
		return FulfillQuoteResult{}, bookingWriteError(err)
	}

	// Link booking to quotation
	if _, err = tx.Exec(ctx, `UPDATE bookings SET quotation_id=$1 WHERE id=$2`, quoteID, booking.ID); err != nil {
		return FulfillQuoteResult{}, err
	}

	// Update quotation status to fulfilled
	if _, err = tx.Exec(ctx, `UPDATE scoped_quotations SET status='fulfilled', booking_id=$1, decided_at=now(), version=version+1, updated_at=now() WHERE organization_id=$2 AND location_id=$3 AND id=$4`,
		booking.ID, scope.OrganizationID, scope.LocationID, quoteID); err != nil {
		return FulfillQuoteResult{}, err
	}

	// Record quote event
	if _, err = tx.Exec(ctx, `INSERT INTO scoped_quotation_events(quotation_id,organization_id,location_id,actor_id,action,note) VALUES($1,$2,$3,$4,'fulfilled',$5)`,
		quoteID, scope.OrganizationID, scope.LocationID, scope.ActorID, "Fulfilled to booking "+booking.ID); err != nil {
		return FulfillQuoteResult{}, err
	}

	// Record fulfillment receipt
	if _, err = tx.Exec(ctx, `INSERT INTO quote_fulfillment_receipts(organization_id,location_id,request_id,quotation_id,booking_id,actor_id,request_key,request_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		scope.OrganizationID, scope.LocationID, requestID, quoteID, booking.ID, scope.ActorID, input.RequestKey, digest[:]); err != nil {
		return FulfillQuoteResult{}, err
	}

	if err = jobAudit(ctx, tx, scope, "quotation.fulfilled", quoteID); err != nil {
		return FulfillQuoteResult{}, err
	}

	fullQuote, err := loadQuoteFull(ctx, tx, scope.OrganizationID, scope.LocationID, quoteID)
	if err != nil {
		return FulfillQuoteResult{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return FulfillQuoteResult{}, bookingWriteError(err)
	}
	return FulfillQuoteResult{Quote: fullQuote, Booking: bookingMutationSummary(booking)}, nil
}

func buildQuoteContentJSON(ctx context.Context, tx pgx.Tx, org, loc, quoteID string) ([]byte, error) {
	type canonItem struct {
		Description    string `json:"description"`
		Quantity       int    `json:"quantity"`
		UnitPriceCents int64  `json:"unitPriceCents"`
		LineTotalCents int64  `json:"lineTotalCents"`
	}
	type canonQuote struct {
		ID              string      `json:"id"`
		RequestID       string      `json:"requestId"`
		ReferenceNumber string      `json:"referenceNumber"`
		Version         int         `json:"version"`
		Currency        string      `json:"currency"`
		SubtotalCents   int64       `json:"subtotalCents"`
		TaxCents        int64       `json:"taxCents"`
		DepositCents    int64       `json:"depositCents"`
		TotalCents      int64       `json:"totalCents"`
		CustomerNote    string      `json:"customerNote"`
		IssuedAt        *time.Time  `json:"issuedAt"`
		Items           []canonItem `json:"items"`
	}
	var cq canonQuote
	err := tx.QueryRow(ctx, `SELECT id::text, COALESCE(request_id::text,''), reference_number, version, currency, subtotal_cents, tax_cents, deposit_cents, total_cents, customer_note, issued_at FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND id=$3`,
		org, loc, quoteID).Scan(&cq.ID, &cq.RequestID, &cq.ReferenceNumber, &cq.Version, &cq.Currency, &cq.SubtotalCents, &cq.TaxCents, &cq.DepositCents, &cq.TotalCents, &cq.CustomerNote, &cq.IssuedAt)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT description, quantity, unit_price_cents, (quantity::bigint*unit_price_cents) FROM scoped_quotation_items WHERE quotation_id=$1 ORDER BY sort_order,id`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ci canonItem
		if err = rows.Scan(&ci.Description, &ci.Quantity, &ci.UnitPriceCents, &ci.LineTotalCents); err != nil {
			return nil, err
		}
		cq.Items = append(cq.Items, ci)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(cq)
}

func uuid(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

type CustomerPayQuoteDepositInput struct {
	SourceID       string `json:"sourceId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type CustomerQuoteDepositResult struct {
	PaymentID   string `json:"paymentId"`
	QuotationID string `json:"quotationId"`
	AmountCents int64  `json:"amountCents"`
	Status      string `json:"status"`
	InvoiceID   string `json:"invoiceId"`
	ReceiptURL  string `json:"receiptUrl,omitempty"`
}

func (s *Store) CustomerPayQuoteDeposit(ctx context.Context, actorID, customerID, quoteID string, client square.Client, input CustomerPayQuoteDepositInput) (CustomerQuoteDepositResult, error) {
	if client == nil {
		return CustomerQuoteDepositResult{}, ErrPaymentProvider
	}
	if actorID == "" || customerID == "" || !uuid(quoteID) {
		return CustomerQuoteDepositResult{}, ErrForbidden
	}
	input.SourceID = strings.TrimSpace(input.SourceID)
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if input.SourceID == "" || len(idempotencyKey) < 8 || len(idempotencyKey) > 128 {
		return CustomerQuoteDepositResult{}, ErrPaymentInput
	}
	for _, c := range idempotencyKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return CustomerQuoteDepositResult{}, ErrPaymentInput
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}
	defer tx.Rollback(ctx)

	var org, loc string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text, a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`,
		actorID, customerID).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerQuoteDepositResult{}, ErrForbidden
	}
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	var qID, refNum, status string
	var depositCents int64
	var expiresAt *time.Time
	err = tx.QueryRow(ctx, `SELECT id::text, reference_number, status, deposit_cents, expires_at FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND customer_id=$4 FOR UPDATE`,
		org, loc, quoteID, customerID).Scan(&qID, &refNum, &status, &depositCents, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerQuoteDepositResult{}, ErrQuoteNotFound
	}
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	if status == "fulfilled" {
		return CustomerQuoteDepositResult{}, ErrQuoteAlreadyFulfilled
	}
	if status != "accepted" {
		return CustomerQuoteDepositResult{}, ErrQuoteStatusConflict
	}
	if expiresAt != nil && time.Now().UTC().After(*expiresAt) {
		return CustomerQuoteDepositResult{}, ErrQuoteExpired
	}
	if depositCents <= 0 {
		return CustomerQuoteDepositResult{}, ErrQuoteInput
	}

	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, org+"/"+loc+"/quote-dep/"+idempotencyKey); err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	// Idempotency check: if payment already recorded for this key, return it
	var existingPmtID, existingInvID, existingStatus string
	var existingAmt int64
	err = tx.QueryRow(ctx, `SELECT id::text, invoice_id::text, amount_cents, status FROM payments WHERE organization_id=$1 AND location_id=$2 AND idempotency_key=$3`,
		org, loc, idempotencyKey).Scan(&existingPmtID, &existingInvID, &existingAmt, &existingStatus)
	if err == nil {
		if existingAmt != depositCents {
			return CustomerQuoteDepositResult{}, ErrPaymentRequest
		}
		if err = tx.Commit(ctx); err != nil {
			return CustomerQuoteDepositResult{}, err
		}
		return CustomerQuoteDepositResult{
			PaymentID:   existingPmtID,
			QuotationID: quoteID,
			AmountCents: existingAmt,
			Status:      existingStatus,
			InvoiceID:   existingInvID,
		}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CustomerQuoteDepositResult{}, err
	}

	// Load bound invoice issuer
	var issuerID, legalName, address, taxReg, curr, prefix string
	var issuerVersion int
	err = tx.QueryRow(ctx, `
		SELECT i.id::text, i.legal_name, i.address, i.tax_registration, i.currency, i.number_prefix, i.version
		FROM location_invoice_issuers b
		JOIN invoice_issuers i ON i.id = b.issuer_id
		WHERE b.organization_id=$1 AND b.location_id=$2 LIMIT 1
	`, org, loc).Scan(&issuerID, &legalName, &address, &taxReg, &curr, &prefix, &issuerVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerQuoteDepositResult{}, ErrInvoiceIssuer
	}
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	issuerSnapshot := InvoiceIssuerSnapshot{
		ID:              issuerID,
		LegalName:       legalName,
		Address:         address,
		TaxRegistration: taxReg,
		Currency:        curr,
		NumberPrefix:    prefix,
		Version:         issuerVersion,
	}
	issuerBytes, err := json.Marshal(issuerSnapshot)
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	var custName, custEmail, custPhone string
	_ = tx.QueryRow(ctx, `SELECT name, email, coalesce(phone, '') FROM customers WHERE id=$1`, customerID).Scan(&custName, &custEmail, &custPhone)
	custSnapshot := InvoiceCustomerSnapshot{
		Name:  custName,
		Email: custEmail,
		Phone: custPhone,
	}
	custBytes, err := json.Marshal(custSnapshot)
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	invoiceNumber := fmt.Sprintf("%sDEP-%s", prefix, refNum)
	var invoiceID string
	err = tx.QueryRow(ctx, `
		INSERT INTO invoices(
			organization_id, location_id, customer_id,
			invoice_number, status, currency, subtotal_cents, tax_cents,
			total_cents, paid_cents, due_cents, notes
		) VALUES (
			$1, $2, $3,
			$4, 'draft', 'CAD', $5, 0,
			$5, 0, $5, $6
		) RETURNING id::text
	`, org, loc, customerID,
		invoiceNumber, depositCents, fmt.Sprintf("Deposit for Quotation %s", refNum)).Scan(&invoiceID)
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO invoice_items(invoice_id, organization_id, location_id, description, quantity, unit_price_cents, total_price_cents)
		VALUES($1, $2, $3, $4, 1, $5, $5)
	`, invoiceID, org, loc, fmt.Sprintf("Deposit payment for Quote %s", refNum), depositCents)
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE invoices SET
			status = 'paid',
			issuer_id = $4,
			issuer_snapshot = $5,
			customer_snapshot = $6,
			issued_by = $7,
			issued_at = now(),
			paid_cents = $8,
			due_cents = 0,
			paid_at = now(),
			updated_at = now()
		WHERE organization_id=$1 AND location_id=$2 AND id=$3
	`, org, loc, invoiceID, issuerID, issuerBytes, custBytes, actorID, depositCents)
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO invoice_events(invoice_id, organization_id, location_id, actor_id, from_status, to_status, notes)
		VALUES($1, $2, $3, $4, 'draft', 'paid', 'Quote deposit invoice issued and settled')
	`, invoiceID, org, loc, actorID)
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	// Call Square provider
	sqResp, err := client.CreatePayment(ctx, square.PaymentRequest{
		SourceID:       input.SourceID,
		IdempotencyKey: idempotencyKey,
		AmountCents:    depositCents,
		Currency:       "CAD",
		ReferenceID:    invoiceID,
		Note:           fmt.Sprintf("Deposit for Quote %s", refNum),
	})
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}
	if sqResp.Status != "COMPLETED" {
		return CustomerQuoteDepositResult{}, ErrPaymentProvider
	}
	if sqResp.AmountCents != depositCents || sqResp.Currency != "CAD" || strings.TrimSpace(sqResp.ID) == "" {
		return CustomerQuoteDepositResult{}, ErrPaymentProvider
	}

	var paymentID string
	err = tx.QueryRow(ctx, `
		INSERT INTO payments(
			organization_id, location_id, invoice_id, customer_id, actor_id,
			amount_cents, currency, method, status, provider_reference,
			idempotency_key, notes
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, 'CAD', 'square_card', 'completed', $7,
			$8, $9
		) RETURNING id::text
	`, org, loc, invoiceID, customerID, actorID,
		depositCents, strings.TrimSpace(sqResp.ID),
		idempotencyKey, fmt.Sprintf("Quote %s deposit online payment", refNum)).Scan(&paymentID)
	if err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: actorID}
	if err = recordInvoiceEvent(ctx, tx, scope, invoiceID, nil, "paid", fmt.Sprintf("Deposit payment of %d cents recorded via Square (%s)", depositCents, sqResp.ID)); err != nil {
		return CustomerQuoteDepositResult{}, err
	}
	if err = jobAudit(ctx, tx, scope, "quotation.deposit_paid", quoteID); err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return CustomerQuoteDepositResult{}, err
	}

	return CustomerQuoteDepositResult{
		PaymentID:   paymentID,
		QuotationID: quoteID,
		AmountCents: depositCents,
		Status:      "completed",
		InvoiceID:   invoiceID,
		ReceiptURL:  sqResp.ReceiptURL,
	}, nil
}
