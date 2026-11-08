package postgres

import (
	"context"
	"crypto/sha256"
	"dapper.local/platform/internal/paymentstate"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
)

var ErrSquareIntentState = errors.New("payment attempt cannot be claimed in its current state")

// SquarePaymentAttempt is internal provider routing data, never a public DTO.
// CredentialReference identifies a server secret; no credential is persisted here.
type SquarePaymentAttempt struct {
	ID, ProviderRequestKey, State, InvoiceID, QuotationID            string
	MerchantID, ProviderLocationID, Environment, CredentialReference string
	Currency                                                         string
	AmountCents                                                      int64
}

// ClaimSquarePaymentIntent must commit before contacting the provider. A second
// concurrent claim cannot start another provider request. An uncertain retry uses
// the same immutable provider key, never a newly generated charge identity.
func (s *Store) ClaimSquarePaymentIntent(ctx context.Context, scope Scope, id string) (SquarePaymentAttempt, error) {
	return s.changeSquarePaymentAttempt(ctx, scope, id, "submitting")
}

// MarkSquarePaymentIntentUncertain records ambiguous provider outcomes without
// declaring success/failure. A durable recovery worker still needs provider lookup.
func (s *Store) MarkSquarePaymentIntentUncertain(ctx context.Context, scope Scope, id string) (SquarePaymentAttempt, error) {
	return s.changeSquarePaymentAttempt(ctx, scope, id, "uncertain")
}
func (s *Store) changeSquarePaymentAttempt(ctx context.Context, scope Scope, id, next string) (SquarePaymentAttempt, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SquarePaymentAttempt{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return SquarePaymentAttempt{}, err
	}
	var attempt SquarePaymentAttempt
	err = tx.QueryRow(ctx, `SELECT i.id::text,i.provider_request_key::text,i.state,COALESCE(i.invoice_id::text,''),COALESCE(i.quotation_id::text,''),b.merchant_id,b.square_location_id,b.environment,b.credential_reference,i.currency,i.amount_cents FROM square_payment_intents i JOIN square_merchant_binding_revisions b ON (b.organization_id,b.location_id,b.version)=(i.organization_id,i.location_id,i.binding_version) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.actor_id=$3 AND i.id=$4 FOR UPDATE OF i`, scope.OrganizationID, scope.LocationID, scope.ActorID, id).Scan(&attempt.ID, &attempt.ProviderRequestKey, &attempt.State, &attempt.InvoiceID, &attempt.QuotationID, &attempt.MerchantID, &attempt.ProviderLocationID, &attempt.Environment, &attempt.CredentialReference, &attempt.Currency, &attempt.AmountCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return SquarePaymentAttempt{}, ErrForbidden
	}
	if err != nil {
		return SquarePaymentAttempt{}, err
	}
	if next == "uncertain" && attempt.State == next {
		if err = tx.Commit(ctx); err != nil {
			return SquarePaymentAttempt{}, err
		}
		return attempt, nil
	}
	if (next == "submitting" && attempt.State != "prepared" && attempt.State != "uncertain") || ((next == "uncertain" || next == "failed") && attempt.State != "submitting") {
		return SquarePaymentAttempt{}, ErrSquareIntentState
	}
	if _, err = tx.Exec(ctx, `UPDATE square_payment_intents SET state=$1,updated_at=now() WHERE id=$2`, next, id); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO square_payment_intent_events(organization_id,location_id,intent_id,state) VALUES($1,$2,$3,$4)`, scope.OrganizationID, scope.LocationID, id, next); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if err = jobAudit(ctx, tx, scope, "payment.intent_"+next, id); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SquarePaymentAttempt{}, err
	}
	attempt.State = next
	return attempt, nil
}

type PrepareSquareInvoiceIntentInput struct {
	InvoiceID      string
	BindingVersion int
	RequestKey     string
	AmountCents    int64
}

func (s *Store) PrepareSquareInvoiceIntent(ctx context.Context, scope Scope, input PrepareSquareInvoiceIntentInput) (SquarePaymentAttempt, error) {
	input.RequestKey = strings.TrimSpace(input.RequestKey)
	if input.InvoiceID == "" || input.BindingVersion < 1 || input.AmountCents <= 0 || len(input.RequestKey) < 16 || len(input.RequestKey) > 128 {
		return SquarePaymentAttempt{}, ErrPaymentInput
	}
	for _, c := range input.RequestKey {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return SquarePaymentAttempt{}, ErrPaymentInput
		}
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return SquarePaymentAttempt{}, err
	}
	digest := sha256.Sum256(payload)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SquarePaymentAttempt{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.OrganizationID+"/"+scope.LocationID+"/"+scope.ActorID+"/square-intent/"+input.RequestKey); err != nil {
		return SquarePaymentAttempt{}, err
	}
	var id string
	var previousHash []byte
	err = tx.QueryRow(ctx, `SELECT id::text,request_hash FROM square_payment_intents WHERE organization_id=$1 AND location_id=$2 AND actor_id=$3 AND request_key=$4`, scope.OrganizationID, scope.LocationID, scope.ActorID, input.RequestKey).Scan(&id, &previousHash)
	if err == nil {
		if string(previousHash) != string(digest[:]) {
			return SquarePaymentAttempt{}, ErrPaymentRequest
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SquarePaymentAttempt{}, err
	} else {
		var customer, currency, status string
		var due int64
		err = tx.QueryRow(ctx, `SELECT customer_id::text,currency,status,due_cents FROM invoices WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, input.InvoiceID).Scan(&customer, &currency, &status, &due)
		if errors.Is(err, pgx.ErrNoRows) {
			return SquarePaymentAttempt{}, ErrInvoiceNotFound
		}
		if err != nil {
			return SquarePaymentAttempt{}, err
		}
		if currency != "CAD" || (status != "issued" && status != "partially_paid") {
			return SquarePaymentAttempt{}, ErrInvoiceStatusConflict
		}
		var bindingExists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM square_merchant_binding_revisions WHERE organization_id=$1 AND location_id=$2 AND version=$3)`, scope.OrganizationID, scope.LocationID, input.BindingVersion).Scan(&bindingExists); err != nil {
			return SquarePaymentAttempt{}, err
		}
		if !bindingExists {
			return SquarePaymentAttempt{}, ErrPaymentProvider
		}
		var outstanding int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_cents),0) FROM square_payment_intents WHERE organization_id=$1 AND location_id=$2 AND invoice_id=$3 AND state IN ('prepared','submitting','uncertain')`, scope.OrganizationID, scope.LocationID, input.InvoiceID).Scan(&outstanding); err != nil {
			return SquarePaymentAttempt{}, err
		}
		if outstanding > due || input.AmountCents > due-outstanding {
			return SquarePaymentAttempt{}, ErrPaymentOverpay
		}
		err = tx.QueryRow(ctx, `INSERT INTO square_payment_intents(organization_id,location_id,customer_id,actor_id,invoice_id,binding_version,request_key,request_hash,amount_cents,currency,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'CAD','prepared') RETURNING id::text`, scope.OrganizationID, scope.LocationID, customer, scope.ActorID, input.InvoiceID, input.BindingVersion, input.RequestKey, digest[:], input.AmountCents).Scan(&id)
		if err != nil {
			return SquarePaymentAttempt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO square_payment_intent_events(organization_id,location_id,intent_id,state) VALUES($1,$2,$3,'prepared')`, scope.OrganizationID, scope.LocationID, id); err != nil {
			return SquarePaymentAttempt{}, err
		}
		if err = jobAudit(ctx, tx, scope, "payment.intent_prepared", id); err != nil {
			return SquarePaymentAttempt{}, err
		}
	}
	var result SquarePaymentAttempt
	err = tx.QueryRow(ctx, `SELECT i.id::text,i.provider_request_key::text,i.state,COALESCE(i.invoice_id::text,''),COALESCE(i.quotation_id::text,''),b.merchant_id,b.square_location_id,b.environment,b.credential_reference,i.currency,i.amount_cents FROM square_payment_intents i JOIN square_merchant_binding_revisions b ON (b.organization_id,b.location_id,b.version)=(i.organization_id,i.location_id,i.binding_version) WHERE i.id=$1`, id).Scan(&result.ID, &result.ProviderRequestKey, &result.State, &result.InvoiceID, &result.QuotationID, &result.MerchantID, &result.ProviderLocationID, &result.Environment, &result.CredentialReference, &result.Currency, &result.AmountCents)
	if err != nil {
		return SquarePaymentAttempt{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SquarePaymentAttempt{}, err
	}
	return result, nil
}

// RecordSquarePaymentCapture receives authenticated adapter evidence, never a
// browser body. It durably records capture separately from invoice allocation.
// A captured intent cannot be claimed again, even if allocation later fails.
func (s *Store) RecordSquarePaymentCapture(ctx context.Context, scope Scope, id string, evidence paymentstate.SettlementEvidence) (SquarePaymentAttempt, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SquarePaymentAttempt{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return SquarePaymentAttempt{}, err
	}
	var attempt SquarePaymentAttempt
	err = tx.QueryRow(ctx, `SELECT i.id::text,i.provider_request_key::text,i.state,COALESCE(i.invoice_id::text,''),COALESCE(i.quotation_id::text,''),b.merchant_id,b.square_location_id,b.environment,b.credential_reference,i.currency,i.amount_cents FROM square_payment_intents i JOIN square_merchant_binding_revisions b ON (b.organization_id,b.location_id,b.version)=(i.organization_id,i.location_id,i.binding_version) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.actor_id=$3 AND i.id=$4 FOR UPDATE OF i`, scope.OrganizationID, scope.LocationID, scope.ActorID, id).Scan(&attempt.ID, &attempt.ProviderRequestKey, &attempt.State, &attempt.InvoiceID, &attempt.QuotationID, &attempt.MerchantID, &attempt.ProviderLocationID, &attempt.Environment, &attempt.CredentialReference, &attempt.Currency, &attempt.AmountCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return SquarePaymentAttempt{}, ErrForbidden
	}
	if err != nil {
		return SquarePaymentAttempt{}, err
	}
	expected := paymentstate.SettlementBinding{Environment: attempt.Environment, OrganizationID: scope.OrganizationID, LocationID: scope.LocationID, MerchantID: attempt.MerchantID, ProviderLocationID: attempt.ProviderLocationID, ReferenceID: attempt.ID, Currency: attempt.Currency, AmountCents: attempt.AmountCents}
	if err = paymentstate.ValidateSettlement(expected, evidence); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if attempt.State == "captured" || attempt.State == "settled" {
		var paymentID string
		if err = tx.QueryRow(ctx, `SELECT provider_payment_id FROM square_payment_capture_observations WHERE intent_id=$1`, id).Scan(&paymentID); err != nil {
			return SquarePaymentAttempt{}, err
		}
		if paymentID != evidence.PaymentID {
			return SquarePaymentAttempt{}, ErrPaymentRequest
		}
		if err = tx.Commit(ctx); err != nil {
			return SquarePaymentAttempt{}, err
		}
		return attempt, nil
	}
	if attempt.State != "submitting" && attempt.State != "uncertain" {
		return SquarePaymentAttempt{}, ErrSquareIntentState
	}
	if _, err = tx.Exec(ctx, `INSERT INTO square_payment_capture_observations(intent_id,organization_id,location_id,environment,merchant_id,square_location_id,provider_payment_id,amount_cents,currency) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, scope.OrganizationID, scope.LocationID, attempt.Environment, attempt.MerchantID, attempt.ProviderLocationID, evidence.PaymentID, attempt.AmountCents, attempt.Currency); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE square_payment_intents SET state='captured',provider_payment_id=$1,updated_at=now() WHERE id=$2`, evidence.PaymentID, id); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO square_payment_intent_events(organization_id,location_id,intent_id,state) VALUES($1,$2,$3,'captured')`, scope.OrganizationID, scope.LocationID, id); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if err = jobAudit(ctx, tx, scope, "payment.intent_capture_observed", id); err != nil {
		return SquarePaymentAttempt{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SquarePaymentAttempt{}, err
	}
	attempt.State = "captured"
	return attempt, nil
}

