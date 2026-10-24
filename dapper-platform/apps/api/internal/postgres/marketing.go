package postgres

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrMarketingConfig = errors.New("marketing configuration invalid")
	ErrMarketingInput  = errors.New("marketing input invalid")
	ErrPopupNotFound   = errors.New("popup form not found")
	ErrConsentNotFound = errors.New("marketing consent not found")
	ErrDuplicatePopup  = errors.New("duplicate popup form name")
)

type MailchimpSettings struct {
	ID               string    `json:"id"`
	OrganizationID   string    `json:"organizationId"`
	LocationID       string    `json:"locationId"`
	APIKeyConfigured bool      `json:"apiKeyConfigured"`
	ServerPrefix     string    `json:"serverPrefix"`
	DefaultListID    string    `json:"defaultListId"`
	DoubleOptIn      bool      `json:"doubleOptIn"`
	SyncEnabled      bool      `json:"syncEnabled"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type UpdateMailchimpSettingsInput struct {
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
	ServerPrefix     string `json:"serverPrefix"`
	DefaultListID    string `json:"defaultListId"`
	DoubleOptIn      bool   `json:"doubleOptIn"`
	SyncEnabled      bool   `json:"syncEnabled"`
}

type MarketingConsent struct {
	ID                      string     `json:"id"`
	OrganizationID          string     `json:"organizationId"`
	LocationID              string     `json:"locationId"`
	CustomerID              *string    `json:"customerId"`
	Email                   string     `json:"email"`
	FirstName               string     `json:"firstName"`
	LastName                string     `json:"lastName"`
	ConsentStatus           string     `json:"consentStatus"`
	ConsentSource           string     `json:"consentSource"`
	ConsentText             string     `json:"consentText"`
	IPAddress               string     `json:"ipAddress"`
	UserAgent               string     `json:"userAgent"`
	ConsentedAt             time.Time  `json:"consentedAt"`
	RevokedAt               *time.Time `json:"revokedAt"`
	SyncStatus              string     `json:"syncStatus"`
	MailchimpSubscriberHash string     `json:"mailchimpSubscriberHash"`
	LastSyncedAt            *time.Time `json:"lastSyncedAt"`
	LastSyncError           string     `json:"lastSyncError"`
}

type RecordConsentInput struct {
	CustomerID    *string `json:"customerId"`
	Email         string  `json:"email"`
	FirstName     string  `json:"firstName"`
	LastName      string  `json:"lastName"`
	ConsentStatus string  `json:"consentStatus"` // opt_in, opt_out, unsubscribed
	ConsentSource string  `json:"consentSource"` // popup_form, website_footer, customer_portal, booking_checkout, manual_staff
	ConsentText   string  `json:"consentText"`
	IPAddress     string  `json:"ipAddress"`
	UserAgent     string  `json:"userAgent"`
}

type PopupFormField struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // text, email, tel, select
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

type PopupForm struct {
	ID                     string           `json:"id"`
	OrganizationID         string           `json:"organizationId"`
	LocationID             string           `json:"locationId"`
	Name                   string           `json:"name"`
	Title                  string           `json:"title"`
	Description            string           `json:"description"`
	Headline               string           `json:"headline"`
	Fields                 []PopupFormField `json:"fields"`
	AudienceID             string           `json:"audienceId"`
	Tags                   []string         `json:"tags"`
	DisplayDelaySeconds    int              `json:"displayDelaySeconds"`
	SelectedPages          []string         `json:"selectedPages"`
	RepeatDisplayLimitDays int              `json:"repeatDisplayLimitDays"`
	SuccessMessage         string           `json:"successMessage"`
	Active                 bool             `json:"active"`
	CreatedAt              time.Time        `json:"createdAt"`
	UpdatedAt              time.Time        `json:"updatedAt"`
}

type CreatePopupFormInput struct {
	Name                   string           `json:"name"`
	Title                  string           `json:"title"`
	Description            string           `json:"description"`
	Headline               string           `json:"headline"`
	Fields                 []PopupFormField `json:"fields"`
	AudienceID             string           `json:"audienceId"`
	Tags                   []string         `json:"tags"`
	DisplayDelaySeconds    int              `json:"displayDelaySeconds"`
	SelectedPages          []string         `json:"selectedPages"`
	RepeatDisplayLimitDays int              `json:"repeatDisplayLimitDays"`
	SuccessMessage         string           `json:"successMessage"`
	Active                 bool             `json:"active"`
}

type UpdatePopupFormInput struct {
	Name                   string           `json:"name"`
	Title                  string           `json:"title"`
	Description            string           `json:"description"`
	Headline               string           `json:"headline"`
	Fields                 []PopupFormField `json:"fields"`
	AudienceID             string           `json:"audienceId"`
	Tags                   []string         `json:"tags"`
	DisplayDelaySeconds    int              `json:"displayDelaySeconds"`
	SelectedPages          []string         `json:"selectedPages"`
	RepeatDisplayLimitDays int              `json:"repeatDisplayLimitDays"`
	SuccessMessage         string           `json:"successMessage"`
	Active                 bool             `json:"active"`
}

type PopupSubmission struct {
	ID             string         `json:"id"`
	FormID         string         `json:"formId"`
	OrganizationID string         `json:"organizationId"`
	LocationID     string         `json:"locationId"`
	Email          string         `json:"email"`
	FirstName      string         `json:"firstName"`
	LastName       string         `json:"lastName"`
	Metadata       map[string]any `json:"metadata"`
	IPAddress      string         `json:"ipAddress"`
	UserAgent      string         `json:"userAgent"`
	SubmittedAt    time.Time      `json:"submittedAt"`
}

type PublicPopupSubmissionInput struct {
	Email     string         `json:"email"`
	FirstName string         `json:"firstName"`
	LastName  string         `json:"lastName"`
	Metadata  map[string]any `json:"metadata"`
	IPAddress string         `json:"ipAddress"`
	UserAgent string         `json:"userAgent"`
}

func (s *Store) GetMailchimpSettings(ctx context.Context, scope Scope) (MailchimpSettings, error) {
	if err := s.authorize(ctx, scope, "marketing.read"); err != nil {
		return MailchimpSettings{}, err
	}
	var set MailchimpSettings
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, organization_id::text, location_id::text, api_key_configured,
		       server_prefix, default_list_id, double_opt_in, sync_enabled, created_at, updated_at
		FROM mailchimp_settings
		WHERE organization_id=$1 AND location_id=$2`,
		scope.OrganizationID, scope.LocationID,
	).Scan(
		&set.ID, &set.OrganizationID, &set.LocationID, &set.APIKeyConfigured,
		&set.ServerPrefix, &set.DefaultListID, &set.DoubleOptIn, &set.SyncEnabled,
		&set.CreatedAt, &set.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MailchimpSettings{
			OrganizationID: scope.OrganizationID,
			LocationID:     scope.LocationID,
			SyncEnabled:    true,
		}, nil
	}
	return set, err
}

