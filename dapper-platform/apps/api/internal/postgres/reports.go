package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrReportInput = errors.New("choose a report, a date range of at most two years and a valid time zone")

type ReportFilter struct {
	From, To string // YYYY-MM-DD, inclusive, in the viewer's time zone
	TZ       string
	Service  string
	Status   string
}

type ReportStat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type ReportPoint struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
	Cents int64  `json:"cents"`
}

// Report is one report ready to show, chart and export: columns and rows of text, headline figures, and one value per day.
type Report struct {
	Kind         string        `json:"kind"`
	From         string        `json:"from"`
	To           string        `json:"to"`
	Columns      []string      `json:"columns"`
	Rows         [][]string    `json:"rows"`
	Stats        []ReportStat  `json:"stats"`
	Series       []ReportPoint `json:"series"`
	ServiceNames []string      `json:"serviceNames"`
	Truncated    bool          `json:"truncated"`
}

const reportRowLimit = 5000

func reportMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

var reportStatusLabel = map[string]string{
	"pending": "Pending", "awaiting_payment": "Awaiting payment", "confirmed": "Approved", "scheduled": "Approved", "in_progress": "Approved",
	"completed": "Completed", "cancelled": "Cancelled",
}

func (f *ReportFilter) normalize() error {
	if f.TZ == "" {
		f.TZ = "UTC"
	}
	f.TZ = canonicalZone(f.TZ)
	if _, err := time.LoadLocation(f.TZ); err != nil || f.TZ == "Local" {
		return ErrReportInput
	}
	a, e1 := time.Parse("2006-01-02", f.From)
	b, e2 := time.Parse("2006-01-02", f.To)
	if e1 != nil || e2 != nil || b.Before(a) || b.Sub(a) > 732*24*time.Hour {
		return ErrReportInput
	}
	return nil
}

// fillDays turns per-day totals into a series with every day of the range present (zero when nothing happened).
func fillDays(from, to string, counts map[string]int, cents map[string]int64) []ReportPoint {
	a, _ := time.Parse("2006-01-02", from)
	b, _ := time.Parse("2006-01-02", to)
	out := []ReportPoint{}
	for d := a; !d.After(b); d = d.AddDate(0, 0, 1) {
		k := d.Format("2006-01-02")
		out = append(out, ReportPoint{Day: k, Count: counts[k], Cents: cents[k]})
	}
	return out
}

