package workspace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"dapper.local/platform/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// alertSender fails for one address and records the others.
type alertSender struct {
	mu   sync.Mutex
	fail string
	sent []string
}

func (a *alertSender) SendEmail(_ context.Context, to, _, _, _ string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if to == a.fail {
		return "", errors.New("synthetic mailbox refused")
	}
	a.sent = append(a.sent, to)
	return "ok", nil
}

// error404 #128: a website booking's business alert goes out even when the customer's own email fails, and each is logged apart.
func TestBookingAlertIndependentOfCustomerEmail(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_alert_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE") }()
	parsed, _ := url.Parse(dsn)
	q := parsed.Query()
	q.Set("search_path", namespace)
	parsed.RawQuery = q.Encode()
	store, err := postgres.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	config := admin.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var org, loc, customer, booking string
	must(db.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Alert Org') RETURNING id::text`).Scan(&org))
	must(db.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic alert site') RETURNING id::text`, org).Scan(&loc))
	_, err = db.Exec(ctx, `INSERT INTO public_sites(organization_id,location_id,slug,name,published) VALUES($1,$2,'dapper-detailing-pros-peterborough','Synthetic site',true)`, org, loc)
	must(err)
	_, err = db.Exec(ctx, `INSERT INTO messaging_settings(organization_id,smtp_from_address,smtp_notify_email) VALUES($1,'shop@example.invalid','owner@example.invalid')`, org)
	must(err)
	must(db.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic Walk','bounce@example.invalid','') RETURNING id::text`, org, loc).Scan(&customer))
	must(db.QueryRow(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,'Synthetic Detail','online','pending',now()+interval '2 days') RETURNING id::text`, org, loc, customer).Scan(&booking))

	sender := &alertSender{fail: "bounce@example.invalid"}
	sendBookingEmail(store, sender, "https://example.invalid", booking, "booking_confirmation")
	if len(sender.sent) != 1 || sender.sent[0] != "owner@example.invalid" {
		t.Fatalf("the business alert did not go out when the customer's email failed: %v", sender.sent)
	}
	var customerDone, alertDone bool
	must(db.QueryRow(ctx, `SELECT bool_or(kind='booking_confirmation' AND delivered_at IS NOT NULL), bool_or(kind='admin:booking_confirmation' AND delivered_at IS NOT NULL) FROM booking_email_log WHERE booking_id=$1`, booking).Scan(&customerDone, &alertDone))
	if customerDone || !alertDone {
		t.Fatalf("log wrong: customer delivered %v, alert delivered %v", customerDone, alertDone)
	}
	// Reporting the same booking again within two minutes sends nothing twice.
	sendBookingEmail(store, sender, "https://example.invalid", booking, "booking_confirmation")
	if len(sender.sent) != 1 {
		t.Fatalf("the alert was sent twice: %v", sender.sent)
	}
}
