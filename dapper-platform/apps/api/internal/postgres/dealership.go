package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrDealershipNotFound       = errors.New("dealership account not found")
	ErrDealershipInput          = errors.New("invalid dealership input")
	ErrDealershipCodeConflict   = errors.New("dealership account code already in use")
	ErrDealershipIntakeNotFound = errors.New("dealership fleet intake record not found")
	ErrDealershipStockConflict  = errors.New("stock number already exists for this dealership")
	ErrDealershipCustomer       = errors.New("dealership linked customer not found")

	emailRegex = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)
)

type DealershipAccount struct {
	ID               string          `json:"id"`
	OrganizationID   string          `json:"organizationId"`
	LocationID       string          `json:"locationId"`
	CustomerID       string          `json:"customerId"`
	AccountName      string          `json:"accountName"`
	AccountCode      string          `json:"accountCode"`
	AccountType      string          `json:"accountType"` // 'dealership', 'fleet', 'commercial'
	ContactName      string          `json:"contactName"`
	ContactEmail     string          `json:"contactEmail"`
	ContactPhone     string          `json:"contactPhone"`
	Address          string          `json:"address"`
	BillingTermsDays int             `json:"billingTermsDays"`
	TaxExempt        bool            `json:"taxExempt"`
	Active           bool            `json:"active"`
	Notes            string          `json:"notes,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	Tags             []DealershipTag `json:"tags,omitempty"`
	// PerCarRateCents is the agreed price per car in CAD cents before tax (nil = not set).
	PerCarRateCents *int64              `json:"perCarRateCents"`
	Contacts        []DealershipContact `json:"contacts"`
}

// DealershipContact is one contact person at a dealership; the first is the primary contact.
type DealershipContact struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type DealershipTag struct {
	ID           string    `json:"id"`
	DealershipID string    `json:"dealershipId"`
	TagName      string    `json:"tagName"`
	TagColor     string    `json:"tagColor"`
	CreatedAt    time.Time `json:"createdAt"`
}

type DealershipFleetIntake struct {
	ID                  string     `json:"id"`
	OrganizationID      string     `json:"organizationId"`
	LocationID          string     `json:"locationId"`
	DealershipID        string     `json:"dealershipId"`
	VehicleID           string     `json:"vehicleId"`
	StockNumber         string     `json:"stockNumber"`
	VIN                 string     `json:"vin,omitempty"`
	PONumber            string     `json:"poNumber,omitempty"`
	LotLocation         string     `json:"lotLocation,omitempty"`
	ServiceID           *string    `json:"serviceId,omitempty"`
	ServiceName         string     `json:"serviceName"`
	Status              string     `json:"status"` // 'intake', 'queued', 'in_progress', 'quality_checked', 'completed', 'released'
	JobID               *string    `json:"jobId,omitempty"`
	AssignedPrincipalID *string    `json:"assignedPrincipalId,omitempty"`
	IntakeNotes         string     `json:"intakeNotes,omitempty"`
	ReceivedAt          time.Time  `json:"receivedAt"`
	CompletedAt         *time.Time `json:"completedAt,omitempty"`
	ReleasedAt          *time.Time `json:"releasedAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type DealershipReport struct {
	DealershipID       string `json:"dealershipId"`
	AccountName        string `json:"accountName"`
	AccountCode        string `json:"accountCode"`
	TotalReceived      int    `json:"totalReceived"`
	TotalCompleted     int    `json:"totalCompleted"`
	TotalReleased      int    `json:"totalReleased"`
	ActiveCount        int    `json:"activeCount"`
	InvoicedTotalCents int64  `json:"invoicedTotalCents"`
	BalanceDueCents    int64  `json:"balanceDueCents"`
}

