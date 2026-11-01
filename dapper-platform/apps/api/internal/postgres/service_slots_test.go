package postgres

import (
	"dapper.local/platform/internal/scheduling"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestServiceSlots(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_slots_%d", time.Now().UnixNano())
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
	var org, loc, staff, a, b string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Slots Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic slots site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:slots-staff') RETURNING id::text`).Scan(&staff))
	for _, action := range []string{"scheduling.manage", "scheduling.reserve", "scheduling.release", "scheduling.read"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, staff, org, loc, action)
		must(err)
	}
	scope := Scope{ActorID: staff, OrganizationID: org, LocationID: loc}
	must(pool.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind) VALUES($1,$2,'Synthetic slots service A','main') RETURNING id::text`, org, loc).Scan(&a))
	must(pool.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind) VALUES($1,$2,'Synthetic slots service B','main') RETURNING id::text`, org, loc).Scan(&b))
	start := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	resource, err := s.CreateSchedulingResource(ctx, scope, SchedulingResource{Name: "Synthetic slots calendar", Timezone: "UTC", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}}}})
	must(err)
	ruleA, err := s.SaveSchedulingRule(ctx, scope, SchedulingRule{ResourceID: resource.ID, ServiceID: a, DurationMinutes: 60, Units: 1, Slots: 1})
	must(err)
	ruleB, err := s.SaveSchedulingRule(ctx, scope, SchedulingRule{ResourceID: resource.ID, ServiceID: b, DurationMinutes: 60, Units: 1, Slots: 2})
	must(err)
	if ruleA.Slots != 1 || ruleB.Slots != 2 {
		t.Fatal("slots not kept", ruleA.Slots, ruleB.Slots)
	}
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, ruleA.ID, ruleA.Version, start); err != nil {
		t.Fatal("first booking of service A refused", err)
	}
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, ruleA.ID, ruleA.Version, start); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("service A with one slot took a second booking at the same time", err)
	}
	for i := 0; i < 2; i++ {
		if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, ruleB.ID, ruleB.Version, start); err != nil {
			t.Fatal("service B was blocked by service A or its own two slots", i, err)
		}
	}
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, ruleB.ID, ruleB.Version, start); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("service B with two slots took a third booking at the same time", err)
	}
	first := start.Format("2006-01-02")
	availA, err := s.RuleAvailability(ctx, scope, resource.ID, ruleA.ID, ruleA.Version, first, 1, true)
	must(err)
	for _, st := range availA.Days[0].Starts {
		if st.Equal(start) {
			t.Fatal("a full time was still offered for service A")
		}
	}
	if len(availA.Days[0].Starts) == 0 {
		t.Fatal("no later time offered for service A")
	}
	// a later time is still free for A
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, ruleA.ID, ruleA.Version, start.Add(2*time.Hour)); err != nil {
		t.Fatal("a free time for service A was refused", err)
	}
}
