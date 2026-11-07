package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSOPManage(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_sop_%d", time.Now().UnixNano())
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
	var org, loc, admin2, stranger string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic SOP Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic SOP site') RETURNING id::text`, org).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:sop-admin') RETURNING id::text`).Scan(&admin2))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:sop-stranger') RETURNING id::text`).Scan(&stranger))
	for _, action := range []string{"sop.read", "sop.manage"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, admin2, org, loc, action)
		must(err)
	}
	scope := Scope{ActorID: admin2, OrganizationID: org, LocationID: loc}
	pdf := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Page >>\nendobj\n2 0 obj\n<< /Type /Page >>\nendobj\n%%EOF\n")

	if _, err = s.AddSOPDocument(ctx, scope, "Safety rules", "safety", pdf); !errors.Is(err, ErrSOPStorageOff) {
		t.Fatal("added a document with no storage folder", err)
	}
	s.SetSOPDirectory(t.TempDir())
	if _, err = s.AddSOPDocument(ctx, Scope{ActorID: stranger, OrganizationID: org, LocationID: loc}, "Safety rules", "safety", pdf); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access added a document", err)
	}
	for _, bad := range []struct {
		title, category string
		data            []byte
	}{{"ab", "safety", pdf}, {"Safety rules", "other", pdf}, {"Safety rules", "safety", []byte("not a pdf at all")}} {
		if _, err = s.AddSOPDocument(ctx, scope, bad.title, bad.category, bad.data); !errors.Is(err, ErrSOPFileInput) {
			t.Fatal("bad document accepted", bad.title, err)
		}
	}
	doc, err := s.AddSOPDocument(ctx, scope, "  Safety   rules ", "safety", pdf)
	must(err)
	if doc.Title != "Safety rules" || doc.PageCount != 2 || doc.Archived {
		t.Fatalf("document wrong: %+v", doc)
	}
	again, err := s.AddSOPDocument(ctx, scope, "Safety rules", "safety", pdf)
	must(err)
	if again.ID != doc.ID {
		t.Fatal("the same PDF was registered twice")
	}
	if data, _, e := s.SOPDocumentFile(ctx, scope, doc.ID); e != nil || !bytes.Equal(data, pdf) {
		t.Fatal("file not served back", e)
	}
	list, err := s.ListSOPDocuments(ctx, scope)
	must(err)
	if len(list) != 1 {
		t.Fatal("document missing from list", list)
	}
	if err = s.RemoveSOPDocument(ctx, Scope{ActorID: stranger, OrganizationID: org, LocationID: loc}, doc.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access removed a document", err)
	}
	must(s.RemoveSOPDocument(ctx, scope, doc.ID))
	if list, err = s.ListSOPDocuments(ctx, scope); err != nil || len(list) != 0 {
		t.Fatal("removed document still listed", list, err)
	}
	if err = s.RemoveSOPDocument(ctx, scope, doc.ID); !errors.Is(err, ErrSOPDocumentNotFound) {
		t.Fatal("removed twice", err)
	}
	back, err := s.AddSOPDocument(ctx, scope, "Safety rules v2", "procedure", pdf)
	must(err)
	if back.ID != doc.ID || back.Archived || back.Title != "Safety rules v2" {
		t.Fatalf("re-adding did not restore the document: %+v", back)
	}
}
