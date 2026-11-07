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

func TestBackdatedBooking(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_past_%d", time.Now().UnixNano())
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
	var org, loc, staff, svc string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Past Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic past site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:past-staff') RETURNING id::text`).Scan(&staff))
	for _, action := range []string{"scheduling.manage", "scheduling.reserve", "scheduling.release", "scheduling.read"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, staff, org, loc, action)
		must(err)
	}
	scope := Scope{ActorID: staff, OrganizationID: org, LocationID: loc}
	must(pool.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind) VALUES($1,$2,'Synthetic past service','main') RETURNING id::text`, org, loc).Scan(&svc))
	resource, err := s.CreateSchedulingResource(ctx, scope, SchedulingResource{Name: "Synthetic past calendar", Timezone: "UTC", Plan: scheduling.Plan{Capacity: 1}})
	must(err)
	var week []HourWindow
	for d := 0; d < 7; d++ {
		week = append(week, HourWindow{Day: d, Start: "08:00", End: "17:00"})
	}
	must(s.SaveWeeklyHours(ctx, scope, resource.ID, week))
	rule, err := s.SaveSchedulingRule(ctx, scope, SchedulingRule{ResourceID: resource.ID, ServiceID: svc, DurationMinutes: 60, Units: 1, Slots: 1})
	must(err)

	day := time.Now().UTC().AddDate(0, -3, -4).Format("2006-01-02")
	none, err := s.RuleAvailability(ctx, scope, resource.ID, rule.ID, rule.Version, day, 1)
	must(err)
	if len(none.Days[0].Starts) != 0 {
		t.Fatal("a past day offered times to someone who did not ask for them", none.Days[0].Starts)
	}
	past, err := s.RuleAvailability(ctx, scope, resource.ID, rule.ID, rule.Version, day, 1, true)
	must(err)
	if len(past.Days[0].Starts) < 8 {
		t.Fatal("a past day inside opening hours offered no times", past.Days[0].Starts)
	}
	at := past.Days[0].Starts[2]
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, rule.ID, rule.Version, at); !errors.Is(err, ErrForbidden) {
		t.Fatal("someone who cannot manage bookings entered a booking in the past", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'bookings.manage')`, staff, org, loc)
	must(err)
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, rule.ID, rule.Version, at); err != nil {
		t.Fatal("an administrator could not enter a booking in the past", err)
	}
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, rule.ID, rule.Version, at); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("the past slot took a second booking though it has one slot", err)
	}
	// outside opening hours in the past is still refused
	if _, err = s.ReserveServiceCapacity(ctx, scope, resource.ID, rule.ID, rule.Version, at.Add(12*time.Hour)); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("a past time outside opening hours was accepted", err)
	}
}
