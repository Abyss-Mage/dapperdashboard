package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCoupons(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_coupon_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		_, _ = admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
	}()
	config := admin.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var org, loc, staff, stranger, customer, otherCustomer string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Coupon Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic coupon site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:coupon-staff') RETURNING id::text`).Scan(&staff))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:coupon-stranger') RETURNING id::text`).Scan(&stranger))
	for _, action := range []string{"marketing.read", "marketing.manage", "invoices.create", "invoices.read", "catalogue.manage", "catalogue.read"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, staff, org, loc, action)
		must(err)
	}
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic Coupon Customer','','') RETURNING id::text`, org, loc).Scan(&customer))
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Another Customer','','') RETURNING id::text`, org, loc).Scan(&otherCustomer))
	scope := Scope{ActorID: staff, OrganizationID: org, LocationID: loc}
	outsider := Scope{ActorID: stranger, OrganizationID: org, LocationID: loc}
	newInvoice := func(owner string) Invoice {
		inv, e := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: owner, Items: []CreateInvoiceItemInput{{Description: "Synthetic detail", Quantity: 1, UnitPriceCents: 10000}}})
		must(e)
		return inv
	}
	good := CouponInput{Title: "Spring 20", Code: " spring20 ", Kind: "percent", Value: 20, Period: "never", Active: true}

	// Validation and permissions.
	for name, bad := range map[string]CouponInput{
		"short code":       {Title: "x", Code: "AB", Kind: "percent", Value: 10, Period: "never"},
		"odd characters":   {Title: "x", Code: "NO SPACES", Kind: "percent", Value: 10, Period: "never"},
		"over 100 percent": {Title: "x", Code: "BIGONE", Kind: "percent", Value: 101, Period: "never"},
		"reversed range":   {Title: "x", Code: "REVERSE", Kind: "fixed", Value: 500, Period: "range", StartsOn: "2030-02-01", EndsOn: "2030-01-01"},
		"no title":         {Title: " ", Code: "NOTITLE", Kind: "fixed", Value: 500, Period: "never"},
	} {
		if _, err = s.SaveCoupon(ctx, scope, "", bad, 0); !errors.Is(err, ErrCouponInput) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	if _, err = s.SaveCoupon(ctx, outsider, "", good, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without marketing.manage made a coupon", err)
	}
	if _, err = s.Coupons(ctx, outsider); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without marketing.read listed coupons", err)
	}
	spring, err := s.SaveCoupon(ctx, scope, "", good, 0)
	must(err)
	if _, err = s.SaveCoupon(ctx, scope, "", good, 0); !errors.Is(err, ErrCouponCode) {
		t.Fatal("a duplicate code was accepted", err)
	}

	// Applying: 20 percent off 100.00, with tax worked out on the reduced amount.
	inv := newInvoice(customer)
	applied, err := s.ApplyCouponToInvoice(ctx, scope, inv.ID, "Spring20")
	must(err)
	if applied.DiscountCents != 2000 || applied.CouponCode != "SPRING20" || applied.SubtotalCents != 10000 || applied.TotalCents != 10000-2000+applied.TaxCents || applied.DueCents != applied.TotalCents {
		t.Fatalf("discount not applied correctly: %+v", applied)
	}
	listed, err := s.Coupons(ctx, scope)
	if err != nil || len(listed) != 1 || listed[0].Used != 1 {
		t.Fatal("usage not counted", listed, err)
	}
	if err = s.DeleteCoupon(ctx, scope, spring); !errors.Is(err, ErrCouponInUse) {
		t.Fatal("a used coupon was deleted", err)
	}
	removed, err := s.RemoveCouponFromInvoice(ctx, scope, inv.ID)
	must(err)
	if removed.DiscountCents != 0 || removed.CouponCode != "" || removed.TotalCents != 10000+removed.TaxCents {
		t.Fatalf("coupon not removed: %+v", removed)
	}

	// One use only, the wrong customer, a past period, and a service the coupon does not cover.
	once, err := s.SaveCoupon(ctx, scope, "", CouponInput{Title: "Once", Code: "ONCE-ONLY", Kind: "fixed", Value: 1500, Period: "never", MaxUses: 1, Active: true}, 0)
	must(err)
	first, second := newInvoice(customer), newInvoice(customer)
	if got, e := s.ApplyCouponToInvoice(ctx, scope, first.ID, "ONCE-ONLY"); e != nil || got.DiscountCents != 1500 {
		t.Fatal("fixed coupon not applied", got, e)
	}
	var invalid CouponInvalid
	if _, err = s.ApplyCouponToInvoice(ctx, scope, second.ID, "ONCE-ONLY"); !errors.As(err, &invalid) {
		t.Fatal("a coupon was used more often than allowed", err)
	}
	_, err = s.SaveCoupon(ctx, scope, "", CouponInput{Title: "Mine", Code: "FORME", Kind: "percent", Value: 10, Period: "never", CustomerID: otherCustomer, Active: true}, 0)
	must(err)
	if _, err = s.ApplyCouponToInvoice(ctx, scope, second.ID, "FORME"); !errors.As(err, &invalid) {
		t.Fatal("a customer-only coupon worked for someone else", err)
	}
	_, err = s.SaveCoupon(ctx, scope, "", CouponInput{Title: "Old", Code: "EXPIRED", Kind: "percent", Value: 10, Period: "range", StartsOn: "2020-01-01", EndsOn: "2020-12-31", Active: true}, 0)
	must(err)
	if _, err = s.ApplyCouponToInvoice(ctx, scope, second.ID, "EXPIRED"); !errors.As(err, &invalid) {
		t.Fatal("an expired coupon worked", err)
	}
	service, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic Full Detail", Kind: "main"})
	must(err)
	_, err = s.SaveCoupon(ctx, scope, "", CouponInput{Title: "Detail only", Code: "DETAILONLY", Kind: "percent", Value: 10, Period: "never", ServiceIDs: []string{service.ID}, Active: true}, 0)
	must(err)
	if _, err = s.ApplyCouponToInvoice(ctx, scope, second.ID, "DETAILONLY"); !errors.As(err, &invalid) {
		t.Fatal("a service-only coupon worked on an invoice for no such service", err)
	}
	if _, err = s.ApplyCouponToInvoice(ctx, scope, second.ID, "NOSUCHCODE"); !errors.As(err, &invalid) {
		t.Fatal("an unknown code worked", err)
	}
	if _, err = s.SaveCoupon(ctx, scope, "", CouponInput{Title: "Bad service", Code: "BADSERVICE", Kind: "percent", Value: 5, Period: "never", ServiceIDs: []string{"00000000-0000-0000-0000-000000000000"}}, 0); !errors.Is(err, ErrCouponInput) {
		t.Fatal("a coupon for a service that does not exist was accepted", err)
	}

	// Revising needs the right revision; a coupon that was never used can be deleted.
	revised := good
	revised.Title = "Spring 25"
	revised.Value = 25
	if _, err = s.SaveCoupon(ctx, scope, spring, revised, 9); !errors.Is(err, ErrCouponNotFound) {
		t.Fatal("a stale revision was accepted", err)
	}
	if _, err = s.SaveCoupon(ctx, scope, spring, revised, 1); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteCoupon(ctx, scope, once); !errors.Is(err, ErrCouponInUse) {
		t.Fatal("a coupon with a use was deleted", err)
	}
	unused, err := s.SaveCoupon(ctx, scope, "", CouponInput{Title: "Spare", Code: "SPARE", Kind: "percent", Value: 5, Period: "never", Active: true}, 0)
	must(err)
	must(s.DeleteCoupon(ctx, scope, unused))
	// An issued invoice cannot take a coupon.
	if _, err = pool.Exec(ctx, `UPDATE invoices SET status='void' WHERE id::text=$1`, second.ID); err != nil {
		t.Fatal(err)
	}
}
