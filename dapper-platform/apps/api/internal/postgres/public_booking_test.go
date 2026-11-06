package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeVerifier struct{ lastPhone, lastCode string }

func (f *fakeVerifier) SendCode(_ context.Context, phone, code string) error {
	f.lastPhone, f.lastCode = phone, code
	return nil
}

func verifyPublicBooking(t *testing.T, ctx context.Context, s *Store, a Scope) {
	t.Helper()
	const slug = "synthetic-public-site"
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Not published: the website refuses everything.
	exec(`INSERT INTO public_sites(organization_id,location_id,slug,name,published) VALUES($1,$2,$3,'Synthetic site',false)`, a.OrganizationID, a.LocationID, slug)
	defer func() {
		exec(`DELETE FROM phone_verifications WHERE organization_id=$1 AND location_id=$2`, a.OrganizationID, a.LocationID)
		exec(`DELETE FROM location_grants WHERE principal_id IN (SELECT id FROM principals WHERE external_subject='system:website') AND location_id=$1`, a.LocationID)
		exec(`DELETE FROM organization_memberships WHERE organization_id=$1 AND principal_id IN (SELECT id FROM principals WHERE external_subject='system:website')`, a.OrganizationID)
		exec(`DELETE FROM public_sites WHERE slug=$1`, slug)
	}()
	if _, err := s.PublicBookingOptions(ctx, slug); !errors.Is(err, ErrPublicSite) {
		t.Fatal("unpublished site served booking options", err)
	}
	if _, err := s.PublicBookingOptions(ctx, "Bad Slug!"); !errors.Is(err, ErrPublicSite) {
		t.Fatal("invalid slug accepted", err)
	}
	exec(`UPDATE public_sites SET published=true WHERE slug=$1`, slug)
	opts, err := s.PublicBookingOptions(ctx, slug)
	if err != nil || opts.PhoneVerification {
		t.Fatal("public options wrong", opts.PhoneVerification, err)
	}
	for _, r := range opts.Rules { // only priced services are offered
		found := false
		for _, v := range opts.Services {
			found = found || v.ID == r.ServiceID && v.PriceCents >= 0
		}
		if !found {
			t.Fatal("an unpriced service was offered online", r.ServiceName)
		}
	}
	// A customer to find again (a stranger must not see it without the code).
	var cid string
	if err = s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic returning','returning@example.invalid','+1 (705) 555-0177') RETURNING id::text`, a.OrganizationID, a.LocationID).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	defer s.pool.Exec(ctx, `DELETE FROM customers WHERE id=$1`, cid)

	// No text-message provider yet: the check is unavailable rather than pretending.
	if err = s.StartPhoneVerification(ctx, slug, "705 555 0177"); !errors.Is(err, ErrPhoneVerificationUnavailable) {
		t.Fatal("verification offered without a provider", err)
	}
	fake := &fakeVerifier{}
	s.SetPhoneVerifier(fake)
	defer s.SetPhoneVerifier(nil)
	if err = s.StartPhoneVerification(ctx, slug, "12"); !errors.Is(err, ErrPublicBookingInput) {
		t.Fatal("short phone accepted", err)
	}
	if err = s.StartPhoneVerification(ctx, slug, "705 555 0177"); err != nil || len(fake.lastCode) != 6 {
		t.Fatal("code not sent", err, fake.lastCode)
	}
	if _, err = s.ReturningCustomers(ctx, slug, "705 555 0177", "nope.nope"); !errors.Is(err, ErrPhoneVerification) {
		t.Fatal("customers shown without verification", err)
	}
	wrong := "000000"
	if fake.lastCode == wrong {
		wrong = "111111"
	}
	if _, err = s.CheckPhoneVerification(ctx, slug, "705 555 0177", wrong); !errors.Is(err, ErrPhoneVerification) {
		t.Fatal("wrong code accepted", err)
	}
	token, err := s.CheckPhoneVerification(ctx, slug, "705 555 0177", fake.lastCode)
	if err != nil || !strings.Contains(token, ".") {
		t.Fatal("right code refused", err)
	}
	if _, err = s.CheckPhoneVerification(ctx, slug, "705 555 0177", fake.lastCode); !errors.Is(err, ErrPhoneVerification) {
		t.Fatal("a used code worked twice", err)
	}
	found, err := s.ReturningCustomers(ctx, slug, "(705) 555-0177", token)
	if err != nil || len(found) != 1 || found[0].ID != cid {
		t.Fatal("verified phone did not find the customer", found, err)
	}
	if _, err = s.ReturningCustomers(ctx, slug, "705 555 0188", token); !errors.Is(err, ErrPhoneVerification) {
		t.Fatal("token worked for a different number", err)
	}
	// Three codes per hour, then a limit.
	for i := 0; i < 2; i++ {
		if err = s.StartPhoneVerification(ctx, slug, "705 555 0177"); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.StartPhoneVerification(ctx, slug, "705 555 0177"); !errors.Is(err, ErrPhoneVerificationLimit) {
		t.Fatal("fourth code in an hour sent", err)
	}
	// Five wrong tries lock the latest code.
	for i := 0; i < 5; i++ {
		s.CheckPhoneVerification(ctx, slug, "705 555 0177", "999999")
	}
	if _, err = s.CheckPhoneVerification(ctx, slug, "705 555 0177", fake.lastCode); !errors.Is(err, ErrPhoneVerificationLimit) {
		t.Fatal("code still worked after five wrong tries", err)
	}
	verifyPublicBookFlow(t, ctx, s, a, slug)
	// Bad input to booking is refused before anything is held.
	if _, err = s.PublicBook(ctx, slug, PublicBookingInput{}); !errors.Is(err, ErrPublicBookingInput) {
		t.Fatal("empty booking accepted", err)
	}
}

func verifyPublicBookFlow(t *testing.T, ctx context.Context, s *Store, a Scope, slug string) {
	t.Helper()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	main, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic web main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	addon, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic web add-on", Kind: "addon", ParentID: &main.ID})
	if err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE catalog_services SET price_cents=20000 WHERE id=$1`, main.ID)
	exec(`UPDATE catalog_services SET price_cents=5000 WHERE id=$1`, addon.ID)
	res, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic web resource", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	week := make([]HourWindow, 0, 7)
	for d := 0; d < 7; d++ {
		week = append(week, HourWindow{Day: d, Start: "08:00", End: "18:00"})
	}
	if err = s.SaveWeeklyHours(ctx, a, res.ID, week); err != nil {
		t.Fatal(err)
	}
	rule, err := s.SaveSchedulingRule(ctx, a, SchedulingRule{ResourceID: res.ID, ServiceID: main.ID, DurationMinutes: 60, Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	var bound bool
	if err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM location_invoice_issuers WHERE organization_id=$1 AND location_id=$2)`, a.OrganizationID, a.LocationID).Scan(&bound); err != nil {
		t.Fatal(err)
	}
	if !bound {
		var issuer string
		if err = s.pool.QueryRow(ctx, `INSERT INTO invoice_issuers(organization_id,legal_name,address,tax_registration,currency,number_prefix,next_number,updated_by) VALUES($1,'Synthetic Issuer Ltd','1 Test Road, Test City',' ','CAD','TST',1,$2) RETURNING id::text`, a.OrganizationID, a.ActorID).Scan(&issuer); err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO location_invoice_issuers(organization_id,location_id,issuer_id) VALUES($1,$2,$3)`, a.OrganizationID, a.LocationID, issuer)
	}
	exec(`DELETE FROM franchise_tax_settings WHERE organization_id=$1 AND location_id=$2`, a.OrganizationID, a.LocationID)
	exec(`INSERT INTO franchise_tax_rates(organization_id,location_id,name,rate_bp) VALUES($1,$2,'Synthetic web HST',1300)`, a.OrganizationID, a.LocationID)
	defer exec(`DELETE FROM franchise_tax_rates WHERE organization_id=$1 AND location_id=$2`, a.OrganizationID, a.LocationID)

	opts, err := s.PublicBookingOptions(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	offered := false
	for _, r := range opts.Rules {
		offered = offered || r.ID == rule.ID && len(r.AvailableAddons) == 1
	}
	if !offered {
		t.Fatal("the priced service and its add-on were not offered online")
	}
	loc, _ := time.LoadLocation("America/Toronto")
	day := time.Now().In(loc).AddDate(0, 0, 3)
	avail, err := s.PublicAvailability(ctx, slug, res.ID, rule.ID, rule.Version, day.Format("2006-01-02"), 1)
	if err != nil || len(avail.Days) != 1 || len(avail.Days[0].Starts) < 5 {
		t.Fatal("no times offered for an open day", avail, err)
	}
	start := avail.Days[0].Starts[2]
	in := PublicBookingInput{RequestKey: "public-booking-key-0001", ResourceID: res.ID, RuleID: rule.ID, RuleVersion: rule.Version, Start: start, AddonIDs: []string{addon.ID}, Name: "Synthetic Web Guest", Email: "web.guest@example.invalid", Phone: "+1 705 555 0166", Notes: "Vehicle: Sedan"}
	got, err := s.PublicBook(ctx, slug, in)
	if err != nil {
		t.Fatal("public booking failed", err)
	}
	if got.TotalCents != 28250 || !strings.HasPrefix(got.InvoiceNumber, "T") && !strings.Contains(got.InvoiceNumber, "-") {
		t.Fatalf("totals wrong (want 28250 total, 8475 deposit): %+v", got)
	}
	var status, channel string
	if err = s.pool.QueryRow(ctx, `SELECT b.status,b.channel FROM bookings b WHERE b.id=$1`, got.BookingID).Scan(&status, &channel); err != nil || status != "confirmed" || channel != "online" {
		t.Fatal("booking not recorded as an approved online booking", status, channel, err)
	}
	// The same request again returns the same booking and invoice and does not cancel anything.
	again, err := s.PublicBook(ctx, slug, in)
	if err != nil || again.BookingID != got.BookingID || again.InvoiceID != got.InvoiceID {
		t.Fatal("a repeated request did not return the same booking", again, err)
	}
	// The time is now taken.
	other := in
	other.RequestKey = "public-booking-key-0002"
	other.Email = "someone.else@example.invalid"
	if _, err = s.PublicBook(ctx, slug, other); err == nil {
		t.Fatal("the same time was sold twice")
	}
	// A second booking with the same email reuses the customer.
	third := in
	third.RequestKey = "public-booking-key-0003"
	third.Start = avail.Days[0].Starts[4]
	third.AddonIDs = nil
	thirdRes, err := s.PublicBook(ctx, slug, third)
	if err != nil {
		t.Fatal("second booking for a returning email failed", err)
	}
	// Only a booking that made the customer earns the confirmation email; a returning address might not be the booker's.
	if !got.NewCustomer || thirdRes.NewCustomer {
		t.Fatal("new-customer flag wrong", got.NewCustomer, thirdRes.NewCustomer)
	}
	var customers int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM customers WHERE organization_id=$1 AND location_id=$2 AND lower(email)='web.guest@example.invalid'`, a.OrganizationID, a.LocationID).Scan(&customers); err != nil || customers != 1 {
		t.Fatal("a returning email created a second customer", customers, err)
	}
	// Coupons: the public preview and the staff invoice agree, tax comes off the reduced amount, and a bad code is refused before anything is held.
	const couponCode = "SYNPUB10"
	if _, err = s.SaveCoupon(ctx, a, "", CouponInput{Title: "Synthetic public coupon", Code: couponCode, Kind: "percent", Value: 10, Period: "never", Active: true}, 0); err != nil {
		t.Fatal(err)
	}
	preview, err := s.PublicCouponPreview(ctx, slug, couponCode, main.ID, 20000)
	if err != nil || preview.DiscountCents != 2000 {
		t.Fatal("public coupon preview wrong", preview, err)
	}
	if _, err = s.PublicCouponPreview(ctx, slug, "NOPE99", main.ID, 20000); !errors.As(err, new(CouponInvalid)) {
		t.Fatal("a code that does not exist was accepted", err)
	}
	var guest string
	if err = s.pool.QueryRow(ctx, `SELECT id::text FROM customers WHERE organization_id=$1 AND location_id=$2 AND lower(email)='web.guest@example.invalid'`, a.OrganizationID, a.LocationID).Scan(&guest); err != nil {
		t.Fatal(err)
	}
	staff := ReserveBookingInput{RequestKey: "staff-coupon-key-0001", CustomerID: guest, ResourceID: res.ID, RuleID: rule.ID, RuleVersion: rule.Version, Start: avail.Days[0].Starts[6], Channel: "walk_in"}
	if err = s.CheckStaffCoupon(ctx, a, staff, "NOPE99"); !errors.As(err, new(CouponInvalid)) {
		t.Fatal("staff booking accepted a bad coupon", err)
	}
	if err = s.CheckStaffCoupon(ctx, a, staff, couponCode); err != nil {
		t.Fatal("staff booking refused a good coupon", err)
	}
	staffBooking, err := s.ReserveBooking(ctx, a, staff)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.InvoiceStaffBooking(ctx, a, staff, staffBooking.ID, couponCode); err != nil {
		t.Fatal("staff coupon invoice failed", err)
	}
	var staffTotal int64
	if err = s.pool.QueryRow(ctx, `SELECT total_cents FROM invoices WHERE booking_id=$1 AND status<>'void'`, staffBooking.ID).Scan(&staffTotal); err != nil || staffTotal != 20340 {
		t.Fatalf("staff coupon invoice total wrong (want 20340 = 18000 + 13%% tax): %d %v", staffTotal, err)
	}
	// A customer's coupon on the website: the booking goes through and the invoice carries the discount (a coupon once made issuing fail).
	withCoupon := third
	withCoupon.RequestKey = "public-booking-key-0004"
	withCoupon.Start = avail.Days[0].Starts[8]
	withCoupon.CouponCode = couponCode
	couponRes, err := s.PublicBook(ctx, slug, withCoupon)
	if err != nil || couponRes.TotalCents != 20340 {
		t.Fatalf("a website booking with a coupon failed or priced wrong (want 20340): %+v %v", couponRes, err)
	}
	// The website's pictures belong to the main site's location only.
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 'I', 'H', 'D', 'R', 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89}
	if err = s.SaveSiteAsset(ctx, a, "zz-test/ping.png", png); err != nil {
		t.Fatal(err)
	}
	defer exec(`DELETE FROM site_assets WHERE path='zz-test/ping.png'`)
	if _, _, err = s.PublicSiteAssetVersion(ctx, "zz-test/ping.png"); err == nil {
		t.Fatal("a file from a location that is not the main site was served")
	}
	t.Setenv("DAPPER_SITE_SLUG", slug)
	if kind, _, err := s.PublicSiteAssetVersion(ctx, "zz-test/ping.png"); err != nil || kind != "image/png" {
		t.Fatal("the main site's own file was not served", kind, err)
	}
	// A failed deposit releases the time.
	s.PublicBookingAbort(ctx, slug, got.BookingID, got.InvoiceID)
	var invStatus string
	if err = s.pool.QueryRow(ctx, `SELECT status FROM invoices WHERE id=$1`, got.InvoiceID).Scan(&invStatus); err != nil || invStatus != "void" {
		t.Fatal("invoice not voided after an abort", invStatus, err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT status FROM bookings WHERE id=$1`, got.BookingID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatal("booking not cancelled after an abort", status, err)
	}
	// A price that disappears stops online booking for that service.
	exec(`UPDATE catalog_services SET price_cents=NULL WHERE id=$1`, main.ID)
	nope := in
	nope.RequestKey = "public-booking-key-0004"
	nope.Start = avail.Days[0].Starts[6]
	nope.AddonIDs = nil
	if _, err = s.PublicBook(ctx, slug, nope); !errors.Is(err, ErrPublicBookingPrice) {
		t.Fatal("an unpriced service was booked online", err)
	}
}

// TestPublicBookingEndToEnd runs in its own isolated schema so the customers, bookings and issued invoices it creates cannot
// disturb the shared fixture's counts.
func TestPublicBookingEndToEnd(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("DAPPER_TEST_DATABASE_URL required for real PostgreSQL evidence")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_test_pub_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer admin.pool.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
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
	var org, loc string
	if err = s.pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic public') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Public location') RETURNING id::text`, org).Scan(&loc); err != nil {
		t.Fatal(err)
	}
	a := Scope{OrganizationID: org, LocationID: loc, ActorID: org}
	if _, err = s.pool.Exec(ctx, `INSERT INTO principals(id,external_subject) VALUES($1,'synthetic:public-admin')`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"customers.read", "customers.create", "catalogue.read", "catalogue.manage", "scheduling.manage", "scheduling.reserve", "scheduling.read", "scheduling.release", "bookings.create", "bookings.read", "bookings.manage", "invoices.read", "invoices.create", "invoices.issue", "invoices.manage", "payments.read", "payments.record", "marketing.manage", "content.manage"} {
		if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, a.ActorID, org, loc, action); err != nil {
			t.Fatal(err)
		}
	}
	verifyPublicBooking(t, ctx, s, a)
}