func (s *Store) UpdateMailchimpSettings(ctx context.Context, scope Scope, input UpdateMailchimpSettingsInput) (MailchimpSettings, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MailchimpSettings{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "marketing.manage"); err != nil {
		return MailchimpSettings{}, err
	}

	var set MailchimpSettings
	err = tx.QueryRow(ctx, `
		INSERT INTO mailchimp_settings (organization_id, location_id, api_key_configured, server_prefix, default_list_id, double_opt_in, sync_enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (organization_id, location_id) DO UPDATE SET
			api_key_configured = EXCLUDED.api_key_configured,
			server_prefix = EXCLUDED.server_prefix,
			default_list_id = EXCLUDED.default_list_id,
			double_opt_in = EXCLUDED.double_opt_in,
			sync_enabled = EXCLUDED.sync_enabled,
			updated_at = now()
		RETURNING id::text, organization_id::text, location_id::text, api_key_configured,
		          server_prefix, default_list_id, double_opt_in, sync_enabled, created_at, updated_at`,
		scope.OrganizationID, scope.LocationID, input.APIKeyConfigured,
		strings.TrimSpace(input.ServerPrefix), strings.TrimSpace(input.DefaultListID),
		input.DoubleOptIn, input.SyncEnabled,
	).Scan(
		&set.ID, &set.OrganizationID, &set.LocationID, &set.APIKeyConfigured,
		&set.ServerPrefix, &set.DefaultListID, &set.DoubleOptIn, &set.SyncEnabled,
		&set.CreatedAt, &set.UpdatedAt,
	)
	if err != nil {
		return MailchimpSettings{}, err
	}
	return set, tx.Commit(ctx)
}

