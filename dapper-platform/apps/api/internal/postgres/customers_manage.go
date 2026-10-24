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
FROM customers c WHERE c.organization_id=$1 AND c.location_id=$2 ORDER BY lower(c.name),c.id LIMIT 1000`, scope.OrganizationID, scope.LocationID)
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
	err = tx.QueryRow(ctx, `UPDATE customers SET name=$4,email=$5,phone=$6 WHERE organization_id=$1 AND location_id=$2 AND id=$3 RETURNING id::text,name,email,phone`, scope.OrganizationID, scope.LocationID, id, name, email, phone).Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrCustomerNotFound
	}
	if err != nil {
		return Customer{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'customer.updated',$4)`, scope.OrganizationID, scope.LocationID, scope.ActorID, c.ID); err != nil {
		return Customer{}, err
	}
	return c, tx.Commit(ctx)
}

// DeleteCustomer removes a customer who has no bookings, work orders, invoices, quotations or other business records. Their vehicles
// and website-account links go with them. Any customer with history is refused so ledgers and audit trails stay intact.
// Needs customer_accounts.manage (stronger than editing).
func (s *Store) DeleteCustomer(ctx context.Context, scope Scope, id string) error {
	if err := s.authorize(ctx, scope, "customer_accounts.manage"); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE)`, scope.OrganizationID, scope.LocationID, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrCustomerNotFound
	}
	for _, q := range []string{
		`DELETE FROM customer_account_events WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3`,
		`DELETE FROM customer_account_links WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3`,
		`DELETE FROM vehicles WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3`,
		`DELETE FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3`,
	} {
		if _, err = tx.Exec(ctx, q, scope.OrganizationID, scope.LocationID, id); err != nil {
			var pg *pgconn.PgError
			if errors.As(err, &pg) && (pg.Code == "23503" || pg.Code == "P0001") {
				return ErrCustomerInUse
			}
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'customer.deleted',$4)`, scope.OrganizationID, scope.LocationID, scope.ActorID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
