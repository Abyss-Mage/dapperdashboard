package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// QuoteIssuer is the business details printed at the top of a quotation.
type QuoteIssuer struct {
	LegalName       string `json:"legalName"`
	Address         string `json:"address"`
	TaxRegistration string `json:"taxRegistration"`
}

// QuoteIssuerFor returns the business details linked to the franchise (the same ones invoices use), or empty details when none are set.
func (s *Store) QuoteIssuerFor(ctx context.Context, scope Scope) (QuoteIssuer, error) {
	if err := s.authorize(ctx, scope, "quotes.read"); err != nil {
		return QuoteIssuer{}, err
	}
	var out QuoteIssuer
	err := s.pool.QueryRow(ctx, `SELECT i.legal_name,i.address,i.tax_registration FROM location_invoice_issuers l JOIN invoice_issuers i ON i.id=l.issuer_id WHERE l.organization_id=$1 AND l.location_id=$2`, scope.OrganizationID, scope.LocationID).Scan(&out.LegalName, &out.Address, &out.TaxRegistration)
	if errors.Is(err, pgx.ErrNoRows) {
		return QuoteIssuer{}, nil
	}
	return out, err
}