func (s *Store) RecordMarketingConsent(ctx context.Context, scope Scope, input RecordConsentInput) (MarketingConsent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MarketingConsent{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "marketing.manage"); err != nil {
		return MarketingConsent{}, err
	}

	return s.recordConsentInternal(ctx, tx, scope.OrganizationID, scope.LocationID, input)
}

func (s *Store) recordConsentInternal(ctx context.Context, tx pgx.Tx, orgID, locID string, input RecordConsentInput) (MarketingConsent, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return MarketingConsent{}, ErrMarketingInput
	}
	status := strings.TrimSpace(input.ConsentStatus)
	if status != "opt_in" && status != "opt_out" && status != "unsubscribed" {
		return MarketingConsent{}, ErrMarketingInput
	}
	source := strings.TrimSpace(input.ConsentSource)
	switch source {
	case "popup_form", "website_footer", "customer_portal", "booking_checkout", "manual_staff":
	default:
		return MarketingConsent{}, ErrMarketingInput
	}
	text := strings.TrimSpace(input.ConsentText)
	if text == "" {
		return MarketingConsent{}, ErrMarketingInput
	}

	hasher := md5.New()
	hasher.Write([]byte(email))
	subscriberHash := hex.EncodeToString(hasher.Sum(nil))

	var c MarketingConsent
	err := tx.QueryRow(ctx, `
		INSERT INTO marketing_consents (
			organization_id, location_id, customer_id, email, first_name, last_name,
			consent_status, consent_source, consent_text, ip_address, user_agent,
			consented_at, sync_status, mailchimp_subscriber_hash
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now(), 'pending', $12
		)
		ON CONFLICT (organization_id, location_id, email) DO UPDATE SET
			customer_id = COALESCE(EXCLUDED.customer_id, marketing_consents.customer_id),
			first_name = CASE WHEN EXCLUDED.first_name <> '' THEN EXCLUDED.first_name ELSE marketing_consents.first_name END,
			last_name = CASE WHEN EXCLUDED.last_name <> '' THEN EXCLUDED.last_name ELSE marketing_consents.last_name END,
			consent_status = EXCLUDED.consent_status,
			consent_source = EXCLUDED.consent_source,
			consent_text = EXCLUDED.consent_text,
			ip_address = EXCLUDED.ip_address,
			user_agent = EXCLUDED.user_agent,
			consented_at = now(),
			revoked_at = CASE WHEN EXCLUDED.consent_status IN ('opt_out', 'unsubscribed') THEN now() ELSE NULL END,
			sync_status = 'pending'
		RETURNING id::text, organization_id::text, location_id::text, customer_id::text,
		          email, first_name, last_name, consent_status, consent_source, consent_text,
		          ip_address, user_agent, consented_at, revoked_at, sync_status,
		          mailchimp_subscriber_hash, last_synced_at, last_sync_error`,
		orgID, locID, input.CustomerID, email,
		strings.TrimSpace(input.FirstName), strings.TrimSpace(input.LastName),
		status, source, text, strings.TrimSpace(input.IPAddress), strings.TrimSpace(input.UserAgent),
		subscriberHash,
	).Scan(
		&c.ID, &c.OrganizationID, &c.LocationID, &c.CustomerID,
		&c.Email, &c.FirstName, &c.LastName, &c.ConsentStatus, &c.ConsentSource, &c.ConsentText,
		&c.IPAddress, &c.UserAgent, &c.ConsentedAt, &c.RevokedAt, &c.SyncStatus,
		&c.MailchimpSubscriberHash, &c.LastSyncedAt, &c.LastSyncError,
	)
	if err != nil {
		return MarketingConsent{}, err
	}
	return c, tx.Commit(ctx)
}

