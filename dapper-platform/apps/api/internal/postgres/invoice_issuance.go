package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var ErrInvoiceIssuer = errors.New("explicit invoice issuer configuration required")
var ErrInvoiceIssuerConflict = errors.New("invoice issuer configuration changed or numbering conflicts")

type InvoiceIssuerSnapshot struct {
	ID              string `json:"id"`
	LegalName       string `json:"legalName"`
	Address         string `json:"address"`
	TaxRegistration string `json:"taxRegistration"`
	Currency        string `json:"currency"`
	NumberPrefix    string `json:"numberPrefix"`
	Version         int    `json:"version"`
}
type InvoiceIssuer struct {
	InvoiceIssuerSnapshot
	NextNumber int64 `json:"nextNumber"`
}
type InvoiceCustomerSnapshot struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}
type InvoiceIssuerInput struct {
	ID              string `json:"id"`
	LegalName       string `json:"legalName"`
	Address         string `json:"address"`
	TaxRegistration string `json:"taxRegistration"`
	Currency        string `json:"currency"`
	NumberPrefix    string `json:"numberPrefix"`
	StartNumber     int64  `json:"startNumber"`
	Version         int    `json:"version"`
}
type InvoiceIssuerBinding struct {
	LocationID string `json:"locationId"`
	IssuerID   string `json:"issuerId"`
}
type InvoiceConfiguration struct {
	Issuers   []InvoiceIssuer        `json:"issuers"`
	Locations []Location             `json:"locations"`
	Bindings  []InvoiceIssuerBinding `json:"bindings"`
}

// financeConfigurationGrant: invoice business details need invoices.configure, held directly or through an organization-wide role such
// as Super Admin or Owner.
func financeConfigurationGrant(ctx context.Context, tx pgx.Tx, actor, org string) error {
	ok, err := actorHasOrgCapability(ctx, tx, actor, org, "invoices.configure")
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

const invoiceIssuerColumns = `id::text,legal_name,address,tax_registration,currency,number_prefix,version,next_number`

func scanInvoiceIssuer(row pgx.Row) (InvoiceIssuer, error) {
	var i InvoiceIssuer
	err := row.Scan(&i.ID, &i.LegalName, &i.Address, &i.TaxRegistration, &i.Currency, &i.NumberPrefix, &i.Version, &i.NextNumber)
	return i, err
}
func issuerWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrInvoiceIssuerConflict
	}
	return err
}

