package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestQuotationLifecycleAndFulfillment(t *testing.T) {
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

	namespace := fmt.Sprintf("dapper_quotes_test_%d", time.Now().UnixNano())
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

	var org, loc, staff, customerActor, otherStaff string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Quotes Org') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic Quotes Bay') RETURNING id::text`, org).Scan(&loc); err != nil {
		t.Fatal(err)
	}

	for _, entry := range []struct {
		name string
		id   *string
	}{
		{"synthetic-quotes-staff", &staff},
		{"synthetic-quotes-customer", &customerActor},
		{"synthetic-quotes-other-staff", &otherStaff},
	} {
		if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) RETURNING id::text`, entry.name).Scan(entry.id); err != nil {
			t.Fatal(err)
		}
	}

	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: staff}

	// Staff grants
	for _, action := range []string{
		"customers.create", "catalogue.manage", "scheduling.manage", "scheduling.reserve",
		"bookings.read", "bookings.create", "bookings.manage",
		"quotes.read", "quotes.create", "quotes.manage", "quotes.issue",
	} {
		if _, err = pool.Exec(ctx, `INSERT INTO location_grants(organization_id,location_id,principal_id,action) VALUES($1,$2,$3,$4)`, org, loc, staff, action); err != nil {
			t.Fatal(err)
		}
	}

	// Create customer and link to customer actor
	customer, err := s.CreateCustomer(ctx, scope, "Quoted Customer", "customer@example.invalid", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, customerActor, org, loc, customer.ID); err != nil {
		t.Fatal(err)
	}

	// Create vehicle
	vehicle, err := s.CreateCustomerVehicle(ctx, customerActor, customer.ID, Vehicle{
		Description:         "2024 Sedan",
		CategoryDescription: "Standard Car",
		Identifier:          "QUOTE-TEST-01",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create scheduling resource, service, rule and offer
	start := time.Now().UTC().Truncate(24 * time.Hour).Add(48 * time.Hour)
	resource, err := s.CreateSchedulingResource(ctx, scope, SchedulingResource{
		Name:     "Quote Bay 1",
		Timezone: "America/Toronto",
		Plan: scheduling.Plan{
			Capacity: 1,
			Open:     []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	service, err := s.SaveService(ctx, scope, CatalogService{Name: "Full Detail Package", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}

	rule, err := s.SaveSchedulingRule(ctx, scope, SchedulingRule{
		ResourceID:      resource.ID,
		ServiceID:       service.ID,
		DurationMinutes: 120,
		BeforeMinutes:   15,
		AfterMinutes:    15,
		Units:           1,
	})
	if err != nil {
		t.Fatal(err)
	}

	offerConfig := BookingOfferConfig{
		ResourceID:        resource.ID,
		RuleID:            rule.ID,
		RuleVersion:       1,
		Title:             "Online Full Detail",
		EquipmentCategory: "Standard Car",
		Mode:              "quote_required",
		Currency:          "CAD",
		SubtotalCents:     0,
		TaxCents:          0,
		DepositCents:      0,
		Terms:             "Test quote terms",
		CancellationTerms: "Test cancellation terms",
		LeadMinutes:       0,
		AdvanceDays:       30,
		SlotMinutes:       15,
		Published:         true,
	}
	offer, err := s.SaveBookingOffer(ctx, scope, "", 0, true, offerConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Submit customer booking request
	reqInput := CustomerBookingRequestInput{
		RequestKey:    "cust-quote-req-0001",
		OfferID:       offer.ID,
		OfferVersion:  1,
		Start:         start.Add(30 * time.Minute),
		CustomerNote:  "Please check exterior scratches",
		TermsReviewed: true,
	}
	bookingReq, err := s.SubmitCustomerBookingRequest(ctx, customerActor, customer.ID, vehicle.ID, reqInput)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("CreateQuote authorization and validation", func(t *testing.T) {
		unauthorizedScope := Scope{OrganizationID: org, LocationID: loc, ActorID: otherStaff}
		_, err := s.CreateQuote(ctx, unauthorizedScope, CreateQuoteInput{
			RequestID:  bookingReq.ID,
			CustomerID: customer.ID,
			VehicleID:  vehicle.ID,
			Items: []QuoteItemInput{
				{Description: "Interior Deep Clean", Quantity: 1, UnitPriceCents: 15000},
			},
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for actor without quotes.create, got %v", err)
		}

		// Empty items rejected
		_, err = s.CreateQuote(ctx, scope, CreateQuoteInput{
			RequestID:  bookingReq.ID,
			CustomerID: customer.ID,
			VehicleID:  vehicle.ID,
			Items:      nil,
		})
		if !errors.Is(err, ErrQuoteInput) {
			t.Fatalf("expected ErrQuoteInput for empty items, got %v", err)
		}

		// Valid draft creation with server-computed subtotal
		q, err := s.CreateQuote(ctx, scope, CreateQuoteInput{
			RequestID:     bookingReq.ID,
			CustomerID:    customer.ID,
			VehicleID:     vehicle.ID,
			InternalNotes: "Customer requested scratch buffing review",
			CustomerNote:  "Includes interior and paint correction",
			TaxCents:      2000,
			DepositCents:  5000,
			Items: []QuoteItemInput{
				{Description: "Interior Deep Clean", Quantity: 1, UnitPriceCents: 15000, ServiceID: &service.ID},
				{Description: "Paint Polish Stage 1", Quantity: 2, UnitPriceCents: 5000},
			},
		})
		if err != nil {
			t.Fatalf("failed to create quote: %v", err)
		}
		if q.Status != "draft" {
			t.Errorf("expected status 'draft', got %q", q.Status)
		}
		// Subtotal = 1*15000 + 2*5000 = 25000
		if q.SubtotalCents != 25000 {
			t.Errorf("expected subtotal 25000, got %d", q.SubtotalCents)
		}
		if q.TotalCents != 27000 {
			t.Errorf("expected total 27000, got %d", q.TotalCents)
		}
		if q.Version != 1 {
			t.Errorf("expected version 1, got %d", q.Version)
		}
		if len(q.Items) != 2 {
			t.Errorf("expected 2 items, got %d", len(q.Items))
		}
		if len(q.Events) != 1 || q.Events[0].Action != "created" {
			t.Errorf("expected 1 'created' event, got %v", q.Events)
		}

		// Customer cannot see draft quote
		custQuotes, err := s.CustomerQuotes(ctx, customerActor, customer.ID)
		if err != nil {
			t.Fatalf("customer quotes failed: %v", err)
		}
		if len(custQuotes) != 0 {
			t.Errorf("customer should not see draft quotes, got %d", len(custQuotes))
		}
	})

	t.Run("EditQuote optimistic concurrency", func(t *testing.T) {
		quotes, err := s.ListQuotes(ctx, scope)
		if err != nil || len(quotes) == 0 {
			t.Fatal("no quotes found")
		}
		draft := quotes[0]

		// Stale edit rejection
		_, err = s.EditQuote(ctx, scope, draft.ID, EditQuoteInput{
			Version: draft.Version + 99,
			Items: []QuoteItemInput{
				{Description: "Updated Item", Quantity: 1, UnitPriceCents: 10000},
			},
		})
		if !errors.Is(err, ErrQuoteStale) {
			t.Fatalf("expected ErrQuoteStale for version mismatch, got %v", err)
		}

		// Valid edit
		edited, err := s.EditQuote(ctx, scope, draft.ID, EditQuoteInput{
			Version:      draft.Version,
			TaxCents:     2600,
			DepositCents: 0,
			CustomerNote: "Updated package description",
			Items: []QuoteItemInput{
				{Description: "Full Detail Package", Quantity: 1, UnitPriceCents: 20000},
			},
		})
		if err != nil {
			t.Fatalf("failed to edit quote: %v", err)
		}
		if edited.Version != draft.Version+1 {
			t.Errorf("expected version %d, got %d", draft.Version+1, edited.Version)
		}
		if edited.SubtotalCents != 20000 {
			t.Errorf("expected subtotal 20000, got %d", edited.SubtotalCents)
		}
	})

	t.Run("IssueQuote and Customer Acceptance", func(t *testing.T) {
		quotes, err := s.ListQuotes(ctx, scope)
		if err != nil || len(quotes) == 0 {
			t.Fatal("no quotes found")
		}
		draft := quotes[0]

		// Issue the quote
		issued, err := s.IssueQuote(ctx, scope, draft.ID, IssueQuoteInput{
			Version: draft.Version,
			Note:    "Sent to customer for review",
		})
		if err != nil {
			t.Fatalf("failed to issue quote: %v", err)
		}
		if issued.Status != "issued" {
			t.Fatalf("expected status 'issued', got %q", issued.Status)
		}
		if issued.IssuedAt == nil || issued.IssuedBy == nil || *issued.IssuedBy != staff {
			t.Errorf("expected issuedAt and issuedBy set, got %v, %v", issued.IssuedAt, issued.IssuedBy)
		}

		// Cannot edit an issued quote
		_, err = s.EditQuote(ctx, scope, issued.ID, EditQuoteInput{
			Version: issued.Version,
			Items: []QuoteItemInput{
				{Description: "Illegal Edit", Quantity: 1, UnitPriceCents: 1000},
			},
		})
		if !errors.Is(err, ErrQuoteStatusConflict) {
			t.Fatalf("expected ErrQuoteStatusConflict editing issued quote, got %v", err)
		}

		// Customer now sees the issued quote
		custQuotes, err := s.CustomerQuotes(ctx, customerActor, customer.ID)
		if err != nil {
			t.Fatalf("customer quotes failed: %v", err)
		}
		if len(custQuotes) != 1 || custQuotes[0].ID != issued.ID {
			t.Fatalf("expected customer to see 1 issued quote, got %d", len(custQuotes))
		}

		// Customer acceptance without reviewing terms fails
		_, err = s.AcceptQuote(ctx, customerActor, customer.ID, issued.ID, AcceptQuoteInput{
			AcceptanceKey: "cust-accept-key-0001",
			TermsReviewed: false,
		})
		if !errors.Is(err, ErrQuoteInput) {
			t.Fatalf("expected ErrQuoteInput without terms reviewed, got %v", err)
		}

		// Other principal cannot accept customer's quote
		_, err = s.AcceptQuote(ctx, otherStaff, customer.ID, issued.ID, AcceptQuoteInput{
			AcceptanceKey: "other-accept-key-0001",
			TermsReviewed: true,
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden for non-owner actor, got %v", err)
		}

		// Customer accepts quote
		accepted, err := s.AcceptQuote(ctx, customerActor, customer.ID, issued.ID, AcceptQuoteInput{
			AcceptanceKey: "cust-accept-key-0001",
			TermsReviewed: true,
		})
		if err != nil {
			t.Fatalf("failed to accept quote: %v", err)
		}
		if accepted.Status != "accepted" {
			t.Fatalf("expected status 'accepted', got %q", accepted.Status)
		}
		if accepted.Acceptance == nil {
			t.Fatal("expected acceptance evidence record")
		}
		if accepted.Acceptance.ActorID != customerActor {
			t.Errorf("expected acceptance actor %s, got %s", customerActor, accepted.Acceptance.ActorID)
		}
		if len(accepted.Acceptance.ContentSHA256) != 64 {
			t.Errorf("expected 64-char sha256 hex, got %s", accepted.Acceptance.ContentSHA256)
		}

		// Acceptance record is immutable (DB trigger rejects update/delete)
		_, err = pool.Exec(ctx, `UPDATE scoped_quotation_acceptances SET customer_id=$1 WHERE quotation_id=$2`, customer.ID, accepted.ID)
		if err == nil {
			t.Fatal("expected DB trigger to reject acceptance update")
		}
		_, err = pool.Exec(ctx, `DELETE FROM scoped_quotation_acceptances WHERE quotation_id=$1`, accepted.ID)
		if err == nil {
			t.Fatal("expected DB trigger to reject acceptance delete")
		}

		// Idempotent retry with same acceptance key returns accepted quote
		retry, err := s.AcceptQuote(ctx, customerActor, customer.ID, issued.ID, AcceptQuoteInput{
			AcceptanceKey: "cust-accept-key-0001",
			TermsReviewed: true,
		})
		if err != nil {
			t.Fatalf("idempotent acceptance retry failed: %v", err)
		}
		if retry.Status != "accepted" {
			t.Errorf("expected status 'accepted', got %q", retry.Status)
		}
	})

	t.Run("FulfillQuote atomic booking conversion", func(t *testing.T) {
		quotes, err := s.ListQuotes(ctx, scope)
		if err != nil || len(quotes) == 0 {
			t.Fatal("no quotes found")
		}
		acceptedQuote := quotes[0]
		if acceptedQuote.Status != "accepted" {
			t.Fatalf("quote must be in 'accepted' status, got %q", acceptedQuote.Status)
		}

		// Staff fulfills accepted quote to booking
		fulfillResult, err := s.FulfillQuote(ctx, scope, FulfillQuoteInput{
			RequestKey:              "staff-fulfill-key-0001",
			QuoteID:                 acceptedQuote.ID,
			ApprovedNoDepositConfig: true,
			Channel:                 "online",
			Notes:                   "Converted from quote " + acceptedQuote.ReferenceNumber,
		})
		if err != nil {
			t.Fatalf("failed to fulfill quote: %v", err)
		}

		if fulfillResult.Quote.Status != "fulfilled" {
			t.Errorf("expected quote status 'fulfilled', got %q", fulfillResult.Quote.Status)
		}
		if fulfillResult.Quote.BookingID == nil {
			t.Fatal("expected quote.BookingID to be set")
		}
		if fulfillResult.Booking.ID != *fulfillResult.Quote.BookingID {
			t.Errorf("expected booking ID match, got %s vs %s", fulfillResult.Booking.ID, *fulfillResult.Quote.BookingID)
		}
		if fulfillResult.Booking.Status != "confirmed" {
			t.Errorf("expected booking status 'confirmed', got %q", fulfillResult.Booking.Status)
		}
		if fulfillResult.Booking.ReservationID == nil {
			t.Fatal("expected capacity reservation ID on confirmed booking")
		}

		// Second fulfillment on same quote fails
		_, err = s.FulfillQuote(ctx, scope, FulfillQuoteInput{
			RequestKey:              "staff-fulfill-key-0002",
			QuoteID:                 acceptedQuote.ID,
			ApprovedNoDepositConfig: true,
		})
		if !errors.Is(err, ErrQuoteAlreadyFulfilled) {
			t.Fatalf("expected ErrQuoteAlreadyFulfilled on second fulfillment, got %v", err)
		}

		// Replay with exact same requestKey returns the existing booking idempotently
		replay, err := s.FulfillQuote(ctx, scope, FulfillQuoteInput{
			RequestKey:              "staff-fulfill-key-0001",
			QuoteID:                 acceptedQuote.ID,
			ApprovedNoDepositConfig: true,
			Channel:                 "online",
			Notes:                   "Converted from quote " + acceptedQuote.ReferenceNumber,
		})
		if err != nil {
			t.Fatalf("idempotent fulfillment replay failed: %v", err)
		}
		if replay.Booking.ID != fulfillResult.Booking.ID {
			t.Errorf("expected same booking ID on replay, got %s vs %s", replay.Booking.ID, fulfillResult.Booking.ID)
		}

		// Verification: exactly one fulfillment receipt exists in DB
		var receiptCount int
		err = pool.QueryRow(ctx, `SELECT count(*) FROM quote_fulfillment_receipts WHERE quotation_id=$1`, acceptedQuote.ID).Scan(&receiptCount)
		if err != nil || receiptCount != 1 {
			t.Errorf("expected exactly 1 receipt in DB, got %d (err: %v)", receiptCount, err)
		}

		// Verification: booking has quotation_id linked
		var linkedQuoteID string
		err = pool.QueryRow(ctx, `SELECT quotation_id::text FROM bookings WHERE id=$1`, fulfillResult.Booking.ID).Scan(&linkedQuoteID)
		if err != nil || linkedQuoteID != acceptedQuote.ID {
			t.Errorf("expected booking.quotation_id = %s, got %s (err: %v)", acceptedQuote.ID, linkedQuoteID, err)
		}
	})

	t.Run("Quote expiration projection and rejection", func(t *testing.T) {
		req2, err := s.SubmitCustomerBookingRequest(ctx, customerActor, customer.ID, vehicle.ID, CustomerBookingRequestInput{
			RequestKey:    "cust-quote-req-0002",
			OfferID:       offer.ID,
			OfferVersion:  1,
			Start:         start.Add(180 * time.Minute),
			CustomerNote:  "Expiring quote test",
			TermsReviewed: true,
		})
		if err != nil {
			t.Fatal(err)
		}

		expFuture := time.Now().UTC().Add(10 * time.Minute)
		expQuote, err := s.CreateQuote(ctx, scope, CreateQuoteInput{
			RequestID:     req2.ID,
			CustomerID:    customer.ID,
			VehicleID:     vehicle.ID,
			InternalNotes: "Will expire soon",
			ExpiresAt:     &expFuture,
			Items: []QuoteItemInput{
				{Description: "Quick Detail", Quantity: 1, UnitPriceCents: 5000},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		issuedExp, err := s.IssueQuote(ctx, scope, expQuote.ID, IssueQuoteInput{
			Version: expQuote.Version,
			Note:    "Issued for expiration test",
		})
		if err != nil {
			t.Fatal(err)
		}
		if issuedExp.Status != "issued" {
			t.Fatalf("expected issued status, got %s", issuedExp.Status)
		}

		// Fast-forward expiration in database
		if _, err = pool.Exec(ctx, `UPDATE scoped_quotations SET expires_at=now() - interval '10 minutes' WHERE id=$1`, issuedExp.ID); err != nil {
			t.Fatal(err)
		}

		// Read-time projection on GetQuote
		readQ, err := s.GetQuote(ctx, scope, issuedExp.ID)
		if err != nil {
			t.Fatal(err)
		}
		if readQ.Status != "expired" {
			t.Errorf("expected projected status 'expired', got %q", readQ.Status)
		}

		// Read-time projection on CustomerQuotes
		custQs, err := s.CustomerQuotes(ctx, customerActor, customer.ID)
		if err != nil {
			t.Fatal(err)
		}
		foundExp := false
		for _, cq := range custQs {
			if cq.ID == issuedExp.ID {
				foundExp = true
				if cq.Status != "expired" {
					t.Errorf("expected customer projected status 'expired', got %q", cq.Status)
				}
			}
		}
		if !foundExp {
			t.Error("expected expired quote in customer quote list")
		}

		// Accept on expired quote returns ErrQuoteExpired
		_, err = s.AcceptQuote(ctx, customerActor, customer.ID, issuedExp.ID, AcceptQuoteInput{
			AcceptanceKey: "cust-exp-accept-0001",
			TermsReviewed: true,
		})
		if !errors.Is(err, ErrQuoteExpired) {
			t.Errorf("expected ErrQuoteExpired on AcceptQuote, got %v", err)
		}

		// Decline on expired quote returns ErrQuoteExpired
		_, err = s.DeclineQuote(ctx, customerActor, customer.ID, issuedExp.ID, DeclineQuoteInput{
			AcceptanceKey: "cust-exp-decline-0001",
			Note:          "Too late",
		})
		if !errors.Is(err, ErrQuoteExpired) {
			t.Errorf("expected ErrQuoteExpired on DeclineQuote, got %v", err)
		}

		// Fulfill on non-accepted quote returns ErrQuoteStatusConflict
		_, err = s.FulfillQuote(ctx, scope, FulfillQuoteInput{
			RequestKey:              "staff-fulfill-exp-0001",
			QuoteID:                 issuedExp.ID,
			ApprovedNoDepositConfig: true,
		})
		if !errors.Is(err, ErrQuoteStatusConflict) {
			t.Errorf("expected ErrQuoteStatusConflict on FulfillQuote, got %v", err)
		}
	})
}
