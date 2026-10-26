package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) EmployeeServices(ctx context.Context, actor string) ([]CatalogService, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	items, err := s.Services(ctx, employeeScope(a))
	if err != nil {
		return nil, err
	}
	return items, tx.Commit(ctx)
}

func employeeBookingLock(ctx context.Context, tx pgx.Tx, a EmployeeWorksite, booking string) error {
	var id string
	err := tx.QueryRow(ctx, `SELECT w.booking_id::text FROM employee_booking_worksites w
 JOIN bookings b ON b.id=w.booking_id AND b.organization_id=w.organization_id AND b.location_id=w.location_id
 WHERE w.organization_id=$1 AND w.location_id=$2 AND (w.dealership_id IS NULL OR w.dealership_id=NULLIF($3,'')::uuid) AND w.booking_id=$4 FOR SHARE OF w,b`, a.OrganizationID, a.LocationID, a.DealershipID, booking).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBookingNotFound
	}
	return err
}
func (s *Store) EmployeeBookingInvoices(ctx context.Context, actor, booking string) ([]Invoice, error) {
	result := []Invoice{}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	if err = employeeBookingLock(ctx, tx, a, booking); err != nil {
		return nil, err
	}
	if err = authorizeTransaction(ctx, tx, employeeScope(a), "invoices.read"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text FROM invoices WHERE organization_id=$1 AND location_id=$2 AND booking_id=$3 ORDER BY created_at DESC,id LIMIT 20`, a.OrganizationID, a.LocationID, booking)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		v, e := s.GetInvoice(ctx, employeeScope(a), id)
		if e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, tx.Commit(ctx)
}
func (s *Store) EmployeeCashReceipt(ctx context.Context, actor, booking string, input RecordPaymentInput) (Payment, Invoice, error) {
	var p Payment
	var inv Invoice
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return p, inv, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return p, inv, err
	}
	if err = employeeBookingLock(ctx, tx, a, booking); err != nil {
		return p, inv, err
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM invoices WHERE id=$1 AND organization_id=$2 AND location_id=$3 AND booking_id=$4`, input.InvoiceID, a.OrganizationID, a.LocationID, booking).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, inv, ErrInvoiceNotFound
	}
	if err != nil {
		return p, inv, err
	}
	input.Method = "cash"
	input.ProviderReference = ""
	p, inv, err = s.RecordPayment(ctx, employeeScope(a), input)
	if err != nil {
		return p, inv, err
	}
	return p, inv, tx.Commit(ctx)
}
