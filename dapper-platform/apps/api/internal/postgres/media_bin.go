package postgres

import (
	"context"
	"strings"
	"time"
)

// BinnedAsset is a built-in picture, logo or film, or an added film, waiting in the Bin.
type BinnedAsset struct {
	Path      string    `json:"path"`
	RemovedAt time.Time `json:"removedAt"`
}

// SetContentBinned moves an uploaded portfolio photo or dealer into the Bin, or back out of it. kind is "portfolio" or "dealers".
func (s *Store) SetContentBinned(ctx context.Context, scope Scope, kind, id string, binned bool) error {
	table := map[string]string{"portfolio": "portfolio_images", "dealers": "website_dealers"}[kind]
	if table == "" {
		return ErrContentInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE `+table+` SET binned_at=CASE WHEN $4 THEN now() END,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id, binned)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrContentNotFound
	}
	action := kind + ".restored"
	if binned {
		action = kind + ".binned"
	}
	if err = contentAudit(ctx, tx, scope, action, id, map[string]bool{"binned": binned}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// BinnedSiteAssets lists the files waiting in this location's Bin, newest first.
func (s *Store) BinnedSiteAssets(ctx context.Context, scope Scope) ([]BinnedAsset, error) {
	rows, err := s.pool.Query(ctx, `SELECT path,removed_at FROM site_asset_hidden WHERE organization_id=$1 AND location_id=$2 AND NOT purged ORDER BY removed_at DESC,path`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BinnedAsset{}
	for rows.Next() {
		var a BinnedAsset
		if err = rows.Scan(&a.Path, &a.RemovedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PurgeSiteAsset deletes a file in the Bin forever: an uploaded replacement or added film is erased, and a built-in file stops appearing anywhere.
func (s *Store) PurgeSiteAsset(ctx context.Context, scope Scope, path string) error {
	path, ok := cleanAssetPath(path)
	if !ok {
		return ErrSiteAsset
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM site_assets WHERE organization_id=$1 AND location_id=$2 AND path=$3`, scope.OrganizationID, scope.LocationID, path); err != nil {
		return err
	}
	if strings.HasPrefix(path, "films/") { // an added film has no built-in file behind it, so nothing is left to hide
		_, err = tx.Exec(ctx, `DELETE FROM site_asset_hidden WHERE organization_id=$1 AND location_id=$2 AND path=$3`, scope.OrganizationID, scope.LocationID, path)
	} else {
		_, err = tx.Exec(ctx, `UPDATE site_asset_hidden SET purged=true WHERE organization_id=$1 AND location_id=$2 AND path=$3`, scope.OrganizationID, scope.LocationID, path)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
