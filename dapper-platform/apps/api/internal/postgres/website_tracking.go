package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrTrackingInput    = errors.New("tracking identifiers are not valid")
	ErrTrackingConflict = errors.New("tracking settings changed since they were loaded")
	gtmPattern          = regexp.MustCompile(`^GTM-[A-Z0-9]{4,12}$`)
	pixelPattern        = regexp.MustCompile(`^[0-9]{8,20}$`)
	mailchimpPattern    = regexp.MustCompile(`^https://(chimpstatic\.com|[a-z0-9-]+\.list-manage\.com)/[A-Za-z0-9._~:/?#@!$&()*+,;=%-]+$`)
)

// WebsiteTracking is what the public site loads: a Google Tag Manager container, a Meta Pixel and a Mailchimp popup script.
type WebsiteTracking struct {
	GTMID              string `json:"gtmId"`
	MetaPixelID        string `json:"metaPixelId"`
	MailchimpScriptURL string `json:"mailchimpScriptUrl"`
	Version            int    `json:"version"`
}

func (t *WebsiteTracking) normalize() error {
	t.GTMID = strings.ToUpper(strings.TrimSpace(t.GTMID))
	t.MetaPixelID = strings.TrimSpace(t.MetaPixelID)
	t.MailchimpScriptURL = strings.TrimSpace(t.MailchimpScriptURL)
	if t.GTMID != "" && !gtmPattern.MatchString(t.GTMID) {
		return ErrTrackingInput
	}
	if t.MetaPixelID != "" && !pixelPattern.MatchString(t.MetaPixelID) {
		return ErrTrackingInput
	}
	if t.MailchimpScriptURL != "" && (len(t.MailchimpScriptURL) > 600 || !mailchimpPattern.MatchString(t.MailchimpScriptURL)) {
		return ErrTrackingInput
	}
	return nil
}

func (s *Store) contentManager(ctx context.Context, tx pgx.Tx, actor, org string) error {
	ok, err := actorHasOrgCapability(ctx, tx, actor, org, "content.manage")
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// GetWebsiteTracking is for administrators who manage website content. It returns empty settings (version 0) before anything is saved.
func (s *Store) GetWebsiteTracking(ctx context.Context, actor, org string) (WebsiteTracking, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WebsiteTracking{}, err
	}
	defer tx.Rollback(ctx)
	if err = s.contentManager(ctx, tx, actor, org); err != nil {
		return WebsiteTracking{}, err
	}
	var t WebsiteTracking
	err = tx.QueryRow(ctx, `SELECT gtm_id,meta_pixel_id,mailchimp_script_url,version FROM website_tracking WHERE organization_id=$1`, org).Scan(&t.GTMID, &t.MetaPixelID, &t.MailchimpScriptURL, &t.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebsiteTracking{}, nil
	}
	return t, err
}

// SaveWebsiteTracking replaces the settings. version is the revision the editor loaded (0 before the first save).
func (s *Store) SaveWebsiteTracking(ctx context.Context, actor, org string, t WebsiteTracking, version int) (WebsiteTracking, error) {
	if err := t.normalize(); err != nil {
		return WebsiteTracking{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WebsiteTracking{}, err
	}
	defer tx.Rollback(ctx)
	if err = s.contentManager(ctx, tx, actor, org); err != nil {
		return WebsiteTracking{}, err
	}
	if version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO website_tracking(organization_id,gtm_id,meta_pixel_id,mailchimp_script_url,updated_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING version`, org, t.GTMID, t.MetaPixelID, t.MailchimpScriptURL, actor).Scan(&t.Version)
	} else {
		err = tx.QueryRow(ctx, `UPDATE website_tracking SET gtm_id=$2,meta_pixel_id=$3,mailchimp_script_url=$4,version=version+1,updated_by=$5,updated_at=now() WHERE organization_id=$1 AND version=$6 RETURNING version`, org, t.GTMID, t.MetaPixelID, t.MailchimpScriptURL, actor, version).Scan(&t.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return WebsiteTracking{}, ErrTrackingConflict
	}
	if err != nil {
		return WebsiteTracking{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return WebsiteTracking{}, err
	}
	return t, nil
}

// PublicWebsiteTracking is what the public site loads. This installation serves one organization's website, so the most
// recently saved settings are the site's settings. It reveals only the identifiers the browser needs to load the tags.
func (s *Store) PublicWebsiteTracking(ctx context.Context) (WebsiteTracking, error) {
	var t WebsiteTracking
	err := s.pool.QueryRow(ctx, `SELECT gtm_id,meta_pixel_id,mailchimp_script_url FROM website_tracking ORDER BY updated_at DESC LIMIT 1`).Scan(&t.GTMID, &t.MetaPixelID, &t.MailchimpScriptURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebsiteTracking{}, nil
	}
	return t, err
}