func (s *Store) SaveInvoiceIssuer(ctx context.Context, actor, org string, input InvoiceIssuerInput) (InvoiceIssuer, error) {
	input.LegalName = strings.TrimSpace(input.LegalName)
	input.Address = strings.TrimSpace(input.Address)
	input.TaxRegistration = strings.TrimSpace(input.TaxRegistration)
	if n := utf8.RuneCountInString(input.LegalName); n < 2 || n > 200 || utf8.RuneCountInString(input.Address) < 5 || utf8.RuneCountInString(input.Address) > 1000 || utf8.RuneCountInString(input.TaxRegistration) > 100 || input.Currency != "CAD" || !regexp.MustCompile(`^[A-Z0-9-]{1,24}$`).MatchString(input.NumberPrefix) {
		return InvoiceIssuer{}, ErrInvoiceIssuer
	}
	if input.ID == "" && (input.Version != 0 || input.StartNumber < 1 || input.StartNumber >= math.MaxInt64-1) || input.ID != "" && (input.Version < 1 || input.StartNumber != 0) {
		return InvoiceIssuer{}, ErrInvoiceIssuer
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InvoiceIssuer{}, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return InvoiceIssuer{}, err
	}
	var issuer InvoiceIssuer
	if input.ID == "" {
		issuer, err = scanInvoiceIssuer(tx.QueryRow(ctx, `INSERT INTO invoice_issuers(organization_id,legal_name,address,tax_registration,currency,number_prefix,next_number,updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+invoiceIssuerColumns, org, input.LegalName, input.Address, input.TaxRegistration, input.Currency, input.NumberPrefix, input.StartNumber, actor))
	} else {
		issuer, err = scanInvoiceIssuer(tx.QueryRow(ctx, `UPDATE invoice_issuers SET legal_name=$3,address=$4,tax_registration=$5,currency=$6,number_prefix=$7,updated_by=$8,version=version+1 WHERE organization_id=$1 AND id=$2 AND version=$9 RETURNING `+invoiceIssuerColumns, org, input.ID, input.LegalName, input.Address, input.TaxRegistration, input.Currency, input.NumberPrefix, actor, input.Version))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return InvoiceIssuer{}, ErrInvoiceIssuerConflict
	}
	if err != nil {
		return InvoiceIssuer{}, issuerWriteError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_issuer_events(organization_id,issuer_id,actor_id,action,snapshot) VALUES($1,$2,$3,'issuer.saved',$4)`, org, issuer.ID, actor, issuer); err != nil {
		return InvoiceIssuer{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InvoiceIssuer{}, err
	}
	return issuer, nil
}

func (s *Store) BindInvoiceIssuer(ctx context.Context, actor, org, location, issuer string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	var id string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM invoice_issuers WHERE organization_id=$1 AND id=$2 FOR SHARE`, org, issuer).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
		return ErrInvoiceIssuer
	} else if err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT id::text FROM locations WHERE organization_id=$1 AND id=$2 FOR SHARE`, org, location).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
		return ErrInvoiceIssuer
	} else if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO location_invoice_issuers(organization_id,location_id,issuer_id) VALUES($1,$2,$3) ON CONFLICT(organization_id,location_id) DO UPDATE SET issuer_id=excluded.issuer_id WHERE location_invoice_issuers.issuer_id<>excluded.issuer_id`, org, location, issuer)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO invoice_issuer_events(organization_id,issuer_id,location_id,actor_id,action,snapshot) VALUES($1,$2,$3,$4,'issuer.bound',$5)`, org, issuer, location, actor, InvoiceIssuerBinding{LocationID: location, IssuerID: issuer}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) InvoiceConfiguration(ctx context.Context, actor, org string) (InvoiceConfiguration, error) {
	result := InvoiceConfiguration{Issuers: []InvoiceIssuer{}, Locations: []Location{}, Bindings: []InvoiceIssuerBinding{}}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT `+invoiceIssuerColumns+` FROM invoice_issuers WHERE organization_id=$1 ORDER BY legal_name,id LIMIT 100`, org)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		i, e := scanInvoiceIssuer(rows)
		if e != nil {
			rows.Close()
			return result, e
		}
		result.Issuers = append(result.Issuers, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text,name,franchise_id::text FROM locations WHERE organization_id=$1 ORDER BY name,id LIMIT 100`, org)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var l Location
		if err = rows.Scan(&l.ID, &l.Name, &l.FranchiseID); err != nil {
			rows.Close()
			return result, err
		}
		result.Locations = append(result.Locations, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = tx.Query(ctx, `SELECT location_id::text,issuer_id::text FROM location_invoice_issuers WHERE organization_id=$1 ORDER BY location_id LIMIT 100`, org)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var b InvoiceIssuerBinding
		if err = rows.Scan(&b.LocationID, &b.IssuerID); err != nil {
			rows.Close()
			return result, err
		}
		result.Bindings = append(result.Bindings, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

type InvoiceIssuanceReview struct {
	IssuerID       string `json:"issuerId"`
	IssuerVersion  int    `json:"issuerVersion"`
	InvoiceVersion int    `json:"invoiceVersion"`
}

func (s *Store) InvoiceIssuancePreview(ctx context.Context, scope Scope, id string) (InvoiceIssuerSnapshot, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InvoiceIssuerSnapshot{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.issue"); err != nil {
		return InvoiceIssuerSnapshot{}, err
	}
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM invoices WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return InvoiceIssuerSnapshot{}, ErrInvoiceNotFound
	} else if err != nil {
		return InvoiceIssuerSnapshot{}, err
	}
	if status != "draft" {
		return InvoiceIssuerSnapshot{}, ErrInvoiceStatusConflict
	}
	issuer, err := scanInvoiceIssuer(tx.QueryRow(ctx, `SELECT i.id::text,i.legal_name,i.address,i.tax_registration,i.currency,i.number_prefix,i.version,i.next_number FROM invoice_issuers i JOIN location_invoice_issuers b ON b.organization_id=i.organization_id AND b.issuer_id=i.id WHERE b.organization_id=$1 AND b.location_id=$2 FOR SHARE OF i,b`, scope.OrganizationID, scope.LocationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return InvoiceIssuerSnapshot{}, ErrInvoiceIssuer
	}
	if err != nil {
		return InvoiceIssuerSnapshot{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InvoiceIssuerSnapshot{}, err
	}
	return issuer.InvoiceIssuerSnapshot, nil
}

// The public HTTP boundary requires the issuer revision that was presented for
// review. Trusted internal callers may issue against their current configuration.
func (s *Store) IssueInvoice(ctx context.Context, scope Scope, id string, reviews ...InvoiceIssuanceReview) (Invoice, error) {
	if len(reviews) > 1 {
		return Invoice{}, ErrInvoiceIssuerConflict
	}
	checkReview := func(snapshot *InvoiceIssuerSnapshot) bool {
		return len(reviews) == 0 || snapshot != nil && snapshot.ID == reviews[0].IssuerID && snapshot.Version == reviews[0].IssuerVersion
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.issue"); err != nil {
		return Invoice{}, err
	}
	// Lock the invoice first only for historical replay. New issuance consistently
	// locks issuer -> binding -> invoice to avoid cycles with configuration writes.
	existing, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3`, scope.OrganizationID, scope.LocationID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}
	if existing.Status != "draft" {
		if !checkReview(existing.IssuerSnapshot) {
			return Invoice{}, ErrInvoiceIssuerConflict
		}
		if existing.IssuerSnapshot != nil && existing.Status != "void" {
			if err = tx.Commit(ctx); err != nil {
				return Invoice{}, err
			}
			return existing, nil
		}
		return Invoice{}, ErrInvoiceStatusConflict
	}
	var issuerID string
	err = tx.QueryRow(ctx, `SELECT issuer_id::text FROM location_invoice_issuers WHERE organization_id=$1 AND location_id=$2`, scope.OrganizationID, scope.LocationID).Scan(&issuerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceIssuer
	}
	if err != nil {
		return Invoice{}, err
	}
	issuer, err := scanInvoiceIssuer(tx.QueryRow(ctx, `SELECT `+invoiceIssuerColumns+` FROM invoice_issuers WHERE organization_id=$1 AND id=$2 FOR UPDATE`, scope.OrganizationID, issuerID))
	if err != nil {
		return Invoice{}, err
	}
	if !checkReview(&issuer.InvoiceIssuerSnapshot) {
		return Invoice{}, ErrInvoiceIssuerConflict
	}
	var boundID string
	err = tx.QueryRow(ctx, `SELECT issuer_id::text FROM location_invoice_issuers WHERE organization_id=$1 AND location_id=$2 FOR SHARE`, scope.OrganizationID, scope.LocationID).Scan(&boundID)
	if err != nil || boundID != issuerID {
		return Invoice{}, ErrInvoiceIssuerConflict
	}
	inv, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3 FOR UPDATE OF i`, scope.OrganizationID, scope.LocationID, id))
	if err != nil {
		return Invoice{}, err
	}
	if inv.Status != "draft" {
		if inv.IssuerSnapshot != nil && inv.Status != "void" {
			if err = tx.Commit(ctx); err != nil {
				return Invoice{}, err
			}
			return inv, nil
		}
		return Invoice{}, ErrInvoiceStatusConflict
	}
	if issuer.Currency != inv.Currency || issuer.NextNumber >= math.MaxInt64-1 {
		return Invoice{}, ErrInvoiceIssuer
	}
	if len(reviews) > 0 && reviews[0].InvoiceVersion != inv.Version {
		return Invoice{}, ErrInvoiceVersion
	}
	var lineSubtotal int64
	var lineCount int
	if err = tx.QueryRow(ctx, `SELECT coalesce(sum(total_price_cents),0),count(*) FROM invoice_items WHERE organization_id=$1 AND location_id=$2 AND invoice_id=$3`, scope.OrganizationID, scope.LocationID, id).Scan(&lineSubtotal, &lineCount); err != nil {
		return Invoice{}, ErrInvoiceInput
	}
	if lineCount == 0 || lineSubtotal != inv.SubtotalCents || inv.SubtotalCents > math.MaxInt64-inv.TaxCents || inv.TotalCents != inv.SubtotalCents+inv.TaxCents || inv.PaidCents != 0 || inv.DueCents != inv.TotalCents {
		return Invoice{}, ErrInvoiceInput
	}
	var customer InvoiceCustomerSnapshot
	if err = tx.QueryRow(ctx, `SELECT name,email,phone FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, scope.OrganizationID, scope.LocationID, inv.CustomerID).Scan(&customer.Name, &customer.Email, &customer.Phone); err != nil {
		return Invoice{}, err
	}
	number := issuer.NumberPrefix + strconv.FormatInt(issuer.NextNumber, 10)
	if _, err = tx.Exec(ctx, `UPDATE invoices SET invoice_number=$4,status='issued',issuer_id=$5,issuer_snapshot=$6,customer_snapshot=$7,issued_by=$8,issued_at=now(),updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id, number, issuer.ID, issuer.InvoiceIssuerSnapshot, customer, scope.ActorID); err != nil {
		return Invoice{}, issuerWriteError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE invoice_issuers SET next_number=next_number+1 WHERE id=$1`, issuer.ID); err != nil {
		return Invoice{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_events(invoice_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,'draft','issued','Invoice explicitly issued with configured issuer and sequence')`, id, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return Invoice{}, err
	}
	issued, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.id=$1`, id))
	if err != nil {
		return Invoice{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	return issued, nil
}
