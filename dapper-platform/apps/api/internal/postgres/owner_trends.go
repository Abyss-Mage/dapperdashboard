package postgres

import (
	"context"
	"errors"
	"time"
)

var ErrTrendsInput = errors.New("valid date range, timezone and channel required")

type DayCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}
type KeyCount struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
	Cents int64  `json:"cents"`
	Due   int64  `json:"dueCents"`
}
type DayCents struct {
	Date  string `json:"date"`
	Cents int64  `json:"cents"`
}

// Each block is nil when the viewer lacks its read grant, so a missing permission is never drawn as zero.
type TrendBookings struct {
	ByDay       []DayCount `json:"byDay"` // excludes cancelled
	ByStatus    []KeyCount `json:"byStatus"`
	ByChannel   []KeyCount `json:"byChannel"` // ignores the channel filter so channels stay comparable
	TopServices []KeyCount `json:"topServices"`
	Total       int64      `json:"total"`
	// Bookings made per day (by when they were made) and completed per day (by appointment day), error404 #94; channel filter applies.
	CreatedByDay   []DayCount `json:"createdByDay"`
	CompletedByDay []DayCount `json:"completedByDay"`
	Created        int64      `json:"created"`
	Completed      int64      `json:"completed"`
}
type TrendPayments struct {
	ByDay    []DayCents `json:"byDay"`
	ByMethod []KeyCount `json:"byMethod"`
	NetCents int64      `json:"netCents"`
}
type TrendInvoices struct {
	ByStatus []KeyCount `json:"byStatus"`
}
type TrendJobs struct {
	CreatedByDay   []DayCount `json:"createdByDay"`
	CompletedByDay []DayCount `json:"completedByDay"`
	Created        int64      `json:"created"`
	Completed      int64      `json:"completed"`
	Open           int64      `json:"open"`
}
type OwnerTrends struct {
	GeneratedAt time.Time      `json:"generatedAt"`
	From        string         `json:"from"`
	To          string         `json:"to"`
	Timezone    string         `json:"timezone"`
	Channel     string         `json:"channel"`
	Bookings    *TrendBookings `json:"bookings"`
	Payments    *TrendPayments `json:"payments"`
	Invoices    *TrendInvoices `json:"invoices"`
	Jobs        *TrendJobs     `json:"jobs"`
}

// netPayments is every recorded payment as (day, method, cents): the new system's payments net of refunds, and the amounts the imported
// WordPress bookings recorded as paid (not void), on their appointment day (error404 #92). $1 org, $2 location, $5 timezone, $6 channel.
const netPayments = `SELECT (p.recorded_at AT TIME ZONE $5)::date AS day, p.method, p.amount_cents-p.refunded_cents AS cents FROM payments p WHERE p.organization_id=$1 AND p.location_id=$2 AND p.status IN ('completed','refunded') AND ($6='' OR EXISTS(SELECT 1 FROM invoices i JOIN bookings b ON b.id=i.booking_id WHERE i.id=p.invoice_id AND b.channel=$6))
 UNION ALL
 SELECT (b.booking_time AT TIME ZONE $5)::date, CASE WHEN l.payment_method='' THEN 'cash' ELSE l.payment_method END, l.paid_cents FROM legacy_booking_billing l JOIN bookings b ON b.id=l.booking_id WHERE l.organization_id=$1 AND l.location_id=$2 AND l.status<>'void' AND l.paid_cents>0 AND ($6='' OR b.channel=$6)`

// Only the two ways a customer actually books are offered as filters; other channels still appear in the "by channel" split.
var trendChannels = map[string]bool{"": true, "online": true, "walk_in": true}

