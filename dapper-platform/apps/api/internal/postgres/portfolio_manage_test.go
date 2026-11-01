package postgres

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPortfolioManage(t *testing.T) {
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
	namespace := fmt.Sprintf("dapper_port_%d", time.Now().UnixNano())
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
	owner, err := s.InitializeAdministrator(ctx, "Synthetic Portfolio Business", "owner@example.invalid", "a-long-synthetic-password-1")
	must(err)
	franchise, err := s.ProvisionFirstFranchise(ctx, owner, FirstFranchiseInput{Name: "Synthetic Franchise", Code: "SYN", Address: Address{Street: "1 Test St", City: "Testville", Region: "ON", PostalCode: "A1A 1A1", Country: "Canada"}})
	must(err)
	scope := Scope{ActorID: owner.PrincipalID, OrganizationID: owner.OrganizationID, LocationID: franchise.ID}
	picture := func(w, h int) []byte {
		var buf bytes.Buffer
		must(png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))))
		return buf.Bytes()
	}
	var ids []string
	for i, size := range [][2]int{{300, 200}, {200, 300}} {
		media, e := s.UploadContentImage(ctx, scope, picture(size[0], size[1]))
		must(e)
		saved, e := s.SavePortfolioImage(ctx, scope, PortfolioImage{MediaID: media, Title: fmt.Sprintf("Synthetic photo %d", i+1), Alt: "A synthetic test picture", Category: "detailing", SortOrder: i, Published: true, RightsReviewed: true})
		must(e)
		ids = append(ids, saved.ID)
	}
	hidden, err := s.UploadContentImage(ctx, scope, picture(50, 50))
	must(err)
	_, err = s.SavePortfolioImage(ctx, scope, PortfolioImage{MediaID: hidden, Title: "Synthetic hidden", Alt: "Not published", Category: "other", SortOrder: 5, Published: false})
	must(err)

	list, err := s.PublicPortfolio(ctx)
	must(err)
	if len(list) != 2 || list[0].ID != ids[0] || list[0].Width != 300 || list[0].Height != 200 || list[1].Width != 200 {
		t.Fatalf("public portfolio wrong: %+v", list)
	}
	must(s.OrderPortfolio(ctx, scope, []string{ids[1], ids[0]}))
	if list, err = s.PublicPortfolio(ctx); err != nil || list[0].ID != ids[1] {
		t.Fatal("order not applied", list, err)
	}
	if err = s.OrderPortfolio(ctx, scope, []string{ids[0], ids[0]}); !errors.Is(err, ErrContentInput) {
		t.Fatal("a repeated photo in the order was accepted", err)
	}
	var stranger string
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:port-stranger') RETURNING id::text`).Scan(&stranger))
	other := scope
	other.ActorID = stranger
	if err = s.DeletePortfolioImage(ctx, other, ids[0]); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access deleted a photo", err)
	}
	must(s.DeletePortfolioImage(ctx, scope, ids[0]))
	if list, err = s.PublicPortfolio(ctx); err != nil || len(list) != 1 || list[0].ID != ids[1] {
		t.Fatal("deleted photo still public", list, err)
	}
	if err = s.DeletePortfolioImage(ctx, scope, ids[0]); !errors.Is(err, ErrContentNotFound) {
		t.Fatal("deleted twice", err)
	}
	var media int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM content_media`).Scan(&media))
	if media != 2 {
		t.Fatal("the deleted photo's picture was kept", media)
	}
	managed, err := s.ContentForManagement(ctx, scope)
	if err != nil || len(managed.Portfolio) != 2 {
		t.Fatal("management list should include the hidden photo", managed.Portfolio, err)
	}
}
