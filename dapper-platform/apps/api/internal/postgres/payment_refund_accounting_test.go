package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRefundAccountingCandidateMigration(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test DB unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("codex_refund_candidate_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, e := admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	config := admin.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := &Store{pool: pool}
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile("payment_refund_accounting.sql")
	if err != nil {
		t.Fatal(err)
	}
	var columnCount, triggerCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name='payments' AND column_name='refunded_cents'`, namespace).Scan(&columnCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_trigger WHERE tgname='payment_refund_adjustment_immutable' AND tgrelid='payment_refund_adjustments'::regclass`).Scan(&triggerCount); err != nil {
		t.Fatal(err)
	}
	if columnCount != 1 || triggerCount != 1 {
		t.Fatal("candidate schema incomplete")
	}
	repeat, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repeat.Exec(ctx, string(sql)); err == nil {
		t.Fatal("duplicate candidate unexpectedly applied")
	}
	if err = repeat.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var payment, org, loc string
	err = pool.QueryRow(ctx, `WITH o AS (INSERT INTO organizations(name) VALUES('Synthetic Refund Org') RETURNING id),l AS (INSERT INTO locations(organization_id,name) SELECT id,'Synthetic Bay' FROM o RETURNING id,organization_id),a AS (INSERT INTO principals(external_subject) VALUES('synthetic-refund-principal') RETURNING id),c AS (INSERT INTO customers(organization_id,location_id,name,email) SELECT organization_id,id,'Synthetic Customer','synthetic@example.test' FROM l RETURNING id,organization_id,location_id),i AS (INSERT INTO invoices(organization_id,location_id,customer_id,invoice_number,status,subtotal_cents,tax_cents,total_cents,due_cents) SELECT organization_id,location_id,id,'SYN-REF-001','draft',5000,0,5000,5000 FROM c RETURNING id,organization_id,location_id,customer_id) INSERT INTO payments(organization_id,location_id,invoice_id,customer_id,actor_id,amount_cents,method,status) SELECT i.organization_id,i.location_id,i.id,i.customer_id,a.id,5000,'square_card','completed' FROM i CROSS JOIN a RETURNING id::text,organization_id::text,location_id::text`).Scan(&payment, &org, &loc)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []int64{-1, 5001} {
		if _, err = pool.Exec(ctx, `UPDATE payments SET refunded_cents=$1 WHERE id=$2`, invalid, payment); err == nil {
			t.Fatal("accepted invalid cumulative amount")
		}
	}
	var adjustment string
	if err = pool.QueryRow(ctx, `INSERT INTO payment_refund_adjustments(organization_id,location_id,payment_id,captured_cents,previous_refunded_cents,refunded_cents,delta_cents) VALUES($1,$2,$3,5000,0,1000,1000) RETURNING id::text`, org, loc, payment).Scan(&adjustment); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE payment_refund_adjustments SET delta_cents=2000 WHERE id=$1`, adjustment); err == nil {
		t.Fatal("adjustment mutable")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM payment_refund_adjustments WHERE id=$1`, adjustment); err == nil {
		t.Fatal("adjustment deletable")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO payment_refund_adjustments(organization_id,location_id,payment_id,captured_cents,previous_refunded_cents,refunded_cents,delta_cents) VALUES($1,$2,$3,5000,0,1000,1000)`, org, loc, payment); err == nil {
		t.Fatal("duplicate cumulative adjustment accepted")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO payment_refund_adjustments(organization_id,location_id,payment_id,captured_cents,previous_refunded_cents,refunded_cents,delta_cents) VALUES($1,$2,$3,5000,1000,6000,5000)`, org, loc, payment); err == nil {
		t.Fatal("history exceeded captured amount")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO payment_refund_adjustments(organization_id,location_id,payment_id,captured_cents,previous_refunded_cents,refunded_cents,delta_cents) VALUES($1,gen_random_uuid(),$2,5000,1000,2000,1000)`, org, payment); err == nil {
		t.Fatal("cross-location adjustment accepted")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO payment_refund_adjustments(organization_id,location_id,payment_id,captured_cents,previous_refunded_cents,refunded_cents,delta_cents) VALUES($1,$2,$3,6000,1000,2000,1000)`, org, loc, payment); err == nil {
		t.Fatal("forged captured amount accepted")
	}
	if _, err = pool.Exec(ctx, `UPDATE payments SET status='refunded' WHERE id=$1`, payment); err != nil {
		t.Fatal(err)
	}
	legacy, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, legacyErr := legacy.Exec(ctx, string(sql))
	_ = legacy.Rollback(ctx)
	if legacyErr == nil || !strings.Contains(legacyErr.Error(), "Reviewed historical refund reconciliation required") {
		t.Fatalf("ambiguous legacy refund not refused: %v", legacyErr)
	}
	if err = store.ValidateSchema(ctx); err != nil {
		t.Fatal("registered ledger changed after candidate tests")
	}
	if err = store.ValidateSchema(ctx); err != nil {
		t.Fatal("candidate changed registered migration ledger")
	}
}
