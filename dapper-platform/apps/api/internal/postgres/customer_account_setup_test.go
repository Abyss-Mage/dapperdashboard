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

// error404 #85: a customer booked by staff gets an account linked to their record and a one-time link that sets their password.
func TestCustomerAccountSetup(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_setup_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.pool.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE") }()
	config := admin.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	defer s.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Migrate(ctx))
	auth, err := NewPasswordAuthenticator(s)
	must(err)
	var org, loc, staff, customer, noEmail string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Setup Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic setup site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:setup-staff') RETURNING id::text`).Scan(&staff))
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Sam Setup','Sam.Setup@Example.invalid','') RETURNING id::text`, org, loc).Scan(&customer))
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'No Mail','','5550001') RETURNING id::text`, org, loc).Scan(&noEmail))

	if _, _, _, _, err = s.PrepareCustomerAccount(ctx, org, loc, staff, noEmail); !errors.Is(err, ErrNoCustomerEmail) {
		t.Fatal("a customer without email got an account", err)
	}
	principal, email, name, fresh, err := s.PrepareCustomerAccount(ctx, org, loc, staff, customer)
	must(err)
	if !fresh || email != "sam.setup@example.invalid" || name != "Sam Setup" {
		t.Fatalf("new account not prepared: %v %q %q", fresh, email, name)
	}
	must(auth.SetPassword(ctx, principal, email, "unknown-random-secret-value"))
	again, _, _, fresh2, err := s.PrepareCustomerAccount(ctx, org, loc, staff, customer)
	must(err)
	if fresh2 || again != principal {
		t.Fatal("a second setup made another account", again, principal)
	}
	var links int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM customer_account_links WHERE customer_id=$1 AND principal_id=$2`, customer, principal).Scan(&links))
	if links != 1 {
		t.Fatal("account not linked to the customer once", links)
	}
	token, err := auth.AccountSetupToken(ctx, principal)
	must(err)
	var hours float64
	must(pool.QueryRow(ctx, `SELECT extract(epoch FROM max(expires_at)-now())/3600 FROM password_reset_tokens WHERE principal_id=$1`, principal).Scan(&hours))
	if hours < 71 || hours > 73 {
		t.Fatal("setup link should last 72 hours", hours)
	}
	must(auth.ConfirmPasswordReset(ctx, token, "chosen-by-the-customer-2026"))
	if _, err = auth.SignIn(ctx, email, "chosen-by-the-customer-2026", time.Hour); err != nil {
		t.Fatal("the customer cannot sign in with the password they chose", err)
	}
	if err = auth.ConfirmPasswordReset(ctx, token, "another-password-entirely"); err == nil {
		t.Fatal("a setup link worked twice")
	}
	// The same person booked at another franchise: their existing account is linked, not duplicated.
	var loc2, other string
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic second site') RETURNING id::text`, org).Scan(&loc2))
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Sam Again',' sam.setup@example.invalid ','') RETURNING id::text`, org, loc2).Scan(&other))
	same, _, _, fresh3, err := s.PrepareCustomerAccount(ctx, org, loc2, staff, other)
	must(err)
	if fresh3 || same != principal {
		t.Fatal("an existing account was duplicated", same, principal)
	}
}
