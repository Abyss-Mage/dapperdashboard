package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
)

// Portal DTOs deliberately omit internal notes, employee identities, merchant
// references and draft invoices. Do not reuse staff response types here.
// CreateCustomerVehicle grants ownership-scoped intake without staff grants.
func (s *Store) CreateCustomerVehicle(ctx context.Context, actor, customer string, input Vehicle) (Vehicle, error) {
	if actor == "" || customer == "" {
		return Vehicle{}, ErrForbidden
	}
	input.Description = strings.TrimSpace(input.Description)
	input.CategoryDescription = strings.TrimSpace(input.CategoryDescription)
	input.Identifier = strings.TrimSpace(input.Identifier)
	if n := utf8.RuneCountInString(input.Description); n < 2 || n > 120 || utf8.RuneCountInString(input.CategoryDescription) > 60 || utf8.RuneCountInString(input.Identifier) > 60 {
		return Vehicle{}, ErrVehicle
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Vehicle{}, err
	}
	defer tx.Rollback(ctx)
	var org, loc string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`, actor, customer).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return Vehicle{}, ErrForbidden
	}
	if err != nil {
		return Vehicle{}, err
	}
	var v Vehicle
	err = tx.QueryRow(ctx, `INSERT INTO vehicles(organization_id,location_id,customer_id,description,category_description,identifier) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text,customer_id::text,description,category_description,identifier`, org, loc, customer, input.Description, input.CategoryDescription, input.Identifier).Scan(&v.ID, &v.CustomerID, &v.Description, &v.CategoryDescription, &v.Identifier)
	if err != nil {
		return Vehicle{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'vehicle.created',$4)`, org, loc, actor, v.ID); err != nil {
		return Vehicle{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Vehicle{}, err
	}
	return v, nil
}

type PortalAccount struct {
	CustomerID   string `json:"customerId"`
	Name         string `json:"name"`
	LocationName string `json:"locationName"`
	// Filled in by CustomerRecords so the customer can see and correct their own details.
	Email   string `json:"email,omitempty"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}
type PortalBooking struct {
	ID          string    `json:"id"`
	ServiceName string    `json:"serviceName"`
	Status      string    `json:"status"`
	BookingTime time.Time `json:"bookingTime"`
	VehicleID   *string   `json:"vehicleId,omitempty"`
}
type PortalInvoice struct {
	ID         string `json:"id"`
	Number     string `json:"number"`
	Status     string `json:"status"`
	Currency   string `json:"currency"`
	TotalCents int64  `json:"totalCents"`
	PaidCents  int64  `json:"paidCents"`
	DueCents   int64  `json:"dueCents"`
	BookingID  string `json:"bookingId"`
}
type PortalRecords struct {
	Account  PortalAccount   `json:"account"`
	Vehicles []Vehicle       `json:"vehicles"`
	Bookings []PortalBooking `json:"bookings"`
	Invoices []PortalInvoice `json:"invoices"`
	// Saved addresses (default first) and whether the customer has a profile picture.
	Addresses []PortalAddress `json:"addresses"`
	HasAvatar bool            `json:"hasAvatar"`
	Limit     int             `json:"limit"`
}

func (s *Store) CustomerAccounts(ctx context.Context, actor string) ([]PortalAccount, error) {
	if actor == "" {
		return nil, ErrForbidden
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id::text,c.name,l.name FROM customer_account_links a
 JOIN principals p ON p.id=a.principal_id AND p.active
 JOIN customers c ON (c.organization_id,c.location_id,c.id)=(a.organization_id,a.location_id,a.customer_id)
 JOIN locations l ON (l.organization_id,l.id)=(a.organization_id,a.location_id)
 WHERE a.principal_id=$1 ORDER BY c.name,c.id LIMIT 100`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PortalAccount{}
	for rows.Next() {
		var item PortalAccount
		if err := rows.Scan(&item.CustomerID, &item.Name, &item.LocationName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CustomerRecords(ctx context.Context, actor, customer string) (PortalRecords, error) {
	if actor == "" || customer == "" {
		return PortalRecords{}, ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PortalRecords{}, err
	}
	defer tx.Rollback(ctx)
	result := PortalRecords{Vehicles: []Vehicle{}, Bookings: []PortalBooking{}, Invoices: []PortalInvoice{}, Addresses: []PortalAddress{}, Limit: 100}
	var org, loc string
	// Hold the membership and active principal through the entire read, so link
	// revocation/deactivation cannot race a separately authorized record query.
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text,c.id::text,c.name,l.name,c.email,c.phone,c.address
 FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active
 JOIN customers c ON (c.organization_id,c.location_id,c.id)=(a.organization_id,a.location_id,a.customer_id)
 JOIN locations l ON (l.organization_id,l.id)=(a.organization_id,a.location_id)
 WHERE a.principal_id=$1 AND a.customer_id=$2 FOR SHARE OF a,p`, actor, customer).Scan(&org, &loc, &result.Account.CustomerID, &result.Account.Name, &result.Account.LocationName, &result.Account.Email, &result.Account.Phone, &result.Account.Address)
	if errors.Is(err, pgx.ErrNoRows) {
		return PortalRecords{}, ErrForbidden
	}
	if err != nil {
		return PortalRecords{}, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,customer_id::text,description,category_description,identifier FROM vehicles WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 ORDER BY created_at DESC,id LIMIT 100`, org, loc, customer)
	if err != nil {
		return PortalRecords{}, err
	}
	for rows.Next() {
		var v Vehicle
		if err = rows.Scan(&v.ID, &v.CustomerID, &v.Description, &v.CategoryDescription, &v.Identifier); err != nil {
			rows.Close()
			return PortalRecords{}, err
		}
		result.Vehicles = append(result.Vehicles, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return PortalRecords{}, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text,service_name,status,booking_time,vehicle_id::text FROM bookings WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 ORDER BY booking_time DESC,id LIMIT 100`, org, loc, customer)
	if err != nil {
		return PortalRecords{}, err
	}
	for rows.Next() {
		var b PortalBooking
		if err = rows.Scan(&b.ID, &b.ServiceName, &b.Status, &b.BookingTime, &b.VehicleID); err != nil {
			rows.Close()
			return PortalRecords{}, err
		}
		result.Bookings = append(result.Bookings, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return PortalRecords{}, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text,invoice_number,status,currency,total_cents,paid_cents,due_cents,COALESCE(booking_id::text,'') FROM invoices WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 AND status IN ('issued','partially_paid','paid','void') ORDER BY created_at DESC,id LIMIT 100`, org, loc, customer)
	if err != nil {
		return PortalRecords{}, err
	}
	for rows.Next() {
		var i PortalInvoice
		if err = rows.Scan(&i.ID, &i.Number, &i.Status, &i.Currency, &i.TotalCents, &i.PaidCents, &i.DueCents, &i.BookingID); err != nil {
			rows.Close()
			return PortalRecords{}, err
		}
		result.Invoices = append(result.Invoices, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return PortalRecords{}, err
	}
	if result.Addresses, err = readPortalAddresses(ctx, tx, org, loc, customer); err != nil {
		return PortalRecords{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customer_avatars WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3)`, org, loc, customer).Scan(&result.HasAvatar); err != nil {
		return PortalRecords{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PortalRecords{}, err
	}
	return result, nil
}
