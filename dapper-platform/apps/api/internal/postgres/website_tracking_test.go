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

func TestWebsiteTrackingSettings(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_track_%d", time.Now().UnixNano())
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
	var org, manager, stranger string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Tracking Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:track-manager') RETURNING id::text`).Scan(&manager))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:track-stranger') RETURNING id::text`).Scan(&stranger))
	for _, p := range []string{manager, stranger} {
		_, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, p)
		must(err)
	}
	var role string
	must(pool.QueryRow(ctx, `INSERT INTO roles(organization_id,name,description) VALUES($1,'Synthetic Content','x') RETURNING id::text`, org).Scan(&role))
	_, err = pool.Exec(ctx, `INSERT INTO role_capabilities(role_id,action) VALUES($1,'content.manage')`, role)
	must(err)
	_, err = pool.Exec(ctx, `INSERT INTO member_role_assignments(organization_id,principal_id,role_id,scope_type,scope_id,assigned_by) VALUES($1,$2,$3,'organization',$1,$2)`, org, manager, role)
	must(err)

	if _, err = s.GetWebsiteTracking(ctx, stranger, org); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without content.manage read the tracking settings", err)
	}
	if _, err = s.SaveWebsiteTracking(ctx, stranger, org, WebsiteTracking{GTMID: "GTM-ABC123"}, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without content.manage saved tracking settings", err)
	}
	if got, err := s.GetWebsiteTracking(ctx, manager, org); err != nil || got.Version != 0 || got.GTMID != "" {
		t.Fatal("nothing saved yet should read as empty", got, err)
	}
	for _, bad := range []WebsiteTracking{{GTMID: "UA-12345"}, {MetaPixelID: "12ab"}, {MailchimpScriptURL: "http://chimpstatic.com/x.js"}, {MailchimpScriptURL: "https://evil.example/x.js"}, {MailchimpScriptURL: "https://chimpstatic.com.evil.example/x.js"}} {
		if _, err = s.SaveWebsiteTracking(ctx, manager, org, bad, 0); !errors.Is(err, ErrTrackingInput) {
			t.Fatalf("invalid tracking accepted: %+v %v", bad, err)
		}
	}
	saved, err := s.SaveWebsiteTracking(ctx, manager, org, WebsiteTracking{GTMID: " gtm-abc123 ", MetaPixelID: "123456789012345", MailchimpScriptURL: "https://chimpstatic.com/mcjs-connected/js/users/abc/def.js"}, 0)
	must(err)
	if saved.Version != 1 || saved.GTMID != "GTM-ABC123" {
		t.Fatalf("first save wrong: %+v", saved)
	}
	if _, err = s.SaveWebsiteTracking(ctx, manager, org, WebsiteTracking{GTMID: "GTM-ABC123"}, 0); !errors.Is(err, ErrTrackingConflict) {
		t.Fatal("a second first-save should conflict", err)
	}
	if _, err = s.SaveWebsiteTracking(ctx, manager, org, WebsiteTracking{GTMID: "GTM-ZZZZ99"}, 7); !errors.Is(err, ErrTrackingConflict) {
		t.Fatal("a stale version was accepted", err)
	}
	updated, err := s.SaveWebsiteTracking(ctx, manager, org, WebsiteTracking{GTMID: "GTM-ZZZZ99"}, 1)
	must(err)
	if updated.Version != 2 {
		t.Fatal("version did not advance", updated)
	}
	public, err := s.PublicWebsiteTracking(ctx)
	must(err)
	if public.GTMID != "GTM-ZZZZ99" || public.MetaPixelID != "" || public.MailchimpScriptURL != "" {
		t.Fatalf("public settings wrong: %+v", public)
	}

	// Website enquiries: anyone can send one; only marketing administrators read and sort them.
	_, err = pool.Exec(ctx, `INSERT INTO role_capabilities(role_id,action) VALUES($1,'marketing.read'),($1,'marketing.manage')`, role)
	must(err)
	for _, bad := range []WebsiteEnquiryInput{{Name: "X", Email: "a@example.invalid"}, {Name: "Synthetic Visitor"}, {Name: "Synthetic Visitor", Email: "not-an-email"}} {
		if _, err = s.CreateWebsiteEnquiry(ctx, bad); !errors.Is(err, ErrEnquiryInput) {
			t.Fatalf("invalid enquiry accepted: %+v %v", bad, err)
		}
	}
	id, err := s.CreateWebsiteEnquiry(ctx, WebsiteEnquiryInput{Name: " Synthetic Visitor ", Email: "visitor@example.invalid", Service: "Ceramic coating", Message: "Hello"})
	must(err)
	if _, _, err = s.WebsiteEnquiries(ctx, stranger, org, ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without marketing.read read enquiries", err)
	}
	items, fresh, err := s.WebsiteEnquiries(ctx, manager, org, "")
	if err != nil || fresh != 1 || len(items) != 1 || items[0].Name != "Synthetic Visitor" || items[0].Status != "new" {
		t.Fatalf("enquiry list wrong: %+v fresh=%d %v", items, fresh, err)
	}
	if err = s.SetWebsiteEnquiryStatus(ctx, stranger, org, id, "read"); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without marketing.manage changed an enquiry", err)
	}
	if err = s.SetWebsiteEnquiryStatus(ctx, manager, org, id, "nonsense"); !errors.Is(err, ErrEnquiryInput) {
		t.Fatal("an unknown status was accepted", err)
	}
	must(s.SetWebsiteEnquiryStatus(ctx, manager, org, id, "archived"))
	if items, _, err = s.WebsiteEnquiries(ctx, manager, org, ""); err != nil || len(items) != 0 {
		t.Fatal("an archived enquiry is still in the default list", items, err)
	}
	if items, _, err = s.WebsiteEnquiries(ctx, manager, org, "archived"); err != nil || len(items) != 1 {
		t.Fatal("the archived list is missing the enquiry", items, err)
	}
}
