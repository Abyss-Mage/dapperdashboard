package postgres

import (
	"context"
	"dapper.local/platform/internal/paymentstate"
	"dapper.local/platform/internal/square"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestSquarePaymentIntentCandidateMigration(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test DB unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("codex_intent_candidate_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
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
	defer pool.Close()
	store := &Store{pool: pool}
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	// The intent tables are now migration 48 and arrive through Migrate above.
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=$1 AND table_name IN ('square_payment_intents','square_payment_intent_events','square_merchant_binding_revisions')`, namespace).Scan(&count); err != nil || count != 3 {
		t.Fatalf("candidate table count%d: %v", count, err)
	}
	var actor, org, loc, customer, invoice string
	err = pool.QueryRow(ctx, `WITH o AS (INSERT INTO organizations(name) VALUES('Synthetic Intent Org') RETURNING id),l AS (INSERT INTO locations(organization_id,name) SELECT id,'Synthetic Bay' FROM o RETURNING id,organization_id),a AS (INSERT INTO principals(external_subject) VALUES('synthetic-intent-principal') RETURNING id),c AS (INSERT INTO customers(organization_id,location_id,name,email) SELECT organization_id,id,'Synthetic Customer','synthetic@example.test' FROM l RETURNING id,organization_id,location_id),i AS (INSERT INTO invoices(organization_id,location_id,customer_id,invoice_number,status,subtotal_cents,tax_cents,total_cents,due_cents) SELECT organization_id,location_id,id,'SYN-INT-001','draft',5000,0,5000,5000 FROM c RETURNING id,organization_id,location_id,customer_id) SELECT a.id::text,i.organization_id::text,i.location_id::text,i.customer_id::text,i.id::text FROM i CROSS JOIN a`).Scan(&actor, &org, &loc, &customer, &invoice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO square_merchant_binding_revisions(organization_id,location_id,version,environment,merchant_id,square_location_id,credential_reference,reviewed_by) VALUES($1,$2,1,'sandbox','synthetic-merchant','synthetic-provider-location','synthetic-secret-selector',$3)`, org, loc, actor); err != nil {
		t.Fatal(err)
	}
	var intent, key string
	err = pool.QueryRow(ctx, `INSERT INTO square_payment_intents(organization_id,location_id,customer_id,actor_id,invoice_id,binding_version,request_key,request_hash,amount_cents,currency,state) VALUES($1,$2,$3,$4,$5,1,'synthetic-request-key-001',decode(repeat('ab',32),'hex'),5000,'CAD','prepared') RETURNING id::text,provider_request_key::text`, org, loc, customer, actor, invoice).Scan(&intent, &key)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`UPDATE square_payment_intents SET amount_cents=1 WHERE id=$1`, `UPDATE square_payment_intents SET provider_request_key=gen_random_uuid() WHERE id=$1`, `UPDATE square_payment_intents SET state='settled',provider_payment_id='synthetic-payment' WHERE id=$1`, `UPDATE square_payment_intents SET provider_payment_id='synthetic-payment' WHERE id=$1`, `DELETE FROM square_payment_intents WHERE id=$1`} {
		if _, err = pool.Exec(ctx, query, intent); err == nil {
			t.Fatalf("unsafe intent mutation accepted: %s", query)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'payments.record')`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	scope := Scope{ActorID: actor, OrganizationID: org, LocationID: loc}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION codex_fail_intent_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='payment.intent_submitting' THEN RAISE EXCEPTION 'synthetic attempt audit failure'; END IF; RETURN NEW; END $$;CREATE TRIGGER codex_fail_intent_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION codex_fail_intent_audit()`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimSquarePaymentIntent(ctx, scope, intent); err == nil {
		t.Fatal("claim ignored audit failure")
	}
	var rollbackState string
	var rollbackEvents int
	if err = pool.QueryRow(ctx, `SELECT state,(SELECT count(*) FROM square_payment_intent_events WHERE intent_id=$1) FROM square_payment_intents WHERE id=$1`, intent).Scan(&rollbackState, &rollbackEvents); err != nil || rollbackState != "prepared" || rollbackEvents != 0 {
		t.Fatalf("claim audit failure left writes%s/%d/%v", rollbackState, rollbackEvents, err)
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER codex_fail_intent_audit ON audit_events;DROP FUNCTION codex_fail_intent_audit()`); err != nil {
		t.Fatal(err)
	}
	type claimResult struct {
		attempt SquarePaymentAttempt
		err     error
	}
	claims := make(chan claimResult, 2)
	for n := 0; n < 2; n++ {
		go func() { a, e := store.ClaimSquarePaymentIntent(ctx, scope, intent); claims <- claimResult{a, e} }()
	}
	winners, losers := 0, 0
	for n := 0; n < 2; n++ {
		result := <-claims
		if result.err == nil {
			winners++
			if result.attempt.ProviderRequestKey != key || result.attempt.State != "submitting" {
				t.Fatal("claim lost immutable identity")
			}
		} else if errors.Is(result.err, ErrSquareIntentState) {
			losers++
		} else {
			t.Fatal(result.err)
		}
	}
	if winners != 1 || losers != 1 {
		t.Fatalf("concurrent claims%d/%d", winners, losers)
	}
	if _, err = store.ClaimSquarePaymentIntent(ctx, scope, intent); !errors.Is(err, ErrSquareIntentState) {
		t.Fatalf("duplicate claim accepted: %v", err)
	}
	if _, err = store.MarkSquarePaymentIntentUncertain(ctx, scope, intent); err != nil {
		t.Fatal(err)
	}
	if _, err = store.MarkSquarePaymentIntentUncertain(ctx, scope, intent); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND organization_id=$2 AND location_id=$3 AND action='payments.record'`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	if _, err = store.MarkSquarePaymentIntentUncertain(ctx, scope, intent); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked idempotent attempt accepted: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'payments.record')`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	retry, err := store.ClaimSquarePaymentIntent(ctx, scope, intent)
	if err != nil || retry.ProviderRequestKey != key {
		t.Fatal("retry changed provider key")
	}
	var eventCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM square_payment_intent_events WHERE intent_id=$1`, intent).Scan(&eventCount); err != nil || eventCount != 3 {
		t.Fatalf("unexpected transition events%d/%v", eventCount, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE square_payment_intents SET state='settled',provider_payment_id='synthetic-payment' WHERE id=$1`, intent); err == nil {
		t.Fatal("settlement without capture/allocation proof accepted")
	}
	if _, err = pool.Exec(ctx, `UPDATE square_payment_intents SET state='captured',provider_payment_id='synthetic-payment' WHERE id=$1`, intent); err == nil {
		t.Fatal("capture without provider observation accepted")
	}
	var replayKey string
	if err = pool.QueryRow(ctx, `SELECT provider_request_key::text FROM square_payment_intents WHERE id=$1`, intent).Scan(&replayKey); err != nil || replayKey != key {
		t.Fatal("recovery retry lost provider idempotency identity")
	}
	if _, err = pool.Exec(ctx, `UPDATE square_payment_intents SET state='failed' WHERE id=$1`, intent); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.read')`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO invoice_items(invoice_id,organization_id,location_id,description,quantity,unit_price_cents,total_price_cents) VALUES($1,$2,$3,'Synthetic intent invoice item',1,5000,5000)`, invoice, org, loc); err != nil {
		t.Fatal(err)
	}
	issueInvoiceForTest(t, ctx, store, scope, Invoice{ID: invoice})
	input := PrepareSquareInvoiceIntentInput{InvoiceID: invoice, BindingVersion: 1, RequestKey: "synthetic-prepare-key-001", AmountCents: 1000}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION codex_fail_prepare_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='payment.intent_prepared' THEN RAISE EXCEPTION 'synthetic prepare audit failure'; END IF; RETURN NEW; END $$;CREATE TRIGGER codex_fail_prepare_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION codex_fail_prepare_audit()`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.PrepareSquareInvoiceIntent(ctx, scope, input); err == nil {
		t.Fatal("preparation ignored audit failure")
	}
	var rolledBackIntents int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM square_payment_intents WHERE request_key=$1`, input.RequestKey).Scan(&rolledBackIntents); err != nil || rolledBackIntents != 0 {
		t.Fatal("prepare failure left durable request")
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER codex_fail_prepare_audit ON audit_events;DROP FUNCTION codex_fail_prepare_audit()`); err != nil {
		t.Fatal(err)
	}
	prepared, err := store.PrepareSquareInvoiceIntent(ctx, scope, input)
	if err != nil || prepared.State != "prepared" || prepared.Currency != "CAD" || prepared.AmountCents != 1000 {
		t.Fatalf("prepare failed: %+v %v", prepared, err)
	}
	replay, err := store.PrepareSquareInvoiceIntent(ctx, scope, input)
	if err != nil || replay.ID != prepared.ID || replay.ProviderRequestKey != prepared.ProviderRequestKey {
		t.Fatal("prepared retry lost immutable identity")
	}
	changed := input
	changed.AmountCents++
	if _, err = store.PrepareSquareInvoiceIntent(ctx, scope, changed); !errors.Is(err, ErrPaymentRequest) {
		t.Fatalf("changed retry accepted: %v", err)
	}
	another := input
	another.RequestKey = "synthetic-prepare-key-002"
	another.AmountCents = 5000
	if _, err = store.PrepareSquareInvoiceIntent(ctx, scope, another); !errors.Is(err, ErrPaymentOverpay) {
		t.Fatalf("outstanding exposure exceeded invoice due: %v", err)
	}
	var preparationEvents int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM square_payment_intent_events WHERE intent_id=$1`, prepared.ID).Scan(&preparationEvents); err != nil || preparationEvents != 1 {
		t.Fatalf("duplicate preparation events%d/%v", preparationEvents, err)
	}
	if _, err = store.ClaimSquarePaymentIntent(ctx, scope, prepared.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.MarkSquarePaymentIntentUncertain(ctx, scope, prepared.ID); err != nil {
		t.Fatal(err)
	}
	evidence := paymentstate.SettlementEvidence{SettlementBinding: paymentstate.SettlementBinding{Environment: "sandbox", OrganizationID: org, LocationID: loc, MerchantID: prepared.MerchantID, ProviderLocationID: prepared.ProviderLocationID, ReferenceID: prepared.ID, Currency: "CAD", AmountCents: 1000}, PaymentID: "synthetic-captured-payment", Status: "COMPLETED"}
	wrong := evidence
	wrong.MerchantID = "other-merchant"
	if _, err = store.RecordSquarePaymentCapture(ctx, scope, prepared.ID, wrong); !errors.Is(err, paymentstate.ErrSettlementBinding) {
		t.Fatalf("wrong merchant capture accepted: %v", err)
	}
	wrong = evidence
	wrong.Status = "APPROVED"
	if _, err = store.RecordSquarePaymentCapture(ctx, scope, prepared.ID, wrong); !errors.Is(err, paymentstate.ErrSettlementBinding) {
		t.Fatalf("authorization recorded as capture: %v", err)
	}
	captured, err := store.RecordSquarePaymentCapture(ctx, scope, prepared.ID, evidence)
	if err != nil || captured.State != "captured" {
		t.Fatalf("capture observation failed: %+v %v", captured, err)
	}
	if _, err = store.RecordSquarePaymentCapture(ctx, scope, prepared.ID, evidence); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimSquarePaymentIntent(ctx, scope, prepared.ID); !errors.Is(err, ErrSquareIntentState) {
		t.Fatal("captured intent recharged")
	}
	var observations, ledgerPayments int
	var invoicePaid int64
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM square_payment_capture_observations WHERE intent_id=$1),(SELECT count(*) FROM payments),paid_cents FROM invoices WHERE id=$2`, prepared.ID, invoice).Scan(&observations, &ledgerPayments, &invoicePaid); err != nil || observations != 1 || ledgerPayments != 0 || invoicePaid != 0 {
		t.Fatalf("observation falsely allocated or duplicated%d/%d/%d/%v", observations, ledgerPayments, invoicePaid, err)
	}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION codex_fail_allocation_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='payment.intent_allocated' THEN RAISE EXCEPTION 'synthetic allocation audit failure'; END IF; RETURN NEW; END $$;CREATE TRIGGER codex_fail_allocation_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION codex_fail_allocation_audit()`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.AllocateSquareInvoiceCapture(ctx, scope, prepared.ID); err == nil {
		t.Fatal("allocation ignored audit failure")
	}
	var retainedState string
	if err = pool.QueryRow(ctx, `SELECT state,(SELECT count(*) FROM square_payment_capture_observations WHERE intent_id=$1),(SELECT count(*) FROM payments) FROM square_payment_intents WHERE id=$1`, prepared.ID).Scan(&retainedState, &observations, &ledgerPayments); err != nil || retainedState != "captured" || observations != 1 || ledgerPayments != 0 {
		t.Fatal("allocation failure lost capture or left ledger writes")
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER codex_fail_allocation_audit ON audit_events;DROP FUNCTION codex_fail_allocation_audit()`); err != nil {
		t.Fatal(err)
	}
	type allocationResult struct {
		receipt SquareIntentAllocation
		err     error
	}
	allocations := make(chan allocationResult, 2)
	for n := 0; n < 2; n++ {
		go func() {
			r, e := store.AllocateSquareInvoiceCapture(ctx, scope, prepared.ID)
			allocations <- allocationResult{r, e}
		}()
	}
	var ledgerID string
	for n := 0; n < 2; n++ {
		outcome := <-allocations
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		if ledgerID != "" && outcome.receipt.PaymentID != ledgerID {
			t.Fatal("concurrent capture allocated twice")
		}
		ledgerID = outcome.receipt.PaymentID
	}
	var allocationCount int
	if err = pool.QueryRow(ctx, `SELECT paid_cents,(SELECT count(*) FROM payments),(SELECT count(*) FROM square_payment_allocations WHERE intent_id=$1) FROM invoices WHERE id=$2`, prepared.ID, invoice).Scan(&invoicePaid, &ledgerPayments, &allocationCount); err != nil || invoicePaid != 1000 || ledgerPayments != 1 || allocationCount != 1 {
		t.Fatalf("allocation mismatch%d/%d/%d/%v", invoicePaid, ledgerPayments, allocationCount, err)
	}
	conflictInput := PrepareSquareInvoiceIntentInput{InvoiceID: invoice, BindingVersion: 1, RequestKey: "synthetic-allocation-conflict", AmountCents: 4000}
	conflictIntent, err := store.PrepareSquareInvoiceIntent(ctx, scope, conflictInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimSquarePaymentIntent(ctx, scope, conflictIntent.ID); err != nil {
		t.Fatal(err)
	}
	conflictEvidence := evidence
	conflictEvidence.ReferenceID = conflictIntent.ID
	conflictEvidence.AmountCents = 4000
	conflictEvidence.PaymentID = "synthetic-unallocated-capture"
	if _, err = store.RecordSquarePaymentCapture(ctx, scope, conflictIntent.ID, conflictEvidence); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.RecordPayment(ctx, scope, RecordPaymentInput{InvoiceID: invoice, AmountCents: 4000, Method: "cash", IdempotencyKey: "synthetic-other-settlement"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.AllocateSquareInvoiceCapture(ctx, scope, conflictIntent.ID); !errors.Is(err, ErrInvoiceStatusConflict) {
		t.Fatalf("allocated capture after invoice separately settled: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT state,(SELECT count(*) FROM square_payment_capture_observations WHERE intent_id=$1),(SELECT count(*) FROM square_payment_allocations WHERE intent_id=$1) FROM square_payment_intents WHERE id=$1`, conflictIntent.ID).Scan(&retainedState, &observations, &allocationCount); err != nil || retainedState != "captured" || observations != 1 || allocationCount != 0 {
		t.Fatal("allocation conflict lost capture or falsely posted")
	}
	if _, err = store.ClaimSquarePaymentIntent(ctx, scope, conflictIntent.ID); !errors.Is(err, ErrSquareIntentState) {
		t.Fatal("conflicted capture allowed another provider claim")
	}
	t.Log("VERIFIED: prepare audit rollback; captured payment retained unallocated after invoice separately settled; recharge denied")
	if _, err = pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'invoices.create') ON CONFLICT DO NOTHING`, actor, org, loc); err != nil {
		t.Fatal(err)
	}
	executionInvoice, err := store.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: customer, InvoiceNumber: "SYN-EXECUTION-001", Items: []CreateInvoiceItemInput{{Description: "Synthetic orchestration service", Quantity: 1, UnitPriceCents: 1000}}})
	if err != nil {
		t.Fatal(err)
	}
	executionInvoice = issueInvoiceForTest(t, ctx, store, scope, executionInvoice)
	executionIntent, err := store.PrepareSquareInvoiceIntent(ctx, scope, PrepareSquareInvoiceIntentInput{InvoiceID: executionInvoice.ID, BindingVersion: 1, RequestKey: "synthetic-execution-001", AmountCents: 500})
	if err != nil {
		t.Fatal(err)
	}
	chargeCalls := 0
	verifiedClient := &syntheticVerifiedSquareClient{}
	verifiedClient.CreatePaymentFunc = func(_ context.Context, request square.PaymentRequest) (square.PaymentResponse, error) {
		chargeCalls++
		if request.IdempotencyKey != executionIntent.ProviderRequestKey || request.ReferenceID != executionIntent.ID || request.LocationID != executionIntent.ProviderLocationID || request.AmountCents != 500 || request.Currency != "CAD" {
			t.Fatal("charge did not use exact saved intent")
		}
		return square.PaymentResponse{ID: "synthetic-executed-payment", ReferenceID: request.ReferenceID, LocationID: request.LocationID, Status: "COMPLETED", AmountCents: request.AmountCents, Currency: request.Currency}, nil
	}
	resolver := func(_ context.Context, environment, reference string) (VerifiedSquareClient, error) {
		if environment != "sandbox" || reference != "synthetic-secret-selector" {
			t.Fatal("resolver not server-bound")
		}
		return verifiedClient, nil
	}
	executionReceipt, err := store.ExecuteSquareInvoiceIntent(ctx, scope, executionIntent.ID, "synthetic-source-token", resolver)
	if err != nil || executionReceipt.PaymentID == "" {
		t.Fatalf("execution failed: %+v %v", executionReceipt, err)
	}
	replayExecution, err := store.ExecuteSquareInvoiceIntent(ctx, scope, executionIntent.ID, "", nil)
	if err != nil || replayExecution.PaymentID != executionReceipt.PaymentID || chargeCalls != 1 || verifiedClient.checks != 1 {
		t.Fatal("settled recovery contacted provider or changed receipt")
	}
	rejectedIntent, err := store.PrepareSquareInvoiceIntent(ctx, scope, PrepareSquareInvoiceIntentInput{InvoiceID: executionInvoice.ID, BindingVersion: 1, RequestKey: "synthetic-identity-reject-001", AmountCents: 500})
	if err != nil {
		t.Fatal(err)
	}
	refusedClient := &syntheticVerifiedSquareClient{identityErr: square.ErrSquareIdentity}
	refusedClient.CreatePaymentFunc = func(context.Context, square.PaymentRequest) (square.PaymentResponse, error) {
		t.Fatal("charge attempted with unverified merchant")
		return square.PaymentResponse{}, nil
	}
	if _, err = store.ExecuteSquareInvoiceIntent(ctx, scope, rejectedIntent.ID, "synthetic-source-token", func(context.Context, string, string) (VerifiedSquareClient, error) { return refusedClient, nil }); !errors.Is(err, ErrPaymentProvider) {
		t.Fatalf("unverified merchant execution accepted: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT state FROM square_payment_intents WHERE id=$1`, rejectedIntent.ID).Scan(&retainedState); err != nil || retainedState != "uncertain" {
		t.Fatal("identity failure not durably recoverable")
	}
	t.Log("VERIFIED: identity check precedes charge; exact saved request; capture+allocation; retry without provider call; identity failure no charge")
	for _, kind := range []string{"wrong-location", "wrong-reference", "wrong-amount", "authorization", "cancelled"} {
		attemptCtx, cancelAttempt := context.WithCancel(ctx)
		ambiguous := &syntheticVerifiedSquareClient{}
		ambiguous.CreatePaymentFunc = func(_ context.Context, request square.PaymentRequest) (square.PaymentResponse, error) {
			if request.IdempotencyKey != rejectedIntent.ProviderRequestKey {
				t.Fatal("uncertain retry changed provider key")
			}
			response := square.PaymentResponse{ID: "synthetic-ambiguous-payment", ReferenceID: request.ReferenceID, LocationID: request.LocationID, Status: "COMPLETED", AmountCents: request.AmountCents, Currency: request.Currency}
			switch kind {
			case "wrong-location":
				response.LocationID = "other-location"
			case "wrong-reference":
				response.ReferenceID = "other-intent"
			case "wrong-amount":
				response.AmountCents++
			case "authorization":
				response.Status = "APPROVED"
			case "cancelled":
				cancelAttempt()
				return square.PaymentResponse{}, context.Canceled
			}
			return response, nil
		}
		_, attemptErr := store.ExecuteSquareInvoiceIntent(attemptCtx, scope, rejectedIntent.ID, "synthetic-source-token", func(context.Context, string, string) (VerifiedSquareClient, error) { return ambiguous, nil })
		cancelAttempt()
		if attemptErr == nil {
			t.Fatalf("ambiguous response counted settled: %s", kind)
		}
		var receiptCount int
		if err = pool.QueryRow(ctx, `SELECT state,(SELECT count(*) FROM square_payment_capture_observations WHERE intent_id=$1) FROM square_payment_intents WHERE id=$1`, rejectedIntent.ID).Scan(&retainedState, &receiptCount); err != nil || retainedState != "uncertain" || receiptCount != 0 {
			t.Fatalf("ambiguous response lost recovery state%s/%s/%d/%v", kind, retainedState, receiptCount, err)
		}
	}
	recoveredClient := &syntheticVerifiedSquareClient{}
	recoveredClient.CreatePaymentFunc = func(context.Context, square.PaymentRequest) (square.PaymentResponse, error) {
		t.Fatal("known-payment recovery attempted charge")
		return square.PaymentResponse{}, nil
	}
	readCalls := 0
	recoveredClient.getPayment = func(_ context.Context, paymentID string) (square.PaymentResponse, error) {
		readCalls++
		return square.PaymentResponse{ID: paymentID, ReferenceID: rejectedIntent.ID, LocationID: rejectedIntent.ProviderLocationID, Status: "COMPLETED", AmountCents: 500, Currency: "CAD"}, nil
	}
	recovered, err := store.RecoverSquareInvoiceIntent(ctx, scope, rejectedIntent.ID, "synthetic-known-provider-payment", func(context.Context, string, string) (VerifiedSquareClient, error) { return recoveredClient, nil })
	if err != nil {
		t.Fatalf("known-payment recovery failed: %v", err)
	}
	replayRecovered, err := store.RecoverSquareInvoiceIntent(ctx, scope, rejectedIntent.ID, "synthetic-known-provider-payment", nil)
	if err != nil || readCalls != 1 || replayRecovered.PaymentID != recovered.PaymentID {
		t.Fatal("known recovery replay contacted provider/changed receipt")
	}
	t.Log("VERIFIED: ambiguous/cancelled outcomes remain uncertain; known-payment authenticated read allocates without charging; replay no provider call")
	if err = store.ValidateSchema(ctx); err != nil {
		t.Fatal("registered ledger modified")
	}
}
