package postgres

import (
	"context"
	"errors"
	"time"
)

type DayMoney struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
	Cents int64  `json:"cents"`
}

// Blocks are nil when the viewer lacks the matching read grant: a missing permission is never drawn as zero.
type FinanceInsights struct {
	InvoicedByDay []DayMoney `json:"invoicedByDay"` // invoices issued per day (draft and void excluded)
	Aging         []KeyCount `json:"aging"`         // outstanding balance by days overdue, as of today
	Invoiced      int64      `json:"invoicedCents"`
	Quotes        []KeyCount `json:"quotes"`        // quotations created in range by status
	RefundedCents int64      `json:"refundedCents"` // refunds on payments recorded in range
}
type WorkInsights struct {
	ByService  []KeyCount `json:"byService"`  // work orders created in range, by service
	OpenAge    []KeyCount `json:"openAge"`    // open work orders by how long they have been open
	AvgHours   *float64   `json:"avgHoursToComplete"`
	CompletedN int64      `json:"completed"`
}
type CustomerInsights struct {
	NewByDay      []DayCount `json:"newByDay"`
	New           int64      `json:"new"`
	Total         int64      `json:"total"`
	Vehicles      *int64     `json:"vehicles"`      // nil without vehicles.read
	BookingsSplit []KeyCount `json:"bookingsSplit"` // customers by number of non-cancelled bookings; nil without bookings.read
}
type OwnerInsights struct {
	GeneratedAt time.Time         `json:"generatedAt"`
	Section     string            `json:"section"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Timezone    string            `json:"timezone"`
	Finance     *FinanceInsights  `json:"finance"`
	Work        *WorkInsights     `json:"work"`
	Customers   *CustomerInsights `json:"customers"`
}

// OwnerInsights returns the extra per-section figures for the Finance, Work Orders and Customers dashboards.
func (s *Store) OwnerInsights(ctx context.Context, scope Scope, section, from, to, tz string) (OwnerInsights, error) {
	if err := scope.validate(); err != nil {
		return OwnerInsights{}, err
	}
	if section != "finance" && section != "work" && section != "customers" {
		return OwnerInsights{}, ErrTrendsInput
	}
	if _, err := time.LoadLocation(tz); err != nil || tz == "" || tz == "Local" {
		return OwnerInsights{}, ErrTrendsInput
	}
	f, errF := time.Parse("2006-01-02", from)
	t, errT := time.Parse("2006-01-02", to)
	if errF != nil || errT != nil || t.Before(f) || t.Sub(f) > 365*24*time.Hour {
		return OwnerInsights{}, ErrTrendsInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OwnerInsights{}, err
	}
	defer tx.Rollback(ctx)
	out := OwnerInsights{Section: section, From: from, To: to, Timezone: tz}
	if err = tx.QueryRow(ctx, `SELECT now()`).Scan(&out.GeneratedAt); err != nil {
		return OwnerInsights{}, err
	}
	org, loc := scope.OrganizationID, scope.LocationID
	allowed := func(grant string) (bool, error) {
		err := authorizeTransaction(ctx, tx, scope, grant)
		if errors.Is(err, ErrForbidden) {
			return false, nil
		}
		return err == nil, err
	}
	keys := func(query string, args ...any) ([]KeyCount, error) {
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		res := []KeyCount{}
		for rows.Next() {
			var k KeyCount
			if err = rows.Scan(&k.Key, &k.Count, &k.Cents, &k.Due); err != nil {
				return nil, err
			}
			res = append(res, k)
		}
		return res, rows.Err()
	}
	const series = `generate_series($3::date,$4::date,'1 day') AS g(d)`
	switch section {
	case "finance":
		fin := &FinanceInsights{}
		any := false
		ok, err := allowed("invoices.read")
		if err != nil {
			return OwnerInsights{}, err
		}
		if ok {
			any = true
			rows, qerr := tx.Query(ctx, `SELECT g.d::date::text, count(i.id), COALESCE(sum(i.total_cents),0) FROM `+series+` LEFT JOIN invoices i ON i.organization_id=$1 AND i.location_id=$2 AND (i.issued_at AT TIME ZONE $5)::date=g.d::date AND i.status NOT IN ('draft','void') GROUP BY g.d ORDER BY g.d`, org, loc, from, to, tz)
			if qerr != nil {
				return OwnerInsights{}, qerr
			}
			for rows.Next() {
				var d DayMoney
				if err = rows.Scan(&d.Date, &d.Count, &d.Cents); err != nil {
					rows.Close()
					return OwnerInsights{}, err
				}
				fin.Invoiced += d.Cents
				fin.InvoicedByDay = append(fin.InvoicedByDay, d)
			}
			rows.Close()
			if err = rows.Err(); err != nil {
				return OwnerInsights{}, err
			}
			if fin.Aging, err = keys(`SELECT bucket,count(*),0::bigint,COALESCE(sum(due_cents),0) FROM (
 SELECT due_cents, CASE
  WHEN due_at IS NULL THEN 'No due date'
  WHEN (due_at AT TIME ZONE $3)::date >= (now() AT TIME ZONE $3)::date THEN 'Not yet due'
  WHEN (now() AT TIME ZONE $3)::date-(due_at AT TIME ZONE $3)::date <= 30 THEN '1–30 days overdue'
  WHEN (now() AT TIME ZONE $3)::date-(due_at AT TIME ZONE $3)::date <= 60 THEN '31–60 days overdue'
  ELSE 'Over 60 days overdue' END AS bucket
 FROM invoices WHERE organization_id=$1 AND location_id=$2 AND status IN ('issued','partially_paid') AND due_cents>0) x
 GROUP BY bucket`, org, loc, tz); err != nil {
				return OwnerInsights{}, err
			}
		}
		if ok, err = allowed("quotes.read"); err != nil {
			return OwnerInsights{}, err
		} else if ok {
			any = true
			if fin.Quotes, err = keys(`SELECT status,count(*),COALESCE(sum(total_cents),0),0::bigint FROM scoped_quotations WHERE organization_id=$1 AND location_id=$2 AND (created_at AT TIME ZONE $5)::date BETWEEN $3::date AND $4::date GROUP BY status ORDER BY count(*) DESC,status`, org, loc, from, to, tz); err != nil {
				return OwnerInsights{}, err
			}
		}
		if ok, err = allowed("payments.read"); err != nil {
			return OwnerInsights{}, err
		} else if ok {
			any = true
			if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(refunded_cents),0) FROM payments WHERE organization_id=$1 AND location_id=$2 AND (recorded_at AT TIME ZONE $5)::date BETWEEN $3::date AND $4::date`, org, loc, from, to, tz).Scan(&fin.RefundedCents); err != nil {
				return OwnerInsights{}, err
			}
		}
		if !any {
			return OwnerInsights{}, ErrForbidden
		}
		out.Finance = fin
	case "work":
		ok, err := allowed("jobs.read")
		if err != nil {
			return OwnerInsights{}, err
		}
		if !ok {
			return OwnerInsights{}, ErrForbidden
		}
		w := &WorkInsights{}
		if w.ByService, err = keys(`SELECT service_name,count(*),0::bigint,0::bigint FROM jobs WHERE organization_id=$1 AND location_id=$2 AND (created_at AT TIME ZONE $5)::date BETWEEN $3::date AND $4::date GROUP BY service_name ORDER BY count(*) DESC,service_name LIMIT 6`, org, loc, from, to, tz); err != nil {
			return OwnerInsights{}, err
		}
		if w.OpenAge, err = keys(`SELECT bucket,count(*),0::bigint,0::bigint FROM (SELECT CASE
  WHEN now()-created_at < interval '1 day' THEN 'Under 1 day'
  WHEN now()-created_at < interval '4 days' THEN '1–3 days'
  WHEN now()-created_at < interval '8 days' THEN '4–7 days'
  ELSE 'Over 7 days' END AS bucket FROM jobs WHERE organization_id=$1 AND location_id=$2 AND status='available') x GROUP BY bucket`, org, loc); err != nil {
			return OwnerInsights{}, err
		}
		if err = tx.QueryRow(ctx, `SELECT count(*), avg(extract(epoch FROM (completed_at-created_at))/3600.0) FROM jobs WHERE organization_id=$1 AND location_id=$2 AND status='completed' AND (completed_at AT TIME ZONE $5)::date BETWEEN $3::date AND $4::date`, org, loc, from, to, tz).Scan(&w.CompletedN, &w.AvgHours); err != nil {
			return OwnerInsights{}, err
		}
		out.Work = w
	case "customers":
		ok, err := allowed("customers.read")
		if err != nil {
			return OwnerInsights{}, err
		}
		if !ok {
			return OwnerInsights{}, ErrForbidden
		}
		c := &CustomerInsights{}
		rows, qerr := tx.Query(ctx, `SELECT g.d::date::text, count(c.id) FROM `+series+` LEFT JOIN customers c ON c.organization_id=$1 AND c.location_id=$2 AND (c.created_at AT TIME ZONE $5)::date=g.d::date GROUP BY g.d ORDER BY g.d`, org, loc, from, to, tz)
		if qerr != nil {
			return OwnerInsights{}, qerr
		}
		for rows.Next() {
			var d DayCount
			if err = rows.Scan(&d.Date, &d.Count); err != nil {
				rows.Close()
				return OwnerInsights{}, err
			}
			c.New += d.Count
			c.NewByDay = append(c.NewByDay, d)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return OwnerInsights{}, err
		}
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM customers WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&c.Total); err != nil {
			return OwnerInsights{}, err
		}
		if ok, err = allowed("vehicles.read"); err != nil {
			return OwnerInsights{}, err
		} else if ok {
			var n int64
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&n); err != nil {
				return OwnerInsights{}, err
			}
			c.Vehicles = &n
		}
		if ok, err = allowed("bookings.read"); err != nil {
			return OwnerInsights{}, err
		} else if ok {
			if c.BookingsSplit, err = keys(`SELECT bucket,count(*),0::bigint,0::bigint FROM (SELECT CASE n WHEN 0 THEN 'No bookings' WHEN 1 THEN '1 booking' WHEN 2 THEN '2–3 bookings' WHEN 3 THEN '2–3 bookings' ELSE '4 or more' END AS bucket FROM (
 SELECT (SELECT count(*) FROM bookings b WHERE b.organization_id=c.organization_id AND b.location_id=c.location_id AND b.customer_id=c.id AND b.status<>'cancelled') AS n FROM customers c WHERE c.organization_id=$1 AND c.location_id=$2) y) x GROUP BY bucket`, org, loc); err != nil {
				return OwnerInsights{}, err
			}
		}
		out.Customers = c
	}
	if err = tx.Commit(ctx); err != nil {
		return OwnerInsights{}, err
	}
	return out, nil
}
