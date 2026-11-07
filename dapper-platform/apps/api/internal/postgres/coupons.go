package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrCouponInput    = errors.New("check the coupon: title, a code of 3 to 32 letters, numbers or dashes, a discount, and its period")
	ErrCouponNotFound = errors.New("coupon not found")
	ErrCouponCode     = errors.New("another coupon already uses this code")
	ErrCouponInUse    = errors.New("this coupon has been used, so it can be switched off but not deleted")
	couponCodePattern = regexp.MustCompile(`^[A-Z0-9-]{3,32}$`)
)

// CouponInvalid says why a code cannot be used right now, in words a customer or staff member can read.
type CouponInvalid struct{ Reason string }

func (e CouponInvalid) Error() string { return e.Reason }

type CouponInput struct {
	Title      string   `json:"title"`
	Code       string   `json:"code"`
	Kind       string   `json:"kind"`
	Value      int      `json:"value"`
	Period     string   `json:"period"`
	StartsOn   string   `json:"startsOn"`
	EndsOn     string   `json:"endsOn"`
	CustomerID string   `json:"customerId"`
	MaxUses    int      `json:"maxUses"`
	ServiceIDs []string `json:"serviceIds"`
	Active     bool     `json:"active"`
}

type Coupon struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Code         string   `json:"code"`
	Kind         string   `json:"kind"`
	Value        int      `json:"value"`
	Period       string   `json:"period"`
	StartsOn     string   `json:"startsOn"`
	EndsOn       string   `json:"endsOn"`
	CustomerID   string   `json:"customerId"`
	CustomerName string   `json:"customerName"`
	MaxUses      int      `json:"maxUses"`
	Used         int      `json:"used"`
	ServiceIDs   []string `json:"serviceIds"`
	Active       bool     `json:"active"`
	Version      int      `json:"version"`
}

type CouponApplication struct {
	CouponID      string
	Code          string
	DiscountCents int64
}

func (in *CouponInput) normalize() error {
	in.Title, in.Code = strings.TrimSpace(in.Title), strings.ToUpper(strings.TrimSpace(in.Code))
	if n := utf8.RuneCountInString(in.Title); n < 1 || n > 100 || !couponCodePattern.MatchString(in.Code) || in.MaxUses < 0 || in.MaxUses > 1000000 {
		return ErrCouponInput
	}
	switch in.Kind {
	case "percent":
		if in.Value < 1 || in.Value > 100 {
			return ErrCouponInput
		}
	case "fixed":
		if in.Value < 1 || in.Value > 100000000 {
			return ErrCouponInput
		}
	default:
		return ErrCouponInput
	}
	switch in.Period {
	case "never":
		in.StartsOn, in.EndsOn = "", ""
	case "range":
		a, e1 := time.Parse("2006-01-02", in.StartsOn)
		b, e2 := time.Parse("2006-01-02", in.EndsOn)
		if e1 != nil || e2 != nil || a.After(b) {
			return ErrCouponInput
		}
	default:
		return ErrCouponInput
	}
	if len(in.ServiceIDs) > 200 {
		return ErrCouponInput
	}
	return nil
}

func couponWriteError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrCouponCode
	}
	return err
}

