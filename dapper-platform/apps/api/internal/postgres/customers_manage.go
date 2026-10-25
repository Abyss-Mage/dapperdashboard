package postgres

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrCustomerInput = errors.New("valid customer name, email and phone required")
var ErrCustomerNotFound = errors.New("customer not found at this franchise")
var ErrCustomerInUse = errors.New("customer has bookings, work orders, invoices or other history and cannot be deleted")

// ErrCustomerDuplicate: another customer at this franchise already has this email or phone number.
var ErrCustomerDuplicate = errors.New("another customer already uses this email or phone number")

// customerUniqueness turns the database's one-email/one-phone rule into ErrCustomerDuplicate.
func customerUniqueness(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && strings.HasPrefix(pg.ConstraintName, "customers_unique_") {
		return ErrCustomerDuplicate
	}
	return err
}

// existingCustomer finds the customer at this franchise with the same email (any case) or the same phone digits, or "".
func existingCustomer(ctx context.Context, q rowQuerier, org, loc, email, phone string) (string, error) {
	var id string
	err := q.QueryRow(ctx, `SELECT id::text FROM customers WHERE organization_id=$1 AND location_id=$2 AND removed_at IS NULL
 AND ((btrim($3)<>'' AND lower(btrim(email))=lower(btrim($3))) OR (regexp_replace($4,'\D','','g')<>'' AND regexp_replace(phone,'\D','','g')=regexp_replace($4,'\D','','g')))
 ORDER BY created_at LIMIT 1`, org, loc, email, phone).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// DirectoryCustomer is a customer with the counts that decide whether deleting is possible.
type DirectoryCustomer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	CreatedAt string `json:"createdAt"`
	Vehicles  int    `json:"vehicles"`
	Bookings  int    `json:"bookings"`
	Jobs      int    `json:"jobs"`
	Invoices  int    `json:"invoices"`
}

// CustomerDirectory lists up to 1000 customers of one franchise with their record counts.
func (s *Store) CustomerDirectory(ctx context.Context, scope Scope) ([]DirectoryCustomer, error) {
	if err := s.authorize(ctx, scope, "customers.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id::text,c.name,c.email,c.phone,to_char(c.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
 (SELECT count(*) FROM vehicles v WHERE v.organization_id=c.organization_id AND v.location_id=c.location_id AND v.customer_id=c.id),
 (SELECT count(*) FROM bookings b WHERE b.organization_id=c.organization_id AND b.location_id=c.location_id AND b.customer_id=c.id),
 (SELECT count(*) FROM jobs j WHERE j.organization_id=c.organization_id AND j.location_id=c.location_id AND j.customer_id=c.id),
 (SELECT count(*) FROM invoices i WHERE i.organization_id=c.organization_id AND i.location_id=c.location_id AND i.customer_id=c.id)
FROM customers c WHERE c.organization_id=$1 AND c.location_id=$2 AND c.removed_at IS NULL ORDER BY lower(c.name),c.id LIMIT 1000`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DirectoryCustomer{}
	for rows.Next() {
		var c DirectoryCustomer
		if err = rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.CreatedAt, &c.Vehicles, &c.Bookings, &c.Jobs, &c.Invoices); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateCustomer changes a customer's name, email and phone (the same limits as creating one).
func (s *Store) UpdateCustomer(ctx context.Context, scope Scope, id, name, email, phone string) (Customer, error) {
	name, email, phone = strings.TrimSpace(name), strings.TrimSpace(email), strings.TrimSpace(phone)
	if n := utf8.RuneCountInString(name); n < 2 || n > 100 || len(email) > 254 || utf8.RuneCountInString(phone) > 30 {
		return Customer{}, ErrCustomerInput
	}
	if err := s.authorize(ctx, scope, "customers.create"); err != nil {
		return Customer{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Customer{}, err
	}
	defer tx.Rollback(ctx)
	var c Customer
	err = tx.QueryRow(ctx, `UPDATE customers SET name=$4,email=$5,phone=$6 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND removed_at IS NULL RETURNING id::text,name,email,phone`, scope.OrganizationID, scope.LocationID, id, name, email, phone).Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrCustomerNotFound
	}
	if err != nil {
		return Customer{}, customerUniqueness(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'customer.updated',$4)`, scope.OrganizationID, scope.LocationID, scope.ActorID, c.ID); err != nil {
		return Customer{}, err
	}
	return c, tx.Commit(ctx)
}

