package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"image"
	"image/jpeg"
	"image/png"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrContentInput = errors.New("invalid publication details")
var ErrContentConflict = errors.New("publication or service changed; refresh before saving")
var ErrContentNotFound = errors.New("published content unavailable")
var siteSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,59}$`)

type PublicSite struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Published bool   `json:"published"`
	Version   int    `json:"version"`
}
type ServicePublication struct {
	ServiceID      string  `json:"serviceId"`
	Name           string  `json:"name"`
	Kind           string  `json:"kind"`
	ParentID       *string `json:"parentId"`
	ServiceVersion int     `json:"serviceVersion"`
	Category       string  `json:"category"`
	Summary        string  `json:"summary"`
	Published      bool    `json:"published"`
	Version        int     `json:"version"`
}
type PortfolioImage struct {
	ID             string `json:"id"`
	MediaID        string `json:"mediaId"`
	Title          string `json:"title"`
	Alt            string `json:"alt"`
	Category       string `json:"category"`
	SortOrder      int    `json:"sortOrder"`
	Published      bool   `json:"published"`
	Version        int    `json:"version"`
	RightsReviewed bool   `json:"rightsReviewed"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
}
type PublishedContent struct {
	Site      PublicSite           `json:"site"`
	Services  []ServicePublication `json:"services"`
	Portfolio []PortfolioImage     `json:"portfolio"`
	Limit     int                  `json:"limit"`
}