type CreateDealershipInput struct {
	CustomerID       string              `json:"customerId"`
	AccountName      string              `json:"accountName"`
	AccountCode      string              `json:"accountCode"`
	AccountType      string              `json:"accountType"` // 'dealership', 'fleet', 'commercial'
	ContactName      string              `json:"contactName"`
	ContactEmail     string              `json:"contactEmail"`
	ContactPhone     string              `json:"contactPhone,omitempty"`
	Address          string              `json:"address"`
	BillingTermsDays int                 `json:"billingTermsDays"`
	TaxExempt        bool                `json:"taxExempt"`
	Notes            string              `json:"notes,omitempty"`
	PerCarRateCents  *int64              `json:"perCarRateCents"`
	Contacts         []DealershipContact `json:"contacts"`
}

type UpdateDealershipInput struct {
	AccountName      string              `json:"accountName"`
	ContactName      string              `json:"contactName"`
	ContactEmail     string              `json:"contactEmail"`
	ContactPhone     string              `json:"contactPhone,omitempty"`
	Address          string              `json:"address"`
	BillingTermsDays int                 `json:"billingTermsDays"`
	TaxExempt        bool                `json:"taxExempt"`
	Active           bool                `json:"active"`
	Notes            string              `json:"notes,omitempty"`
	PerCarRateCents  *int64              `json:"perCarRateCents"`
	Contacts         []DealershipContact `json:"contacts"`
}

type CreateFleetIntakeInput struct {
	VehicleDescription string  `json:"vehicleDescription"`
	Category           string  `json:"categoryDescription,omitempty"`
	StockNumber        string  `json:"stockNumber"`
	VIN                string  `json:"vin,omitempty"`
	PONumber           string  `json:"poNumber,omitempty"`
	LotLocation        string  `json:"lotLocation,omitempty"`
	ServiceID          *string `json:"serviceId,omitempty"`
	ServiceName        string  `json:"serviceName"`
	IntakeNotes        string  `json:"intakeNotes,omitempty"`
}

const dealershipSelectColumns = `d.id::text, d.organization_id::text, d.location_id::text, d.customer_id::text, d.account_name, d.account_code, d.account_type, d.contact_name, d.contact_email, d.contact_phone, d.billing_terms_days, d.tax_exempt, d.active, d.notes, d.created_at, d.updated_at, d.per_car_rate_cents, d.address`

func scanDealership(row pgx.Row) (DealershipAccount, error) {
	var a DealershipAccount
	err := row.Scan(
		&a.ID, &a.OrganizationID, &a.LocationID, &a.CustomerID,
		&a.AccountName, &a.AccountCode, &a.AccountType,
		&a.ContactName, &a.ContactEmail, &a.ContactPhone,
		&a.BillingTermsDays, &a.TaxExempt, &a.Active, &a.Notes,
		&a.CreatedAt, &a.UpdatedAt, &a.PerCarRateCents, &a.Address,
	)
	a.Contacts = []DealershipContact{}
	if errors.Is(err, pgx.ErrNoRows) {
		return DealershipAccount{}, ErrDealershipNotFound
	}
	return a, err
}