func (s *Store) ListMarketingConsents(ctx context.Context, scope Scope, limit, offset int) ([]MarketingConsent, int, error) {
	if err := s.authorize(ctx, scope, "marketing.read"); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM marketing_consents WHERE organization_id=$1 AND location_id=$2`, scope.OrganizationID, scope.LocationID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, organization_id::text, location_id::text, customer_id::text,
		       email, first_name, last_name, consent_status, consent_source, consent_text,
		       ip_address, user_agent, consented_at, revoked_at, sync_status,
		       mailchimp_subscriber_hash, last_synced_at, last_sync_error
		FROM marketing_consents
		WHERE organization_id=$1 AND location_id=$2
		ORDER BY consented_at DESC
		LIMIT $3 OFFSET $4`,
		scope.OrganizationID, scope.LocationID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []MarketingConsent
	for rows.Next() {
		var c MarketingConsent
		if err := rows.Scan(
			&c.ID, &c.OrganizationID, &c.LocationID, &c.CustomerID,
			&c.Email, &c.FirstName, &c.LastName, &c.ConsentStatus, &c.ConsentSource, &c.ConsentText,
			&c.IPAddress, &c.UserAgent, &c.ConsentedAt, &c.RevokedAt, &c.SyncStatus,
			&c.MailchimpSubscriberHash, &c.LastSyncedAt, &c.LastSyncError,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, c)
	}
	return list, total, nil
}

