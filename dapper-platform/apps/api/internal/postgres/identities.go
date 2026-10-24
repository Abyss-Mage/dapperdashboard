package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) FindOIDCPrincipal(ctx context.Context, issuer, subject string) (string, error) {
	var principalID string
	err := s.pool.QueryRow(ctx, `SELECT p.id::text FROM oidc_identities i JOIN principals p ON p.id=i.principal_id WHERE i.issuer=$1 AND i.subject=$2 AND p.active`, issuer, subject).Scan(&principalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrSession
	}
	if err != nil {
		return "", err
	}
	return principalID, nil
}
