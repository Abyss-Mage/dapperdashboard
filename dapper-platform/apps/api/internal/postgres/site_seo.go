package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// SiteSEO is what search engines and link previews see for the public website. Empty fields fall back to the page's own text.
type SiteSEO struct {
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	Keywords           string             `json:"keywords"`
	Image              string             `json:"image"`
	GoogleVerification string             `json:"googleVerification"`
	BingVerification   string             `json:"bingVerification"`
	Pages              map[string]PageSEO `json:"pages"`
}

// PageSEO overrides the site-wide settings for one page (keyed by its path, e.g. "/services"). Hidden keeps it out of search
// results and the sitemap.
type PageSEO struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Keywords    string `json:"keywords"`
	Image       string `json:"image"`
	Hidden      bool   `json:"hidden"`
}

var seoPath = regexp.MustCompile(`^/[a-z0-9-]*$`)
var seoVerification = regexp.MustCompile(`^[A-Za-z0-9_-]{0,100}$`)

func seoText(v string, max int) bool { return utf8.RuneCountInString(v) <= max && !strings.ContainsAny(v, "<>") }

// seoImage accepts a picture on this site ("/site/assets/…") or a secure address.
func seoImage(v string) bool {
	return v == "" || len(v) <= 400 && !strings.ContainsAny(v, " \t\r\n<>\"'\\") && (strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") || strings.HasPrefix(v, "https://"))
}

func (p *PageSEO) clean() bool {
	p.Title, p.Description, p.Keywords, p.Image = strings.TrimSpace(p.Title), strings.TrimSpace(p.Description), strings.TrimSpace(p.Keywords), strings.TrimSpace(p.Image)
	return seoText(p.Title, 70) && seoText(p.Description, 320) && seoText(p.Keywords, 500) && seoImage(p.Image)
}

func (s *Store) seoGrant(ctx context.Context, tx pgx.Tx, actor, org string) error {
	ok, err := actorHasOrgCapability(ctx, tx, actor, org, "access.manage")
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// SiteSEOFor reads a website's search settings for editing (access.manage). The site is named by its slug, so the owner edits the
// public website whichever franchise the dashboard has open.
func (s *Store) SiteSEOFor(ctx context.Context, actor, org, slug string) (SiteSEO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SiteSEO{}, err
	}
	defer tx.Rollback(ctx)
	if err = s.seoGrant(ctx, tx, actor, org); err != nil {
		return SiteSEO{}, err
	}
	var out SiteSEO
	err = tx.QueryRow(ctx, `SELECT seo FROM public_sites WHERE organization_id=$1 AND slug=$2`, org, slug).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteSEO{}, ErrContentNotFound
	}
	return out, err
}

// SaveSiteSEO stores the website's search settings (access.manage: the owner and administrators, owner 2026-11-13).
func (s *Store) SaveSiteSEO(ctx context.Context, actor, org, slug string, in SiteSEO) (SiteSEO, error) {
	site := PageSEO{Title: in.Title, Description: in.Description, Keywords: in.Keywords, Image: in.Image}
	if !site.clean() || len(in.Pages) > 100 {
		return SiteSEO{}, ErrContentInput
	}
	in.Title, in.Description, in.Keywords, in.Image = site.Title, site.Description, site.Keywords, site.Image
	in.GoogleVerification, in.BingVerification = strings.TrimSpace(in.GoogleVerification), strings.TrimSpace(in.BingVerification)
	if !seoVerification.MatchString(in.GoogleVerification) || !seoVerification.MatchString(in.BingVerification) {
		return SiteSEO{}, ErrContentInput
	}
	pages := map[string]PageSEO{}
	for path, p := range in.Pages {
		if !seoPath.MatchString(path) || len(path) > 80 || !p.clean() {
			return SiteSEO{}, ErrContentInput
		}
		if p != (PageSEO{}) {
			pages[path] = p
		}
	}
	in.Pages = pages
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SiteSEO{}, err
	}
	defer tx.Rollback(ctx)
	if err = s.seoGrant(ctx, tx, actor, org); err != nil {
		return SiteSEO{}, err
	}
	data, err := json.Marshal(in)
	if err != nil {
		return SiteSEO{}, err
	}
	var loc string
	err = tx.QueryRow(ctx, `UPDATE public_sites SET seo=$3 WHERE organization_id=$1 AND slug=$2 RETURNING location_id::text`, org, slug, data).Scan(&loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteSEO{}, ErrContentNotFound
	}
	if err != nil {
		return SiteSEO{}, err
	}
	if err = contentAudit(ctx, tx, Scope{OrganizationID: org, LocationID: loc, ActorID: actor}, "site.seo.revised", loc, in); err != nil {
		return SiteSEO{}, err
	}
	return in, tx.Commit(ctx)
}
