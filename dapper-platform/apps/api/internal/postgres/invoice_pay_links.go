package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Pay links (error404 #18, decision 2026-11-11): the dealership invoice email carries a private link that opens that one invoice to view,
// download and pay by card, with no account or password. Only the secret's hash is stored. Since 2026-11-11 (error404 #26) a link opens
// the dealership's portal and lasts 30 days.

var ErrPayLink = errors.New("pay link not found or expired")

func payLinkHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateInvoicePayLink makes a new link for an issued invoice the caller may issue in this franchise and returns its secret.
func (s *Store) CreateInvoicePayLink(ctx context.Context, scope Scope, invoiceID string) (string, error) {
	if err := s.authorize(ctx, scope, "invoices.issue"); err != nil {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	tag, err := s.pool.Exec(ctx, `INSERT INTO invoice_pay_links(token_hash,invoice_id,expires_at) SELECT $4,id,now()+interval '30 days' FROM invoices
 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND status IN ('issued','partially_paid','paid')`, scope.OrganizationID, scope.LocationID, invoiceID, payLinkHash(token))
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 1 {
		return "", ErrInvoiceNotFound
	}
	return token, nil
}

// PayLinkInvoice opens the invoice behind a link: the website's scope at the invoice's franchise (which may read and take payment for it)
// and the invoice. Drafts and voided invoices are not shown.
func (s *Store) PayLinkInvoice(ctx context.Context, token string) (Scope, DealerInvoice, error) {
	if len(token) < 40 || len(token) > 64 {
		return Scope{}, DealerInvoice{}, ErrPayLink
	}
	var org, loc, id string
	err := s.pool.QueryRow(ctx, `SELECT i.organization_id::text,i.location_id::text,i.id::text FROM invoice_pay_links l JOIN invoices i ON i.id=l.invoice_id
 WHERE l.token_hash=$1 AND l.expires_at>now() AND i.status IN ('issued','partially_paid','paid')`, payLinkHash(token)).Scan(&org, &loc, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Scope{}, DealerInvoice{}, ErrPayLink
	}
	if err != nil {
		return Scope{}, DealerInvoice{}, err
	}
	scope, err := s.websiteScopeAt(ctx, org, loc)
	if err != nil {
		return Scope{}, DealerInvoice{}, err
	}
	inv, err := s.GetDealerInvoice(ctx, scope, id)
	return scope, inv, err
}
