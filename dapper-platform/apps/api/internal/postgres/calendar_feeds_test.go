package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestICSHelpers(t *testing.T) {
	if got := icsEscape("a,b;c\\d\ne"); got != `a\,b\;c\\d\ne` {
		t.Fatalf("escape wrong: %q", got)
	}
	long := strings.Repeat("é", 100)
	folded := icsFold("SUMMARY:" + long)
	for _, part := range strings.Split(folded, "\r\n") {
		if len(part) > 76 || !utf8.ValidString(part) {
			t.Fatalf("a folded line is too long or split a character: %d", len(part))
		}
	}
	if strings.ReplaceAll(strings.ReplaceAll(folded, "\r\n ", ""), "\r\n", "") != "SUMMARY:"+long {
		t.Fatal("folding changed the text")
	}
}

func TestCalendarFeeds(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_feed_%d", time.Now().UnixNano())
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
	var org, loc, manager, stranger, customer string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Feed Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic feed site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:feed-manager') RETURNING id::text`).Scan(&manager))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:feed-stranger') RETURNING id::text`).Scan(&stranger))
	_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'bookings.read')`, manager, org, loc)
	must(err)
	must(pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic, Customer','','') RETURNING id::text`, org, loc).Scan(&customer))
	for i, status := range []string{"confirmed", "cancelled"} {
		_, err = pool.Exec(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_name,channel,status,booking_time,service_address) VALUES($1,$2,$3,$4,'internal',$5,now()+interval '2 days','12 Test Road, Peterborough')`, org, loc, customer, fmt.Sprintf("Synthetic service %d", i), status)
		must(err)
	}

	managerScope := Scope{ActorID: manager, OrganizationID: org, LocationID: loc}
	strangerScope := Scope{ActorID: stranger, OrganizationID: org, LocationID: loc}
	if _, _, err = s.CreateFranchiseCalendarFeed(ctx, strangerScope, "x"); !errors.Is(err, ErrForbidden) {
		t.Fatal("someone without bookings.read made a calendar link", err)
	}
	if _, _, err = s.CreateFranchiseCalendarFeed(ctx, managerScope, strings.Repeat("x", 81)); !errors.Is(err, ErrCalendarFeedInput) {
		t.Fatal("an over-long name was accepted", err)
	}
	feed, token, err := s.CreateFranchiseCalendarFeed(ctx, managerScope, "Front desk")
	must(err)
	if feed.Kind != "franchise" || len(token) != 43 {
		t.Fatalf("unexpected feed %+v token length %d", feed, len(token))
	}
	var stored int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM calendar_feeds WHERE encode(token_hash,'escape') LIKE '%'||$1||'%'`, token).Scan(&stored))
	if stored != 0 {
		t.Fatal("the token itself was stored")
	}
	ics, err := s.CalendarFeedICS(ctx, token)
	must(err)
	for _, want := range []string{"BEGIN:VCALENDAR", "X-WR-CALNAME:Front desk", "SUMMARY:Synthetic service 0 · Synthetic\\, Customer", "LOCATION:12 Test Road\\, Peterborough", "END:VCALENDAR"} {
		if !strings.Contains(ics, want) {
			t.Fatalf("calendar is missing %q:\n%s", want, ics)
		}
	}
	if strings.Contains(ics, "Synthetic service 1") {
		t.Fatal("a cancelled booking is in the calendar")
	}
	if _, err = s.CalendarFeedICS(ctx, "not-a-token"); !errors.Is(err, ErrCalendarFeedNotFound) {
		t.Fatal("a bad token was accepted", err)
	}
	listed, err := s.FranchiseCalendarFeeds(ctx, managerScope)
	if err != nil || len(listed) != 1 {
		t.Fatal("link not listed", listed, err)
	}
	if _, err = s.FranchiseCalendarFeeds(ctx, strangerScope); !errors.Is(err, ErrForbidden) {
		t.Fatal("someone without bookings.read listed links", err)
	}
	must(s.RevokeFranchiseCalendarFeed(ctx, managerScope, feed.ID))
	if _, err = s.CalendarFeedICS(ctx, token); !errors.Is(err, ErrCalendarFeedNotFound) {
		t.Fatal("a revoked link still works", err)
	}
	if err = s.RevokeFranchiseCalendarFeed(ctx, managerScope, feed.ID); !errors.Is(err, ErrCalendarFeedNotFound) {
		t.Fatal("revoking twice should say it is gone", err)
	}
}
