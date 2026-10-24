package postgres

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// UploadTagRetentionDays is the approved retention for dealership key-tag photos. Customer-car photos have no expiry.
const UploadTagRetentionDays = 30

// PurgeExpiredUploads deletes up to limit expired key-tag posts (rows first, in one transaction, with a purge record),
// then removes their photo files. A file that cannot be removed is left for the next run; it is never served because
// its row is gone. It returns how many posts were purged.
func (s *Store) PurgeExpiredUploads(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 1000 {
		limit = 200
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT p.id::text,p.organization_id::text,p.location_id::text,p.kind,p.created_at,
 COALESCE((SELECT array_agg(f.storage_key) FROM employee_upload_photos f WHERE f.post_id=p.id),'{}')
 FROM employee_upload_posts p WHERE p.retention_until IS NOT NULL AND p.retention_until < clock_timestamp()
 ORDER BY p.retention_until LIMIT $1 FOR UPDATE OF p SKIP LOCKED`, limit)
	if err != nil {
		return 0, err
	}
	type expired struct {
		id, org, loc, kind string
		posted             time.Time
		keys               []string
	}
	var batch []expired
	for rows.Next() {
		var e expired
		if err = rows.Scan(&e.id, &e.org, &e.loc, &e.kind, &e.posted, &e.keys); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, e)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(batch) == 0 {
		return 0, err
	}
	for _, e := range batch {
		if _, err = tx.Exec(ctx, `INSERT INTO employee_upload_purges(post_id,organization_id,location_id,kind,photo_count,posted_at) VALUES($1,$2,$3,$4,$5,$6)`, e.id, e.org, e.loc, e.kind, len(e.keys), e.posted); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM employee_upload_posts WHERE id=$1`, e.id); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	if s.uploadDir != "" {
		for _, e := range batch {
			for _, key := range e.keys {
				_ = os.Remove(filepath.Join(s.uploadDir, key))
			}
		}
	}
	return len(batch), nil
}