// RunReport builds the appointments, revenue or customers report for a franchise. Each kind needs the matching read permissions.
func (s *Store) RunReport(ctx context.Context, scope Scope, kind string, f ReportFilter) (Report, error) {
	if err := f.normalize(); err != nil {
		return Report{}, err
	}
	need := map[string][]string{"appointments": {"bookings.read"}, "revenue": {"payments.read", "invoices.read"}, "customers": {"customers.read", "bookings.read"}}[kind]
	if need == nil {
		return Report{}, ErrReportInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback(ctx)
	for _, action := range need {
		if err = authorizeTransaction(ctx, tx, scope, action); err != nil {
			return Report{}, err
		}
	}
	r := Report{Kind: kind, From: f.From, To: f.To, Rows: [][]string{}, Stats: []ReportStat{}, ServiceNames: []string{}}
	if names, e := tx.Query(ctx, `SELECT DISTINCT service_name FROM bookings WHERE organization_id=$1 AND location_id=$2 AND service_name<>'' ORDER BY 1 LIMIT 500`, scope.OrganizationID, scope.LocationID); e == nil {
		for names.Next() {
			var n string
			if names.Scan(&n) == nil {
				r.ServiceNames = append(r.ServiceNames, n)
			}
		}
		names.Close()
	}
	switch kind {
	case "appointments":
		err = reportAppointments(ctx, tx, scope, f, &r)
	case "revenue":
		err = reportRevenue(ctx, tx, scope, f, &r)
	default:
		err = reportCustomers(ctx, tx, scope, f, &r)
	}
	if err != nil {
		return Report{}, err
	}
	return r, tx.Commit(ctx)
}

const reportBookingFilter = ` AND (b.booking_time AT TIME ZONE $3)::date BETWEEN $4::date AND $5::date AND ($6='' OR b.service_name=$6) AND ($7='' OR b.status=$7 OR ($7='confirmed' AND b.status IN ('pending','awaiting_payment','scheduled','in_progress')))`

func reportAppointments(ctx context.Context, tx pgx.Tx, scope Scope, f ReportFilter, r *Report) error {
	r.Columns = []string{"ID", "Date", "Customer", "Service", "Status", "Booked", "Total", "Paid"}
	rows, err := tx.Query(ctx, `SELECT b.id::text, to_char(b.booking_time AT TIME ZONE $3,'YYYY-MM-DD HH24:MI'), c.name, b.service_name, b.status, b.channel, COALESCE(i.total_cents,0), COALESCE(i.paid_cents,0)
 FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id)
 LEFT JOIN LATERAL (SELECT total_cents,paid_cents FROM invoices i WHERE i.booking_id=b.id AND i.status<>'void' ORDER BY created_at LIMIT 1) i ON true
 WHERE b.organization_id=$1 AND b.location_id=$2`+reportBookingFilter+` ORDER BY b.booking_time DESC LIMIT `+fmt.Sprint(reportRowLimit+1),
		scope.OrganizationID, scope.LocationID, f.TZ, f.From, f.To, f.Service, f.Status)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts, cents := map[string]int{}, map[string]int64{}
	stat := map[string]int{}
	var billed, paid int64
	for rows.Next() {
		var id, when, customer, service, status, channel string
		var total, got int64
		if err = rows.Scan(&id, &when, &customer, &service, &status, &channel, &total, &got); err != nil {
			return err
		}
		if len(r.Rows) >= reportRowLimit {
			r.Truncated = true
			continue
		}
		label := reportStatusLabel[status]
		if label == "" {
			label = status
		}
		stat[label]++
		counts[when[:10]]++
		billed += total
		paid += got
		r.Rows = append(r.Rows, []string{"#" + id[:6], when, customer, service, label, channel, reportMoney(total), reportMoney(got)})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	r.Stats = []ReportStat{{"Appointments", fmt.Sprint(len(r.Rows))}, {"Approved", fmt.Sprint(stat["Approved"] + stat["Pending"] + stat["Awaiting payment"])}, {"Completed", fmt.Sprint(stat["Completed"])}, {"Cancelled", fmt.Sprint(stat["Cancelled"])}, {"Invoiced", reportMoney(billed)}, {"Paid", reportMoney(paid)}}
	r.Series = fillDays(f.From, f.To, counts, cents)
	return nil
}

func reportRevenue(ctx context.Context, tx pgx.Tx, scope Scope, f ReportFilter, r *Report) error {
	r.Columns = []string{"Date", "Customer", "Invoice", "Service", "Method", "Amount", "Refunded", "Net"}
	// A status here is the invoice's status (issued, partially_paid, paid).
	rows, err := tx.Query(ctx, `SELECT to_char(p.recorded_at AT TIME ZONE $3,'YYYY-MM-DD HH24:MI'), c.name, i.invoice_number, COALESCE(b.service_name,''), p.method, p.amount_cents, p.refunded_cents
 FROM payments p JOIN invoices i ON i.id=p.invoice_id JOIN customers c ON (c.organization_id,c.location_id,c.id)=(p.organization_id,p.location_id,p.customer_id)
 LEFT JOIN bookings b ON b.id=i.booking_id
 WHERE p.organization_id=$1 AND p.location_id=$2 AND p.status IN ('completed','refunded') AND (p.recorded_at AT TIME ZONE $3)::date BETWEEN $4::date AND $5::date
 AND ($6='' OR b.service_name=$6) AND ($7='' OR i.status=$7) ORDER BY p.recorded_at DESC LIMIT `+fmt.Sprint(reportRowLimit+1),
		scope.OrganizationID, scope.LocationID, f.TZ, f.From, f.To, f.Service, f.Status)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts, cents := map[string]int{}, map[string]int64{}
	var net, refunded int64
	for rows.Next() {
		var when, customer, number, service, method string
		var amount, back int64
		if err = rows.Scan(&when, &customer, &number, &service, &method, &amount, &back); err != nil {
			return err
		}
		if len(r.Rows) >= reportRowLimit {
			r.Truncated = true
			continue
		}
		net += amount - back
		refunded += back
		counts[when[:10]]++
		cents[when[:10]] += amount - back
		r.Rows = append(r.Rows, []string{when, customer, number, service, method, reportMoney(amount), reportMoney(back), reportMoney(amount - back)})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	var invoices int
	var billed, outstanding, discounts, tax int64
	if err = tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(i.total_cents),0),COALESCE(sum(i.due_cents),0),COALESCE(sum(i.discount_cents),0),COALESCE(sum(i.tax_cents),0)
 FROM invoices i LEFT JOIN bookings b ON b.id=i.booking_id
 WHERE i.organization_id=$1 AND i.location_id=$2 AND i.status NOT IN ('draft','void') AND (COALESCE(i.issued_at,i.created_at) AT TIME ZONE $3)::date BETWEEN $4::date AND $5::date
 AND ($6='' OR b.service_name=$6) AND ($7='' OR i.status=$7)`, scope.OrganizationID, scope.LocationID, f.TZ, f.From, f.To, f.Service, f.Status).Scan(&invoices, &billed, &outstanding, &discounts, &tax); err != nil {
		return err
	}
	r.Stats = []ReportStat{{"Collected (net)", reportMoney(net)}, {"Payments", fmt.Sprint(len(r.Rows))}, {"Refunded", reportMoney(refunded)}, {"Invoices issued", fmt.Sprint(invoices)}, {"Invoiced", reportMoney(billed)}, {"Still owed", reportMoney(outstanding)}, {"Discounts given", reportMoney(discounts)}, {"Tax on invoices", reportMoney(tax)}}
	r.Series = fillDays(f.From, f.To, counts, cents)
	return nil
}

func reportCustomers(ctx context.Context, tx pgx.Tx, scope Scope, f ReportFilter, r *Report) error {
	r.Columns = []string{"Customer", "Email", "Phone", "Appointments", "Completed", "Invoiced", "Paid", "Last appointment"}
	rows, err := tx.Query(ctx, `SELECT c.name,c.email,c.phone,count(b.id),count(b.id) FILTER (WHERE b.status='completed'),COALESCE(sum(i.total_cents),0),COALESCE(sum(i.paid_cents),0),to_char(max(b.booking_time) AT TIME ZONE $3,'YYYY-MM-DD')
 FROM customers c JOIN bookings b ON (b.organization_id,b.location_id,b.customer_id)=(c.organization_id,c.location_id,c.id)
 LEFT JOIN LATERAL (SELECT total_cents,paid_cents FROM invoices i WHERE i.booking_id=b.id AND i.status<>'void' ORDER BY created_at LIMIT 1) i ON true
 WHERE c.organization_id=$1 AND c.location_id=$2 AND c.removed_at IS NULL`+reportBookingFilter+` GROUP BY c.id ORDER BY count(b.id) DESC, c.name LIMIT `+fmt.Sprint(reportRowLimit+1),
		scope.OrganizationID, scope.LocationID, f.TZ, f.From, f.To, f.Service, f.Status)
	if err != nil {
		return err
	}
	defer rows.Close()
	var bookings int
	var billed, paid int64
	for rows.Next() {
		var name, email, phone, last string
		var n, done int
		var total, got int64
		if err = rows.Scan(&name, &email, &phone, &n, &done, &total, &got, &last); err != nil {
			return err
		}
		if len(r.Rows) >= reportRowLimit {
			r.Truncated = true
			continue
		}
		bookings += n
		billed += total
		paid += got
		r.Rows = append(r.Rows, []string{name, email, phone, fmt.Sprint(n), fmt.Sprint(done), reportMoney(total), reportMoney(got), last})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	var fresh int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM customers WHERE organization_id=$1 AND location_id=$2 AND removed_at IS NULL AND (created_at AT TIME ZONE $3)::date BETWEEN $4::date AND $5::date`, scope.OrganizationID, scope.LocationID, f.TZ, f.From, f.To).Scan(&fresh); err != nil {
		return err
	}
	r.Stats = []ReportStat{{"Customers booked", fmt.Sprint(len(r.Rows))}, {"New customers", fmt.Sprint(fresh)}, {"Appointments", fmt.Sprint(bookings)}, {"Invoiced", reportMoney(billed)}, {"Paid", reportMoney(paid)}}
	counts := map[string]int{}
	_ = counts
	r.Series = []ReportPoint{}
	return nil
}
