package postgres

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var ErrSiteAsset = errors.New("that picture or film is not valid")

var siteAssetPath = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_.-]{1,200}$`)

// SiteAsset is one uploaded replacement, without its bytes.
type SiteAsset struct {
	Path        string    `json:"path"`
	ContentType string    `json:"contentType"`
	Bytes       int       `json:"bytes"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

const maxSiteAssetBytes = 25 << 20

func cleanAssetPath(p string) (string, bool) {
	p = strings.TrimPrefix(strings.TrimSpace(p), "/")
	return p, siteAssetPath.MatchString(p) && !strings.Contains(p, "..") && !strings.HasPrefix(p, "frames/")
}

// SaveSiteAsset stores a picture or film for a path. The kind is read from the file itself, never from what the browser says.
func (s *Store) SaveSiteAsset(ctx context.Context, scope Scope, path string, data []byte) error {
	path, ok := cleanAssetPath(path)
	kind := http.DetectContentType(data)
	switch kind {
	case "image/jpeg", "image/png", "image/webp", "video/mp4", "video/webm":
	default:
		ok = false
	}
	if !ok || len(data) == 0 || len(data) > maxSiteAssetBytes {
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
	if _, err = tx.Exec(ctx, `INSERT INTO site_assets(organization_id,location_id,path,content_type,data,updated_by) VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(organization_id,location_id,path) DO UPDATE SET content_type=$4,data=$5,updated_by=$6,updated_at=now()`, scope.OrganizationID, scope.LocationID, path, kind, data, scope.ActorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteSiteAsset(ctx context.Context, scope Scope, path string) error {
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
	return tx.Commit(ctx)
}

func (s *Store) SiteAssets(ctx context.Context, scope Scope) ([]SiteAsset, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT path,content_type,length(data),updated_at FROM site_assets WHERE organization_id=$1 AND location_id=$2 ORDER BY created_at,path`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SiteAsset{}
	for rows.Next() {
		var a SiteAsset
		if err = rows.Scan(&a.Path, &a.ContentType, &a.Bytes, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PublicSiteAsset is what the website serves for an overridden path (the website has one owner, so the path alone finds it).
func (s *Store) PublicSiteAsset(ctx context.Context, path string) ([]byte, string, time.Time, error) {
	path, ok := cleanAssetPath(path)
	if !ok {
		return nil, "", time.Time{}, ErrSiteAsset
	}
	var data []byte
	var kind string
	var at time.Time
	err := s.pool.QueryRow(ctx, `SELECT data,content_type,updated_at FROM site_assets WHERE path=$1 ORDER BY updated_at DESC LIMIT 1`, path).Scan(&data, &kind, &at)
	return data, kind, at, err
}

// PublicSiteAssetPaths lists every overridden path, so the front door only asks for the ones that exist.
func (s *Store) PublicSiteAssetPaths(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT path FROM site_assets`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PublicFilms lists the films the owner added (paths under films/), oldest first. Without any, the website shows its built-in films.
func (s *Store) PublicFilms(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT path FROM site_assets WHERE path LIKE 'films/%' AND content_type LIKE 'video/%' ORDER BY created_at,path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, "/site/assets/"+p)
	}
	return out, rows.Err()
}
