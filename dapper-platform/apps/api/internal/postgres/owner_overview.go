package postgres

import (
	"context"
	"errors"
	"time"
)

type OwnerOverview struct {
	GeneratedAt time.Time         `json:"generatedAt"`
	WindowEnd   time.Time         `json:"windowEnd"`
	Metrics     map[string]*int64 `json:"metrics"`
}

func (s *Store) OwnerOverview(ctx context.Context, scope Scope) (OwnerOverview, error) {
	if err := scope.validate(); err != nil {
		return OwnerOverview{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OwnerOverview{}, err
	}
	defer tx.Rollback(ctx)
	result := OwnerOverview{Metrics: map[string]*int64{"customers": nil, "upcomingBookings": nil, "openJobs": nil, "outstandingCents": nil, "netPaymentsCents": nil}}
	if err = tx.QueryRow(ctx, `SELECT now()`).Scan(&result.GeneratedAt); err != nil {
		return OwnerOverview{}, err
	}
	result.WindowEnd = result.GeneratedAt.Add(7 * 24 * time.Hour)
	allowed := false
	for _, item := range []struct{ key, grant, query string }{
		{"customers", "customers.read", `SELECT count(*) FROM customers WHERE organization_id=$1 AND location_id=$2 AND removed_at IS NULL AND NOT EXISTS(SELECT 1 FROM dealership_accounts d WHERE d.organization_id=customers.organization_id AND d.location_id=customers.location_id AND d.customer_id=customers.id)`},
		{"upcomingBookings", "bookings.read", `SELECT count(*) FROM bookings WHERE organization_id=$1 AND location_id=$2 AND booking_time>=now() AND booking_time<now()+interval '7 days' AND status NOT IN ('completed','cancelled')`},
		{"openJobs", "jobs.read", `SELECT count(*) FROM jobs WHERE organization_id=$1 AND location_id=$2 AND status='available'`},
		// Owed right now (error404 #93): what is due on issued invoices, the draft invoice of every approved booking not yet done, and
		// what imported WordPress bookings still owed.
		{"outstandingCents", "invoices.read", `SELECT COALESCE((SELECT sum(due_cents) FROM invoices WHERE organization_id=$1 AND location_id=$2 AND status IN ('issued','partially_paid')),0)
 + COALESCE((SELECT sum(i.total_cents) FROM invoices i JOIN bookings b ON b.id=i.booking_id WHERE i.organization_id=$1 AND i.location_id=$2 AND i.status='draft' AND b.status='confirmed'),0)
 + COALESCE((SELECT sum(l.total_cents-l.paid_cents) FROM legacy_booking_billing l JOIN bookings b ON b.id=l.booking_id WHERE l.organization_id=$1 AND l.location_id=$2 AND l.status='partially_paid' AND b.status<>'cancelled'),0)`},
		{"netPaymentsCents", "payments.read", `SELECT COALESCE((SELECT sum(amount_cents-refunded_cents) FROM payments WHERE organization_id=$1 AND location_id=$2 AND status IN ('completed','refunded')),0)
 + COALESCE((SELECT sum(paid_cents) FROM legacy_booking_billing WHERE organization_id=$1 AND location_id=$2 AND status<>'void'),0)`},
	} {
		err = authorizeTransaction(ctx, tx, scope, item.grant)
		if errors.Is(err, ErrForbidden) {
			continue
		}
		if err != nil {
			return OwnerOverview{}, err
		}
		allowed = true
		var value int64
		if err = tx.QueryRow(ctx, item.query, scope.OrganizationID, scope.LocationID).Scan(&value); err != nil {
			return OwnerOverview{}, err
		}
		result.Metrics[item.key] = &value
	}
	if !allowed {
		return OwnerOverview{}, ErrForbidden
	}
	if err = tx.Commit(ctx); err != nil {
		return OwnerOverview{}, err
	}
	return result, nil
}
