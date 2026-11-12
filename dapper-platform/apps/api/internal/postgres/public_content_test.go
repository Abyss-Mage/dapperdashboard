package postgres

import (
	"strings"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestContentImageNormalization(t *testing.T) {
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.White)
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	input := append(buffer.Bytes(), []byte("<script>private-metadata</script>")...)
	output, mime, err := NormalizeContentImage(input)
	if err != nil || mime != "image/png" || bytes.Contains(output, []byte("private-metadata")) {
		t.Fatal("image metadata/payload was not stripped", err)
	}
	for _, bad := range [][]byte{nil, []byte(`<svg onload="alert(1)"></svg>`), []byte("not an image"), make([]byte, 6*1024*1024+1)} {
		if _, _, err = NormalizeContentImage(bad); !errors.Is(err, ErrContentInput) {
			t.Fatal("unsafe image accepted")
		}
	}
}

func TestPublicContentPersistence(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_content_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := admin.pool.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error("isolated schema cleanup failed")
		}
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
	if err = s.ValidateSchema(ctx); err != nil {
		t.Fatal(err)
	}
	var org, loc, actor, otherOrg, otherLoc, otherActor string
	for i, entry := range []struct{ o, l, a *string }{{&org, &loc, &actor}, {&otherOrg, &otherLoc, &otherActor}} {
		if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES($1) RETURNING id::text`, fmt.Sprintf("Content synthetic %d", i)).Scan(entry.o); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Content location') RETURNING id::text`, *entry.o).Scan(entry.l); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) RETURNING id::text`, fmt.Sprintf("synthetic-content-%d", i)).Scan(entry.a); err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"content.manage", "catalogue.manage", "catalogue.read"} {
			if _, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, *entry.a, *entry.o, *entry.l, action); err != nil {
				t.Fatal(err)
			}
		}
	}
	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: actor}
	foreign := Scope{OrganizationID: org, LocationID: loc, ActorID: otherActor}
	site, err := s.SavePublicSite(ctx, scope, PublicSite{Slug: "synthetic-dapper", Name: "Synthetic content site"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PublicContent(ctx, site.Slug); !errors.Is(err, ErrContentNotFound) {
		t.Fatal("draft site exposed", err)
	}
	if _, err = s.SavePublicSite(ctx, foreign, site); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign site mutation accepted", err)
	}
	service, err := s.SaveService(ctx, scope, CatalogService{Name: "Shared service", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	publication := ServicePublication{ServiceID: service.ID, ServiceVersion: service.Version, Category: "sedan", Summary: "Synthetic service description", Published: true}
	publication, err = s.SaveServicePublication(ctx, scope, publication)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveServicePublication(ctx, scope, ServicePublication{ServiceID: service.ID, ServiceVersion: service.Version, Category: "sedan", Summary: "Stale", Published: true}); !errors.Is(err, ErrContentConflict) {
		t.Fatal("stale publication accepted", err)
	}
	site.Published = true
	site, err = s.SavePublicSite(ctx, scope, site)
	if err != nil {
		t.Fatal(err)
	}
	public, err := s.PublicContent(ctx, site.Slug)
	if err != nil || len(public.Services) != 1 {
		t.Fatal("published service missing", err)
	}
	// The sales strip: checked input, shown only while on and before its end date, edited only with content rights.
	for _, bad := range []SiteBanner{{Active: true}, {Active: true, Text: "Sale", LinkURL: "javascript:alert(1)", LinkLabel: "Go"}, {Active: true, Text: "Sale", LinkURL: "//evil.example", LinkLabel: "Go"}, {Active: true, Text: "Sale", LinkURL: "/book-appointment"}, {Active: true, Text: "Sale", Tone: "pink"}} {
		if _, err = s.SaveSiteBanner(ctx, scope, bad); !errors.Is(err, ErrContentInput) {
			t.Fatal("bad banner accepted", bad, err)
		}
	}
	if _, err = s.SaveSiteBanner(ctx, foreign, SiteBanner{Active: true, Text: "Sale"}); !errors.Is(err, ErrForbidden) {
		t.Fatal("banner saved without content rights", err)
	}
	banner := SiteBanner{Active: true, Text: "Synthetic sale: 10% off ceramic", LinkLabel: "Book now", LinkURL: "/book-appointment"}
	if _, err = s.SaveSiteBanner(ctx, scope, banner); err != nil {
		t.Fatal(err)
	}
	if public, err = s.PublicContent(ctx, site.Slug); err != nil || public.Banner == nil || public.Banner.Text != banner.Text || public.Banner.Tone != "red" {
		t.Fatal("live banner missing from the website", public.Banner, err)
	}
	past := time.Now().Add(-time.Hour)
	banner.EndsAt = &past
	if _, err = s.SaveSiteBanner(ctx, scope, banner); err != nil {
		t.Fatal(err)
	}
	if public, err = s.PublicContent(ctx, site.Slug); err != nil || public.Banner != nil {
		t.Fatal("an ended banner still shows", err)
	}
	if managed, err := s.ContentForManagement(ctx, scope); err != nil || managed.Banner == nil || managed.Banner.EndsAt == nil {
		t.Fatal("the dashboard lost the ended banner", err)
	}
	// Search settings (owner 2026-11-13): access.manage only, checked input, empty page entries dropped, served to the website.
	seo := SiteSEO{Title: "Synthetic detailing", Description: "Synthetic description", Keywords: "detailing, ceramic", GoogleVerification: "abc_123-XYZ",
		Pages: map[string]PageSEO{"/services": {Title: "Synthetic services", Hidden: true}, "/about-us": {}}}
	if _, err = s.SaveSiteSEO(ctx, actor, org, site.Slug, seo); !errors.Is(err, ErrForbidden) {
		t.Fatal("search settings saved with content rights only", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage')`, actor, org); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []SiteSEO{{Title: strings.Repeat("x", 71)}, {Description: "<script>"}, {Image: "javascript:alert(1)"}, {GoogleVerification: "\"><b>"}, {Pages: map[string]PageSEO{"/../x": {Title: "x"}}}, {Pages: map[string]PageSEO{"/x": {Image: "//evil.example/a.png"}}}} {
		if _, err = s.SaveSiteSEO(ctx, actor, org, site.Slug, bad); !errors.Is(err, ErrContentInput) {
			t.Fatal("bad search settings accepted", bad, err)
		}
	}
	if _, err = s.SaveSiteSEO(ctx, actor, org, site.Slug, seo); err != nil {
		t.Fatal(err)
	}
	if public, err = s.PublicContent(ctx, site.Slug); err != nil || public.SEO.Keywords != seo.Keywords || !public.SEO.Pages["/services"].Hidden || len(public.SEO.Pages) != 1 {
		t.Fatal("search settings not served", public.SEO, err)
	}
	// Upgrade offers: checked input, one per package (matched without case), only switched-on ones reach the website.
	upsellOrg := scope.OrganizationID
	// Offers belong to the organization's catalogue, so the owner here holds the organization-level grant (the franchise-only person does not).
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'catalogue.manage'),($1,$2,'catalogue.read')`, actor, upsellOrg); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []PackageUpsell{{FromPack: "VIP Interior", ToPack: "vip interior"}, {FromPack: "x", ToPack: "Luxury"}, {FromPack: "VIP Interior", ToPack: "Luxury", Perks: []string{"1", "2", "3", "4", "5", "6", "7"}}} {
		if _, err = s.SavePackageUpsell(ctx, scope.ActorID, upsellOrg, bad); !errors.Is(err, ErrUpsellInput) {
			t.Fatal("bad upgrade offer accepted", bad, err)
		}
	}
	if _, err = s.SavePackageUpsell(ctx, foreign.ActorID, upsellOrg, PackageUpsell{FromPack: "VIP Interior", ToPack: "Luxury"}); err == nil {
		t.Fatal("upgrade offer saved without catalogue rights")
	}
	offer := PackageUpsell{FromPack: "VIP Interior Detail", ToPack: "Luxury Full Body Detail", Pitch: "Add the outside too", Perks: []string{" Hand wash ", ""}, Active: true}
	if _, err = s.SavePackageUpsell(ctx, scope.ActorID, upsellOrg, offer); err != nil {
		t.Fatal(err)
	}
	offer.FromPack = "vip interior detail"
	if _, err = s.SavePackageUpsell(ctx, scope.ActorID, upsellOrg, offer); err != nil {
		t.Fatal("replacing an offer failed", err)
	}
	// #34: an offer's discount is a percent or an amount, not both, and comes off only the offer's own target package.
	if _, err = s.SavePackageUpsell(ctx, scope.ActorID, upsellOrg, PackageUpsell{FromPack: "VIP Interior Detail", ToPack: "Luxury Full Body Detail", DiscountPercent: 10, DiscountCents: 500, Active: true}); !errors.Is(err, ErrUpsellInput) {
		t.Fatal("an offer with two discounts was accepted", err)
	}
	offer.DiscountPercent = 10
	if _, err = s.SavePackageUpsell(ctx, scope.ActorID, upsellOrg, offer); err != nil {
		t.Fatal(err)
	}
	if off, err := upsellDiscount(ctx, s.pool, upsellOrg, "VIP Interior Detail", "Sedan - Luxury Full Body Detail", 22900); err != nil || off != 2290 {
		t.Fatal("upgrade discount wrong", off, err)
	}
	if off, err := upsellDiscount(ctx, s.pool, upsellOrg, "VIP Interior Detail", "Sedan - Exterior Wash", 7500); err != nil || off != 0 {
		t.Fatal("upgrade discount given on another package", off, err)
	}
	if _, err = s.SavePackageUpsell(ctx, scope.ActorID, upsellOrg, PackageUpsell{FromPack: "Exterior Wash", ToPack: "Full Detail", Active: false}); err != nil {
		t.Fatal(err)
	}
	all, err := s.PackageUpsells(ctx, scope.ActorID, upsellOrg)
	if err != nil || len(all) != 2 {
		t.Fatal("offers not one per package", all, err)
	}
	if public, err = s.PublicContent(ctx, site.Slug); err != nil || len(public.Upsells) != 1 || public.Upsells[0].ToPack != "Luxury Full Body Detail" || len(public.Upsells[0].Perks) != 1 || public.Upsells[0].Perks[0] != "Hand wash" {
		t.Fatal("website offers wrong", public.Upsells, err)
	}
	if err = s.DeletePackageUpsell(ctx, scope.ActorID, upsellOrg, "VIP INTERIOR DETAIL"); err != nil {
		t.Fatal(err)
	}
	if public, err = s.PublicContent(ctx, site.Slug); err != nil || len(public.Upsells) != 0 {
		t.Fatal("a removed offer still shows", public.Upsells, err)
	}
	// Later checks grant the organization catalogue themselves.
	if _, err = pool.Exec(ctx, `DELETE FROM organization_grants WHERE principal_id=$1 AND organization_id=$2`, actor, upsellOrg); err != nil {
		t.Fatal(err)
	}
	service.Name = "Shared service renamed in Workspace"
	service, err = s.SaveService(ctx, scope, service)
	if err != nil {
		t.Fatal(err)
	}
	public, err = s.PublicContent(ctx, site.Slug)
	if err != nil || public.Services[0].Name != service.Name {
		t.Fatal("public catalogue copied a stale service name", err)
	}
	if _, err = s.SaveServicePublication(ctx, scope, publication); !errors.Is(err, ErrContentConflict) {
		t.Fatal("stale operational service revision accepted", err)
	}
	var encoded bytes.Buffer
	png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 3, 3)))
	media, err := s.UploadContentImage(ctx, scope, encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.PublicImage(ctx, media); !errors.Is(err, ErrContentNotFound) {
		t.Fatal("unpublished upload exposed", err)
	}
	if _, _, err = s.ManagedContentImage(ctx, foreign, media); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign draft media exposed", err)
	}
	photo := PortfolioImage{MediaID: media, Title: "Synthetic test image", Alt: "Synthetic three pixel image", Category: "detailing", SortOrder: 1}
	photo, err = s.SavePortfolioImage(ctx, scope, photo)
	if err != nil {
		t.Fatal(err)
	}
	photo.Published = true
	if _, err = s.SavePortfolioImage(ctx, scope, photo); !errors.Is(err, ErrContentInput) {
		t.Fatal("publication without rights review accepted")
	}
	photo.RightsReviewed = true
	photo, err = s.SavePortfolioImage(ctx, scope, photo)
	if err != nil {
		t.Fatal(err)
	}
	if _, mime, e := s.PublicImage(ctx, media); e != nil || mime != "image/png" {
		t.Fatal("published image missing", e)
	}
	wrongMedia := photo
	wrongMedia.ID = ""
	wrongMedia.Version = 0
	otherScope := Scope{OrganizationID: otherOrg, LocationID: otherLoc, ActorID: otherActor}
	if _, err = s.SavePortfolioImage(ctx, otherScope, wrongMedia); !errors.Is(err, ErrContentInput) {
		t.Fatal("cross-location media reference accepted", err)
	}
	stale := photo
	stale.Version--
	if _, err = s.SavePortfolioImage(ctx, scope, stale); !errors.Is(err, ErrContentConflict) {
		t.Fatal("stale image revision accepted", err)
	}
	photo.Published = false
	photo, err = s.SavePortfolioImage(ctx, scope, photo)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.PublicImage(ctx, media); !errors.Is(err, ErrContentNotFound) {
		t.Fatal("unpublished image still accessible", err)
	}
	// Dealers follow the same rules: unpublished logos stay private, publishing needs the rights confirmation, only published ones are listed.
	dealer := WebsiteDealer{MediaID: media, Name: "Synthetic dealership", SortOrder: 1}
	dealer, err = s.SaveWebsiteDealer(ctx, scope, dealer)
	if err != nil {
		t.Fatal(err)
	}
	if list, e := s.PublicDealers(ctx); e != nil || len(list) != 0 {
		t.Fatal("unpublished dealer listed", e)
	}
	dealer.Published = true
	if _, err = s.SaveWebsiteDealer(ctx, scope, dealer); !errors.Is(err, ErrContentInput) {
		t.Fatal("dealer published without rights confirmation")
	}
	dealer.RightsReviewed = true
	if dealer, err = s.SaveWebsiteDealer(ctx, scope, dealer); err != nil {
		t.Fatal(err)
	}
	if list, e := s.PublicDealers(ctx); e != nil || len(list) != 1 || list[0].Name != dealer.Name {
		t.Fatal("published dealer missing", e)
	}
	if _, _, e := s.PublicImage(ctx, media); e != nil {
		t.Fatal("dealer logo not served", e)
	}
	// The Bin: a binned item leaves the website but is kept, comes back on restore, and another location cannot touch it.
	if err = s.SetContentBinned(ctx, otherScope, "dealers", dealer.ID, true); !errors.Is(err, ErrContentNotFound) {
		t.Fatal("binned another location's dealer", err)
	}
	if err = s.SetContentBinned(ctx, scope, "dealers", dealer.ID, true); err != nil {
		t.Fatal(err)
	}
	if list, e := s.PublicDealers(ctx); e != nil || len(list) != 0 {
		t.Fatal("binned dealer still listed", e)
	}
	if _, _, e := s.PublicImage(ctx, media); !errors.Is(e, ErrContentNotFound) {
		t.Fatal("binned logo still served", e)
	}
	if managed, e := s.ContentForManagement(ctx, scope); e != nil || len(managed.Dealers) != 1 || managed.Dealers[0].BinnedAt == nil {
		t.Fatal("binned dealer not kept for the Bin", e)
	}
	if err = s.SetContentBinned(ctx, scope, "dealers", dealer.ID, false); err != nil {
		t.Fatal(err)
	}
	dealer.Version += 2
	if list, e := s.PublicDealers(ctx); e != nil || len(list) != 1 {
		t.Fatal("restored dealer missing", e)
	}
	if err = s.SetSiteAssetHidden(ctx, scope, "team/photo.jpg", true); err != nil {
		t.Fatal(err)
	}
	if binned, e := s.BinnedSiteAssets(ctx, scope); e != nil || len(binned) != 1 || binned[0].Path != "team/photo.jpg" {
		t.Fatal("removed file not in the Bin", binned, e)
	}
	if err = s.PurgeSiteAsset(ctx, scope, "team/photo.jpg"); err != nil {
		t.Fatal(err)
	}
	if binned, e := s.BinnedSiteAssets(ctx, scope); e != nil || len(binned) != 0 {
		t.Fatal("deleted file still in the Bin", binned, e)
	}
	if off, e := s.HiddenSiteAssets(ctx, scope); e != nil || len(off) != 1 {
		t.Fatal("a deleted built-in file must stay off the website", off, e)
	}
	if err = s.SetLogoSize(ctx, scope, "partners/partner-1.png", "huge"); !errors.Is(err, ErrSiteAsset) {
		t.Fatal("invalid logo size accepted", err)
	}
	for _, size := range []string{"s", "xl"} { // the second save changes the first
		if err = s.SetLogoSize(ctx, scope, "partners/partner-1.png", size); err != nil {
			t.Fatal(err)
		}
	}
	if sizes, e := s.LogoSizes(ctx, scope); e != nil || sizes["partners/partner-1.png"] != "xl" {
		t.Fatal("logo size not saved", sizes, e)
	}
	if sizes, e := s.LogoSizes(ctx, otherScope); e == nil && len(sizes) != 0 {
		t.Fatal("logo sizes leaked across locations", sizes)
	}
	if _, err = s.SaveWebsiteDealer(ctx, otherScope, WebsiteDealer{MediaID: media, Name: "Cross scope"}); !errors.Is(err, ErrContentInput) {
		t.Fatal("cross-location logo accepted", err)
	}
	if err = s.DeleteWebsiteDealer(ctx, scope, dealer.ID); err != nil {
		t.Fatal(err)
	}
	if list, e := s.PublicDealers(ctx); e != nil || len(list) != 0 {
		t.Fatal("deleted dealer still listed", e)
	}
	// Audit insertion failure must roll back the actual publication, not just its log.
	if _, err = pool.Exec(ctx, `CREATE FUNCTION reject_content_audit() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'synthetic audit rejection'; END$$; CREATE TRIGGER reject_content_audit BEFORE INSERT ON content_events FOR EACH ROW EXECUTE FUNCTION reject_content_audit()`); err != nil {
		t.Fatal(err)
	}
	blocked := site
	blocked.Name = "Should roll back"
	if _, err = s.SavePublicSite(ctx, scope, blocked); err == nil {
		t.Fatal("audit failure ignored")
	}
	public, err = s.PublicContent(ctx, site.Slug)
	if err != nil || public.Site.Name != site.Name {
		t.Fatal("failed audit allowed site change", err)
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER reject_content_audit ON content_events`); err != nil {
		t.Fatal(err)
	}
	deletedPublication, err := s.DeleteService(ctx, scope, service.ID, service.Version)
	if err != nil || deletedPublication.Permanent || !deletedPublication.Archived {
		t.Fatal("published service deletion lost metadata or failed retention", err)
	}
	public, err = s.PublicContent(ctx, site.Slug)
	if err != nil || len(public.Services) != 0 {
		t.Fatal("archived service remained in public selection", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'catalogue.manage'),($1,$2,'catalogue.read')`, actor, org); err != nil {
		t.Fatal(err)
	}
	central, err := s.SaveCentralService(ctx, actor, org, CentralServiceInput{Name: "Synthetic central service", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	linked, err := s.ImportCentralService(ctx, scope, central.ID, central.Version)
	if err != nil || linked.CentralServiceID == nil || *linked.CentralServiceID != central.ID {
		t.Fatal("central import failed", err)
	}
	// Switching on a central service twice returns the same franchise service (it is never copied twice).
	if again, err := s.ImportCentralService(ctx, scope, central.ID, central.Version); err != nil || again.ID != linked.ID {
		t.Fatal("second switch-on did not return the same service", again, err)
	}
	if _, err = s.ImportCentralService(ctx, foreign, central.ID, central.Version); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign central mapping accepted", err)
	}
	star := true
	if _, err = s.SaveServicePublication(ctx, scope, ServicePublication{ServiceID: linked.ID, ServiceVersion: linked.Version, Category: "boat", Summary: "Synthetic star one", Published: true, SetStar: &star}); err != nil {
		t.Fatal(err)
	}
	second, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic second star", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	starred, err := s.SaveServicePublication(ctx, scope, ServicePublication{ServiceID: second.ID, ServiceVersion: second.Version, Category: "boat", Summary: "Synthetic star two", Published: true, SetStar: &star})
	if err != nil || !starred.Starred {
		t.Fatalf("second star: %v %+v", err, starred)
	}
	var stars int
	var holder string
	if err = s.pool.QueryRow(ctx, `SELECT count(*),min(service_id::text) FROM service_publications WHERE organization_id=$1 AND location_id=$2 AND category='boat' AND starred`, scope.OrganizationID, scope.LocationID).Scan(&stars, &holder); err != nil || stars != 1 || holder != second.ID {
		t.Fatalf("a category must have one star, the newest: %v %d %s", err, stars, holder)
	}
	// Leave the website as the checks below expect it.
	if _, err = s.pool.Exec(ctx, `DELETE FROM service_publications WHERE service_id = ANY($1::uuid[])`, []string{linked.ID, second.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveServicePublication(ctx, scope, ServicePublication{ServiceID: linked.ID, ServiceVersion: linked.Version, Category: "sedan", Summary: "Linked shared service", Published: true}); err != nil {
		t.Fatal(err)
	}
	central, err = s.SaveCentralService(ctx, actor, org, CentralServiceInput{ID: central.ID, Version: central.Version, Name: "Central name changed", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	public, err = s.PublicContent(ctx, site.Slug)
	if err != nil || len(public.Services) != 1 || public.Services[0].Name != central.Name {
		t.Fatal("central name did not reach website", err)
	}
	current, err := s.Services(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	var linkedCurrent CatalogService
	for _, item := range current {
		if item.ID == linked.ID {
			linkedCurrent = item
		}
	}
	if linkedCurrent.Name != central.Name || linkedCurrent.Version != 2 {
		t.Fatal("central name did not reach operational catalogue")
	}
	linkedCurrent.Name = "Unauthorized local override"
	if _, err = s.SaveService(ctx, scope, linkedCurrent); !errors.Is(err, ErrServiceConflict) {
		t.Fatal("linked local override accepted", err)
	}
	child, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic dependent local addon", Kind: "addon", ParentID: &linked.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ArchiveCentralService(ctx, actor, org, central.ID, central.Version); !errors.Is(err, ErrCentralServiceHasChildren) {
		t.Fatal("central archival bypassed location dependency", err)
	}
	if _, err = s.ArchiveService(ctx, scope, child.ID, child.Version); err != nil {
		t.Fatal(err)
	}
	centralAddon, err := s.SaveCentralService(ctx, actor, org, CentralServiceInput{Name: "Synthetic central add-on", Kind: "addon", ParentID: &central.ID})
	if err != nil {
		t.Fatal(err)
	}
	linkedAddon, err := s.ImportCentralService(ctx, scope, centralAddon.ID, centralAddon.Version)
	if err != nil || linkedAddon.ParentID == nil || *linkedAddon.ParentID != linked.ID {
		t.Fatal("central add-on lost local parent identity", err)
	}
	if _, err = s.ArchiveCentralService(ctx, actor, org, centralAddon.ID, centralAddon.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ArchiveCentralService(ctx, actor, org, central.ID, central.Version); err != nil {
		t.Fatal(err)
	}
	public, err = s.PublicContent(ctx, site.Slug)
	if err != nil || len(public.Services) != 0 {
		t.Fatal("central archive did not reach public projection", err)
	}
	centralRows, e := s.ListCentralServices(ctx, actor, org)
	if e != nil {
		t.Fatal(e)
	}
	for _, row := range centralRows {
		if row.ID == central.ID {
			removed, e := s.DeleteCentralService(ctx, actor, org, central.ID, row.Version)
			if e != nil || removed.Permanent || !removed.Archived {
				t.Fatal("archived central links lost retained identity", e)
			}
		}
	}
	var historical string
	if err = pool.QueryRow(ctx, `SELECT snapshot->>'name' FROM catalog_service_revisions WHERE service_id=$1 AND version=1`, linked.ID).Scan(&historical); err != nil || historical != "Synthetic central service" {
		t.Fatal("central edit rewrote historical revision", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='content.manage'`, actor); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SavePublicSite(ctx, scope, site); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked publisher retained write access", err)
	}
	if _, _, err = s.ManagedContentImage(ctx, scope, media); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked publisher retained private media access", err)
	}
	managed, err := s.ContentForManagement(ctx, scope)
	if err != nil || len(managed.Portfolio) != 0 {
		t.Fatal("catalogue-only manager saw private gallery", err)
	}
}
