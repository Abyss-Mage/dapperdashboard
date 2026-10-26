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

func TestPeopleRosterShowsOnboardingState(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_roster_%d", time.Now().UnixNano())
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
	id := func(query string, args ...any) string {
		t.Helper()
		var v string
		must(pool.QueryRow(ctx, query, args...).Scan(&v))
		return v
	}
	org := id(`INSERT INTO organizations(name) VALUES('Synthetic Roster Org') RETURNING id::text`)
	boss := id(`INSERT INTO principals(external_subject) VALUES('synthetic:roster-boss') RETURNING id::text`)
	ready := id(`INSERT INTO principals(external_subject) VALUES('synthetic:roster-ready') RETURNING id::text`)
	partial := id(`INSERT INTO principals(external_subject) VALUES('synthetic:roster-partial') RETURNING id::text`)
	loose := id(`INSERT INTO principals(external_subject) VALUES('synthetic:roster-loose') RETURNING id::text`)
	outsider := id(`INSERT INTO principals(external_subject) VALUES('synthetic:roster-outsider') RETURNING id::text`)
	for _, p := range []string{boss, ready, partial, loose} {
		_, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, p)
		must(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'access.manage')`, org, boss)
	must(err)
	site := id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic roster site') RETURNING id::text`, org)
	for _, p := range []string{ready, partial} {
		_, err = pool.Exec(ctx, `INSERT INTO employee_worksite_assignments(principal_id,organization_id,location_id,timezone,assigned_by) VALUES($1,$2,$3,'America/Toronto',$4)`, p, org, site, boss)
		must(err)
	}
	doc := id(`INSERT INTO sop_documents(organization_id,title,category,file_name,sha256_hash,page_count) VALUES($1,'Synthetic policy','onboarding','synthetic.pdf',repeat('a',64),2) RETURNING id::text`, org)
	_, err = pool.Exec(ctx, `INSERT INTO member_profiles(principal_id,full_name,phone) VALUES($1,'Ready Person','+14165550101')`, ready)
	must(err)
	_, err = pool.Exec(ctx, `INSERT INTO sop_acknowledgments(organization_id,location_id,sop_document_id,employee_id,actor_id,acknowledged_sha256,training_track) VALUES($1,$2,$3,$4,$4,repeat('a',64),$5)`, org, site, doc, ready, employeeAckTrack)
	must(err)

	people, err := s.PeopleRoster(ctx, boss, org)
	must(err)
	byID := map[string]PersonRow{}
	for _, p := range people {
		byID[p.PrincipalID] = p
	}
	if len(people) != 4 {
		t.Fatalf("expected the four members, got %d", len(people))
	}
	if got := byID[ready]; got.OnboardingStatus != "complete" || got.DocumentsAcked != 1 || got.DocumentsTotal != 1 || got.LocationName != "Synthetic roster site" {
		t.Fatalf("fully onboarded employee misreported: %+v", got)
	}
	if got := byID[partial]; got.OnboardingStatus != "incomplete" || len(got.Missing) != 2 || got.Missing[0] != "details" || got.Missing[1] != "documents" {
		t.Fatalf("employee missing details and documents misreported: %+v", got)
	}
	if got := byID[loose]; got.OnboardingStatus != "not_assigned" || got.Missing[0] != "franchise" {
		t.Fatalf("unassigned member misreported: %+v", got)
	}
	if !byID[boss].Administrator || byID[ready].Administrator {
		t.Fatal("administrator flag wrong")
	}
	if _, err = s.PeopleRoster(ctx, outsider, org); !errors.Is(err, ErrForbidden) {
		t.Fatal("a non-administrator could read the roster", err)
	}
	if _, err = s.PeopleRoster(ctx, ready, org); !errors.Is(err, ErrForbidden) {
		t.Fatal("an employee could read the roster", err)
	}
}