// OwnerTrends returns daily series and breakdowns for one franchise over an inclusive date range (max about ten years, so "all time" works).
// Dates are calendar days in the supplied IANA timezone. Every block is authorized separately.
func (s *Store) OwnerTrends(ctx context.Context, scope Scope, from, to, tz, channel string) (OwnerTrends, error) {
	if err := scope.validate(); err != nil {
		return OwnerTrends{}, err
	}
	tz = canonicalZone(tz)
	if _, err := time.LoadLocation(tz); err != nil || tz == "" || tz == "Local" || !trendChannels[channel] {
		return OwnerTrends{}, ErrTrendsInput
	}
	f, errF := time.Parse("2006-01-02", from)
	t, errT := time.Parse("2006-01-02", to)
	if errF != nil || errT != nil || t.Before(f) || t.Sub(f) > 3660*24*time.Hour {
		return OwnerTrends{}, ErrTrendsInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OwnerTrends{}, err
	}
	defer tx.Rollback(ctx)
	out := OwnerTrends{From: from, To: to, Timezone: tz, Channel: channel}
	if err = tx.QueryRow(ctx, `SELECT now()`).Scan(&out.GeneratedAt); err != nil {
		return OwnerTrends{}, err
	}
	org, loc := scope.OrganizationID, scope.LocationID
	allowed := func(grant string) (bool, error) {
		err := authorizeTransaction(ctx, tx, scope, grant)
		if errors.Is(err, ErrForbidden) {
			return false, nil
		}
		return err == nil, err
	}
	days := func(query string, args ...any) ([]DayCount, error) {
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		res := []DayCount{}
		for rows.Next() {
			var d DayCount
			if err = rows.Scan(&d.Date, &d.Count); err != nil {
				return nil, err
			}
			res = append(res, d)
		}
		return res, rows.Err()
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
	ok, err := allowed("bookings.read")
	if err != nil {
		return OwnerTrends{}, err
	}
	if ok {
		b := &TrendBookings{}
		if b.ByDay, err = days(`SELECT g.d::date::text, count(b.id) FROM `+series+` LEFT JOIN bookings b ON b.organization_id=$1 AND b.location_id=$2 AND (b.booking_time AT TIME ZONE $5)::date=g.d::date AND ($6='' OR b.channel=$6) AND b.status<>'cancelled' GROUP BY g.d ORDER BY g.d`, org, loc, from, to, tz, channel); err != nil {
			return OwnerTrends{}, err
		}
		const inRange = `FROM bookings WHERE organization_id=$1 AND location_id=$2 AND (booking_time AT TIME ZONE $5)::date BETWEEN $3::date AND $4::date`
		if b.ByStatus, err = keys(`SELECT status,count(*),0::bigint,0::bigint `+inRange+` AND ($6='' OR channel=$6) GROUP BY status ORDER BY count(*) DESC,status`, org, loc, from, to, tz, channel); err != nil {
			return OwnerTrends{}, err
		}
		if b.ByChannel, err = keys(`SELECT channel,count(*),0::bigint,0::bigint `+inRange+` AND status<>'cancelled' GROUP BY channel ORDER BY count(*) DESC,channel`, org, loc, from, to, tz); err != nil {
			return OwnerTrends{}, err
		}
		if b.TopServices, err = keys(`SELECT service_name,count(*),0::bigint,0::bigint `+inRange+` AND ($6='' OR channel=$6) AND status<>'cancelled' GROUP BY service_name ORDER BY count(*) DESC,service_name LIMIT 6`, org, loc, from, to, tz, channel); err != nil {
			return OwnerTrends{}, err
		}
		for _, d := range b.ByDay {
			b.Total += d.Count
		}
		if b.CreatedByDay, err = days(`SELECT g.d::date::text, count(b.id) FROM `+series+` LEFT JOIN bookings b ON b.organization_id=$1 AND b.location_id=$2 AND (b.created_at AT TIME ZONE $5)::date=g.d::date AND ($6='' OR b.channel=$6) GROUP BY g.d ORDER BY g.d`, org, loc, from, to, tz, channel); err != nil {
			return OwnerTrends{}, err
		}
		if b.CompletedByDay, err = days(`SELECT g.d::date::text, count(b.id) FROM `+series+` LEFT JOIN bookings b ON b.organization_id=$1 AND b.location_id=$2 AND b.status='completed' AND (b.booking_time AT TIME ZONE $5)::date=g.d::date AND ($6='' OR b.channel=$6) GROUP BY g.d ORDER BY g.d`, org, loc, from, to, tz, channel); err != nil {
			return OwnerTrends{}, err
		}
		for i := range b.CreatedByDay {
			b.Created += b.CreatedByDay[i].Count
			b.Completed += b.CompletedByDay[i].Count
		}
		out.Bookings = b
	}
	if ok, err = allowed("payments.read"); err != nil {
		return OwnerTrends{}, err
	} else if ok {
		p := &TrendPayments{}
		rows, qerr := tx.Query(ctx, `SELECT g.d::date::text, COALESCE(sum(p.cents),0) FROM `+series+` LEFT JOIN (`+netPayments+`) p ON p.day=g.d::date GROUP BY g.d ORDER BY g.d`, org, loc, from, to, tz, channel)
		if qerr != nil {
			return OwnerTrends{}, qerr
		}
		for rows.Next() {
			var d DayCents
			if err = rows.Scan(&d.Date, &d.Cents); err != nil {
				rows.Close()
				return OwnerTrends{}, err
			}
			p.NetCents += d.Cents
			p.ByDay = append(p.ByDay, d)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return OwnerTrends{}, err
		}
		if p.ByMethod, err = keys(`SELECT method,count(*),COALESCE(sum(cents),0),0::bigint FROM (`+netPayments+`) p WHERE day BETWEEN $3::date AND $4::date GROUP BY method ORDER BY 3 DESC,method`, org, loc, from, to, tz, channel); err != nil {
			return OwnerTrends{}, err
		}
		out.Payments = p
	}
	if ok, err = allowed("invoices.read"); err != nil {
		return OwnerTrends{}, err
	} else if ok {
		i := &TrendInvoices{}
		// Imported WordPress invoices count too, dated by their appointment (error404 #94).
		if i.ByStatus, err = keys(`SELECT status,count(*),COALESCE(sum(total),0),COALESCE(sum(due),0) FROM (
 SELECT status,total_cents AS total,due_cents AS due FROM invoices WHERE organization_id=$1 AND location_id=$2 AND (created_at AT TIME ZONE $5)::date BETWEEN $3::date AND $4::date AND NOT (status='void' AND issued_at IS NULL) AND ($6='' OR EXISTS(SELECT 1 FROM bookings b WHERE b.id=invoices.booking_id AND b.channel=$6))
 UNION ALL
 SELECT l.status,l.total_cents,CASE WHEN l.status='void' THEN 0 ELSE l.total_cents-l.paid_cents END FROM legacy_booking_billing l JOIN bookings b ON b.id=l.booking_id WHERE l.organization_id=$1 AND l.location_id=$2 AND (b.booking_time AT TIME ZONE $5)::date BETWEEN $3::date AND $4::date AND ($6='' OR b.channel=$6)
) x GROUP BY status ORDER BY count(*) DESC,status`, org, loc, from, to, tz, channel); err != nil {
			return OwnerTrends{}, err
		}
		out.Invoices = i
	}
	if ok, err = allowed("jobs.read"); err != nil {
		return OwnerTrends{}, err
	} else if ok {
		j := &TrendJobs{}
		if j.CreatedByDay, err = days(`SELECT g.d::date::text, count(j.id) FROM `+series+` LEFT JOIN jobs j ON j.organization_id=$1 AND j.location_id=$2 AND (j.created_at AT TIME ZONE $5)::date=g.d::date GROUP BY g.d ORDER BY g.d`, org, loc, from, to, tz); err != nil {
			return OwnerTrends{}, err
		}
		if j.CompletedByDay, err = days(`SELECT g.d::date::text, count(j.id) FROM `+series+` LEFT JOIN jobs j ON j.organization_id=$1 AND j.location_id=$2 AND j.status='completed' AND (j.completed_at AT TIME ZONE $5)::date=g.d::date GROUP BY g.d ORDER BY g.d`, org, loc, from, to, tz); err != nil {
			return OwnerTrends{}, err
		}
		for _, d := range j.CreatedByDay {
			j.Created += d.Count
		}
		for _, d := range j.CompletedByDay {
			j.Completed += d.Count
		}
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE organization_id=$1 AND location_id=$2 AND status='available'`, org, loc).Scan(&j.Open); err != nil {
			return OwnerTrends{}, err
		}
		out.Jobs = j
	}
	if out.Bookings == nil && out.Payments == nil && out.Invoices == nil && out.Jobs == nil {
		return OwnerTrends{}, ErrForbidden
	}
	if err = tx.Commit(ctx); err != nil {
		return OwnerTrends{}, err
	}
	return out, nil
}
