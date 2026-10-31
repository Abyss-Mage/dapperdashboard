package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

var ErrLogoInput = errors.New("the logo must be a PNG, JPEG or WebP image under 512 KB")

const maxLogoBytes = 512 * 1024

// SaveLocationLogo sets a franchise's logo for its invoices. It needs invoices.configure, like the invoice template. The file type
// is decided from the bytes, never from the name or the declared type, and SVG is refused because it can carry scripts.
func (s *Store) SaveLocationLogo(ctx context.Context, actor, org, loc string, data []byte) (string, error) {
	if len(data) < 100 || len(data) > maxLogoBytes {
		return "", ErrLogoInput
	}
	kind := http.DetectContentType(data)
	if kind != "image/png" && kind != "image/jpeg" && kind != "image/webp" {
		return "", ErrLogoInput
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return "", err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM locations WHERE organization_id=$1 AND id=$2)`, org, loc).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", ErrForbidden
	}
	if _, err = tx.Exec(ctx, `INSERT INTO location_logos(organization_id,location_id,content_type,data,sha256,updated_by) VALUES($1,$2,$3,$4,$5,$6)
 ON CONFLICT(organization_id,location_id) DO UPDATE SET content_type=excluded.content_type,data=excluded.data,sha256=excluded.sha256,updated_by=excluded.updated_by,updated_at=now()`, org, loc, kind, data, digest, actor); err != nil {
		return "", err
	}
	return digest, tx.Commit(ctx)
}

// DeleteLocationLogo removes a franchise's logo (invoices go back to the default logo).
func (s *Store) DeleteLocationLogo(ctx context.Context, actor, org, loc string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM location_logos WHERE organization_id=$1 AND location_id=$2`, org, loc); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LocationLogo returns a franchise's logo to any member of the organization; found is false when it has none.
func (s *Store) LocationLogo(ctx context.Context, actor, org, loc string) (contentType string, data []byte, digest string, found bool, err error) {
	if err = s.memberOf(ctx, actor, org); err != nil {
		return
	}
	return s.readLocationLogo(ctx, org, loc)
}

func (s *Store) readLocationLogo(ctx context.Context, org, loc string) (contentType string, data []byte, digest string, found bool, err error) {
	err = s.pool.QueryRow(ctx, `SELECT content_type,data,sha256 FROM location_logos WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&contentType, &data, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, "", false, nil
	}
	return contentType, data, digest, err == nil, err
}

// EmployeeLocationLogo is the logo of the franchise the employee works at.
func (s *Store) EmployeeLocationLogo(ctx context.Context, actor string) (contentType string, data []byte, digest string, found bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	a, err := employeeWorksite(ctx, tx, actor)
	_ = tx.Rollback(ctx)
	if err != nil {
		return
	}
	return s.readLocationLogo(ctx, a.OrganizationID, a.LocationID)
}

// logoDigest is the digest of a franchise's logo, or "" when it has none (used to build a cache-busting address).
func logoDigest(ctx context.Context, q rowQuerier, org, loc string) string {
	var d string
	if err := q.QueryRow(ctx, `SELECT sha256 FROM location_logos WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&d); err != nil {
		return ""
	}
	return d
}