// Coupons lists a franchise's coupons with how many times each was used. It needs marketing.read.
func (s *Store) Coupons(ctx context.Context, scope Scope) ([]Coupon, error) {
	if err := s.authorize(ctx, scope, "marketing.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id::text,c.title,c.code,c.kind,c.value,c.period,COALESCE(c.starts_on::text,''),COALESCE(c.ends_on::text,''),COALESCE(c.customer_id::text,''),COALESCE(cu.name,''),c.max_uses,
 (SELECT count(*) FROM coupon_redemptions r JOIN invoices i ON i.id=r.invoice_id WHERE r.coupon_id=c.id AND i.status<>'void'),
 COALESCE((SELECT array_agg(s.service_id::text ORDER BY s.service_id) FROM coupon_services s WHERE s.coupon_id=c.id),'{}'),c.active,c.version
 FROM coupons c LEFT JOIN customers cu ON (cu.organization_id,cu.location_id,cu.id)=(c.organization_id,c.location_id,c.customer_id)
 WHERE c.organization_id=$1 AND c.location_id=$2 ORDER BY c.created_at DESC LIMIT 500`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Coupon{}
	for rows.Next() {
		var c Coupon
		if err = rows.Scan(&c.ID, &c.Title, &c.Code, &c.Kind, &c.Value, &c.Period, &c.StartsOn, &c.EndsOn, &c.CustomerID, &c.CustomerName, &c.MaxUses, &c.Used, &c.ServiceIDs, &c.Active, &c.Version); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveCoupon creates a coupon (id "") or revises one (version is the revision the editor loaded). It needs marketing.manage.
func (s *Store) SaveCoupon(ctx context.Context, scope Scope, id string, in CouponInput, version int) (string, error) {
	if err := in.normalize(); err != nil {
		return "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "marketing.manage"); err != nil {
		return "", err
	}
	if in.CustomerID != "" {
		var ok bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND removed_at IS NULL)`, scope.OrganizationID, scope.LocationID, in.CustomerID).Scan(&ok); err != nil || !ok {
			return "", ErrCouponInput
		}
	}
	if len(in.ServiceIDs) > 0 {
		var found int
		if err = tx.QueryRow(ctx, `SELECT count(DISTINCT id) FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id::text = ANY($3)`, scope.OrganizationID, scope.LocationID, in.ServiceIDs).Scan(&found); err != nil {
			return "", err
		}
		if found != len(uniqueStrings(in.ServiceIDs)) {
			return "", ErrCouponInput
		}
	}
	if id == "" {
		err = tx.QueryRow(ctx, `INSERT INTO coupons(organization_id,location_id,title,code,kind,value,period,starts_on,ends_on,customer_id,max_uses,active)
 VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::date,NULLIF($9,'')::date,NULLIF($10,'')::uuid,$11,$12) RETURNING id::text`,
			scope.OrganizationID, scope.LocationID, in.Title, in.Code, in.Kind, in.Value, in.Period, in.StartsOn, in.EndsOn, in.CustomerID, in.MaxUses, in.Active).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, `UPDATE coupons SET title=$4,code=$5,kind=$6,value=$7,period=$8,starts_on=NULLIF($9,'')::date,ends_on=NULLIF($10,'')::date,customer_id=NULLIF($11,'')::uuid,max_uses=$12,active=$13,version=version+1
 WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND version=$14 RETURNING id::text`,
			scope.OrganizationID, scope.LocationID, id, in.Title, in.Code, in.Kind, in.Value, in.Period, in.StartsOn, in.EndsOn, in.CustomerID, in.MaxUses, in.Active, version).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrCouponNotFound
		}
	}
	if err != nil {
		return "", couponWriteError(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM coupon_services WHERE coupon_id=$1`, id); err != nil {
		return "", err
	}
	for _, svc := range uniqueStrings(in.ServiceIDs) {
		if _, err = tx.Exec(ctx, `INSERT INTO coupon_services(coupon_id,service_id) VALUES($1,$2::uuid)`, id, svc); err != nil {
			return "", err
		}
	}
	return id, tx.Commit(ctx)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// DeleteCoupon removes a coupon that was never used; a used one can only be switched off. It needs marketing.manage.
func (s *Store) DeleteCoupon(ctx context.Context, scope Scope, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "marketing.manage"); err != nil {
		return err
	}
	var used bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM coupon_redemptions r JOIN coupons c ON c.id=r.coupon_id WHERE c.id::text=$1 AND c.organization_id=$2 AND c.location_id=$3)`, id, scope.OrganizationID, scope.LocationID).Scan(&used); err != nil {
		return err
	}
	if used {
		return ErrCouponInUse
	}
	tag, err := tx.Exec(ctx, `DELETE FROM coupons WHERE id::text=$1 AND organization_id=$2 AND location_id=$3`, id, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrCouponNotFound
	}
	return tx.Commit(ctx)
}