func (s *Store) UpdateConsentSyncStatus(ctx context.Context, scope Scope, consentID, syncStatus, subscriberHash, syncError string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "marketing.manage"); err != nil {
		return err
	}

	res, err := tx.Exec(ctx, `
		UPDATE marketing_consents
		SET sync_status=$1, mailchimp_subscriber_hash=$2, last_synced_at=now(), last_sync_error=$3
		WHERE id=$4 AND organization_id=$5 AND location_id=$6`,
		syncStatus, subscriberHash, syncError, consentID, scope.OrganizationID, scope.LocationID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrConsentNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) CreatePopupForm(ctx context.Context, scope Scope, input CreatePopupFormInput) (PopupForm, error) {
	name := strings.TrimSpace(input.Name)
	title := strings.TrimSpace(input.Title)
	if len(name) < 2 || len(name) > 120 || len(title) < 2 || len(title) > 160 {
		return PopupForm{}, ErrMarketingInput
	}
	if input.DisplayDelaySeconds < 0 || input.DisplayDelaySeconds > 300 {
		input.DisplayDelaySeconds = 5
	}
	if input.RepeatDisplayLimitDays < 0 || input.RepeatDisplayLimitDays > 365 {
		input.RepeatDisplayLimitDays = 7
	}
	msg := strings.TrimSpace(input.SuccessMessage)
	if msg == "" {
		msg = "Thank you for subscribing!"
	}
	if len(input.SelectedPages) == 0 {
		input.SelectedPages = []string{"/"}
	}
	if len(input.Fields) == 0 {
		input.Fields = []PopupFormField{
			{Name: "email", Type: "email", Label: "Email Address", Required: true},
			{Name: "first_name", Type: "text", Label: "First Name", Required: false},
		}
	}

	fieldsJSON, _ := json.Marshal(input.Fields)
	tagsJSON, _ := json.Marshal(input.Tags)
	pagesJSON, _ := json.Marshal(input.SelectedPages)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PopupForm{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "marketing.manage"); err != nil {
		return PopupForm{}, err
	}

	var form PopupForm
	var rawFields, rawTags, rawPages []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO popup_forms (
			organization_id, location_id, name, title, description, headline,
			fields, audience_id, tags, display_delay_seconds, selected_pages,
			repeat_display_limit_days, success_message, active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, now(), now()
		)
		RETURNING id::text, organization_id::text, location_id::text, name, title, description, headline,
		          fields, audience_id, tags, display_delay_seconds, selected_pages, repeat_display_limit_days,
		          success_message, active, created_at, updated_at`,
		scope.OrganizationID, scope.LocationID, name, title,
		strings.TrimSpace(input.Description), strings.TrimSpace(input.Headline),
		fieldsJSON, strings.TrimSpace(input.AudienceID), tagsJSON,
		input.DisplayDelaySeconds, pagesJSON, input.RepeatDisplayLimitDays,
		msg, input.Active,
	).Scan(
		&form.ID, &form.OrganizationID, &form.LocationID, &form.Name, &form.Title, &form.Description, &form.Headline,
		&rawFields, &form.AudienceID, &rawTags, &form.DisplayDelaySeconds, &rawPages, &form.RepeatDisplayLimitDays,
		&form.SuccessMessage, &form.Active, &form.CreatedAt, &form.UpdatedAt,
	)
	if err != nil {
		return PopupForm{}, err
	}
	_ = json.Unmarshal(rawFields, &form.Fields)
	_ = json.Unmarshal(rawTags, &form.Tags)
	_ = json.Unmarshal(rawPages, &form.SelectedPages)

	return form, tx.Commit(ctx)
}

func (s *Store) UpdatePopupForm(ctx context.Context, scope Scope, formID string, input UpdatePopupFormInput) (PopupForm, error) {
	name := strings.TrimSpace(input.Name)
	title := strings.TrimSpace(input.Title)
	if len(name) < 2 || len(name) > 120 || len(title) < 2 || len(title) > 160 {
		return PopupForm{}, ErrMarketingInput
	}
	if input.DisplayDelaySeconds < 0 || input.DisplayDelaySeconds > 300 {
		input.DisplayDelaySeconds = 5
	}
	if input.RepeatDisplayLimitDays < 0 || input.RepeatDisplayLimitDays > 365 {
		input.RepeatDisplayLimitDays = 7
	}
	msg := strings.TrimSpace(input.SuccessMessage)
	if msg == "" {
		msg = "Thank you for subscribing!"
	}
	if len(input.SelectedPages) == 0 {
		input.SelectedPages = []string{"/"}
	}
	if len(input.Fields) == 0 {
		input.Fields = []PopupFormField{
			{Name: "email", Type: "email", Label: "Email Address", Required: true},
		}
	}

	fieldsJSON, _ := json.Marshal(input.Fields)
	tagsJSON, _ := json.Marshal(input.Tags)
	pagesJSON, _ := json.Marshal(input.SelectedPages)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PopupForm{}, err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "marketing.manage"); err != nil {
		return PopupForm{}, err
	}

	var form PopupForm
	var rawFields, rawTags, rawPages []byte
	err = tx.QueryRow(ctx, `
		UPDATE popup_forms
		SET name=$1, title=$2, description=$3, headline=$4, fields=$5,
		    audience_id=$6, tags=$7, display_delay_seconds=$8, selected_pages=$9,
		    repeat_display_limit_days=$10, success_message=$11, active=$12, updated_at=now()
		WHERE id=$13 AND organization_id=$14 AND location_id=$15
		RETURNING id::text, organization_id::text, location_id::text, name, title, description, headline,
		          fields, audience_id, tags, display_delay_seconds, selected_pages, repeat_display_limit_days,
		          success_message, active, created_at, updated_at`,
		name, title, strings.TrimSpace(input.Description), strings.TrimSpace(input.Headline),
		fieldsJSON, strings.TrimSpace(input.AudienceID), tagsJSON,
		input.DisplayDelaySeconds, pagesJSON, input.RepeatDisplayLimitDays,
		msg, input.Active, formID, scope.OrganizationID, scope.LocationID,
	).Scan(
		&form.ID, &form.OrganizationID, &form.LocationID, &form.Name, &form.Title, &form.Description, &form.Headline,
		&rawFields, &form.AudienceID, &rawTags, &form.DisplayDelaySeconds, &rawPages, &form.RepeatDisplayLimitDays,
		&form.SuccessMessage, &form.Active, &form.CreatedAt, &form.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PopupForm{}, ErrPopupNotFound
	}
	if err != nil {
		return PopupForm{}, err
	}
	_ = json.Unmarshal(rawFields, &form.Fields)
	_ = json.Unmarshal(rawTags, &form.Tags)
	_ = json.Unmarshal(rawPages, &form.SelectedPages)

	return form, tx.Commit(ctx)
}

func (s *Store) GetPopupForm(ctx context.Context, scope Scope, formID string) (PopupForm, error) {
	if err := s.authorize(ctx, scope, "marketing.read"); err != nil {
		return PopupForm{}, err
	}
	var form PopupForm
	var rawFields, rawTags, rawPages []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, organization_id::text, location_id::text, name, title, description, headline,
		       fields, audience_id, tags, display_delay_seconds, selected_pages, repeat_display_limit_days,
		       success_message, active, created_at, updated_at
		FROM popup_forms
		WHERE id=$1 AND organization_id=$2 AND location_id=$3`,
		formID, scope.OrganizationID, scope.LocationID,
	).Scan(
		&form.ID, &form.OrganizationID, &form.LocationID, &form.Name, &form.Title, &form.Description, &form.Headline,
		&rawFields, &form.AudienceID, &rawTags, &form.DisplayDelaySeconds, &rawPages, &form.RepeatDisplayLimitDays,
		&form.SuccessMessage, &form.Active, &form.CreatedAt, &form.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PopupForm{}, ErrPopupNotFound
	}
	if err != nil {
		return PopupForm{}, err
	}
	_ = json.Unmarshal(rawFields, &form.Fields)
	_ = json.Unmarshal(rawTags, &form.Tags)
	_ = json.Unmarshal(rawPages, &form.SelectedPages)
	return form, nil
}

