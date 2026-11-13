package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Customer account setup (owner 2026-11-14, error404 #85): when staff book a new customer who has an email address, the customer gets a
// sign-in account linked to their customer record and an email with a one-time link (72 hours) to choose a password. Only whoever controls
// that inbox can finish it, so the link itself is the proof of ownership. Staff can resend it from the customer.

// ErrNoCustomerEmail: the customer has no usable email address, so no account can be set up.
var ErrNoCustomerEmail = errors.New("the customer has no email address")

// AccountSetupLifetime is how long the emailed setup link works.
const AccountSetupLifetime = 72 * time.Hour

func init() {
	emailDefaults["account_setup"] = emailDefault{
		label:        "Customer account setup",
		subject:      "Set up your Dapper Detailing account",
		body:         cardEmail(`<p style="margin: 0 0 16px 0">Hi {{name}},</p><p style="margin: 0">Dapper Detailing has made an account for you, where you can see your bookings and invoices. Use the button below within 3 days to choose your password.</p><p style="margin: 16px 0 0 0;font-size: 13px;color: #666">If you were not expecting this, ignore this email.</p>`, "Choose my password", "{{link}}"),
		placeholders: []string{"name", "link"},
		needsLink:    true,
		html:         true,
	}
}

// PrepareCustomerAccount makes sure the customer has a sign-in account linked to their record and says whether it is new (no password yet).
// An existing account with the same email is linked rather than duplicated. actor is the staff member who created the customer or asked
// for the email; the caller has already checked they may (a successful booking or customers.create).
func (s *Store) PrepareCustomerAccount(ctx context.Context, org, loc, actor, customer string) (principal, email, name string, fresh bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	var raw string
	err = tx.QueryRow(ctx, `SELECT coalesce(email,''),name FROM customers WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND removed_at IS NULL FOR SHARE`, org, loc, customer).Scan(&raw, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrCustomerNotFound
		return
	}
	if err != nil {
		return
	}
	var ok bool
	if email, ok = normalizedEmail(raw); !ok {
		err = ErrNoCustomerEmail
		return
	}
	err = tx.QueryRow(ctx, `SELECT p.id::text FROM password_credentials c JOIN principals p ON p.id=c.principal_id WHERE c.email=$1 AND p.active`, email).Scan(&principal)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		fresh = true
		// One account per customer record; a retry after a half-finished setup reuses it.
		if err = tx.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES($1) ON CONFLICT(external_subject) DO UPDATE SET external_subject=excluded.external_subject RETURNING id::text`, "customer-account:"+customer).Scan(&principal); err != nil {
			return
		}
	case err != nil:
		return
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, org, principal); err != nil {
		return
	}
	tag, e := tx.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4) ON CONFLICT(principal_id,customer_id) DO NOTHING`, principal, org, loc, customer)
	if err = e; err != nil {
		return
	}
	if tag.RowsAffected() > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO customer_account_events(organization_id,location_id,customer_id,principal_id,actor_id,enabled,ownership_reviewed) VALUES($1,$2,$3,$4,$5,true,true)`, org, loc, customer, principal, actor); err != nil {
			return
		}
	}
	err = tx.Commit(ctx)
	return
}

// AccountSetupToken is a one-time link token for a new customer account to choose its first password (AccountSetupLifetime). It uses the
// password reset mechanism, so /reset-password finishes it.
func (a *PasswordAuthenticator) AccountSetupToken(ctx context.Context, principal string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	if _, err := a.store.pool.Exec(ctx, `INSERT INTO password_reset_tokens(token_hash,principal_id,expires_at) VALUES($1,$2,$3)`, digest[:], principal, time.Now().Add(AccountSetupLifetime)); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
