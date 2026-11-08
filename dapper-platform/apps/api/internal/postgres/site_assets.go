package postgres

import (
	"context"
	"errors"
	"net/http"
	"os"
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

// mainSiteSlug is the public website everything below belongs to. Another franchise's uploads never reach it.
func mainSiteSlug() string {
	if v := os.Getenv("DAPPER_SITE_SLUG"); v != "" {
		return v
	}
	return "dapper-detailing-pros-peterborough"
}

const onMainSite = ` JOIN public_sites ps ON ps.organization_id=a.organization_id AND ps.location_id=a.location_id AND ps.slug=$1 `

// PublicSiteAssetVersion says whether a path is replaced and when, without reading the file, so a browser that already has it gets a cheap "not modified".
func (s *Store) PublicSiteAssetVersion(ctx context.Context, path string) (string, time.Time, error) {
	path, ok := cleanAssetPath(path)
	if !ok {
		return "", time.Time{}, ErrSiteAsset
	}
	var kind string
	var at time.Time
	err := s.pool.QueryRow(ctx, `SELECT a.content_type,a.updated_at FROM site_assets a`+onMainSite+`WHERE a.path=$2`, mainSiteSlug(), path).Scan(&kind, &at)
	return kind, at, err
}

// PublicSiteAssetData is the file itself.
func (s *Store) PublicSiteAssetData(ctx context.Context, path string) ([]byte, error) {
	path, _ = cleanAssetPath(path)
	var data []byte
	err := s.pool.QueryRow(ctx, `SELECT a.data FROM site_assets a`+onMainSite+`WHERE a.path=$2`, mainSiteSlug(), path).Scan(&data)
	return data, err
}

// PublicSiteAssetPaths lists every overridden path, so the front door only asks for the ones that exist.
func (s *Store) PublicSiteAssetPaths(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT a.path FROM site_assets a`+onMainSite, mainSiteSlug())
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
	rows, err := s.pool.Query(ctx, `SELECT a.path FROM site_assets a`+onMainSite+`WHERE a.path LIKE 'films/%' AND a.content_type LIKE 'video/%' AND NOT EXISTS(SELECT 1 FROM site_asset_hidden h WHERE (h.organization_id,h.location_id,h.path)=(a.organization_id,a.location_id,a.path)) ORDER BY a.created_at,a.path`, mainSiteSlug())
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

// SetSiteAssetHidden takes a built-in file off the website (hide=true) or puts it back.
func (s *Store) SetSiteAssetHidden(ctx context.Context, scope Scope, path string, hide bool) error {
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
	if hide {
		_, err = tx.Exec(ctx, `INSERT INTO site_asset_hidden(organization_id,location_id,path) VALUES($1,$2,$3) ON CONFLICT(organization_id,location_id,path) DO UPDATE SET removed_at=now(),purged=false`, scope.OrganizationID, scope.LocationID, path)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM site_asset_hidden WHERE organization_id=$1 AND location_id=$2 AND path=$3`, scope.OrganizationID, scope.LocationID, path)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) hiddenPaths(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := s.pool.Query(ctx, q, args...)
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

// HiddenSiteAssets lists what this location has taken off the website (for the dashboard).
func (s *Store) HiddenSiteAssets(ctx context.Context, scope Scope) ([]string, error) {
	return s.hiddenPaths(ctx, `SELECT path FROM site_asset_hidden WHERE organization_id=$1 AND location_id=$2 ORDER BY path`, scope.OrganizationID, scope.LocationID)
}

// PublicHiddenSiteAssets lists what the main site no longer shows.
func (s *Store) PublicHiddenSiteAssets(ctx context.Context) ([]string, error) {
	return s.hiddenPaths(ctx, `SELECT a.path FROM site_asset_hidden a`+onMainSite+`ORDER BY a.path`, mainSiteSlug())
}
