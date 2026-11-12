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
	return s.ExecuteSquareInvoiceIntentFor(ctx, scope, id, sourceID, "", resolve)
}

// ExecuteSquareInvoiceIntentFor is the same with the Square customer a saved card belongs to (Square needs it for a card on file).
func (s *Store) ExecuteSquareInvoiceIntentFor(ctx context.Context, scope Scope, id, sourceID, customerID string, resolve SquareCredentialResolver) (SquareIntentAllocation, error) {
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
	response, err := client.CreatePayment(ctx, square.PaymentRequest{SourceID: sourceID, CustomerID: customerID, IdempotencyKey: attempt.ProviderRequestKey, AmountCents: attempt.AmountCents, Currency: attempt.Currency, LocationID: attempt.ProviderLocationID, ReferenceID: attempt.ID})
	if errors.Is(err, square.ErrSquareDeclined) {
		// Nothing was charged, so this attempt must not keep holding the balance: the customer has to be able to try another card.
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = s.changeSquarePaymentAttempt(recovery, scope, id, "failed")
		return SquareIntentAllocation{}, ErrPaymentDeclined
	}
	if errors.Is(err, square.ErrSquareRejected) {
		// Square refused the request itself (4xx): nothing was charged, so the attempt fails and frees the balance; the reason goes back.
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = s.changeSquarePaymentAttempt(recovery, scope, id, "failed")
		return SquareIntentAllocation{}, fmt.Errorf("%w: %w", ErrPaymentProvider, err)
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

// ReleaseStuckSquareIntents checks an invoice's payment attempts whose answer never came back (older than 2 minutes) with Square, by
// their reference: one Square has no payment for failed, so it stops holding the balance; one Square did charge is settled. An attempt
// that cannot be checked is left as it is. It returns what happened to each one checked, for the person paying.
func (s *Store) ReleaseStuckSquareIntents(ctx context.Context, scope Scope, invoiceID string, resolve SquareCredentialResolver) []string {
	var notes []string
	// A "prepared" attempt was never sent to Square (it becomes "submitting" right before the charge), so after 2 minutes it can only
	// be an abandoned one: it stops holding the balance without asking Square.
	_, _ = s.pool.Exec(ctx, `UPDATE square_payment_intents SET state='failed',updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND invoice_id::text=$3 AND state='prepared' AND updated_at < now()-interval '2 minutes'`,
		scope.OrganizationID, scope.LocationID, invoiceID)
	if resolve == nil {
		return []string{"card payments are not set up on the server"}
	}
	rows, err := s.pool.Query(ctx, `SELECT i.id::text,i.created_at,b.square_location_id,b.environment,b.credential_reference FROM square_payment_intents i
 JOIN square_merchant_binding_revisions b ON (b.organization_id,b.location_id,b.version)=(i.organization_id,i.location_id,i.binding_version)
 WHERE i.organization_id=$1 AND i.location_id=$2 AND i.invoice_id::text=$3 AND i.actor_id=$4 AND i.state IN ('uncertain','submitting') AND i.updated_at < now()-interval '2 minutes' LIMIT 10`,
		scope.OrganizationID, scope.LocationID, invoiceID, scope.ActorID)
	if err != nil {
		return []string{"the earlier attempts could not be read"}
	}
	type stuck struct {
		id, loc, env, ref string
		at                time.Time
	}
	var list []stuck
	for rows.Next() {
		var x stuck
		if rows.Scan(&x.id, &x.at, &x.loc, &x.env, &x.ref) == nil {
			list = append(list, x)
		}
	}
	rows.Close()
	for _, x := range list {
		started := x.at.In(documentZone()).Format("Jan 2 3:04 PM")
		client, err := resolve(ctx, x.env, x.ref)
		if err != nil || client == nil {
			notes = append(notes, "attempt of "+started+": Square could not be reached with this franchise's connection")
			continue
		}
		finder, ok := client.(interface {
			FindPaymentByReference(context.Context, string, string, time.Time) (square.PaymentResponse, bool, error)
		})
		if !ok {
			continue
		}
		p, found, err := finder.FindPaymentByReference(ctx, x.loc, x.id, x.at.Add(-time.Minute))
		if err != nil {
			notes = append(notes, "attempt of "+started+": looking it up at Square failed ("+err.Error()+")")
			continue
		}
		if found {
			if _, err = s.RecoverSquareInvoiceIntent(ctx, scope, x.id, p.ID, resolve); err != nil {
				notes = append(notes, "attempt of "+started+": Square has its payment "+p.ID+" ("+p.Status+") but it could not be recorded here ("+err.Error()+")")
			}
			continue
		}
		_, _ = s.pool.Exec(ctx, `UPDATE square_payment_intents SET state='failed',updated_at=now() WHERE id=$1 AND state IN ('uncertain','submitting')`, x.id)
	}
	return notes
}

// HeldSquareIntents describes the payment attempts still holding an invoice's balance (amount, state, when), for the message the
// person paying sees when the balance is held.
func (s *Store) HeldSquareIntents(ctx context.Context, scope Scope, invoiceID string, markOthers bool) []string {
	rows, err := s.pool.Query(ctx, `SELECT amount_cents,state,created_at,updated_at,actor_id=$4 FROM square_payment_intents WHERE organization_id=$1 AND location_id=$2 AND invoice_id::text=$3 AND state IN ('prepared','submitting','uncertain') ORDER BY created_at`,
		scope.OrganizationID, scope.LocationID, invoiceID, scope.ActorID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var amount int64
		var state string
		var created, updated time.Time
		var mine bool
		if rows.Scan(&amount, &state, &created, &updated, &mine) != nil {
			continue
		}
		who := ""
		if markOthers && !mine {
			who = ", started from the dashboard or another page"
		}
		out = append(out, invoiceMoney(amount)+" started "+created.In(documentZone()).Format("Jan 2 3:04 PM")+" ("+state+", last change "+updated.In(documentZone()).Format("3:04 PM")+who+")")
	}
	return out
}

// ReleaseHeldSquareIntents is the owner's decision, after checking the Square account itself, that the card attempts still holding an
// invoice's balance did not charge anything (for when the server cannot check them, e.g. they went through a Square account that is
// no longer connected). They fail and the release is written to the invoice's history. Returns how many were released.
func (s *Store) ReleaseHeldSquareIntents(ctx context.Context, scope Scope, invoiceID string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.manage"); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `UPDATE square_payment_intents SET state='failed',updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND invoice_id::text=$3 AND state IN ('prepared','submitting','uncertain')`, scope.OrganizationID, scope.LocationID, invoiceID)
	if err != nil {
		return 0, err
	}
	n := int(tag.RowsAffected())
	if n > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO invoice_events(invoice_id,organization_id,location_id,actor_id,from_status,to_status,notes) SELECT id,organization_id,location_id,$4,status,status,$5 FROM invoices WHERE organization_id=$1 AND location_id=$2 AND id::text=$3`,
			scope.OrganizationID, scope.LocationID, invoiceID, scope.ActorID, fmt.Sprintf("Released %d unconfirmed card payment attempt(s) after the owner checked Square", n)); err != nil {
			return 0, err
		}
	}
	return n, tx.Commit(ctx)
}