func validContentText(value string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(value))
	return n >= min && n <= max
}
func validServiceCategory(value string) bool {
	switch value {
	case "2-wheeler", "hatchback", "sedan", "suv", "pickup", "van", "ceramic", "ppf", "aircraft", "boat":
		return true
	}
	return false
}
func contentAudit(ctx context.Context, tx pgx.Tx, scope Scope, action, id string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO content_events(organization_id,location_id,actor_id,action,entity_id,snapshot) VALUES($1,$2,$3,$4,$5,$6)`, scope.OrganizationID, scope.LocationID, scope.ActorID, action, id, data)
	return err
}
func (s *Store) SavePublicSite(ctx context.Context, scope Scope, input PublicSite) (PublicSite, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(input.Slug)
	if !siteSlug.MatchString(input.Slug) || !validContentText(input.Name, 2, 120) || input.Version < 0 {
		return PublicSite{}, ErrContentInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PublicSite{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return PublicSite{}, err
	}
	var out PublicSite
	if input.Version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO public_sites(organization_id,location_id,slug,name,published) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING slug,name,published,version`, scope.OrganizationID, scope.LocationID, input.Slug, input.Name, input.Published).Scan(&out.Slug, &out.Name, &out.Published, &out.Version)
	} else {
		// Published URL identity is stable; changing a slug needs a redirect migration.
		err = tx.QueryRow(ctx, `UPDATE public_sites SET name=$4,published=$5,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND slug=$3 AND version=$6 RETURNING slug,name,published,version`, scope.OrganizationID, scope.LocationID, input.Slug, input.Name, input.Published, input.Version).Scan(&out.Slug, &out.Name, &out.Published, &out.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrContentConflict
	}
	if err != nil {
		return out, err
	}
	if err = contentAudit(ctx, tx, scope, "site.revised", out.Slug, out); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) SaveServicePublication(ctx context.Context, scope Scope, input ServicePublication) (ServicePublication, error) {
	input.Summary = strings.TrimSpace(input.Summary)
	if input.ServiceID == "" || input.ServiceVersion < 1 || input.Version < 0 || !validServiceCategory(input.Category) || !validContentText(input.Summary, 1, 2000) {
		return input, ErrContentInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return input, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "catalogue.manage"); err != nil {
		return input, err
	}
	// Lock the current operational record. Publication never supplies a name or price.
	err = tx.QueryRow(ctx, `SELECT name,kind,parent_id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND version=$4 AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, input.ServiceID, input.ServiceVersion).Scan(&input.Name, &input.Kind, &input.ParentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return input, ErrContentConflict
	}
	if err != nil {
		return input, err
	}
	var version int
	if input.Version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO service_publications(organization_id,location_id,service_id,category,summary,published) SELECT $1,$2,$3,$4,$5,$6 WHERE EXISTS(SELECT 1 FROM public_sites WHERE organization_id=$1 AND location_id=$2) ON CONFLICT DO NOTHING RETURNING version`, scope.OrganizationID, scope.LocationID, input.ServiceID, input.Category, input.Summary, input.Published).Scan(&version)
	} else {
		err = tx.QueryRow(ctx, `UPDATE service_publications SET category=$4,summary=$5,published=$6,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND service_id=$3 AND version=$7 RETURNING version`, scope.OrganizationID, scope.LocationID, input.ServiceID, input.Category, input.Summary, input.Published, input.Version).Scan(&version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return input, ErrContentConflict
	}
	if err != nil {
		return input, err
	}
	input.Version = version
	if err = contentAudit(ctx, tx, scope, "service.publication_revised", input.ServiceID, input); err != nil {
		return input, err
	}
	return input, tx.Commit(ctx)
}
func (s *Store) ContentForManagement(ctx context.Context, scope Scope) (PublishedContent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PublishedContent{}, err
	}
	defer tx.Rollback(ctx)
	contentAllowed := true
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		contentAllowed = false
		if !errors.Is(err, ErrForbidden) {
			return PublishedContent{}, err
		}
		if err = authorizeTransaction(ctx, tx, scope, "catalogue.manage"); err != nil {
			return PublishedContent{}, err
		}
	}
	out, err := readSiteContent(ctx, tx, scope.OrganizationID, scope.LocationID, false)
	if err != nil {
		return out, err
	}
	if !contentAllowed {
		out.Portfolio = []PortfolioImage{}
	}
	return out, tx.Commit(ctx)
}
func (s *Store) PublicContent(ctx context.Context, slug string) (PublishedContent, error) {
	if !siteSlug.MatchString(slug) {
		return PublishedContent{}, ErrContentNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PublishedContent{}, err
	}
	defer tx.Rollback(ctx)
	var org, loc string
	err = tx.QueryRow(ctx, `SELECT organization_id::text,location_id::text FROM public_sites WHERE slug=$1 AND published`, slug).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublishedContent{}, ErrContentNotFound
	}
	if err != nil {
		return PublishedContent{}, err
	}
	out, err := readSiteContent(ctx, tx, org, loc, true)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func readSiteContent(ctx context.Context, tx pgx.Tx, org, loc string, public bool) (PublishedContent, error) {
	out := PublishedContent{Services: []ServicePublication{}, Portfolio: []PortfolioImage{}, Limit: 200}
	err := tx.QueryRow(ctx, `SELECT slug,name,published,version FROM public_sites WHERE organization_id=$1 AND location_id=$2`, org, loc).Scan(&out.Site.Slug, &out.Site.Name, &out.Site.Published, &out.Site.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT s.id::text,s.name,s.kind,s.parent_id::text,s.version,coalesce(p.category,''),coalesce(p.summary,''),coalesce(p.published,false),coalesce(p.version,0) FROM catalog_services s LEFT JOIN service_publications p ON (p.organization_id,p.location_id,p.service_id)=(s.organization_id,s.location_id,s.id) WHERE s.organization_id=$1 AND s.location_id=$2 AND NOT s.archived AND (NOT $3::boolean OR p.published) AND (NOT $3::boolean OR s.kind='main' OR EXISTS(SELECT 1 FROM catalog_addon_parents l JOIN catalog_services parent ON parent.id=l.main_id JOIN service_publications pub ON pub.service_id=parent.id WHERE l.addon_id=s.id AND NOT parent.archived AND pub.published)) ORDER BY s.name,s.id LIMIT 200`, org, loc, public)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item ServicePublication
		if err = rows.Scan(&item.ServiceID, &item.Name, &item.Kind, &item.ParentID, &item.ServiceVersion, &item.Category, &item.Summary, &item.Published, &item.Version); err != nil {
			rows.Close()
			return out, err
		}
		out.Services = append(out.Services, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, `SELECT p.id::text,p.media_id::text,p.title,p.alt,p.category,p.sort_order,p.published,p.version,m.width,m.height FROM portfolio_images p JOIN content_media m ON m.id=p.media_id WHERE p.organization_id=$1 AND p.location_id=$2 AND (NOT $3::boolean OR p.published) ORDER BY p.sort_order,p.id LIMIT 200`, org, loc, public)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item PortfolioImage
		if err = rows.Scan(&item.ID, &item.MediaID, &item.Title, &item.Alt, &item.Category, &item.SortOrder, &item.Published, &item.Version, &item.Width, &item.Height); err != nil {
			return out, err
		}
		out.Portfolio = append(out.Portfolio, item)
	}
	return out, rows.Err()
}

// Re-encode raster uploads to remove EXIF and active/polyglot payloads. No SVG/remote fetch.
func NormalizeContentImage(data []byte) ([]byte, string, error) {
	if len(data) == 0 || len(data) > 6*1024*1024 {
		return nil, "", ErrContentInput
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 24000000 {
		return nil, "", ErrContentInput
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", ErrContentInput
	}
	var output bytes.Buffer
	if format == "jpeg" {
		err = jpeg.Encode(&output, img, &jpeg.Options{Quality: 88})
	} else {
		err = png.Encode(&output, img)
	}
	if err != nil || output.Len() > 6*1024*1024 {
		return nil, "", ErrContentInput
	}
	return output.Bytes(), "image/" + format, nil
}
func (s *Store) UploadContentImage(ctx context.Context, scope Scope, data []byte) (string, error) {
	// Reject unauthorized requests before expensive image decoding.
	if err := s.authorize(ctx, scope, "content.manage"); err != nil {
		return "", err
	}
	normalized, mime, err := NormalizeContentImage(data)
	if err != nil {
		return "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return "", err
	}
	var id string
	var width, height int
	if config, _, e := image.DecodeConfig(bytes.NewReader(normalized)); e == nil {
		width, height = config.Width, config.Height
	}
	err = tx.QueryRow(ctx, `INSERT INTO content_media(organization_id,location_id,content_type,data,width,height) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, scope.OrganizationID, scope.LocationID, mime, normalized, width, height).Scan(&id)
	if err != nil {
		return "", err
	}
	if err = contentAudit(ctx, tx, scope, "portfolio.image_uploaded", id, map[string]any{"contentType": mime, "bytes": len(normalized)}); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}
