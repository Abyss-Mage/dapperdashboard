package postgres

import (
	"context"
	"dapper.local/platform/internal/paymentstate"
	"dapper.local/platform/internal/square"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type VerifiedSquareClient interface {
	square.Client
	square.LocationVerifier
}
type SquareCredentialResolver func(context.Context, string, string) (VerifiedSquareClient, error)

// ExecuteSquareInvoiceIntent is an internal authority, not a browser-callable
// settlement endpoint. The resolver is server configuration; sourceID is never
// saved/logged. Unknown outcomes retain the exact provider idempotency identity.
// ErrPaymentDeclined is a card the bank refused: nothing was charged. Every other provider failure leaves the outcome unknown.
var ErrPaymentDeclined = fmt.Errorf("%w: the card was declined", ErrPaymentProvider)

func (s *Store) ExecuteSquareInvoiceIntent(ctx context.Context, scope Scope, id, sourceID string, resolve SquareCredentialResolver) (SquareIntentAllocation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return SquareIntentAllocation{}, err
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT state FROM square_payment_intents WHERE id=$1 AND organization_id=$2 AND location_id=$3 AND actor_id=$4`, id, scope.OrganizationID, scope.LocationID, scope.ActorID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return SquareIntentAllocation{}, ErrForbidden
	}
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SquareIntentAllocation{}, err
	}
	if state == "captured" || state == "settled" {
		return s.AllocateSquareInvoiceCapture(ctx, scope, id)
	}
	if resolve == nil || strings.TrimSpace(sourceID) == "" {
		return SquareIntentAllocation{}, ErrPaymentProvider
	}
	attempt, err := s.ClaimSquarePaymentIntent(ctx, scope, id)
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	uncertain := func() error {
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		// ErrSquareIntentState means Square's notification already settled it while this call was timing out: the booking must stay.
		if _, err := s.MarkSquarePaymentIntentUncertain(recovery, scope, id); err != nil && !errors.Is(err, ErrSquareIntentState) {
			return err
		}
		return ErrPaymentProvider
	}
	client, err := resolve(ctx, attempt.Environment, attempt.CredentialReference)
	if err != nil || client == nil {
		return SquareIntentAllocation{}, uncertain()
	}
	if err = client.VerifyLocation(ctx, square.LocationBinding{Environment: attempt.Environment, MerchantID: attempt.MerchantID, LocationID: attempt.ProviderLocationID, Currency: attempt.Currency}); err != nil {
		return SquareIntentAllocation{}, uncertain()
	}
	response, err := client.CreatePayment(ctx, square.PaymentRequest{SourceID: sourceID, IdempotencyKey: attempt.ProviderRequestKey, AmountCents: attempt.AmountCents, Currency: attempt.Currency, LocationID: attempt.ProviderLocationID, ReferenceID: attempt.ID})
	if errors.Is(err, square.ErrSquareDeclined) {
		// Nothing was charged, so this attempt must not keep holding the balance: the customer has to be able to try another card.
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = s.changeSquarePaymentAttempt(recovery, scope, id, "failed")
		return SquareIntentAllocation{}, ErrPaymentDeclined
	}
	if err != nil {
		return SquareIntentAllocation{}, uncertain()
	}
	if response.LocationID != attempt.ProviderLocationID || response.ReferenceID != attempt.ID {
		return SquareIntentAllocation{}, uncertain()
	}
	evidence := paymentstate.SettlementEvidence{SettlementBinding: paymentstate.SettlementBinding{Environment: attempt.Environment, OrganizationID: scope.OrganizationID, LocationID: scope.LocationID, MerchantID: attempt.MerchantID, ProviderLocationID: response.LocationID, ReferenceID: response.ReferenceID, Currency: response.Currency, AmountCents: response.AmountCents}, PaymentID: response.ID, Status: response.Status}
	if _, err = s.RecordSquarePaymentCapture(ctx, scope, id, evidence); err != nil {
		// Even a captured provider result cannot be called settled unless its durable
		// observation commits; retain a recovery-needed state and return no success.
		_ = uncertain()
		return SquareIntentAllocation{}, err
	}
	return s.AllocateSquareInvoiceCapture(ctx, scope, id)
}

// SettleSquareIntentFromWebhook finishes a charge whose answer never reached us. Square's notification names our payment attempt in
// reference_id; the payment is then read back from Square, so the notification alone settles nothing. Anything else is ignored.
func (s *Store) SettleSquareIntentFromWebhook(ctx context.Context, intentID, paymentID string) error {
	if !uuid(intentID) {
		return nil
	}
	var scope Scope
	err := s.pool.QueryRow(ctx, `SELECT organization_id::text,location_id::text,actor_id::text FROM square_payment_intents WHERE id=$1 AND invoice_id IS NOT NULL AND state IN ('submitting','uncertain')`, intentID).Scan(&scope.OrganizationID, &scope.LocationID, &scope.ActorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.RecoverSquareInvoiceIntent(ctx, scope, intentID, paymentID, s.SquareCredentialResolver())
	return err
}

// RecoverSquareInvoiceIntent reads an already-known provider payment; it never
// creates or retries a charge. A verified webhook or operator reconciliation
// supplies the known ID, while saved intent/credential binding proves ownership.
func (s *Store) RecoverSquareInvoiceIntent(ctx context.Context, scope Scope, id, paymentID string, resolve SquareCredentialResolver) (SquareIntentAllocation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "payments.record"); err != nil {
		return SquareIntentAllocation{}, err
	}
	var attempt SquarePaymentAttempt
	err = tx.QueryRow(ctx, `SELECT i.id::text,i.provider_request_key::text,i.state,COALESCE(i.invoice_id::text,''),COALESCE(i.quotation_id::text,''),b.merchant_id,b.square_location_id,b.environment,b.credential_reference,i.currency,i.amount_cents FROM square_payment_intents i JOIN square_merchant_binding_revisions b ON (b.organization_id,b.location_id,b.version)=(i.organization_id,i.location_id,i.binding_version) WHERE i.organization_id=$1 AND i.location_id=$2 AND i.actor_id=$3 AND i.id=$4`, scope.OrganizationID, scope.LocationID, scope.ActorID, id).Scan(&attempt.ID, &attempt.ProviderRequestKey, &attempt.State, &attempt.InvoiceID, &attempt.QuotationID, &attempt.MerchantID, &attempt.ProviderLocationID, &attempt.Environment, &attempt.CredentialReference, &attempt.Currency, &attempt.AmountCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return SquareIntentAllocation{}, ErrForbidden
	}
	if err != nil {
		return SquareIntentAllocation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SquareIntentAllocation{}, err
	}
	if attempt.State == "captured" || attempt.State == "settled" {
		return s.AllocateSquareInvoiceCapture(ctx, scope, id)
	}
	if (attempt.State != "submitting" && attempt.State != "uncertain") || resolve == nil || strings.TrimSpace(paymentID) == "" {
		return SquareIntentAllocation{}, ErrSquareIntentState
	}
	client, err := resolve(ctx, attempt.Environment, attempt.CredentialReference)
	if err != nil || client == nil {
		return SquareIntentAllocation{}, ErrPaymentProvider
	}
	reader, ok := client.(square.PaymentReader)
	if !ok {
		return SquareIntentAllocation{}, ErrPaymentProvider
	}
	if err = client.VerifyLocation(ctx, square.LocationBinding{Environment: attempt.Environment, MerchantID: attempt.MerchantID, LocationID: attempt.ProviderLocationID, Currency: attempt.Currency}); err != nil {
		return SquareIntentAllocation{}, ErrPaymentProvider
	}
	response, err := reader.GetPayment(ctx, paymentID)
	if err != nil {
		return SquareIntentAllocation{}, ErrPaymentProvider
	}
	if response.ID != paymentID {
		return SquareIntentAllocation{}, ErrPaymentProvider
	}
	// A payment taken on the Square Reader (Point of Sale app) has no reference id; it is tied to this request by the note we sent with it.
	if response.ReferenceID == "" && strings.Contains(response.Note, attempt.ID) {
		response.ReferenceID = attempt.ID
	}
	evidence := paymentstate.SettlementEvidence{SettlementBinding: paymentstate.SettlementBinding{Environment: attempt.Environment, OrganizationID: scope.OrganizationID, LocationID: scope.LocationID, MerchantID: attempt.MerchantID, ProviderLocationID: response.LocationID, ReferenceID: response.ReferenceID, Currency: response.Currency, AmountCents: response.AmountCents}, PaymentID: response.ID, Status: response.Status}
	if _, err = s.RecordSquarePaymentCapture(ctx, scope, id, evidence); err != nil {
		return SquareIntentAllocation{}, err
	}
	return s.AllocateSquareInvoiceCapture(ctx, scope, id)
}
