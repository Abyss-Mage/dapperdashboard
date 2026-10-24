package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/scheduling"
	"dapper.local/platform/internal/square"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyQuotationHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, handler http.Handler, actor, org, loc, customer, token, origin string) {
	t.Helper()
	scope := postgres.Scope{ActorID: actor, OrganizationID: org, LocationID: loc}

	// Create customer vehicle
	var custActor string
	if err := setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('http-quote-customer-actor') RETURNING id::text`).Scan(&custActor); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, custActor, org, loc, customer); err != nil {
		t.Fatal(err)
	}
	custSession, err := store.IssueSession(ctx, custActor, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	vehicle, err := store.CreateCustomerVehicle(ctx, custActor, customer, postgres.Vehicle{
		Description:         "HTTP Test Vehicle",
		CategoryDescription: "Standard Car",
		Identifier:          "HTTP-QUOTE-01",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create scheduling resource, service, rule and offer for the booking request
	start := time.Now().UTC().Truncate(time.Hour).Add(72 * time.Hour)
	resource, err := store.CreateSchedulingResource(ctx, scope, postgres.SchedulingResource{
		Name:     "HTTP Quote Bay",
		Timezone: "America/Toronto",
		Plan: scheduling.Plan{
			Capacity: 1,
			Open:     []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	service, err := store.SaveService(ctx, scope, postgres.CatalogService{Name: "HTTP Main Service", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}

	rule, err := store.SaveSchedulingRule(ctx, scope, postgres.SchedulingRule{
		ResourceID:      resource.ID,
		ServiceID:       service.ID,
		DurationMinutes: 60,
		Units:           1,
	})
	if err != nil {
		t.Fatal(err)
	}

	offer, err := store.SaveBookingOffer(ctx, scope, "", 0, true, postgres.BookingOfferConfig{
		ResourceID:        resource.ID,
		RuleID:            rule.ID,
		RuleVersion:       1,
		Title:             "HTTP Quote Offer",
		EquipmentCategory: "Standard Car",
		Mode:              "quote_required",
		Currency:          "CAD",
		Terms:             "HTTP quote terms",
		CancellationTerms: "HTTP cancellation",
		SlotMinutes:       15,
		AdvanceDays:       30,
		Published:         true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Submit booking request
	bookingReq, err := store.SubmitCustomerBookingRequest(ctx, custActor, customer, vehicle.ID, postgres.CustomerBookingRequestInput{
		RequestKey:    "http-cust-quote-req-001",
		OfferID:       offer.ID,
		OfferVersion:  1,
		Start:         start.Add(30 * time.Minute),
		CustomerNote:  "HTTP request note",
		TermsReviewed: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	send := func(method, path string, input any, bearer, requestOrigin string) *httptest.ResponseRecorder {
		var body []byte
		if input != nil {
			body, _ = json.Marshal(input)
		}
		r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Origin", requestOrigin)
		r.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	base := "/api/v1/organizations/" + org + "/locations/" + loc + "/quotes"

	// 1. Auth and origin checks
	if send("GET", base, nil, "", origin).Code != 401 {
		t.Fatal("anonymous access to quotes permitted")
	}
	if send("GET", base, nil, token, "https://untrusted.invalid").Code != 200 && send("POST", base, map[string]any{}, token, "https://untrusted.invalid").Code != 403 {
		t.Fatal("untrusted origin mutation accepted")
	}

	// 2. Grant quote capabilities to staff actor
	for _, action := range []string{"quotes.read", "quotes.create", "quotes.manage", "quotes.issue"} {
		if _, err := setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor, org, loc, action); err != nil {
			t.Fatal(err)
		}
	}

	// 3. POST create draft quote
	createBody := map[string]any{
		"requestId":     bookingReq.ID,
		"customerId":    customer,
		"vehicleId":     vehicle.ID,
		"internalNotes": "Staff internal note",
		"customerNote":  "Please review quotation",
		"taxCents":      1300,
		"depositCents":  2500,
		"items": []map[string]any{
			{"description": "Detailing Stage 1", "quantity": 1, "unitPriceCents": 10000},
		},
	}
	createRec := send("POST", base, createBody, token, origin)
	if createRec.Code != 201 {
		t.Fatalf("create quote failed with status %d: %s", createRec.Code, createRec.Body.String())
	}
	var createdQuote postgres.Quote
	if err := json.Unmarshal(createRec.Body.Bytes(), &createdQuote); err != nil {
		t.Fatal(err)
	}
	if createdQuote.Status != "draft" || createdQuote.SubtotalCents != 10000 || createdQuote.TotalCents != 11300 {
		t.Fatalf("unexpected quote values: %+v", createdQuote)
	}

	// 4. GET list quotes
	listRec := send("GET", base, nil, token, origin)
	if listRec.Code != 200 {
		t.Fatalf("list quotes failed with status %d", listRec.Code)
	}
	var listResp struct {
		Items []postgres.Quote `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Items) == 0 {
		t.Fatal("expected at least 1 quote in list")
	}

	// 5. GET single quote
	getRec := send("GET", base+"/"+createdQuote.ID, nil, token, origin)
	if getRec.Code != 200 {
		t.Fatalf("get quote failed with status %d", getRec.Code)
	}

	// 6. PUT edit quote
	editBody := map[string]any{
		"version":       createdQuote.Version,
		"internalNotes": "Updated staff note",
		"customerNote":  "Revised quotation details",
		"taxCents":      1500,
		"depositCents":  2500,
		"items": []map[string]any{
			{"description": "Detailing Stage 1 Revised", "quantity": 1, "unitPriceCents": 12000},
		},
	}
	editRec := send("PUT", base+"/"+createdQuote.ID, editBody, token, origin)
	if editRec.Code != 200 {
		t.Fatalf("edit quote failed with status %d: %s", editRec.Code, editRec.Body.String())
	}
	var editedQuote postgres.Quote
	if err := json.Unmarshal(editRec.Body.Bytes(), &editedQuote); err != nil {
		t.Fatal(err)
	}
	if editedQuote.Version != createdQuote.Version+1 || editedQuote.SubtotalCents != 12000 {
		t.Fatalf("unexpected edited quote: %+v", editedQuote)
	}

	// 7. POST issue quote
	issueBody := map[string]any{
		"version": editedQuote.Version,
		"note":    "Issuing to customer",
	}
	issueRec := send("POST", base+"/"+createdQuote.ID+"/issue", issueBody, token, origin)
	if issueRec.Code != 200 {
		t.Fatalf("issue quote failed with status %d: %s", issueRec.Code, issueRec.Body.String())
	}
	var issuedQuote postgres.Quote
	if err := json.Unmarshal(issueRec.Body.Bytes(), &issuedQuote); err != nil {
		t.Fatal(err)
	}
	if issuedQuote.Status != "issued" {
		t.Fatalf("expected status 'issued', got %s", issuedQuote.Status)
	}

	// 8. Customer portal: list and get quotes
	portalBase := "/api/v1/portal/accounts/" + customer + "/quotes"
	custQuotesRec := send("GET", portalBase, nil, custSession, origin)
	if custQuotesRec.Code != 200 {
		t.Fatalf("customer quotes failed with status %d", custQuotesRec.Code)
	}
	var custQuotesResp struct {
		Items []postgres.Quote `json:"items"`
	}
	if err := json.Unmarshal(custQuotesRec.Body.Bytes(), &custQuotesResp); err != nil {
		t.Fatal(err)
	}
	if len(custQuotesResp.Items) != 1 {
		t.Fatalf("expected 1 customer quote, got %d", len(custQuotesResp.Items))
	}

	// 9. Customer portal: accept quote
	acceptBody := map[string]any{
		"acceptanceKey": "http-cust-accept-00001",
		"termsReviewed": true,
	}
	acceptRec := send("POST", portalBase+"/"+createdQuote.ID+"/accept", acceptBody, custSession, origin)
	if acceptRec.Code != 200 {
		t.Fatalf("customer accept quote failed with status %d: %s", acceptRec.Code, acceptRec.Body.String())
	}
	var acceptedQuote postgres.Quote
	if err := json.Unmarshal(acceptRec.Body.Bytes(), &acceptedQuote); err != nil {
		t.Fatal(err)
	}
	if acceptedQuote.Status != "accepted" || acceptedQuote.Acceptance == nil {
		t.Fatalf("expected accepted status with acceptance record: %+v", acceptedQuote)
	}

	// 10. Attempt fulfillment without paying required deposit -> 422
	fulfillPremature := send("POST", base+"/"+createdQuote.ID+"/fulfill", map[string]any{
		"requestKey": "http-staff-fulfill-no-deposit",
		"quoteId":    createdQuote.ID,
		"channel":    "online",
	}, token, origin)
	if fulfillPremature.Code != 422 {
		t.Fatalf("expected 422 deposit required, got %d: %s", fulfillPremature.Code, fulfillPremature.Body.String())
	}

	// Authorization alone must not create a settled deposit invoice/payment.
	var invoicesBefore, paymentsBefore, invoicesAfter, paymentsAfter int64
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM invoices),(SELECT count(*) FROM payments)`).Scan(&invoicesBefore, &paymentsBefore); err != nil {
		t.Fatal(err)
	}
	approvedOnly := &square.MockClient{CreatePaymentFunc: func(_ context.Context, req square.PaymentRequest) (square.PaymentResponse, error) {
		return square.PaymentResponse{ID: "synthetic-quote-authorization", Status: "APPROVED", AmountCents: req.AmountCents, Currency: req.Currency}, nil
	}}
	_, authorizationErr := store.CustomerPayQuoteDeposit(ctx, custActor, customer, createdQuote.ID, approvedOnly, postgres.CustomerPayQuoteDepositInput{SourceID: "synthetic-source", IdempotencyKey: "codex-quote-approved-only-001"})
	if !errors.Is(authorizationErr, postgres.ErrPaymentProvider) {
		t.Fatalf("authorization treated as quote deposit: %v", authorizationErr)
	}
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM invoices),(SELECT count(*) FROM payments)`).Scan(&invoicesAfter, &paymentsAfter); err != nil {
		t.Fatal(err)
	}
	if invoicesBefore != invoicesAfter || paymentsBefore != paymentsAfter {
		t.Fatal("authorization left deposit invoice/payment rows")
	}
	t.Log("VERIFIED: quote deposit APPROVED rolls back invoice/payment rows")
	// 11. Customer portal: pay deposit
	depPayRes := send("POST", portalBase+"/"+createdQuote.ID+"/pay-deposit", map[string]any{
		"sourceId":       "cnon:card-nonce-ok",
		"idempotencyKey": "http-cust-dep-pay-0001",
	}, custSession, origin)
	if depPayRes.Code != 201 {
		t.Fatalf("customer pay deposit failed with status %d: %s", depPayRes.Code, depPayRes.Body.String())
	}
	var depResult postgres.CustomerQuoteDepositResult
	if err := json.Unmarshal(depPayRes.Body.Bytes(), &depResult); err != nil {
		t.Fatal(err)
	}
	if depResult.Status != "completed" || depResult.InvoiceID == "" {
		t.Fatalf("unexpected deposit result: %+v", depResult)
	}

	// 12. Staff fulfills accepted quote with verified deposit
	fulfillBody := map[string]any{
		"requestKey": "http-staff-fulfill-00001",
		"quoteId":    createdQuote.ID,
		"channel":    "online",
		"notes":      "HTTP fulfilled quote with verified deposit",
	}
	fulfillRec := send("POST", base+"/"+createdQuote.ID+"/fulfill", fulfillBody, token, origin)
	if fulfillRec.Code != 200 {
		t.Fatalf("fulfill quote failed with status %d: %s", fulfillRec.Code, fulfillRec.Body.String())
	}
	var fulfillResult postgres.FulfillQuoteResult
	if err := json.Unmarshal(fulfillRec.Body.Bytes(), &fulfillResult); err != nil {
		t.Fatal(err)
	}
	if fulfillResult.Quote.Status != "fulfilled" || fulfillResult.Booking.Status != "confirmed" {
		t.Fatalf("unexpected fulfill result: %+v", fulfillResult)
	}
}
