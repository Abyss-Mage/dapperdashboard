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
		{"customers", "customers.read", `SELECT count(*) FROM customers WHERE organization_id=$1 AND location_id=$2 AND removed_at IS NULL`},
		{"upcomingBookings", "bookings.read", `SELECT count(*) FROM bookings WHERE organization_id=$1 AND location_id=$2 AND booking_time>=now() AND booking_time<now()+interval '7 days' AND status NOT IN ('completed','cancelled')`},
		{"openJobs", "jobs.read", `SELECT count(*) FROM jobs WHERE organization_id=$1 AND location_id=$2 AND status='available'`},
		{"outstandingCents", "invoices.read", `SELECT COALESCE(sum(due_cents),0) FROM invoices WHERE organization_id=$1 AND location_id=$2 AND status IN ('issued','partially_paid')`},
		{"netPaymentsCents", "payments.read", `SELECT COALESCE(sum(amount_cents-refunded_cents),0) FROM payments WHERE organization_id=$1 AND location_id=$2 AND status IN ('completed','refunded')`},
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
