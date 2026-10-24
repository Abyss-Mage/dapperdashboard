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

// TestSquareSandboxEndToEnd runs OUR payment flow (binding -> prepare -> claim -> provider charge ->
// capture observation -> invoice allocation) against the real Square SANDBOX in a disposable schema.
// It is skipped unless scripts/verify-square-sandbox.ps1 supplies the sandbox token and IDs.
func TestSquareSandboxEndToEnd(t *testing.T) {
	dsn, token, merchant, location := os.Getenv("DAPPER_TEST_DATABASE_URL"), os.Getenv("SQUARE_SANDBOX_TOKEN"), os.Getenv("SQUARE_SANDBOX_MERCHANT_ID"), os.Getenv("SQUARE_SANDBOX_LOCATION_ID")
	if dsn == "" || token == "" || merchant == "" || location == "" {
		t.Skip("real PostgreSQL and Square sandbox configuration required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_sqsandbox_%d", time.Now().UnixNano())
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
	store := &Store{pool: pool}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var actor, org, loc, customer, invoice string
	err = pool.QueryRow(ctx, `WITH o AS (INSERT INTO organizations(name) VALUES('Synthetic Sandbox Org') RETURNING id),l AS (INSERT INTO locations(organization_id,name) SELECT id,'Synthetic Sandbox Bay' FROM o RETURNING id,organization_id),a AS (INSERT INTO principals(external_subject) VALUES('synthetic-sandbox-principal') RETURNING id),c AS (INSERT INTO customers(organization_id,location_id,name,email) SELECT organization_id,id,'Synthetic Customer','synthetic@example.test' FROM l RETURNING id,organization_id,location_id),i AS (INSERT INTO invoices(organization_id,location_id,customer_id,invoice_number,status,subtotal_cents,tax_cents,total_cents,due_cents) SELECT organization_id,location_id,id,'SYN-SBX-001','draft',5000,0,5000,5000 FROM c RETURNING id,organization_id,location_id,customer_id) SELECT a.id::text,i.organization_id::text,i.location_id::text,i.customer_id::text,i.id::text FROM i CROSS JOIN a`).Scan(&actor, &org, &loc, &customer, &invoice)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"payments.record", "invoices.read"} {
		if _, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, actor, org, loc, action); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO invoice_items(invoice_id,organization_id,location_id,description,quantity,unit_price_cents,total_price_cents) VALUES($1,$2,$3,'Synthetic sandbox item',1,5000,5000)`, invoice, org, loc); err != nil {
		t.Fatal(err)
	}
	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: actor}
	issueInvoiceForTest(t, ctx, store, scope, Invoice{ID: invoice})
	if _, err = pool.Exec(ctx, `INSERT INTO square_merchant_binding_revisions(organization_id,location_id,version,environment,merchant_id,square_location_id,credential_reference,reviewed_by) VALUES($1,$2,1,'sandbox',$3,$4,'sandbox-default',$5)`, org, loc, merchant, location, actor); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewSquareCredentialResolver(map[string]SquareCredentialConfig{"sandbox-default": {Environment: "sandbox", AccessToken: token}})
	if err != nil || resolver == nil {
		t.Fatal("resolver rejected sandbox configuration")
	}

	// 1. A real charge settles through our own authority.
	good, err := store.PrepareSquareInvoiceIntent(ctx, scope, PrepareSquareInvoiceIntentInput{InvoiceID: invoice, BindingVersion: 1, RequestKey: "sandbox-good-0001-request", AmountCents: 150})
	if err != nil {
		t.Fatal(err)
	}
	allocation, err := store.ExecuteSquareInvoiceIntent(ctx, scope, good.ID, "cnon:card-nonce-ok", resolver)
	if err != nil || allocation.PaymentID == "" || allocation.InvoiceID != invoice {
		t.Fatalf("sandbox charge did not settle: %+v %v", allocation, err)
	}
	// 2. Replaying the same intent never charges twice.
	again, err := store.ExecuteSquareInvoiceIntent(ctx, scope, good.ID, "cnon:card-nonce-ok", resolver)
	if err != nil || again.PaymentID != allocation.PaymentID {
		t.Fatalf("replay changed the receipt: %+v %v", again, err)
	}
	var payments int
	var paid int64
	var state string
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM payments),(SELECT paid_cents FROM invoices WHERE id=$1),(SELECT state FROM square_payment_intents WHERE id=$2)`, invoice, good.ID).Scan(&payments, &paid, &state); err != nil || payments != 1 || paid != 150 || state != "settled" {
		t.Fatalf("ledger after sandbox charge: payments=%d paid=%d state=%s %v", payments, paid, state, err)
	}
	// 3. A declined test card never reaches the ledger.
	bad, err := store.PrepareSquareInvoiceIntent(ctx, scope, PrepareSquareInvoiceIntentInput{InvoiceID: invoice, BindingVersion: 1, RequestKey: "sandbox-decline-0002-key", AmountCents: 150})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ExecuteSquareInvoiceIntent(ctx, scope, bad.ID, "cnon:card-nonce-declined", resolver); !errors.Is(err, ErrPaymentProvider) {
		t.Fatalf("declined card was not refused: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM payments),(SELECT paid_cents FROM invoices WHERE id=$1)`, invoice).Scan(&payments, &paid); err != nil || payments != 1 || paid != 150 {
		t.Fatalf("declined card changed the ledger: payments=%d paid=%d %v", payments, paid, err)
	}
	// 4. A wrong merchant binding is refused before any charge is attempted.
	if _, err = pool.Exec(ctx, `INSERT INTO square_merchant_binding_revisions(organization_id,location_id,version,environment,merchant_id,square_location_id,credential_reference,reviewed_by) VALUES($1,$2,2,'sandbox','WRONGMERCHANT',$3,'sandbox-default',$4)`, org, loc, location, actor); err != nil {
		t.Fatal(err)
	}
	wrong, err := store.PrepareSquareInvoiceIntent(ctx, scope, PrepareSquareInvoiceIntentInput{InvoiceID: invoice, BindingVersion: 2, RequestKey: "sandbox-wrongmerchant-03", AmountCents: 150})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ExecuteSquareInvoiceIntent(ctx, scope, wrong.ID, "cnon:card-nonce-ok", resolver); !errors.Is(err, ErrPaymentProvider) {
		t.Fatalf("wrong merchant was charged: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payments`).Scan(&payments); err != nil || payments != 1 {
		t.Fatalf("wrong merchant reached ledger: %d %v", payments, err)
	}
}
