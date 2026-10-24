package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifySquareWebhookHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, actor, org, loc, customer, token, origin string) {
	t.Helper()

	webhookURL := "https://127.0.0.1:8443/api/v1/integrations/square/webhooks"
	signatureKey := "wh-secret-test-key-256"

	mockSq := &square.MockClient{
		VerifyWebhookSignatureFunc: func(body []byte, sig, url, key string) bool {
			return sig == "valid-mock-signature" && key == signatureKey && url == webhookURL
		},
	}

	handler := NewHandlerWithFullServices(store, origin, nil, mockSq, nil, webhookURL, signatureKey)
	unconfiguredHandler := NewHandlerWithFullServices(store, origin, nil, nil, nil, "", "")

	sendWebhook := func(h http.Handler, body, signature string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/integrations/square/webhooks", strings.NewReader(body))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Content-Type", "application/json")
		if signature != "" {
			r.Header.Set("x-square-hmacsha256-signature", signature)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	// 1. Unconfigured handler returns 503
	if unconfiguredHandler == nil || sendWebhook(unconfiguredHandler, `{}`, "valid-mock-signature").Code != 503 {
		t.Fatal("unconfigured square webhook accepted request")
	}

	// 2. Missing or invalid signature returns 401
	if sendWebhook(handler, `{}`, "").Code != 401 {
		t.Fatal("webhook without signature accepted")
	}
	if sendWebhook(handler, `{}`, "bad-signature").Code != 401 {
		t.Fatal("webhook with bad signature accepted")
	}

	// 3. Invalid JSON structure returns 400
	if sendWebhook(handler, `not-json`, "valid-mock-signature").Code != 400 {
		t.Fatal("invalid json accepted by webhook")
	}

	if sendWebhook(handler, strings.Repeat("x", 128*1024+1), "valid-mock-signature").Code != 413 {
		t.Fatal("oversized webhook not rejected")
	}
	for _, body := range []string{`{"type":"payment.updated","data":{"object":null}}`, `{"type":"payment.updated","data":{"object":{"payment":{"id":"synthetic"}}}}`, `{"type":"payment.updated","data":{"object":{"payment":"invalid"}}}`} {
		if sendWebhook(handler, body, "valid-mock-signature").Code != 400 {
			t.Fatalf("malformed payment object acknowledged: %s", body)
		}
	}
	if sendWebhook(handler, `{"type":"unhandled.synthetic.event"}`, "valid-mock-signature").Code != 200 {
		t.Fatal("unknown signed event rejected")
	}
	// 4. Create an invoice and a pending payment with provider_reference
	scope := postgres.Scope{ActorID: actor, OrganizationID: org, LocationID: loc}
	inv, err := store.CreateInvoice(ctx, scope, postgres.CreateInvoiceInput{
		CustomerID:    customer,
		InvoiceNumber: "SQ-WH-TEST-001",
		TaxCents:      130,
		Notes:         "Square Webhook Test Invoice",
		Items: []postgres.CreateInvoiceItemInput{
			{Description: "Webhook Service", Quantity: 1, UnitPriceCents: 1000},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test invoice: %v", err)
	}

	// Issue invoice properly through Store with reviewed issuer snapshot
	var issuerID string
	var issuerVersion int
	if err = setup.QueryRow(ctx, `SELECT i.id::text, i.version FROM location_invoice_issuers b JOIN invoice_issuers i ON i.id = b.issuer_id WHERE b.organization_id=$1 AND b.location_id=$2 LIMIT 1`, org, loc).Scan(&issuerID, &issuerVersion); err != nil {
		t.Fatalf("failed to query bound issuer: %v", err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id, organization_id, location_id, action) VALUES($1, $2, $3, 'invoices.issue') ON CONFLICT DO NOTHING`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	issuedInv, err := store.IssueInvoice(ctx, scope, inv.ID, postgres.InvoiceIssuanceReview{
		IssuerID:       issuerID,
		IssuerVersion:  issuerVersion,
		InvoiceVersion: inv.Version,
	})
	if err != nil {
		t.Fatalf("failed to issue invoice: %v", err)
	}
	if issuedInv.Status != "issued" {
		t.Fatalf("expected issued status, got %s", issuedInv.Status)
	}

	sqPaymentRef := "sq_wh_test_pay_" + time.Now().Format("20060102150405.000")
	// Insert a payment with pending status
	var paymentID string
	err = setup.QueryRow(ctx, `
		INSERT INTO payments(
			organization_id, location_id, invoice_id, customer_id, actor_id,
			amount_cents, currency, method, status, provider_reference,
			idempotency_key, notes
		) VALUES (
			$1, $2, $3, $4, $5,
			1130, 'CAD', 'square_card', 'pending', $6,
			'wh-idem-001', 'Pending Square Checkout'
		) RETURNING id::text
	`, org, loc, inv.ID, customer, actor, sqPaymentRef).Scan(&paymentID)
	if err != nil {
		t.Fatalf("failed to insert pending payment: %v", err)
	}

	// 5. Send webhook payment.updated with COMPLETED
	completedEvent := map[string]any{
		"type":     "payment.updated",
		"event_id": "evt-001",
		"data": map[string]any{
			"type": "payment",
			"id":   sqPaymentRef,
			"object": map[string]any{
				"payment": map[string]any{
					"id":     sqPaymentRef,
					"status": "COMPLETED",
					"amount_money": map[string]any{
						"amount":   1130,
						"currency": "CAD",
					},
				},
			},
		},
	}
	completedJSON, _ := json.Marshal(completedEvent)
	authorizedJSON := strings.Replace(string(completedJSON), `"status":"COMPLETED"`, `"status":"APPROVED"`, 1)
	authorizedResponse := sendWebhook(handler, authorizedJSON, "valid-mock-signature")
	var authorizedPaid int64
	var authorizedStatus string
	if err = setup.QueryRow(ctx, `SELECT p.status,i.paid_cents FROM payments p JOIN invoices i ON i.id=p.invoice_id WHERE p.id=$1`, paymentID).Scan(&authorizedStatus, &authorizedPaid); err != nil {
		t.Fatal(err)
	}
	if authorizedResponse.Code != 200 || authorizedStatus != "pending" || authorizedPaid != 0 {
		t.Fatalf("APPROVED webhook settled: code%d/status%s/paid%d", authorizedResponse.Code, authorizedStatus, authorizedPaid)
	}
	t.Log("VERIFIED: APPROVED webhook leaves pending payment and zero invoice paid amount")
	// Inject persistence failure only in this disposable schema.
	if _, err = setup.Exec(ctx, `CREATE FUNCTION codex_reject_payment_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic persistence failure'; END $$; CREATE TRIGGER codex_reject_payment_write BEFORE UPDATE ON payments FOR EACH ROW EXECUTE FUNCTION codex_reject_payment_write()`); err != nil {
		t.Fatal(err)
	}
	failedResponse := sendWebhook(handler, string(completedJSON), "valid-mock-signature")
	var unchangedStatus string
	if err = setup.QueryRow(ctx, `SELECT status FROM payments WHERE id=$1`, paymentID).Scan(&unchangedStatus); err != nil {
		t.Fatal(err)
	}
	if failedResponse.Code != 503 || unchangedStatus != "pending" {
		t.Fatalf("diagnostic behavior changed: status%d/payment%s", failedResponse.Code, unchangedStatus)
	}
	t.Log("VERIFIED: webhook replied503 when persistence failed; payment unchanged; replay succeeds after fault removal")
	if _, err = setup.Exec(ctx, `DROP TRIGGER codex_reject_payment_write ON payments; DROP FUNCTION codex_reject_payment_write()`); err != nil {
		t.Fatal(err)
	}

	resp := sendWebhook(handler, string(completedJSON), "valid-mock-signature")
	if resp.Code != 200 {
		t.Fatalf("webhook payment.updated failed: code %d, body: %s", resp.Code, resp.Body.String())
	}

	// Verify payment updated to completed and invoice updated to paid
	var pmtStatus string
	if err = setup.QueryRow(ctx, `SELECT status FROM payments WHERE id=$1`, paymentID).Scan(&pmtStatus); err != nil || pmtStatus != "completed" {
		t.Fatalf("expected payment status completed, got %s (err: %v)", pmtStatus, err)
	}
	var invPaid, invDue int64
	var invStatus string
	if err = setup.QueryRow(ctx, `SELECT paid_cents, due_cents, status FROM invoices WHERE id=$1`, inv.ID).Scan(&invPaid, &invDue, &invStatus); err != nil {
		t.Fatalf("failed to query invoice: %v", err)
	}
	if invPaid != 1130 || invDue != 0 || invStatus != "paid" {
		t.Fatalf("expected invoice paid (paid: 1130, due: 0), got paid: %d, due: %d, status: %s", invPaid, invDue, invStatus)
	}

	// 6. Send webhook payment.updated with REFUNDED
	refundedEvent := map[string]any{
		"type":     "payment.updated",
		"event_id": "evt-002",
		"data": map[string]any{
			"type": "payment",
			"id":   sqPaymentRef,
			"object": map[string]any{
				"payment": map[string]any{
					"id":     sqPaymentRef,
					"status": "REFUNDED",
					"amount_money": map[string]any{
						"amount":   1130,
						"currency": "CAD",
					},
					"refunded_money": map[string]any{
						"amount":   1130,
						"currency": "CAD",
					},
				},
			},
		},
	}
	refundedJSON, _ := json.Marshal(refundedEvent)
	resp = sendWebhook(handler, string(refundedJSON), "valid-mock-signature")
	if resp.Code != 200 {
		t.Fatalf("webhook refund failed: code %d, body: %s", resp.Code, resp.Body.String())
	}

	// Verify payment status refunded and invoice restored to issued
	if err = setup.QueryRow(ctx, `SELECT status FROM payments WHERE id=$1`, paymentID).Scan(&pmtStatus); err != nil || pmtStatus != "refunded" {
		t.Fatalf("expected payment status refunded, got %s (err: %v)", pmtStatus, err)
	}
	if err = setup.QueryRow(ctx, `SELECT paid_cents, due_cents, status FROM invoices WHERE id=$1`, inv.ID).Scan(&invPaid, &invDue, &invStatus); err != nil {
		t.Fatalf("failed to query invoice after refund: %v", err)
	}
	if invPaid != 0 || invDue != 1130 || invStatus != "issued" {
		t.Fatalf("expected invoice restored to issued (paid: 0, due: 1130), got paid: %d, due: %d, status: %s", invPaid, invDue, invStatus)
	}
}
