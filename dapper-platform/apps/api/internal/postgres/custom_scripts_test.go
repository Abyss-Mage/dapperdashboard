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

func TestCustomScripts(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_cs_%d", time.Now().UnixNano())
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
	var org, owner, other string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Script Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:script-owner') RETURNING id::text`).Scan(&owner))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:script-other') RETURNING id::text`).Scan(&other))
	_, err = pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage')`, owner, org)
	must(err)

	snippet := CustomScript{Name: "  Chat   widget ", Placement: "body", Code: "<script>window.x=1</script>", Paths: []string{"/", " /services ", ""}, Active: true}
	if _, err = s.SaveCustomScript(ctx, other, org, snippet); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access management added a script", err)
	}
	if _, err = s.CustomScripts(ctx, other, org); !errors.Is(err, ErrForbidden) {
		t.Fatal("scripts listed for a member without access", err)
	}
	for _, bad := range []CustomScript{
		{Name: "", Placement: "head", Code: "x"}, {Name: "a", Placement: "footer", Code: "x"}, {Name: "a", Placement: "head", Code: "  "},
		{Name: "a", Placement: "head", Code: "x", Paths: []string{"services"}}, {Name: "a", Placement: "head", Code: "x", Paths: []string{"//evil.example"}},
	} {
		if _, err = s.SaveCustomScript(ctx, owner, org, bad); !errors.Is(err, ErrCustomScriptInput) {
			t.Fatal("bad script accepted", bad, err)
		}
	}
	saved, err := s.SaveCustomScript(ctx, owner, org, snippet)
	must(err)
	if saved.ID == "" || saved.Name != "Chat widget" || len(saved.Paths) != 2 || saved.Paths[1] != "/services" {
		t.Fatalf("script wrong: %+v", saved)
	}
	off := saved
	off.Active, off.Code = false, "<script>window.x=2</script>"
	if _, err = s.SaveCustomScript(ctx, owner, org, off); err != nil {
		t.Fatal(err)
	}
	if pub, e := s.PublicCustomScripts(ctx); e != nil || len(pub) != 0 {
		t.Fatal("a switched-off script is public", pub, e)
	}
	off.Active = true
	if _, err = s.SaveCustomScript(ctx, owner, org, off); err != nil {
		t.Fatal(err)
	}
	if pub, e := s.PublicCustomScripts(ctx); e != nil || len(pub) != 1 || pub[0].Code != off.Code {
		t.Fatal("active script not public", pub, e)
	}
	if err = s.DeleteCustomScript(ctx, other, org, saved.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("deleted without access", err)
	}
	must(s.DeleteCustomScript(ctx, owner, org, saved.ID))
	if err = s.DeleteCustomScript(ctx, owner, org, saved.ID); !errors.Is(err, ErrCustomScriptNotFound) {
		t.Fatal("deleted twice", err)
	}
	list, err := s.CustomScripts(ctx, owner, org)
	if err != nil || len(list) != 0 {
		t.Fatal("scripts remain", list, err)
	}
}