func (s *Store) CreateDealershipAccount(ctx context.Context, scope Scope, input CreateDealershipInput) (DealershipAccount, error) {
	if !uuid(input.CustomerID) {
		return DealershipAccount{}, ErrDealershipInput
	}
	input.AccountName = strings.TrimSpace(input.AccountName)
	input.AccountCode = strings.ToUpper(strings.TrimSpace(input.AccountCode))
	input.AccountType = strings.TrimSpace(input.AccountType)
	input.ContactName = strings.TrimSpace(input.ContactName)
	input.ContactEmail = strings.TrimSpace(input.ContactEmail)
	input.ContactPhone = strings.TrimSpace(input.ContactPhone)
	input.Notes = strings.TrimSpace(input.Notes)

	contacts, cerr := cleanDealershipContacts(input.Contacts, input.ContactName, input.ContactEmail, input.ContactPhone)
	if cerr != nil || len(input.AccountName) < 2 || len(input.AccountName) > 160 ||
		len(input.AccountCode) < 2 || len(input.AccountCode) > 32 ||
		(input.AccountType != "dealership" && input.AccountType != "fleet" && input.AccountType != "commercial") ||
		input.BillingTermsDays < 0 || input.BillingTermsDays > 180 ||
		input.PerCarRateCents != nil && (*input.PerCarRateCents < 0 || *input.PerCarRateCents > 100000000000) ||
		utf8.RuneCountInString(input.Notes) > 2000 || utf8.RuneCountInString(input.Address) > 300 {
		return DealershipAccount{}, ErrDealershipInput
	}
	// Payment terms and tax exemption are not set per dealership; new accounts take the standard defaults.
	input.ContactName, input.ContactEmail, input.ContactPhone = contacts[0].Name, contacts[0].Email, contacts[0].Phone
	input.BillingTermsDays, input.TaxExempt = 30, false

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return DealershipAccount{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return DealershipAccount{}, err
	}

	// Verify customer exists in this scope
	var custExists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3)`,
		scope.OrganizationID, scope.LocationID, input.CustomerID).Scan(&custExists)
	if err != nil || !custExists {
		return DealershipAccount{}, ErrDealershipCustomer
	}

	var accountID string
	err = tx.QueryRow(ctx, `
		INSERT INTO dealership_accounts(
			organization_id, location_id, customer_id, account_name,
			account_code, account_type, contact_name, contact_email,
			contact_phone, billing_terms_days, tax_exempt, notes, per_car_rate_cents, address
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14
		) RETURNING id::text
	`, scope.OrganizationID, scope.LocationID, input.CustomerID, input.AccountName,
		input.AccountCode, input.AccountType, input.ContactName, input.ContactEmail,
		input.ContactPhone, input.BillingTermsDays, input.TaxExempt, input.Notes, input.PerCarRateCents, strings.TrimSpace(input.Address)).Scan(&accountID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "account_code") {
			return DealershipAccount{}, ErrDealershipCodeConflict
		}
		return DealershipAccount{}, err
	}

	if err = replaceDealershipContacts(ctx, tx, scope, accountID, contacts); err != nil {
		return DealershipAccount{}, err
	}
	if err = jobAudit(ctx, tx, scope, "dealership.created", accountID); err != nil {
		return DealershipAccount{}, err
	}

	acc, err := scanDealership(tx.QueryRow(ctx, `SELECT `+dealershipSelectColumns+` FROM dealership_accounts d WHERE id=$1`, accountID))
	if err != nil {
		return DealershipAccount{}, err
	}
	one := []DealershipAccount{acc}
	if err = attachDealershipContacts(ctx, tx, scope, one); err != nil {
		return DealershipAccount{}, err
	}
	acc = one[0]

	return acc, tx.Commit(ctx)
}

func (s *Store) GetDealershipAccount(ctx context.Context, scope Scope, id string) (DealershipAccount, error) {
	if !uuid(id) {
		return DealershipAccount{}, ErrDealershipNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DealershipAccount{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.read"); err != nil {
		return DealershipAccount{}, err
	}

	acc, err := scanDealership(tx.QueryRow(ctx, `SELECT `+dealershipSelectColumns+` FROM dealership_accounts d WHERE d.organization_id=$1 AND d.location_id=$2 AND d.id=$3`,
		scope.OrganizationID, scope.LocationID, id))
	if err != nil {
		return DealershipAccount{}, err
	}

	{
		one := []DealershipAccount{acc}
		if err = attachDealershipContacts(ctx, tx, scope, one); err != nil {
			return DealershipAccount{}, err
		}
		acc = one[0]
	}
	// Load tags
	rows, err := tx.Query(ctx, `SELECT id::text, dealership_id::text, tag_name, tag_color, created_at FROM dealership_tags WHERE dealership_id=$1 ORDER BY tag_name`, id)
	if err != nil {
		return DealershipAccount{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var t DealershipTag
		if err := rows.Scan(&t.ID, &t.DealershipID, &t.TagName, &t.TagColor, &t.CreatedAt); err != nil {
			return DealershipAccount{}, err
		}
		acc.Tags = append(acc.Tags, t)
	}

	return acc, tx.Commit(ctx)
}

func (s *Store) ListDealershipAccounts(ctx context.Context, scope Scope) ([]DealershipAccount, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.read"); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `SELECT `+dealershipSelectColumns+` FROM dealership_accounts d WHERE d.organization_id=$1 AND d.location_id=$2 ORDER BY d.account_name ASC`,
		scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []DealershipAccount
	for rows.Next() {
		acc, err := scanDealership(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	rows.Close()
	if err = attachDealershipContacts(ctx, tx, scope, accounts); err != nil {
		return nil, err
	}

	return accounts, tx.Commit(ctx)
}

func (s *Store) UpdateDealershipAccount(ctx context.Context, scope Scope, id string, input UpdateDealershipInput) (DealershipAccount, error) {
	if !uuid(id) {
		return DealershipAccount{}, ErrDealershipNotFound
	}
	input.AccountName = strings.TrimSpace(input.AccountName)
	input.ContactName = strings.TrimSpace(input.ContactName)
	input.ContactEmail = strings.TrimSpace(input.ContactEmail)
	input.ContactPhone = strings.TrimSpace(input.ContactPhone)
	input.Notes = strings.TrimSpace(input.Notes)

	contacts, cerr := cleanDealershipContacts(input.Contacts, input.ContactName, input.ContactEmail, input.ContactPhone)
	if cerr != nil || len(input.AccountName) < 2 || len(input.AccountName) > 160 ||
		input.PerCarRateCents != nil && (*input.PerCarRateCents < 0 || *input.PerCarRateCents > 100000000000) ||
		utf8.RuneCountInString(input.Notes) > 2000 || utf8.RuneCountInString(input.Address) > 300 {
		return DealershipAccount{}, ErrDealershipInput
	}
	input.ContactName, input.ContactEmail, input.ContactPhone = contacts[0].Name, contacts[0].Email, contacts[0].Phone

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return DealershipAccount{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return DealershipAccount{}, err
	}

	res, err := tx.Exec(ctx, `
		UPDATE dealership_accounts SET
			account_name = $4,
			contact_name = $5,
			contact_email = $6,
			contact_phone = $7,
			active = $8,
			notes = $9,
			per_car_rate_cents = $10,
			address = $11,
			updated_at = now()
		WHERE organization_id=$1 AND location_id=$2 AND id=$3
	`, scope.OrganizationID, scope.LocationID, id, input.AccountName,
		input.ContactName, input.ContactEmail, input.ContactPhone,
		input.Active, input.Notes, input.PerCarRateCents, strings.TrimSpace(input.Address))
	if err != nil {
		return DealershipAccount{}, err
	}
	if res.RowsAffected() == 0 {
		return DealershipAccount{}, ErrDealershipNotFound
	}

	if err = replaceDealershipContacts(ctx, tx, scope, id, contacts); err != nil {
		return DealershipAccount{}, err
	}
	if err = jobAudit(ctx, tx, scope, "dealership.updated", id); err != nil {
		return DealershipAccount{}, err
	}

	acc, err := scanDealership(tx.QueryRow(ctx, `SELECT `+dealershipSelectColumns+` FROM dealership_accounts d WHERE id=$1`, id))
	if err != nil {
		return DealershipAccount{}, err
	}
	one := []DealershipAccount{acc}
	if err = attachDealershipContacts(ctx, tx, scope, one); err != nil {
		return DealershipAccount{}, err
	}
	acc = one[0]

	return acc, tx.Commit(ctx)
}

func (s *Store) AddDealershipTag(ctx context.Context, scope Scope, dealershipID, tagName, tagColor string) (DealershipTag, error) {
	if !uuid(dealershipID) {
		return DealershipTag{}, ErrDealershipNotFound
	}
	tagName = strings.TrimSpace(tagName)
	tagColor = strings.ToLower(strings.TrimSpace(tagColor))
	if tagColor == "" {
		tagColor = "#0066cc"
	}
	if len(tagName) < 1 || len(tagName) > 60 {
		return DealershipTag{}, ErrDealershipInput
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return DealershipTag{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return DealershipTag{}, err
	}

	var tag DealershipTag
	err = tx.QueryRow(ctx, `
		INSERT INTO dealership_tags(organization_id, location_id, dealership_id, tag_name, tag_color)
		VALUES($1, $2, $3, $4, $5)
		ON CONFLICT (dealership_id, tag_name) DO UPDATE SET tag_color=EXCLUDED.tag_color
		RETURNING id::text, dealership_id::text, tag_name, tag_color, created_at
	`, scope.OrganizationID, scope.LocationID, dealershipID, tagName, tagColor).Scan(
		&tag.ID, &tag.DealershipID, &tag.TagName, &tag.TagColor, &tag.CreatedAt,
	)
	if err != nil {
		return DealershipTag{}, err
	}

	return tag, tx.Commit(ctx)
}

func (s *Store) RemoveDealershipTag(ctx context.Context, scope Scope, dealershipID, tagID string) error {
	if !uuid(dealershipID) || !uuid(tagID) {
		return ErrDealershipInput
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return err
	}

	res, err := tx.Exec(ctx, `DELETE FROM dealership_tags WHERE organization_id=$1 AND location_id=$2 AND dealership_id=$3 AND id=$4`,
		scope.OrganizationID, scope.LocationID, dealershipID, tagID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrDealershipNotFound
	}
	return tx.Commit(ctx)
}

const fleetIntakeSelectColumns = `i.id::text, i.organization_id::text, i.location_id::text, i.dealership_id::text, i.vehicle_id::text, i.stock_number, i.vin, i.po_number, i.lot_location, i.service_id::text, i.service_name, i.status, i.job_id::text, i.assigned_principal_id::text, i.intake_notes, i.received_at, i.completed_at, i.released_at, i.created_at, i.updated_at`

func scanFleetIntake(row pgx.Row) (DealershipFleetIntake, error) {
	var item DealershipFleetIntake
	err := row.Scan(
		&item.ID, &item.OrganizationID, &item.LocationID, &item.DealershipID,
		&item.VehicleID, &item.StockNumber, &item.VIN, &item.PONumber,
		&item.LotLocation, &item.ServiceID, &item.ServiceName, &item.Status,
		&item.JobID, &item.AssignedPrincipalID, &item.IntakeNotes,
		&item.ReceivedAt, &item.CompletedAt, &item.ReleasedAt,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DealershipFleetIntake{}, ErrDealershipIntakeNotFound
	}
	return item, err
}

func (s *Store) CreateFleetIntake(ctx context.Context, scope Scope, dealershipID string, input CreateFleetIntakeInput) (DealershipFleetIntake, error) {
	if !uuid(dealershipID) {
		return DealershipFleetIntake{}, ErrDealershipNotFound
	}
	input.VehicleDescription = strings.TrimSpace(input.VehicleDescription)
	input.Category = strings.TrimSpace(input.Category)
	input.StockNumber = strings.TrimSpace(input.StockNumber)
	input.VIN = strings.ToUpper(strings.TrimSpace(input.VIN))
	input.PONumber = strings.TrimSpace(input.PONumber)
	input.LotLocation = strings.TrimSpace(input.LotLocation)
	input.ServiceName = strings.TrimSpace(input.ServiceName)
	input.IntakeNotes = strings.TrimSpace(input.IntakeNotes)

	if len(input.VehicleDescription) < 2 || len(input.VehicleDescription) > 120 ||
		len(input.StockNumber) < 1 || len(input.StockNumber) > 60 ||
		len(input.ServiceName) < 2 || len(input.ServiceName) > 120 ||
		utf8.RuneCountInString(input.IntakeNotes) > 2000 {
		return DealershipFleetIntake{}, ErrDealershipInput
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return DealershipFleetIntake{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return DealershipFleetIntake{}, err
	}

	// Verify dealership account exists and get linked customer_id
	var customerID string
	err = tx.QueryRow(ctx, `SELECT customer_id::text FROM dealership_accounts WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND active=true`,
		scope.OrganizationID, scope.LocationID, dealershipID).Scan(&customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DealershipFleetIntake{}, ErrDealershipNotFound
	}
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	// Create vehicle under dealership customer
	var vehicleID string
	err = tx.QueryRow(ctx, `
		INSERT INTO vehicles(organization_id, location_id, customer_id, description, category_description, identifier)
		VALUES($1, $2, $3, $4, $5, $6)
		RETURNING id::text
	`, scope.OrganizationID, scope.LocationID, customerID, input.VehicleDescription, input.Category, input.StockNumber).Scan(&vehicleID)
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	var intakeID string
	err = tx.QueryRow(ctx, `
		INSERT INTO dealership_fleet_intake(
			organization_id, location_id, dealership_id, vehicle_id,
			stock_number, vin, po_number, lot_location,
			service_id, service_name, status, intake_notes
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, 'intake', $11
		) RETURNING id::text
	`, scope.OrganizationID, scope.LocationID, dealershipID, vehicleID,
		input.StockNumber, input.VIN, input.PONumber, input.LotLocation,
		input.ServiceID, input.ServiceName, input.IntakeNotes).Scan(&intakeID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "stock_number") {
			return DealershipFleetIntake{}, ErrDealershipStockConflict
		}
		return DealershipFleetIntake{}, err
	}

	// Record intake event
	_, err = tx.Exec(ctx, `
		INSERT INTO dealership_intake_events(intake_id, organization_id, location_id, actor_id, action, from_status, to_status, notes)
		VALUES($1, $2, $3, $4, 'intake_received', NULL, 'intake', $5)
	`, intakeID, scope.OrganizationID, scope.LocationID, scope.ActorID, fmt.Sprintf("Stock #%s received for %s", input.StockNumber, input.ServiceName))
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	intake, err := scanFleetIntake(tx.QueryRow(ctx, `SELECT `+fleetIntakeSelectColumns+` FROM dealership_fleet_intake i WHERE i.id=$1`, intakeID))
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	return intake, tx.Commit(ctx)
}

func (s *Store) ListFleetIntake(ctx context.Context, scope Scope, dealershipID, statusFilter string) ([]DealershipFleetIntake, error) {
	if !uuid(dealershipID) {
		return nil, ErrDealershipNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.read"); err != nil {
		return nil, err
	}

	query := `SELECT ` + fleetIntakeSelectColumns + ` FROM dealership_fleet_intake i WHERE i.organization_id=$1 AND i.location_id=$2 AND i.dealership_id=$3`
	args := []any{scope.OrganizationID, scope.LocationID, dealershipID}

	if statusFilter != "" {
		query += ` AND i.status=$4 ORDER BY i.received_at DESC`
		args = append(args, statusFilter)
	} else {
		query += ` ORDER BY i.received_at DESC`
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DealershipFleetIntake
	for rows.Next() {
		it, err := scanFleetIntake(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}

	return items, tx.Commit(ctx)
}

func (s *Store) UpdateFleetIntakeStatus(ctx context.Context, scope Scope, intakeID, newStatus, notes string) (DealershipFleetIntake, error) {
	if !uuid(intakeID) {
		return DealershipFleetIntake{}, ErrDealershipIntakeNotFound
	}
	newStatus = strings.TrimSpace(newStatus)
	notes = strings.TrimSpace(notes)
	validStatuses := map[string]bool{
		"intake": true, "queued": true, "in_progress": true,
		"quality_checked": true, "completed": true, "released": true,
	}
	if !validStatuses[newStatus] {
		return DealershipFleetIntake{}, ErrDealershipInput
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return DealershipFleetIntake{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return DealershipFleetIntake{}, err
	}

	var currentStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM dealership_fleet_intake WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, intakeID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return DealershipFleetIntake{}, ErrDealershipIntakeNotFound
	}
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	var completedAtClause, releasedAtClause string
	if newStatus == "completed" && currentStatus != "completed" {
		completedAtClause = ", completed_at = now()"
	}
	if newStatus == "released" && currentStatus != "released" {
		releasedAtClause = ", released_at = now()"
	}

	updateQuery := fmt.Sprintf(`
		UPDATE dealership_fleet_intake SET
			status = $4%s%s,
			updated_at = now()
		WHERE organization_id=$1 AND location_id=$2 AND id=$3
	`, completedAtClause, releasedAtClause)

	_, err = tx.Exec(ctx, updateQuery, scope.OrganizationID, scope.LocationID, intakeID, newStatus)
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	action := "status_changed"
	if newStatus == "released" {
		action = "released"
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO dealership_intake_events(intake_id, organization_id, location_id, actor_id, action, from_status, to_status, notes)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8)
	`, intakeID, scope.OrganizationID, scope.LocationID, scope.ActorID, action, currentStatus, newStatus, notes)
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	updated, err := scanFleetIntake(tx.QueryRow(ctx, `SELECT `+fleetIntakeSelectColumns+` FROM dealership_fleet_intake i WHERE i.id=$1`, intakeID))
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	return updated, tx.Commit(ctx)
}