// CustomerDeletion says what DeleteCustomer did.
type CustomerDeletion struct {
	Deleted bool `json:"deleted"`
	// Removed: the customer had issued invoices, payments or signed records. Those stay for tax records; the customer no longer appears anywhere.
	Removed bool `json:"removed"`
}

// DeleteCustomer deletes a customer with everything attached to them: vehicles, website account links, bookings and their held
// times, jobs and draft invoices. Issued invoices, payments and other signed records cannot be erased (they must be kept for tax
// records), so a customer with any of those is removed instead: hidden from every list and look-up, their email and phone freed
// for a new customer, while those records stay. Needs customer_accounts.manage.
func (s *Store) DeleteCustomer(ctx context.Context, scope Scope, id string) (CustomerDeletion, error) {
	if err := s.authorize(ctx, scope, "customer_accounts.manage"); err != nil {
		return CustomerDeletion{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CustomerDeletion{}, err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND removed_at IS NULL FOR UPDATE)`, scope.OrganizationID, scope.LocationID, id).Scan(&exists); err != nil {
		return CustomerDeletion{}, err
	}
	if !exists {
		return CustomerDeletion{}, ErrCustomerNotFound
	}
	if _, err = tx.Exec(ctx, `SAVEPOINT erase`); err != nil {
		return CustomerDeletion{}, err
	}
	const scoped = `organization_id=$1 AND location_id=$2`
	const theirBookings = `SELECT id FROM bookings WHERE ` + scoped + ` AND customer_id=$3`
	const theirJobs = `SELECT id FROM jobs WHERE ` + scoped + ` AND (customer_id=$3 OR booking_id IN (` + theirBookings + `))`
	const theirInvoices = `SELECT id FROM invoices WHERE ` + scoped + ` AND customer_id=$3`
	// The held times of their jobs and bookings, removed once nothing points at them.
	var holds []string
	rows, err := tx.Query(ctx, `SELECT reservation_id::text FROM jobs WHERE id IN (`+theirJobs+`) AND reservation_id IS NOT NULL UNION SELECT reservation_id::text FROM bookings WHERE `+scoped+` AND customer_id=$3 AND reservation_id IS NOT NULL`, scope.OrganizationID, scope.LocationID, id)
	if err != nil {
		return CustomerDeletion{}, err
	}
	for rows.Next() {
		var h string
		if err = rows.Scan(&h); err != nil {
			rows.Close()
			return CustomerDeletion{}, err
		}
		holds = append(holds, h)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return CustomerDeletion{}, err
	}
	if holds == nil {
		holds = []string{}
	}
	erase := []string{
		`DELETE FROM invoice_items WHERE invoice_id IN (` + theirInvoices + `)`,
		`DELETE FROM invoice_events WHERE invoice_id IN (` + theirInvoices + `)`,
		`DELETE FROM invoice_draft_revisions WHERE invoice_id IN (` + theirInvoices + `)`,
		`DELETE FROM invoices WHERE ` + scoped + ` AND customer_id=$3`,
		`DELETE FROM booking_job_requests WHERE job_id IN (` + theirJobs + `) OR booking_id IN (` + theirBookings + `)`,
		`DELETE FROM job_contributors WHERE job_id IN (` + theirJobs + `)`,
		`DELETE FROM jobs WHERE id IN (` + theirJobs + `)`,
		`DELETE FROM booking_events WHERE booking_id IN (` + theirBookings + `)`,
		`DELETE FROM booking_capacity_requests WHERE booking_id IN (` + theirBookings + `)`,
		`DELETE FROM booking_reschedule_requests WHERE booking_id IN (` + theirBookings + `)`,
		`DELETE FROM employee_booking_worksites WHERE booking_id IN (` + theirBookings + `)`,
		`DELETE FROM bookings WHERE ` + scoped + ` AND customer_id=$3`,
		`DELETE FROM capacity_reservation_revisions WHERE organization_id=$1 AND location_id=$2 AND $3::uuid IS NOT NULL AND reservation_id = ANY($4::uuid[])`,
		`DELETE FROM capacity_reservations WHERE ` + scoped + ` AND $3::uuid IS NOT NULL AND id = ANY($4::uuid[])`,
		`DELETE FROM customer_account_events WHERE ` + scoped + ` AND customer_id=$3`,
		`DELETE FROM customer_account_links WHERE ` + scoped + ` AND customer_id=$3`,
		`DELETE FROM vehicles WHERE ` + scoped + ` AND customer_id=$3`,
		`DELETE FROM customers WHERE ` + scoped + ` AND id=$3`,
	}
	erased := true
	for _, q := range erase {
		args := []any{scope.OrganizationID, scope.LocationID, id}
		if strings.Contains(q, "$4") {
			args = append(args, holds)
		}
		if _, err = tx.Exec(ctx, q, args...); err != nil {
			var pg *pgconn.PgError
			if errors.As(err, &pg) && (pg.Code == "23503" || pg.Code == "P0001") {
				erased = false
				break
			}
			return CustomerDeletion{}, err
		}
	}
	action := "customer.deleted"
	if !erased {
		// Something here must be kept (an issued invoice, a payment, a signed quote...): undo the erase and remove the customer instead.
		if _, err = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT erase`); err != nil {
			return CustomerDeletion{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE customers SET removed_at=now() WHERE `+scoped+` AND id=$3`, scope.OrganizationID, scope.LocationID, id); err != nil {
			return CustomerDeletion{}, err
		}
		action = "customer.removed"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4,$5)`, scope.OrganizationID, scope.LocationID, scope.ActorID, action, id); err != nil {
		return CustomerDeletion{}, err
	}
	return CustomerDeletion{Deleted: erased, Removed: !erased}, tx.Commit(ctx)
}

// CustomerRecords is everything recorded for one customer, shown when the customer is opened.
type CustomerRecords struct {
	Vehicles []CustomerRecordLine `json:"vehicles"`
	Bookings []CustomerRecordLine `json:"bookings"`
	Jobs     []CustomerRecordLine `json:"jobs"`
	Invoices []CustomerRecordLine `json:"invoices"`
}
type CustomerRecordLine struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	At     string `json:"at"`
	Cents  *int64 `json:"cents,omitempty"`
}