func (s *Store) SavePortfolioImage(ctx context.Context, scope Scope, input PortfolioImage) (PortfolioImage, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Alt = strings.TrimSpace(input.Alt)
	if !validContentText(input.Title, 2, 120) || !validContentText(input.Alt, 2, 300) || input.MediaID == "" || input.Version < 0 || input.SortOrder < 0 || input.SortOrder > 10000 || (input.Published && !input.RightsReviewed) {
		return input, ErrContentInput
	}
	switch input.Category {
	case "detailing", "ceramic", "ppf", "other":
	default:
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
		err = tx.QueryRow(ctx, `INSERT INTO portfolio_images(organization_id,location_id,media_id,title,alt,category,sort_order,published) SELECT $1,$2,$3,$4,$5,$6,$7,$8 WHERE EXISTS(SELECT 1 FROM public_sites WHERE organization_id=$1 AND location_id=$2) RETURNING id::text,version`, scope.OrganizationID, scope.LocationID, input.MediaID, input.Title, input.Alt, input.Category, input.SortOrder, input.Published).Scan(&input.ID, &input.Version)
	} else {
		err = tx.QueryRow(ctx, `UPDATE portfolio_images SET media_id=$4,title=$5,alt=$6,category=$7,sort_order=$8,published=$9,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND version=$10 RETURNING version`, scope.OrganizationID, scope.LocationID, input.ID, input.MediaID, input.Title, input.Alt, input.Category, input.SortOrder, input.Published, input.Version).Scan(&input.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return input, ErrContentConflict
	}
	if err != nil {
		return input, err
	}
	if err = contentAudit(ctx, tx, scope, "portfolio.revised", input.ID, input); err != nil {
		return input, err
	}
	return input, tx.Commit(ctx)
}
func (s *Store) PublicImage(ctx context.Context, id string) ([]byte, string, error) {
	var data []byte
	var mime string
	err := s.pool.QueryRow(ctx, `SELECT m.data,m.content_type FROM content_media m WHERE m.id=$1 AND EXISTS(SELECT 1 FROM portfolio_images p JOIN public_sites s ON (s.organization_id,s.location_id)=(p.organization_id,p.location_id) WHERE p.media_id=m.id AND p.published AND s.published)`, id).Scan(&data, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrContentNotFound
	}
	return data, mime, err
}

func (s *Store) ManagedContentImage(ctx context.Context, scope Scope, id string) ([]byte, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return nil, "", err
	}
	var data []byte
	var mime string
	err = tx.QueryRow(ctx, `SELECT data,content_type FROM content_media WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id).Scan(&data, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrContentNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return data, mime, tx.Commit(ctx)
}