type SquareIntentAllocation struct{ IntentID, PaymentID, InvoiceID string }

// AllocateSquareInvoiceCapture never invokes Square. Verified capture evidence
// was committed separately and remains recoverable if allocation is rejected.
func (s *Store) AllocateSquareInvoiceCapture(ctx context.Context, scope Scope, id string) (SquareIntentAllocation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return SquareIntentAllocation{}, err
	}
	var invoiceID string
	err = tx.QueryRow(ctx, `SELECT COALESCE(invoice_id::text,'') FROM square_payment_intents WHERE organization_id=$1 AND location_id=$2 AND actor_id=$3 AND id=$4`, scope.OrganizationID, scope.LocationID, scope.ActorID, id).Scan(&invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SquareIntentAllocation{}, ErrForbidden
	}
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	if invoiceID == "" {
		return SquareIntentAllocation{}, ErrInvoiceNotFound
	}
	var customer, currency, invoiceStatus string
	var due, paid, total int64
	err = tx.QueryRow(ctx, `SELECT customer_id::text,currency,status,due_cents,paid_cents,total_cents FROM invoices WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, invoiceID).Scan(&customer, &currency, &invoiceStatus, &due, &paid, &total)
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	var state, intentCustomer, providerID, providerKey string
	var amount int64
	err = tx.QueryRow(ctx, `SELECT i.state,i.customer_id::text,o.provider_payment_id,i.provider_request_key::text,i.amount_cents FROM square_payment_intents i JOIN square_payment_capture_observations o ON o.intent_id=i.id WHERE i.id=$1 AND i.organization_id=$2 AND i.location_id=$3 AND i.actor_id=$4 FOR UPDATE OF i`, id, scope.OrganizationID, scope.LocationID, scope.ActorID).Scan(&state, &intentCustomer, &providerID, &providerKey, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return SquareIntentAllocation{}, ErrSquareIntentState
	}
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	result := SquareIntentAllocation{IntentID: id, InvoiceID: invoiceID}
	if state == "settled" {
		if err = tx.QueryRow(ctx, `SELECT payment_id::text FROM square_payment_allocations WHERE intent_id=$1`, id).Scan(&result.PaymentID); err != nil {
			return SquareIntentAllocation{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return SquareIntentAllocation{}, err
		}
		return result, nil
	}
	if state != "captured" {
		return SquareIntentAllocation{}, ErrSquareIntentState
	}
	if customer != intentCustomer || currency != "CAD" || (invoiceStatus != "issued" && invoiceStatus != "partially_paid") {
		return SquareIntentAllocation{}, ErrInvoiceStatusConflict
	}
	if amount > due {
		return SquareIntentAllocation{}, ErrPaymentOverpay
	}
	var alreadyRecorded bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE provider_reference=$1 AND method IN ('square_card','square_terminal'))`, providerID).Scan(&alreadyRecorded); err != nil {
		return SquareIntentAllocation{}, err
	}
	if alreadyRecorded {
		return SquareIntentAllocation{}, ErrPaymentRequest
	}
	err = tx.QueryRow(ctx, `INSERT INTO payments(organization_id,location_id,invoice_id,customer_id,actor_id,amount_cents,currency,method,status,provider_reference,idempotency_key,notes) VALUES($1,$2,$3,$4,$5,$6,'CAD','square_card','completed',$7,$8,'Verified durable Square intent allocation') RETURNING id::text`, scope.OrganizationID, scope.LocationID, invoiceID, customer, scope.ActorID, amount, providerID, providerKey).Scan(&result.PaymentID)
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	nextPaid := paid + amount
	nextDue := total - nextPaid
	nextStatus := "partially_paid"
	if nextDue == 0 {
		nextStatus = "paid"
	}
	if _, err = tx.Exec(ctx, `UPDATE invoices SET paid_cents=$1,due_cents=$2,status=$3,paid_at=CASE WHEN $3='paid' THEN now() ELSE NULL END,updated_at=now() WHERE id=$4`, nextPaid, nextDue, nextStatus, invoiceID); err != nil {
		return SquareIntentAllocation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO square_payment_allocations(intent_id,organization_id,location_id,payment_id,amount_cents) VALUES($1,$2,$3,$4,$5)`, id, scope.OrganizationID, scope.LocationID, result.PaymentID, amount); err != nil {
		return SquareIntentAllocation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE square_payment_intents SET state='settled',updated_at=now() WHERE id=$1`, id); err != nil {
		return SquareIntentAllocation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO square_payment_intent_events(organization_id,location_id,intent_id,state) VALUES($1,$2,$3,'settled')`, scope.OrganizationID, scope.LocationID, id); err != nil {
		return SquareIntentAllocation{}, err
	}
	if err = recordInvoiceEvent(ctx, tx, scope, invoiceID, &invoiceStatus, nextStatus, "Verified Square capture allocated from durable intent"); err != nil {
		return SquareIntentAllocation{}, err
	}
	if err = jobAudit(ctx, tx, scope, "payment.intent_allocated", id); err != nil {
		return SquareIntentAllocation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SquareIntentAllocation{}, err
	}
	return result, nil
}
