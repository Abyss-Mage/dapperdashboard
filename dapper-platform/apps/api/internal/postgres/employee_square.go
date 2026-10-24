package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// EmployeeSquareScope resolves an employee's assigned worksite scope for card payments. The scope
// comes from the administrator's assignment, never from request data, and needs payments.record.
func (s *Store) EmployeeSquareScope(ctx context.Context, actor string) (Scope, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Scope{}, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return Scope{}, err
	}
	scope := employeeScope(a)
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return Scope{}, err
	}
	return scope, nil
}

// EmployeeCardInvoiceScope additionally proves the booking belongs to the employee's dealership and the
// invoice belongs to that booking before a card charge may be prepared for it.
func (s *Store) EmployeeCardInvoiceScope(ctx context.Context, actor, booking, invoice string) (Scope, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Scope{}, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return Scope{}, err
	}
	if err = employeeBookingLock(ctx, tx, a, booking); err != nil {
		return Scope{}, err
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM invoices WHERE id=$1 AND organization_id=$2 AND location_id=$3 AND booking_id=$4`, invoice, a.OrganizationID, a.LocationID, booking).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Scope{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Scope{}, err
	}
	scope := employeeScope(a)
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return Scope{}, err
	}
	return scope, nil
}