// CustomerRecordsFor lists a customer's vehicles, bookings, jobs and invoices (up to 200 of each, newest first).
func (s *Store) CustomerRecordsFor(ctx context.Context, scope Scope, id string) (CustomerRecords, error) {
	out := CustomerRecords{Vehicles: []CustomerRecordLine{}, Bookings: []CustomerRecordLine{}, Jobs: []CustomerRecordLine{}, Invoices: []CustomerRecordLine{}}
	if err := s.authorize(ctx, scope, "customers.read"); err != nil {
		return out, err
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND removed_at IS NULL)`, scope.OrganizationID, scope.LocationID, id).Scan(&exists); err != nil {
		return out, err
	}
	if !exists {
		return out, ErrCustomerNotFound
	}
	stamp := func(col string) string { return `to_char(` + col + ` AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')` }
	for _, part := range []struct {
		into *[]CustomerRecordLine
		sql  string
	}{
		{&out.Vehicles, `SELECT id::text,description,category_description,` + stamp("created_at") + `,NULL::bigint FROM vehicles WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 ORDER BY created_at DESC LIMIT 200`},
		{&out.Bookings, `SELECT id::text,service_name,status,` + stamp("booking_time") + `,NULL::bigint FROM bookings WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 ORDER BY booking_time DESC LIMIT 200`},
		{&out.Jobs, `SELECT id::text,service_name,status,` + stamp("created_at") + `,NULL::bigint FROM jobs WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 ORDER BY created_at DESC LIMIT 200`},
		{&out.Invoices, `SELECT id::text,COALESCE(NULLIF(invoice_number,''),'Draft invoice'),status,` + stamp("created_at") + `,total_cents FROM invoices WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 ORDER BY created_at DESC LIMIT 200`},
	} {
		rows, err := s.pool.Query(ctx, part.sql, scope.OrganizationID, scope.LocationID, id)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var l CustomerRecordLine
			if err = rows.Scan(&l.ID, &l.Title, &l.Status, &l.At, &l.Cents); err != nil {
				rows.Close()
				return out, err
			}
			*part.into = append(*part.into, l)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return out, err
		}
	}
	return out, nil
}
