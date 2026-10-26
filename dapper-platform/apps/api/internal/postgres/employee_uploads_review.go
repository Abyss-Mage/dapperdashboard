package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReviewedUpload is an employee post as seen by a supervisor or administrator.
type ReviewedUpload struct {
	UploadPost
	Employee string `json:"employee"`
}

type UploadView struct {
	ViewedAt time.Time `json:"viewedAt"`
	Viewer   string    `json:"viewer"`
	Employee string    `json:"employee"`
	PostID   string    `json:"postId"`
	PhotoID  string    `json:"photoId"`
}

// ListLocationUploads returns posts made at the scope's location. It needs attendance.read at that location.
func (s *Store) ListLocationUploads(ctx context.Context, scope Scope, kind string) ([]ReviewedUpload, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.read"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT p.id::text,p.kind,COALESCE(d.account_name,''),COALESCE(p.customer_name,''),COALESCE(p.inspection_type,''),p.notes,p.latitude,p.longitude,p.accuracy_meters,p.captured_at,p.created_at,COALESCE(p.booking_id::text,''),COALESCE(bk.service_name,''),bk.booking_time,p.retention_until,COALESCE(NULLIF(c_u.email,''),u.external_subject)
FROM employee_upload_posts p JOIN principals u ON u.id=p.principal_id LEFT JOIN password_credentials c_u ON c_u.principal_id=u.id
LEFT JOIN bookings bk ON bk.id=p.booking_id
LEFT JOIN dealership_accounts d ON (d.organization_id,d.location_id,d.id)=(p.organization_id,p.location_id,p.dealership_id)
WHERE p.organization_id=$1 AND p.location_id=$2 AND ($3='' OR p.kind=$3) ORDER BY p.created_at DESC LIMIT 100`, scope.OrganizationID, scope.LocationID, kind)
	if err != nil {
		return nil, err
	}
	posts := []ReviewedUpload{}
	index := map[string]int{}
	ids := []string{}
	for rows.Next() {
		var p ReviewedUpload
		if err = rows.Scan(&p.ID, &p.Kind, &p.DealershipName, &p.CustomerName, &p.InspectionType, &p.Notes, &p.Latitude, &p.Longitude, &p.AccuracyMeters, &p.CapturedAt, &p.CreatedAt, &p.BookingID, &p.ServiceName, &p.BookingTime, &p.RetentionUntil, &p.Employee); err != nil {
			rows.Close()
			return nil, err
		}
		p.Photos = []UploadPhoto{}
		index[p.ID] = len(posts)
		ids = append(ids, p.ID)
		posts = append(posts, p)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(posts) == 0 {
		return posts, err
	}
	photoRows, err := tx.Query(ctx, `SELECT id::text,post_id::text,ordinal,width,height,size_bytes FROM employee_upload_photos WHERE post_id::text = ANY($1) ORDER BY post_id,ordinal`, ids)
	if err != nil {
		return nil, err
	}
	defer photoRows.Close()
	for photoRows.Next() {
		var ph UploadPhoto
		var post string
		if err = photoRows.Scan(&ph.ID, &post, &ph.Ordinal, &ph.Width, &ph.Height, &ph.SizeBytes); err != nil {
			return nil, err
		}
		posts[index[post]].Photos = append(posts[index[post]].Photos, ph)
	}
	return posts, photoRows.Err()
}

// ReviewUploadPhoto returns a photo from the scope's location and records the view in the same transaction.
// If the view cannot be recorded, or the file cannot be read, nothing is returned. Owners viewing their own
// photos through this path are not logged.
func (s *Store) ReviewUploadPhoto(ctx context.Context, scope Scope, photo string) ([]byte, error) {
	if s.uploadDir == "" {
		return nil, ErrUploadsUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.read"); err != nil {
		return nil, err
	}
	var key, post, owner string
	err = tx.QueryRow(ctx, `SELECT f.storage_key,p.id::text,p.principal_id::text FROM employee_upload_photos f JOIN employee_upload_posts p ON p.id=f.post_id WHERE f.id=$1 AND p.organization_id=$2 AND p.location_id=$3`, photo, scope.OrganizationID, scope.LocationID).Scan(&key, &post, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUploadNotFound
	}
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(s.uploadDir, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrUploadNotFound
	}
	if err != nil {
		return nil, err
	}
	if owner != scope.ActorID {
		if _, err = tx.Exec(ctx, `INSERT INTO employee_upload_views(organization_id,location_id,post_id,photo_id,owner_id,viewer_id) VALUES($1,$2,$3,$4,$5,$6)`, scope.OrganizationID, scope.LocationID, post, photo, owner, scope.ActorID); err != nil {
			return nil, err
		}
	}
	return data, tx.Commit(ctx)
}

// ListUploadViews returns the latest 200 recorded views at the location. It needs attendance.manage.
func (s *Store) ListUploadViews(ctx context.Context, scope Scope) ([]UploadView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "attendance.manage"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT v.viewed_at,COALESCE(NULLIF(c_vw.email,''),vw.external_subject),COALESCE(NULLIF(c_ow.email,''),ow.external_subject),v.post_id::text,v.photo_id::text FROM employee_upload_views v JOIN principals vw ON vw.id=v.viewer_id LEFT JOIN password_credentials c_vw ON c_vw.principal_id=vw.id JOIN principals ow ON ow.id=v.owner_id LEFT JOIN password_credentials c_ow ON c_ow.principal_id=ow.id WHERE v.organization_id=$1 AND v.location_id=$2 ORDER BY v.id DESC LIMIT 200`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UploadView{}
	for rows.Next() {
		var v UploadView
		if err = rows.Scan(&v.ViewedAt, &v.Viewer, &v.Employee, &v.PostID, &v.PhotoID); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
