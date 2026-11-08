package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// PublicPortfolioImage is one published website photo.
type PublicPortfolioImage struct {
	ID       string `json:"id"`
	MediaID  string `json:"mediaId"`
	Title    string `json:"title"`
	Alt      string `json:"alt"`
	Category string `json:"category"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// PublicPortfolio lists the published portfolio photos of every published franchise site, in the order they were arranged.
// This installation serves one business's website, so the website shows them all. It reveals nothing unpublished.
func (s *Store) PublicPortfolio(ctx context.Context) ([]PublicPortfolioImage, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id::text,p.media_id::text,p.title,p.alt,p.category,m.width,m.height
FROM portfolio_images p
JOIN public_sites s ON (s.organization_id,s.location_id)=(p.organization_id,p.location_id)
JOIN content_media m ON m.id=p.media_id
WHERE p.published AND p.binned_at IS NULL AND s.published ORDER BY p.sort_order,p.id LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicPortfolioImage{}
	for rows.Next() {
		var item PublicPortfolioImage
		if err = rows.Scan(&item.ID, &item.MediaID, &item.Title, &item.Alt, &item.Category, &item.Width, &item.Height); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// DeletePortfolioImage removes a photo from the website and deletes the stored picture when nothing else uses it.
func (s *Store) DeletePortfolioImage(ctx context.Context, scope Scope, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return err
	}
	var media string
	err = tx.QueryRow(ctx, `DELETE FROM portfolio_images WHERE organization_id=$1 AND location_id=$2 AND id=$3 RETURNING media_id::text`, scope.OrganizationID, scope.LocationID, id).Scan(&media)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrContentNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM content_media m WHERE m.id=$1 AND NOT EXISTS(SELECT 1 FROM portfolio_images p WHERE p.media_id=m.id) AND NOT EXISTS(SELECT 1 FROM website_dealers d WHERE d.media_id=m.id)`, media); err != nil {
		return err
	}
	if err = contentAudit(ctx, tx, scope, "portfolio.deleted", id, map[string]string{"mediaId": media}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// OrderPortfolio sets the display order of this franchise's photos to the order of ids.
func (s *Store) OrderPortfolio(ctx context.Context, scope Scope, ids []string) error {
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
		tag, e := tx.Exec(ctx, `UPDATE portfolio_images SET sort_order=$4,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id, i*10)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return ErrContentNotFound
		}
	}
	if err = contentAudit(ctx, tx, scope, "portfolio.ordered", ids[0], ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
