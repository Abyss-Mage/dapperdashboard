package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// WebsiteDealer is one dealership shown on the website: its name and logo. Publishing needs RightsReviewed (the owner confirms the
// dealer's logo may be shown); the flag itself is not stored, the same as for portfolio photos.
type WebsiteDealer struct {
	ID             string `json:"id"`
	MediaID        string `json:"mediaId"`
	Name           string `json:"name"`
	SortOrder      int    `json:"sortOrder"`
	Published      bool   `json:"published"`
	Version        int    `json:"version"`
	RightsReviewed bool   `json:"rightsReviewed"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
}

// SaveWebsiteDealer creates or revises a dealer entry (optimistic concurrency on Version).
func (s *Store) SaveWebsiteDealer(ctx context.Context, scope Scope, input WebsiteDealer) (WebsiteDealer, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !validContentText(input.Name, 2, 120) || input.MediaID == "" || input.Version < 0 || input.SortOrder < 0 || input.SortOrder > 10000 || (input.Published && !input.RightsReviewed) {
		return input, ErrContentInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return input, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return input, err
	}
	var media string
	err = tx.QueryRow(ctx, `SELECT id::text FROM content_media WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, scope.OrganizationID, scope.LocationID, input.MediaID).Scan(&media)
	if errors.Is(err, pgx.ErrNoRows) {
		return input, ErrContentInput
	}
	if err != nil {
		return input, err
	}
	if input.ID == "" && input.Version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO website_dealers(organization_id,location_id,media_id,name,sort_order,published) SELECT $1,$2,$3,$4,$5,$6 WHERE EXISTS(SELECT 1 FROM public_sites WHERE organization_id=$1 AND location_id=$2) RETURNING id::text,version`, scope.OrganizationID, scope.LocationID, input.MediaID, input.Name, input.SortOrder, input.Published).Scan(&input.ID, &input.Version)
	} else {
		err = tx.QueryRow(ctx, `UPDATE website_dealers SET media_id=$4,name=$5,sort_order=$6,published=$7,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND version=$8 RETURNING version`, scope.OrganizationID, scope.LocationID, input.ID, input.MediaID, input.Name, input.SortOrder, input.Published, input.Version).Scan(&input.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return input, ErrContentConflict
	}
	if err != nil {
		return input, err
	}
	if err = contentAudit(ctx, tx, scope, "dealer.revised", input.ID, input); err != nil {
		return input, err
	}
	return input, tx.Commit(ctx)
}

// DeleteWebsiteDealer removes a dealer from the website and deletes the stored logo when nothing else uses it.
func (s *Store) DeleteWebsiteDealer(ctx context.Context, scope Scope, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return err
	}
	var media string
	err = tx.QueryRow(ctx, `DELETE FROM website_dealers WHERE organization_id=$1 AND location_id=$2 AND id=$3 RETURNING media_id::text`, scope.OrganizationID, scope.LocationID, id).Scan(&media)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrContentNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM content_media m WHERE m.id=$1 AND NOT EXISTS(SELECT 1 FROM portfolio_images p WHERE p.media_id=m.id) AND NOT EXISTS(SELECT 1 FROM website_dealers d WHERE d.media_id=m.id)`, media); err != nil {
		return err
	}
	if err = contentAudit(ctx, tx, scope, "dealer.deleted", id, map[string]string{"mediaId": media}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// OrderWebsiteDealers sets the display order of this franchise's dealers to the order of ids.
func (s *Store) OrderWebsiteDealers(ctx context.Context, scope Scope, ids []string) error {
	if len(ids) == 0 || len(ids) > 200 {
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
	seen := map[string]bool{}
	for i, id := range ids {
		if seen[id] {
			return ErrContentInput
		}
		seen[id] = true
		tag, e := tx.Exec(ctx, `UPDATE website_dealers SET sort_order=$4,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id, i*10)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return ErrContentNotFound
		}
	}
	if err = contentAudit(ctx, tx, scope, "dealer.ordered", ids[0], ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PublicDealers lists the published dealers of every published franchise site, in the order they were arranged. It reveals nothing unpublished.
func (s *Store) PublicDealers(ctx context.Context) ([]WebsiteDealer, error) {
	rows, err := s.pool.Query(ctx, `SELECT d.id::text,d.media_id::text,d.name,d.sort_order,m.width,m.height
FROM website_dealers d
JOIN public_sites s ON (s.organization_id,s.location_id)=(d.organization_id,d.location_id)
JOIN content_media m ON m.id=d.media_id
WHERE d.published AND s.published ORDER BY d.sort_order,d.id LIMIT 60`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WebsiteDealer{}
	for rows.Next() {
		item := WebsiteDealer{Published: true}
		if err = rows.Scan(&item.ID, &item.MediaID, &item.Name, &item.SortOrder, &item.Width, &item.Height); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