// evaluateCoupon decides whether a code can be used for this customer, main service and subtotal, and how much it takes off.
func (s *Store) evaluateCoupon(ctx context.Context, q rowQuerier, scope Scope, code, customerID string, mainServiceID *string, subtotal int64) (CouponApplication, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	bad := func(reason string) (CouponApplication, error) { return CouponApplication{}, CouponInvalid{reason} }
	if !couponCodePattern.MatchString(code) {
		return bad("That coupon code is not valid.")
	}
	var id, kind, period string
	var value, maxUses int
	var active, started, ended bool
	var customer *string
	err := q.QueryRow(ctx, `SELECT id::text,kind,value,period,max_uses,active,customer_id::text,
 (period='never' OR (now() AT TIME ZONE 'America/Toronto')::date >= starts_on),
 (period='range' AND (now() AT TIME ZONE 'America/Toronto')::date > ends_on)
 FROM coupons WHERE organization_id=$1 AND location_id=$2 AND code=$3`, scope.OrganizationID, scope.LocationID, code).Scan(&id, &kind, &value, &period, &maxUses, &active, &customer, &started, &ended)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
		return bad("That coupon code is not valid.")
	}
	if err != nil {
		return CouponApplication{}, err
	}
	if !started {
		return bad("This coupon is not active yet.")
	}
	if ended {
		return bad("This coupon has expired.")
	}
	if customer != nil && *customer != customerID {
		return bad("This coupon is for a different customer.")
	}
	var restricted, matches bool
	if err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM coupon_services WHERE coupon_id::text=$1), EXISTS(SELECT 1 FROM coupon_services WHERE coupon_id::text=$1 AND service_id::text=$2)`, id, derefString(mainServiceID)).Scan(&restricted, &matches); err != nil {
		return CouponApplication{}, err
	}
	if restricted && !matches {
		return bad("This coupon does not apply to this service.")
	}
	if maxUses > 0 {
		var used int
		if err = q.QueryRow(ctx, `SELECT count(*) FROM coupon_redemptions r JOIN invoices i ON i.id=r.invoice_id WHERE r.coupon_id::text=$1 AND i.status<>'void'`, id).Scan(&used); err != nil {
			return CouponApplication{}, err
		}
		if used >= maxUses {
			return bad("This coupon has been used the maximum number of times.")
		}
	}
	discount := int64(value)
	if kind == "percent" {
		discount = (subtotal*int64(value) + 50) / 100
	}
	if discount > subtotal {
		discount = subtotal
	}
	if discount <= 0 {
		return bad("This coupon does not reduce this booking's price.")
	}
	return CouponApplication{CouponID: id, Code: code, DiscountCents: discount}, nil
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// CheckCoupon tells a customer (before they book) whether the code works and what it takes off. It changes nothing.
func (s *Store) CheckCoupon(ctx context.Context, scope Scope, code, customerID string, mainServiceID *string, subtotal int64) (CouponApplication, error) {
	return s.evaluateCoupon(ctx, s.pool, scope, code, customerID, mainServiceID, subtotal)
}

// ApplyCouponToInvoice puts a coupon on a draft invoice (replacing one already there) and recomputes tax on the reduced amount.
// It needs invoices.create. An issued invoice cannot be changed.
func (s *Store) ApplyCouponToInvoice(ctx context.Context, scope Scope, invoiceID, code string) (Invoice, error) {
	return s.setInvoiceCoupon(ctx, scope, invoiceID, code, true)
}

// RemoveCouponFromInvoice takes the coupon off a draft invoice.
func (s *Store) RemoveCouponFromInvoice(ctx context.Context, scope Scope, invoiceID string) (Invoice, error) {
	return s.setInvoiceCoupon(ctx, scope, invoiceID, "", false)
}

func (s *Store) setInvoiceCoupon(ctx context.Context, scope Scope, invoiceID, code string, apply bool) (Invoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.create"); err != nil {
		return Invoice{}, err
	}
	var status, customerID string
	var subtotal int64
	var serviceID *string
	err = tx.QueryRow(ctx, `SELECT i.status,i.customer_id::text,i.subtotal_cents,b.service_id::text FROM invoices i LEFT JOIN bookings b ON b.id=i.booking_id
 WHERE i.id::text=$1 AND i.organization_id=$2 AND i.location_id=$3 FOR UPDATE OF i`, invoiceID, scope.OrganizationID, scope.LocationID).Scan(&status, &customerID, &subtotal, &serviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}
	if status != "draft" {
		return Invoice{}, ErrInvoiceStatusConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM coupon_redemptions WHERE invoice_id::text=$1`, invoiceID); err != nil {
		return Invoice{}, err
	}
	var app CouponApplication
	if apply {
		if app, err = s.evaluateCoupon(ctx, tx, scope, code, customerID, serviceID, subtotal); err != nil {
			return Invoice{}, err
		}
	}
	tax, _, err := s.taxOnSubtotal(ctx, scope, subtotal-app.DiscountCents)
	if err != nil {
		return Invoice{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE invoices SET discount_cents=$2,coupon_code=$3,tax_cents=$4,total_cents=subtotal_cents-$2+$4,due_cents=subtotal_cents-$2+$4-paid_cents,updated_at=now() WHERE id::text=$1`, invoiceID, app.DiscountCents, app.Code, tax); err != nil {
		return Invoice{}, err
	}
	note := "Coupon removed"
	if apply {
		if _, err = tx.Exec(ctx, `INSERT INTO coupon_redemptions(coupon_id,invoice_id,customer_id,amount_cents) VALUES($1::uuid,$2::uuid,$3::uuid,$4)`, app.CouponID, invoiceID, customerID, app.DiscountCents); err != nil {
			return Invoice{}, err
		}
		note = "Coupon " + app.Code + " applied"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO invoice_events(invoice_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1::uuid,$2,$3,$4,'draft','draft',$5)`, invoiceID, scope.OrganizationID, scope.LocationID, scope.ActorID, note); err != nil {
		return Invoice{}, err
	}
	inv, err := scanInvoice(tx.QueryRow(ctx, `SELECT `+invoiceSelectColumns+` FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id) WHERE i.id::text=$1`, invoiceID))
	if err != nil {
		return Invoice{}, err
	}
	inv.Items, err = invoiceItemsInTransaction(ctx, tx, invoiceID)
	if err != nil {
		return Invoice{}, err
	}
	return inv, tx.Commit(ctx)
}
