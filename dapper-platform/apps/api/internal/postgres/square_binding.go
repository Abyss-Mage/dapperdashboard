package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrSquareBinding = errors.New("a reviewed Square merchant binding is required for this location")

// SetSquareCredentialResolver installs server-held credentials (never request data).
func (s *Store) SetSquareCredentialResolver(r SquareCredentialResolver) { s.squareResolver = r }

// SetSquareApplicationIDs installs the PUBLIC Square application IDs (per environment) the browser
// card SDK needs. They are not secrets; access tokens never use this path.
func (s *Store) SetSquareApplicationIDs(ids map[string]string) {
	s.squareMu.Lock()
	defer s.squareMu.Unlock()
	s.squareApplicationIDs = ids
}
func (s *Store) SquareApplicationID(environment string) string {
	s.squareMu.RLock()
	defer s.squareMu.RUnlock()
	return s.squareApplicationIDs[environment]
}

type SquareMerchantBinding struct {
	Version             int       `json:"version"`
	Environment         string    `json:"environment"`
	MerchantID          string    `json:"merchantId"`
	SquareLocationID    string    `json:"squareLocationId"`
	CredentialReference string    `json:"credentialReference"`
	ReviewedAt          time.Time `json:"reviewedAt"`
}

// CurrentSquareMerchantBinding returns the latest reviewed binding, or ErrSquareBinding.
func (s *Store) CurrentSquareMerchantBinding(ctx context.Context, scope Scope, action string) (SquareMerchantBinding, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SquareMerchantBinding{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, action); err != nil {
		return SquareMerchantBinding{}, err
	}
	var b SquareMerchantBinding
	err = tx.QueryRow(ctx, `SELECT version,environment,merchant_id,square_location_id,credential_reference,reviewed_at FROM square_merchant_binding_revisions WHERE organization_id=$1 AND location_id=$2 ORDER BY version DESC LIMIT 1`, scope.OrganizationID, scope.LocationID).
		Scan(&b.Version, &b.Environment, &b.MerchantID, &b.SquareLocationID, &b.CredentialReference, &b.ReviewedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SquareMerchantBinding{}, ErrSquareBinding
	}
	return b, err
}

// SaveSquareMerchantBinding appends a new immutable revision. It needs the organization-level
// invoices.configure grant and the exact current version (0 when none exists). The caller must
// have verified the credential/location with the provider before calling.
func (s *Store) SaveSquareMerchantBinding(ctx context.Context, actor, org, loc string, expectedVersion int, in SquareMerchantBinding) (SquareMerchantBinding, error) {
	in.MerchantID, in.SquareLocationID, in.CredentialReference = strings.TrimSpace(in.MerchantID), strings.TrimSpace(in.SquareLocationID), strings.TrimSpace(in.CredentialReference)
	if (in.Environment != "sandbox" && in.Environment != "production") || in.MerchantID == "" || in.SquareLocationID == "" || in.CredentialReference == "" || len(in.MerchantID) > 128 || len(in.SquareLocationID) > 128 || len(in.CredentialReference) > 128 || expectedVersion < 0 {
		return SquareMerchantBinding{}, ErrPaymentInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SquareMerchantBinding{}, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return SquareMerchantBinding{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, org+"/"+loc+"/square-binding"); err != nil {
		return SquareMerchantBinding{}, err
	}
	var current int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0) FROM square_merchant_binding_revisions WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&current); err != nil {
		return SquareMerchantBinding{}, err
	}
	if current != expectedVersion {
		return SquareMerchantBinding{}, ErrPaymentRequest
	}
	var out SquareMerchantBinding
	err = tx.QueryRow(ctx, `INSERT INTO square_merchant_binding_revisions(organization_id,location_id,version,environment,merchant_id,square_location_id,credential_reference,reviewed_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING version,environment,merchant_id,square_location_id,credential_reference,reviewed_at`,
		org, loc, current+1, in.Environment, in.MerchantID, in.SquareLocationID, in.CredentialReference, actor).
		Scan(&out.Version, &out.Environment, &out.MerchantID, &out.SquareLocationID, &out.CredentialReference, &out.ReviewedAt)
	if err != nil {
		return SquareMerchantBinding{}, err
	}
	return out, tx.Commit(ctx)
}