func (s *Store) ListPopupForms(ctx context.Context, scope Scope) ([]PopupForm, error) {
	if err := s.authorize(ctx, scope, "marketing.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, organization_id::text, location_id::text, name, title, description, headline,
		       fields, audience_id, tags, display_delay_seconds, selected_pages, repeat_display_limit_days,
		       success_message, active, created_at, updated_at
		FROM popup_forms
		WHERE organization_id=$1 AND location_id=$2
		ORDER BY created_at DESC`,
		scope.OrganizationID, scope.LocationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []PopupForm
	for rows.Next() {
		var form PopupForm
		var rawFields, rawTags, rawPages []byte
		if err := rows.Scan(
			&form.ID, &form.OrganizationID, &form.LocationID, &form.Name, &form.Title, &form.Description, &form.Headline,
			&rawFields, &form.AudienceID, &rawTags, &form.DisplayDelaySeconds, &rawPages, &form.RepeatDisplayLimitDays,
			&form.SuccessMessage, &form.Active, &form.CreatedAt, &form.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawFields, &form.Fields)
		_ = json.Unmarshal(rawTags, &form.Tags)
		_ = json.Unmarshal(rawPages, &form.SelectedPages)
		list = append(list, form)
	}
	return list, nil
}

func (s *Store) DeletePopupForm(ctx context.Context, scope Scope, formID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err = authorizeTransaction(ctx, tx, scope, "marketing.manage"); err != nil {
		return err
	}

	res, err := tx.Exec(ctx, `
		DELETE FROM popup_forms
		WHERE id=$1 AND organization_id=$2 AND location_id=$3`,
		formID, scope.OrganizationID, scope.LocationID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrPopupNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) GetPublicActivePopupForms(ctx context.Context, orgID, locID, currentPath string) ([]PopupForm, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, organization_id::text, location_id::text, name, title, description, headline,
		       fields, audience_id, tags, display_delay_seconds, selected_pages, repeat_display_limit_days,
		       success_message, active, created_at, updated_at
		FROM popup_forms
		WHERE organization_id=$1 AND location_id=$2 AND active=true
		ORDER BY created_at DESC`,
		orgID, locID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	normPath := strings.TrimSpace(currentPath)
	if normPath == "" {
		normPath = "/"
	}

	var matched []PopupForm
	for rows.Next() {
		var form PopupForm
		var rawFields, rawTags, rawPages []byte
		if err := rows.Scan(
			&form.ID, &form.OrganizationID, &form.LocationID, &form.Name, &form.Title, &form.Description, &form.Headline,
			&rawFields, &form.AudienceID, &rawTags, &form.DisplayDelaySeconds, &rawPages, &form.RepeatDisplayLimitDays,
			&form.SuccessMessage, &form.Active, &form.CreatedAt, &form.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawFields, &form.Fields)
		_ = json.Unmarshal(rawTags, &form.Tags)
		_ = json.Unmarshal(rawPages, &form.SelectedPages)

		match := false
		for _, page := range form.SelectedPages {
			if page == "*" || page == normPath || (strings.HasSuffix(page, "*") && strings.HasPrefix(normPath, strings.TrimSuffix(page, "*"))) {
				match = true
				break
			}
		}
		if match {
			matched = append(matched, form)
		}
	}
	return matched, nil
}

func (s *Store) SubmitPublicPopup(ctx context.Context, formID string, input PublicPopupSubmissionInput) (PopupSubmission, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return PopupSubmission{}, ErrMarketingInput
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PopupSubmission{}, err
	}
	defer tx.Rollback(ctx)

	var orgID, locID, formTitle, formSuccess string
	var active bool
	err = tx.QueryRow(ctx, `
		SELECT organization_id::text, location_id::text, title, success_message, active
		FROM popup_forms
		WHERE id=$1`,
		formID,
	).Scan(&orgID, &locID, &formTitle, &formSuccess, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return PopupSubmission{}, ErrPopupNotFound
	}
	if err != nil {
		return PopupSubmission{}, err
	}
	if !active {
		return PopupSubmission{}, fmt.Errorf("%w: popup is inactive", ErrMarketingInput)
	}

	metaJSON, _ := json.Marshal(input.Metadata)

	var sub PopupSubmission
	var rawMeta []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO popup_submissions (form_id, organization_id, location_id, email, first_name, last_name, metadata, ip_address, user_agent, submitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		RETURNING id::text, form_id::text, organization_id::text, location_id::text, email, first_name, last_name, metadata, ip_address, user_agent, submitted_at`,
		formID, orgID, locID, email,
		strings.TrimSpace(input.FirstName), strings.TrimSpace(input.LastName),
		metaJSON, strings.TrimSpace(input.IPAddress), strings.TrimSpace(input.UserAgent),
	).Scan(
		&sub.ID, &sub.FormID, &sub.OrganizationID, &sub.LocationID,
		&sub.Email, &sub.FirstName, &sub.LastName, &rawMeta,
		&sub.IPAddress, &sub.UserAgent, &sub.SubmittedAt,
	)
	if err != nil {
		return PopupSubmission{}, err
	}
	_ = json.Unmarshal(rawMeta, &sub.Metadata)

	// Record explicit marketing consent conforming to CASL
	consentText := fmt.Sprintf("Subscribed via popup form '%s'", formTitle)
	hasher := md5.New()
	hasher.Write([]byte(email))
	subscriberHash := hex.EncodeToString(hasher.Sum(nil))

	_, err = tx.Exec(ctx, `
		INSERT INTO marketing_consents (
			organization_id, location_id, email, first_name, last_name,
			consent_status, consent_source, consent_text, ip_address, user_agent,
			consented_at, sync_status, mailchimp_subscriber_hash
		) VALUES (
			$1, $2, $3, $4, $5, 'opt_in', 'popup_form', $6, $7, $8, now(), 'pending', $9
		)
		ON CONFLICT (organization_id, location_id, email) DO UPDATE SET
			first_name = CASE WHEN EXCLUDED.first_name <> '' THEN EXCLUDED.first_name ELSE marketing_consents.first_name END,
			last_name = CASE WHEN EXCLUDED.last_name <> '' THEN EXCLUDED.last_name ELSE marketing_consents.last_name END,
			consent_status = 'opt_in',
			consent_source = 'popup_form',
			consent_text = EXCLUDED.consent_text,
			ip_address = EXCLUDED.ip_address,
			user_agent = EXCLUDED.user_agent,
			consented_at = now(),
			revoked_at = NULL,
			sync_status = 'pending'`,
		orgID, locID, email,
		strings.TrimSpace(input.FirstName), strings.TrimSpace(input.LastName),
		consentText, strings.TrimSpace(input.IPAddress), strings.TrimSpace(input.UserAgent),
		subscriberHash,
	)
	if err != nil {
		return PopupSubmission{}, err
	}

	return sub, tx.Commit(ctx)
}

func (s *Store) ListPopupSubmissions(ctx context.Context, scope Scope, formID string) ([]PopupSubmission, error) {
	if err := s.authorize(ctx, scope, "marketing.read"); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT s.id::text, s.form_id::text, s.organization_id::text, s.location_id::text,
		       s.email, s.first_name, s.last_name, s.metadata, s.ip_address, s.user_agent, s.submitted_at
		FROM popup_submissions s
		JOIN popup_forms f ON f.id=s.form_id
		WHERE s.form_id=$1 AND f.organization_id=$2 AND f.location_id=$3
		ORDER BY s.submitted_at DESC
		LIMIT 100`,
		formID, scope.OrganizationID, scope.LocationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []PopupSubmission
	for rows.Next() {
		var sub PopupSubmission
		var rawMeta []byte
		if err := rows.Scan(
			&sub.ID, &sub.FormID, &sub.OrganizationID, &sub.LocationID,
			&sub.Email, &sub.FirstName, &sub.LastName, &rawMeta,
			&sub.IPAddress, &sub.UserAgent, &sub.SubmittedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawMeta, &sub.Metadata)
		list = append(list, sub)
	}
	return list, nil
}