func (s *Store) AssignFleetIntake(ctx context.Context, scope Scope, intakeID string, assignedPrincipalID *string) (DealershipFleetIntake, error) {
	if !uuid(intakeID) {
		return DealershipFleetIntake{}, ErrDealershipIntakeNotFound
	}
	if assignedPrincipalID != nil && !uuid(*assignedPrincipalID) {
		return DealershipFleetIntake{}, ErrDealershipInput
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return DealershipFleetIntake{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return DealershipFleetIntake{}, err
	}

	// If assigning, verify principal exists and is active
	if assignedPrincipalID != nil {
		var active bool
		err = tx.QueryRow(ctx, `SELECT active FROM principals WHERE id=$1`, *assignedPrincipalID).Scan(&active)
		if err != nil || !active {
			return DealershipFleetIntake{}, ErrDealershipInput
		}
	}

	res, err := tx.Exec(ctx, `
		UPDATE dealership_fleet_intake SET
			assigned_principal_id = $4,
			updated_at = now()
		WHERE organization_id=$1 AND location_id=$2 AND id=$3
	`, scope.OrganizationID, scope.LocationID, intakeID, assignedPrincipalID)
	if err != nil {
		return DealershipFleetIntake{}, err
	}
	if res.RowsAffected() == 0 {
		return DealershipFleetIntake{}, ErrDealershipIntakeNotFound
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO dealership_intake_events(intake_id, organization_id, location_id, actor_id, action, notes)
		VALUES($1, $2, $3, $4, 'assigned', 'Staff assignment updated')
	`, intakeID, scope.OrganizationID, scope.LocationID, scope.ActorID)
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	updated, err := scanFleetIntake(tx.QueryRow(ctx, `SELECT `+fleetIntakeSelectColumns+` FROM dealership_fleet_intake i WHERE i.id=$1`, intakeID))
	if err != nil {
		return DealershipFleetIntake{}, err
	}

	return updated, tx.Commit(ctx)
}

func (s *Store) GetDealershipReport(ctx context.Context, scope Scope, dealershipID string) (DealershipReport, error) {
	if !uuid(dealershipID) {
		return DealershipReport{}, ErrDealershipNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DealershipReport{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "dealership.read"); err != nil {
		return DealershipReport{}, err
	}

	var rep DealershipReport
	var customerID string
	err = tx.QueryRow(ctx, `
		SELECT id::text, customer_id::text, account_name, account_code
		FROM dealership_accounts
		WHERE organization_id=$1 AND location_id=$2 AND id=$3
	`, scope.OrganizationID, scope.LocationID, dealershipID).Scan(
		&rep.DealershipID, &customerID, &rep.AccountName, &rep.AccountCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DealershipReport{}, ErrDealershipNotFound
	}
	if err != nil {
		return DealershipReport{}, err
	}

	// Count fleet vehicles
	err = tx.QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE status='completed'),
			count(*) FILTER (WHERE status='released'),
			count(*) FILTER (WHERE status NOT IN ('completed', 'released'))
		FROM dealership_fleet_intake
		WHERE organization_id=$1 AND location_id=$2 AND dealership_id=$3
	`, scope.OrganizationID, scope.LocationID, dealershipID).Scan(
		&rep.TotalReceived, &rep.TotalCompleted, &rep.TotalReleased, &rep.ActiveCount,
	)
	if err != nil {
		return DealershipReport{}, err
	}

	// Financials from invoices table for this customer
	err = tx.QueryRow(ctx, `
		SELECT coalesce(sum(total_cents), 0), coalesce(sum(due_cents), 0)
		FROM invoices
		WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 AND status NOT IN ('draft', 'void')
	`, scope.OrganizationID, scope.LocationID, customerID).Scan(
		&rep.InvoicedTotalCents, &rep.BalanceDueCents,
	)
	if err != nil {
		return DealershipReport{}, err
	}

	return rep, tx.Commit(ctx)
}

// cleanDealershipContacts validates the contact people. When none are given the primary contact fields are used, so older callers
// keep working. The first contact is returned as the primary one.
func cleanDealershipContacts(list []DealershipContact, name, email, phone string) ([]DealershipContact, error) {
	if len(list) == 0 && strings.TrimSpace(name) != "" {
		list = []DealershipContact{{Name: name, Email: email, Phone: phone}}
	}
	if len(list) == 0 || len(list) > 10 {
		return nil, ErrDealershipInput
	}
	out := make([]DealershipContact, 0, len(list))
	for _, c := range list {
		c.Name, c.Email, c.Phone = strings.TrimSpace(c.Name), strings.TrimSpace(c.Email), strings.TrimSpace(c.Phone)
		if n := utf8.RuneCountInString(c.Name); n < 2 || n > 120 || len(c.Phone) > 40 || c.Email != "" && !emailRegex.MatchString(c.Email) {
			return nil, ErrDealershipInput
		}
		out = append(out, c)
	}
	// The account's primary contact needs an email (invoices and notices go there).
	if !emailRegex.MatchString(out[0].Email) {
		return nil, ErrDealershipInput
	}
	return out, nil
}

func replaceDealershipContacts(ctx context.Context, tx pgx.Tx, scope Scope, id string, list []DealershipContact) error {
	if _, err := tx.Exec(ctx, `DELETE FROM dealership_contacts WHERE organization_id=$1 AND location_id=$2 AND dealership_id=$3`, scope.OrganizationID, scope.LocationID, id); err != nil {
		return err
	}
	for i, c := range list {
		if _, err := tx.Exec(ctx, `INSERT INTO dealership_contacts(organization_id,location_id,dealership_id,position,name,email,phone) VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.OrganizationID, scope.LocationID, id, i, c.Name, c.Email, c.Phone); err != nil {
			return err
		}
	}
	return nil
}

// attachDealershipContacts fills Contacts for accounts of one location with a single query.
func attachDealershipContacts(ctx context.Context, tx pgx.Tx, scope Scope, accounts []DealershipAccount) error {
	if len(accounts) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT id::text,dealership_id::text,name,email,phone FROM dealership_contacts WHERE organization_id=$1 AND location_id=$2 ORDER BY dealership_id,position,id`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return err
	}
	defer rows.Close()
	by := map[string][]DealershipContact{}
	for rows.Next() {
		var d string
		var c DealershipContact
		if err = rows.Scan(&c.ID, &d, &c.Name, &c.Email, &c.Phone); err != nil {
			return err
		}
		by[d] = append(by[d], c)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for i := range accounts {
		accounts[i].Contacts = by[accounts[i].ID]
		if accounts[i].Contacts == nil {
			accounts[i].Contacts = []DealershipContact{}
		}
	}
	return nil
}
