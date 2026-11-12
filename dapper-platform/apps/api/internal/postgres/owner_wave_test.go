package postgres

import (
	"dapper.local/platform/internal/scheduling"
	"bytes"
	"strings"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Most older tests build their own invoices for bookings, so the automatic draft invoice is switched off for them; this test turns it on.
func init() { autoInvoiceEnabled = false }

func TestOwnerWaveFeatures(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_wave_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if _, e := admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error(e)
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var org, actor string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Wave Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('sub_wave_actor', true) RETURNING id::text`).Scan(&actor))
	_, err = pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id, principal_id) VALUES($1, $2)`, org, actor)
	must(err)
	for _, action := range []string{"structure.read", "structure.manage", "catalogue.read", "catalogue.manage", "invoices.configure"} {
		_, err = pool.Exec(ctx, `INSERT INTO organization_grants(organization_id, principal_id, action) VALUES($1, $2, $3)`, org, actor, action)
		must(err)
	}
	fran, err := s.CreateFranchise(ctx, actor, org, FranchiseInput{Code: "WAVE", Name: "Wave Franchise", Address: testAddress})
	must(err)
	loc := fran.ID
	for _, action := range []string{"customers.read", "customers.create", "vehicles.read", "vehicles.create", "catalogue.read", "catalogue.manage", "scheduling.manage", "scheduling.reserve", "bookings.create", "bookings.read", "dealership.manage", "dealership.read"} {
		_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id, organization_id, location_id, action) VALUES($1, $2, $3, $4)`, actor, org, loc, action)
		must(err)
	}
	scope := Scope{ActorID: actor, OrganizationID: org, LocationID: loc}
	customer, err := s.CreateCustomer(ctx, scope, "Synthetic Wave Customer", "wave@example.invalid", "+15555550111")
	must(err)

	t.Run("a service duration becomes a booking rule, and slots set the capacity", func(t *testing.T) {
		minutes := 90
		svc, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic Wave Detail", Kind: "main", DurationMinutes: &minutes})
		must(err)
		if svc.DurationMinutes == nil || *svc.DurationMinutes != 90 {
			t.Fatalf("duration not stored: %+v", svc)
		}
		must(s.SaveBookingSlots(ctx, scope, 3, "America/Toronto"))
		var capacity, rules int
		must(pool.QueryRow(ctx, `SELECT capacity FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&capacity))
		must(pool.QueryRow(ctx, `SELECT count(*) FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND service_id=$3 AND duration_minutes=90 AND addon_ids='[]'::jsonb`, org, loc, svc.ID).Scan(&rules))
		if capacity != 3 || rules != 1 {
			t.Fatalf("slots %d, rules %d", capacity, rules)
		}
		untimed, err := s.SaveService(ctx, scope, CatalogService{Name: "Synthetic untimed service", Kind: "main"})
		must(err)
		_, err = pool.Exec(ctx, `INSERT INTO service_scheduling_rules(organization_id,location_id,resource_id,service_id,addon_ids,duration_minutes,before_minutes,after_minutes,units,working_time,slots) SELECT $1::uuid,$2::uuid,id,$3::uuid,'[]'::jsonb,47,5,0,1,false,2 FROM scheduling_resources WHERE organization_id=$1::uuid AND location_id=$2::uuid`, org, loc, untimed.ID)
		must(err)
		minutes = 120
		start := time.Now().UTC().Truncate(time.Hour).Add(72 * time.Hour)
		if _, err = s.CreateSchedulingResource(ctx, scope, SchedulingResource{Name: "Synthetic second bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}}}}); err != nil {
			t.Fatal(err)
		}
		var bays, bookable int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&bays))
		must(pool.QueryRow(ctx, `SELECT count(*) FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND service_id=$3 AND addon_ids='[]'::jsonb`, org, loc, svc.ID).Scan(&bookable))
		if bays < 2 || bookable != bays {
			t.Fatalf("a new bay must take the franchise's services: %d bays, %d rules", bays, bookable)
		}
		var copied int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND service_id=$3 AND addon_ids='[]'::jsonb AND duration_minutes=47 AND before_minutes=5`, org, loc, untimed.ID).Scan(&copied))
		if copied != 2 {
			t.Fatalf("a service with no typed duration must be bookable on the new bay: %d rules", copied)
		}
		svc, err = s.SaveService(ctx, scope, CatalogService{ID: svc.ID, Version: svc.Version, Name: svc.Name, Kind: "main", DurationMinutes: &minutes})
		must(err)
		var version int
		must(pool.QueryRow(ctx, `SELECT version FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND service_id=$3 AND duration_minutes=120`, org, loc, svc.ID).Scan(&version))
		if version != 2 {
			t.Fatalf("changed duration should revise the rule, version %d", version)
		}
		if err = s.SaveBookingSlots(ctx, scope, 0, "America/Toronto"); err == nil {
			t.Fatal("zero slots accepted")
		}
	})

	t.Run("a central service carries its duration to every franchise", func(t *testing.T) {
		minutes := 45
		central, err := s.SaveCentralService(ctx, actor, org, CentralServiceInput{Name: "Synthetic Central Wash", Kind: "main", DurationSet: true, DurationMinutes: &minutes})
		must(err)
		var local *int
		must(pool.QueryRow(ctx, `SELECT duration_minutes FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND central_service_id=$3`, org, loc, central.ID).Scan(&local))
		if local == nil || *local != 45 {
			t.Fatalf("franchise copy has duration %v", local)
		}
		var rules int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND duration_minutes=45`, org, loc).Scan(&rules))
		if rules != 2 { // one for each of this franchise's two bays
			t.Fatalf("central duration did not create the franchise rule: %d", rules)
		}
	})

	t.Run("a booking raises a draft invoice and is visible to staff", func(t *testing.T) {
		autoInvoiceEnabled = true
		defer func() { autoInvoiceEnabled = false }()
		booking, err := s.CreateBooking(ctx, scope, Booking{CustomerID: customer.ID, ServiceName: "Synthetic automatic invoice", Channel: "phone", BookingTime: time.Now().Add(48 * time.Hour)})
		must(err)
		var count, lines int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM invoices WHERE booking_id=$1 AND status='draft'`, booking.ID).Scan(&count))
		must(pool.QueryRow(ctx, `SELECT count(*) FROM invoice_items WHERE invoice_id=(SELECT id FROM invoices WHERE booking_id=$1)`, booking.ID).Scan(&lines))
		if count != 1 || lines != 1 {
			t.Fatalf("draft invoices %d, lines %d", count, lines)
		}
		var linked int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM employee_booking_worksites WHERE booking_id=$1 AND dealership_id IS NULL`, booking.ID).Scan(&linked))
		if linked != 1 {
			t.Fatal("booking is not part of the location's staff work")
		}
	})

	t.Run("dealership per-car rate and several contacts", func(t *testing.T) {
		rate := int64(4500)
		dealer, err := s.CreateDealershipAccount(ctx, scope, CreateDealershipInput{CustomerID: customer.ID, AccountName: "Synthetic Motors", AccountCode: "SYNMOT", AccountType: "dealership", PerCarRateCents: &rate,
			Contacts: []DealershipContact{{Name: "Synthetic Sales", Email: "sales@example.invalid", Phone: "+15555550112"}, {Name: "Synthetic Service", Email: "service@example.invalid"}}})
		must(err)
		if dealer.PerCarRateCents == nil || *dealer.PerCarRateCents != 4500 || len(dealer.Contacts) != 2 || dealer.ContactName != "Synthetic Sales" || dealer.TaxExempt || dealer.BillingTermsDays != 30 {
			t.Fatalf("dealership not saved as expected: %+v", dealer)
		}
		if listed, derr := s.CustomerDirectory(ctx, scope); derr != nil {
			t.Fatal(derr)
		} else {
			for _, c := range listed {
				if c.ID == customer.ID {
					t.Fatal("a dealership's customer record appears in the customers list")
				}
			}
		}
		if _, err = s.CreateDealershipAccount(ctx, scope, CreateDealershipInput{CustomerID: customer.ID, AccountName: "Synthetic No Contact", AccountCode: "NOCON", AccountType: "dealership"}); err == nil {
			t.Fatal("a dealership with no contact person was accepted")
		}
		updated, err := s.UpdateDealershipAccount(ctx, scope, dealer.ID, UpdateDealershipInput{AccountName: "Synthetic Motors", Active: true, Contacts: []DealershipContact{{Name: "Synthetic Manager", Email: "manager@example.invalid"}}})
		must(err)
		if len(updated.Contacts) != 1 || updated.ContactName != "Synthetic Manager" || updated.PerCarRateCents != nil || updated.BillingTermsDays != 30 {
			t.Fatalf("update did not replace contacts or keep terms: %+v", updated)
		}
		list, err := s.ListDealershipAccounts(ctx, scope)
		must(err)
		if len(list) != 1 || len(list[0].Contacts) != 1 {
			t.Fatalf("list lost the contacts: %+v", list)
		}

		// Dealership invoices: VIN / stock lines, own number series, due date from terms, filters, PDF, and the customer invoice series untouched.
		for _, action := range []string{"invoices.read", "invoices.create", "invoices.manage"} {
			_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id, organization_id, location_id, action) VALUES($1, $2, $3, $4) ON CONFLICT DO NOTHING`, actor, org, loc, action)
			must(err)
		}
		for _, bad := range []DealerInvoiceInput{{DealershipID: dealer.ID}, {DealershipID: dealer.ID, InvoiceDate: "2026-11-10", DueDate: "2026-11-01", Lines: []DealerInvoiceLine{{Description: "Detail", Quantity: 1, UnitPriceCents: 4500}}},
			{DealershipID: dealer.ID, DiscountCents: 5000, Lines: []DealerInvoiceLine{{Description: "Detail", Quantity: 1, UnitPriceCents: 4500}}},
			{DealershipID: dealer.ID, DiscountCents: 100, DiscountPercent: 5, Lines: []DealerInvoiceLine{{Description: "Detail", Quantity: 1, UnitPriceCents: 4500}}},
			{DealershipID: dealer.ID, InvoiceNumber: "#bad", Lines: []DealerInvoiceLine{{Description: "Detail", Quantity: 1, UnitPriceCents: 4500}}},
			{DealershipID: dealer.ID, Lines: []DealerInvoiceLine{{Description: "Detail", Quantity: 1, UnitPriceCents: 4500, VIN: "1HG-CM82633A"}}}, {DealershipID: customer.ID, Lines: []DealerInvoiceLine{{Description: "Detail", Quantity: 1, UnitPriceCents: 4500}}}} {
			if _, err = s.CreateDealerInvoice(ctx, scope, bad); !errors.Is(err, ErrDealerInvoiceInput) {
				t.Fatalf("bad dealership invoice accepted: %+v %v", bad, err)
			}
		}
		invDay, dueDay := dealerToday().AddDate(0, 0, -9), dealerToday().AddDate(0, 0, 39)
		draft, err := s.CreateDealerInvoice(ctx, scope, DealerInvoiceInput{DealershipID: dealer.ID, Notes: "Synthetic lot", Lines: []DealerInvoiceLine{
			{Description: "Lot detail", Quantity: 1, UnitPriceCents: 4500, VIN: "1hgcm82633a004352", StockNumber: "A-17"}, {Description: "Lot detail", Quantity: 1, UnitPriceCents: 4500, StockNumber: "A-18"}}})
		must(err)
		if draft.Status != "draft" || draft.SubtotalCents != 9000 || len(draft.Lines) != 2 || draft.Lines[0].VIN != "1HGCM82633A004352" || draft.Lines[1].StockNumber != "A-18" || draft.InvoiceDate == "" || draft.DueDate == "" || draft.DealershipName != "Synthetic Motors" {
			t.Fatalf("dealership draft wrong: %+v", draft)
		}
		draft, err = s.UpdateDealerInvoice(ctx, scope, draft.ID, DealerInvoiceInput{DealershipID: dealer.ID, InvoiceDate: invDay.Format("2006-01-02"), DueDate: dueDay.Format("2006-01-02"), PONumber: "PO-7781", DiscountPercent: 10, Version: draft.Version, Lines: []DealerInvoiceLine{{Description: "Lot detail", Detail: "Interior and exterior", Quantity: 3, UnitPriceCents: 4500, VIN: "JH4KA8260MC000000"}}})
		must(err)
		if len(draft.Lines) != 1 || draft.Lines[0].VIN != "JH4KA8260MC000000" || draft.SubtotalCents != 13500 || draft.DiscountCents != 1350 || draft.TotalCents != 13500-1350+draft.TaxCents || draft.PONumber != "PO-7781" || draft.Lines[0].Detail != "Interior and exterior" || draft.DueDate != dueDay.Format("2006-01-02") {
			t.Fatalf("dealership draft not updated: %+v", draft)
		}
		issued := issueInvoiceForTest(t, ctx, s, scope, draft.Invoice)
		if issued.InvoiceNumber != "DLR-1" || issued.DueAt == nil || issued.IssuedAt == nil || issued.DueAt.UTC().Format("2006-01-02") != dueDay.AddDate(0, 0, 1).Format("2006-01-02") {
			t.Fatalf("dealership invoice not in its own series or without its due date: %s %v", issued.InvoiceNumber, issued.DueAt)
		}
		plain, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: customer.ID, Items: []CreateInvoiceItemInput{{Description: "Walk-in detail", Quantity: 1, UnitPriceCents: 1000}}})
		must(err)
		if p := issueInvoiceForTest(t, ctx, s, scope, plain); p.InvoiceNumber != "TEST-1" || p.DueAt != nil {
			t.Fatalf("customer invoice took the dealership series or a due date: %s", p.InvoiceNumber)
		}
		must2 := func(items []DealerInvoiceSummary, err error) []DealerInvoiceSummary { t.Helper(); must(err); return items }
		if got := must2(s.ListDealerInvoices(ctx, scope, DealerInvoiceFilter{DealershipID: dealer.ID, Status: "issued"})); len(got) != 1 || got[0].InvoiceNumber != "DLR-1" {
			t.Fatalf("filter by dealership and status wrong: %+v", got)
		}
		if got := must2(s.ListDealerInvoices(ctx, scope, DealerInvoiceFilter{Status: "overdue"})); len(got) != 0 {
			t.Fatal("a new invoice counts as overdue")
		}
		_, err = pool.Exec(ctx, `UPDATE invoices SET due_at=now()-interval '1 day' WHERE id=$1`, issued.ID)
		must(err)
		if got := must2(s.ListDealerInvoices(ctx, scope, DealerInvoiceFilter{Status: "overdue", From: "2000-01-01", To: "2999-12-31"})); len(got) != 1 || !got[0].Overdue {
			t.Fatalf("overdue filter wrong: %+v", got)
		}
		if got := must2(s.ListDealerInvoices(ctx, scope, DealerInvoiceFilter{To: "2000-01-01"})); len(got) != 0 {
			t.Fatal("date filter ignored")
		}
		if _, err = s.ListDealerInvoices(ctx, scope, DealerInvoiceFilter{Status: "lost"}); !errors.Is(err, ErrDealerInvoiceInput) {
			t.Fatal("unknown status accepted")
		}
		full, err := s.GetDealerInvoice(ctx, scope, issued.ID)
		must(err)
		if name, pdf := s.InvoicePDF(ctx, full.Invoice); name != "Invoice-DLR-1.pdf" || !bytes.Contains(pdf, []byte("JH4KA8260MC000000")) || !bytes.Contains(pdf, []byte("Due:")) || !bytes.Contains(pdf, []byte("PO-7781")) || !bytes.Contains(pdf, []byte("Discount \\(10%\\)")) || !bytes.Contains(pdf, []byte(invDay.Format("02/Jan/2006"))) {
			t.Fatalf("dealership PDF missing its VIN or due date: %s", name)
		}
		if _, err = s.GetDealerInvoice(ctx, scope, plain.ID); !errors.Is(err, ErrInvoiceNotFound) {
			t.Fatal("a customer invoice was served as a dealership invoice")
		}
		// A typed number is used as it is, must be free, and leaves the series alone.
		typed, err := s.CreateDealerInvoice(ctx, scope, DealerInvoiceInput{DealershipID: dealer.ID, InvoiceNumber: "HY-2026-01", Lines: []DealerInvoiceLine{{Description: "Lot detail", Quantity: 1, UnitPriceCents: 4500}}})
		must(err)
		if _, err = s.CreateDealerInvoice(ctx, scope, DealerInvoiceInput{DealershipID: dealer.ID, InvoiceNumber: "HY-2026-01", Lines: []DealerInvoiceLine{{Description: "Lot detail", Quantity: 1, UnitPriceCents: 4500}}}); !errors.Is(err, ErrDealerNumberTaken) {
			t.Fatal("a typed number already on a draft was accepted twice", err)
		}
		if got := issueInvoiceForTest(t, ctx, s, scope, typed.Invoice); got.InvoiceNumber != "HY-2026-01" {
			t.Fatalf("typed number not used: %s", got.InvoiceNumber)
		}
		dup, err := s.CreateDealerInvoice(ctx, scope, DealerInvoiceInput{DealershipID: dealer.ID, Lines: []DealerInvoiceLine{{Description: "Lot detail", Quantity: 1, UnitPriceCents: 4500}}})
		must(err)
		_, err = pool.Exec(ctx, `UPDATE invoices SET manual_number='DLR-1' WHERE id=$1`, dup.ID)
		must(err)
		if _, err = s.IssueInvoice(ctx, scope, dup.ID); !errors.Is(err, ErrDealerNumberTaken) {
			t.Fatal("an issued number was reused", err)
		}
		// The pay link opens only its own invoice, and nothing else.
		token, err := s.CreateInvoicePayLink(ctx, scope, issued.ID)
		must(err)
		if _, err = s.CreateInvoicePayLink(ctx, scope, dup.ID); !errors.Is(err, ErrInvoiceNotFound) {
			t.Fatal("a pay link was made for a draft", err)
		}
		if _, got, err := s.PayLinkInvoice(ctx, token); err != nil || got.ID != issued.ID {
			t.Fatalf("pay link did not open its invoice: %v", err)
		}
		if _, _, err = s.PayLinkInvoice(ctx, token+"x"); !errors.Is(err, ErrPayLink) {
			t.Fatal("a wrong pay link opened an invoice", err)
		}
		// #25: dealership and customer invoices are numbered apart, so a customer invoice may carry a dealership number.
		_, err = s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: customer.ID, InvoiceNumber: "HY-2026-01", Items: []CreateInvoiceItemInput{{Description: "Walk-in detail", Quantity: 1, UnitPriceCents: 1000}}})
		must(err)
		// #24: the editor's running totals use the same tax as saving.
		pv, err := s.PreviewDealerInvoice(ctx, scope, DealerInvoiceInput{DiscountPercent: 10, Lines: []DealerInvoiceLine{{Quantity: 2, UnitPriceCents: 4500}}})
		must(err)
		wantTax, _, err := s.taxOnSubtotal(ctx, scope, 8100)
		must(err)
		if pv.SubtotalCents != 9000 || pv.DiscountCents != 900 || pv.TaxCents != wantTax || pv.TotalCents != 8100+wantTax {
			t.Fatalf("preview wrong: %+v (tax %d)", pv, wantTax)
		}
		// #23: an unpaid issued invoice goes back to a draft with its number, is edited and issued again under the same number.
		reopened, err := s.ReopenDealerInvoice(ctx, scope, typed.ID)
		must(err)
		if reopened.Status != "draft" || reopened.ManualNumber != "HY-2026-01" {
			t.Fatalf("reopen wrong: %s %q", reopened.Status, reopened.ManualNumber)
		}
		edited, err := s.UpdateDealerInvoice(ctx, scope, typed.ID, DealerInvoiceInput{DealershipID: dealer.ID, InvoiceNumber: "HY-2026-01", Version: reopened.Version, Lines: []DealerInvoiceLine{{Description: "Lot detail", Quantity: 2, UnitPriceCents: 4500}}})
		must(err)
		if again := issueInvoiceForTest(t, ctx, s, scope, edited.Invoice); again.InvoiceNumber != "HY-2026-01" || again.SubtotalCents != 9000 {
			t.Fatalf("re-issued invoice wrong: %s %d", again.InvoiceNumber, again.SubtotalCents)
		}
		if _, err = s.ReopenDealerInvoice(ctx, scope, dup.ID); !errors.Is(err, ErrInvoiceStatusConflict) {
			t.Fatal("a draft was reopened", err)
		}
		// #26/#30/#31: the dealership portal, by login and by the email link, shows only this dealership's issued invoices.
		for _, action := range []string{"dealership.read", "dealership.manage"} {
			_, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id, organization_id, location_id, action) VALUES($1, $2, $3, $4) ON CONFLICT DO NOTHING`, actor, org, loc, action)
			must(err)
		}
		if _, err = s.CreateDealerPortalUser(ctx, scope, dealer.ID, "Synthetic Buyer", "buyer@example.test", "short"); !errors.Is(err, ErrPortalInput) {
			t.Fatal("a short portal password was accepted", err)
		}
		_, err = s.CreateDealerPortalUser(ctx, scope, dealer.ID, "Synthetic Buyer", "Buyer@Example.test", "portal-pass-123")
		must(err)
		if _, err = s.DealerPortalSignIn(ctx, "buyer@example.test", "wrong-pass-123"); !errors.Is(err, ErrPortalSession) {
			t.Fatal("a wrong portal password was accepted", err)
		}
		session, err := s.DealerPortalSignIn(ctx, "buyer@example.test", "portal-pass-123")
		must(err)
		who, err := s.DealerPortalWho(ctx, session)
		must(err)
		if who.DealershipID != dealer.ID || who.ViaLink || who.UserName != "Synthetic Buyer" {
			t.Fatalf("portal session wrong: %+v", who)
		}
		mine, err := s.PortalInvoices(ctx, who)
		must(err)
		seen := map[string]bool{}
		for _, x := range mine {
			seen[x.ID] = true
		}
		if !seen[issued.ID] || seen[dup.ID] {
			t.Fatal("portal list shows drafts or misses issued invoices")
		}
		if _, err = s.DealerPortalInvoice(ctx, who, dup.ID); !errors.Is(err, ErrInvoiceNotFound) {
			t.Fatal("portal opened a draft", err)
		}
		if _, err = s.DealerPortalInvoice(ctx, who, plain.ID); !errors.Is(err, ErrInvoiceNotFound) {
			t.Fatal("portal opened a customer invoice", err)
		}
		// #44: the dealership opening it in the portal marks it Viewed; the owner's list can filter by it.
		if got := must2(s.ListDealerInvoices(ctx, scope, DealerInvoiceFilter{Status: "viewed"})); len(got) != 0 {
			t.Fatal("an unopened invoice counted as viewed")
		}
		if v, err := s.DealerPortalInvoice(ctx, who, issued.ID); err != nil || v.ViewedAt == nil {
			t.Fatal("opening in the portal did not mark it viewed", err)
		}
		if got := must2(s.ListDealerInvoices(ctx, scope, DealerInvoiceFilter{Status: "viewed"})); len(got) != 1 || got[0].ID != issued.ID {
			t.Fatalf("viewed filter wrong: %+v", got)
		}
		linkSession, linkInvoice, err := s.DealerPortalFromLink(ctx, token)
		must(err)
		if lw, err := s.DealerPortalWho(ctx, linkSession); err != nil || linkInvoice != issued.ID || !lw.ViaLink || lw.DealershipID != dealer.ID {
			t.Fatalf("email link did not open the portal: %v", err)
		}
		st, err := s.PortalStatement(ctx, who, "2000-01-01", "2999-12-31", "all")
		must(err)
		if len(st.Rows) < 2 || st.BalanceCents <= 0 || st.Aging.Current+st.Aging.Days30+st.Aging.Days60+st.Aging.Days90+st.Aging.Over90 != st.BalanceCents {
			t.Fatalf("statement wrong: %+v", st)
		}
		if _, err = s.PortalStatement(ctx, who, "2026-12-01", "2026-01-01", "all"); !errors.Is(err, ErrPortalInput) {
			t.Fatal("a backwards date range was accepted")
		}
		if name, pdf := s.PortalStatementPDF(ctx, who, st, true, true); !strings.HasPrefix(name, "Statement-") || !bytes.Contains(pdf, []byte("Balance Due")) || !bytes.Contains(pdf, []byte("Over 90 days")) {
			t.Fatalf("statement PDF wrong: %s", name)
		}
		must(s.DealerPortalSignOut(ctx, session))
		if _, err = s.DealerPortalWho(ctx, session); !errors.Is(err, ErrPortalSession) {
			t.Fatal("portal session survived sign out")
		}
		current, err := s.DealerSeries(ctx, actor, org)
		must(err)
		if current.Prefix != "DLR-" || current.NextNumber != 2 || current.Version != 1 {
			t.Fatalf("series did not move on after issuing: %+v", current)
		}
		if _, err = s.SaveDealerSeries(ctx, actor, org, DealerInvoiceSeries{Prefix: "FLEET-", NextNumber: 500}); !errors.Is(err, ErrInvoiceVersion) {
			t.Fatal("a stale series edit was accepted", err)
		}
		series, err := s.SaveDealerSeries(ctx, actor, org, DealerInvoiceSeries{Prefix: "fleet-", NextNumber: 500, Version: current.Version})
		must(err)
		if series.Prefix != "FLEET-" || series.NextNumber != 500 {
			t.Fatalf("series not saved: %+v", series)
		}
	})

	t.Run("square application keys are saved sealed and never read back", func(t *testing.T) {
		if err := s.SaveSquareAppCredentials(ctx, actor, org, "sandbox", "sandbox-app-id", "synthetic-secret-value"); err == nil {
			t.Fatal("keys saved without the server key")
		}
		must(s.SetMFAKey(make([]byte, 32)))
		must(s.SaveSquareAppCredentials(ctx, actor, org, "sandbox", "sandbox-app-id", "synthetic-secret-value"))
		items, err := s.SquareAppStatuses(ctx, actor, org)
		must(err)
		if len(items) != 2 || !items[0].Configured || items[0].ApplicationID != "sandbox-app-id" || items[0].Source != "saved" || items[1].Configured {
			t.Fatalf("statuses %+v", items)
		}
		if !s.SquareOAuthAvailable("sandbox") || s.SquareOAuthAvailable("production") {
			t.Fatal("saved keys are not in use")
		}
		var sealed []byte
		must(pool.QueryRow(ctx, `SELECT secret_sealed FROM square_app_credentials WHERE environment='sandbox'`).Scan(&sealed))
		if string(sealed) == "synthetic-secret-value" || len(sealed) < 29 {
			t.Fatal("secret is not sealed")
		}
		// A restart loads them again.
		s.setSquareApp("sandbox", "", "")
		must(s.LoadSquareAppCredentials(ctx))
		if !s.SquareOAuthAvailable("sandbox") {
			t.Fatal("saved keys not reloaded")
		}
		if err = s.SaveSquareAppCredentials(ctx, "00000000-0000-4000-8000-000000000000", org, "production", "live-app-id", "synthetic-secret-value"); err == nil {
			t.Fatal("a stranger saved keys")
		}
		must(s.ClearSquareAppCredentials(ctx, actor, org, "sandbox"))
		if s.SquareOAuthAvailable("sandbox") {
			t.Fatal("cleared keys still in use")
		}
		// #50: a franchise's own Square app keys are used for it, and only for it; removing them goes back to the shared ones.
		must(s.SaveSquareAppCredentials(ctx, actor, org, "sandbox", "shared-app-id", "synthetic-secret-value"))
		must(s.SaveLocationSquareApp(ctx, actor, org, loc, "sandbox", "franchise-app-id", "franchise-secret-value"))
		if app, ok := s.squareAppFor(ctx, org, loc, "sandbox"); !ok || app.ApplicationID != "franchise-app-id" || app.Secret != "franchise-secret-value" {
			t.Fatalf("franchise app not used: %+v", app)
		}
		if id := s.SquareApplicationIDFor(ctx, Scope{OrganizationID: org, LocationID: loc}, "sandbox"); id != "franchise-app-id" {
			t.Fatalf("card form app %q", id)
		}
		// Owner 2026-11-13: no shared fallback; a franchise without its own keys takes no cards.
		if id := s.SquareApplicationIDFor(ctx, Scope{OrganizationID: org, LocationID: "00000000-0000-4000-8000-000000000000"}, "sandbox"); id != "" {
			t.Fatalf("another franchise got someone else's app: %q", id)
		}
		// Keys from the wrong mode, or a live ID on the test side, are refused before Square is asked.
		for _, k := range [][3]string{{"sandbox", "sq0idp-live-id", "EAAAsyntheticsyntheticsynthetic"}, {"production", "sandbox-sq0idb-x", "EAAAsyntheticsyntheticsynthetic"}, {"sandbox", "sandbox-sq0idb-x", "short"}, {"staging", "sandbox-sq0idb-x", "EAAAsyntheticsyntheticsynthetic"}} {
			if err := s.ConnectSquareKeys(ctx, actor, org, loc, k[0], k[1], k[2]); !errors.Is(err, ErrSquareKeysInput) {
				t.Fatalf("keys %v: %v", k, err)
			}
		}
		apps, err := s.LocationSquareApps(ctx, actor, org, loc)
		must(err)
		if apps[0].Source != "franchise" || apps[0].ApplicationID != "franchise-app-id" || apps[1].Configured {
			t.Fatalf("franchise statuses %+v", apps)
		}
		must(s.ClearLocationSquareApp(ctx, actor, org, loc, "sandbox"))
		if app, _ := s.squareAppFor(ctx, org, loc, "sandbox"); app.ApplicationID != "shared-app-id" {
			t.Fatalf("cleared franchise keys still in use: %+v", app)
		}
		must(s.ClearSquareAppCredentials(ctx, actor, org, "sandbox"))
	})
}
