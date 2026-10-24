package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMarketingAndPopups(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()

	namespace := fmt.Sprintf("dapper_marketing_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if _, e := admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()

	config := admin.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	defer s.Close()

	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	var org, loc, staff, otherStaff string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Marketing Org') RETURNING id::text`).Scan(&org); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Marketing Loc') RETURNING id::text`, org).Scan(&loc); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('marketing_staff_subject', true) RETURNING id::text`).Scan(&staff); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO principals(external_subject, active) VALUES('unauthorized_staff_subject', true) RETURNING id::text`).Scan(&otherStaff); err != nil {
		t.Fatal(err)
	}

	// Grant marketing.read and marketing.manage to staff
	for _, act := range []string{"marketing.read", "marketing.manage"} {
		if _, err = pool.Exec(ctx, `INSERT INTO location_grants(organization_id, location_id, principal_id, action) VALUES($1, $2, $3, $4)`, org, loc, staff, act); err != nil {
			t.Fatal(err)
		}
	}

	scope := Scope{OrganizationID: org, LocationID: loc, ActorID: staff}
	unauthorizedScope := Scope{OrganizationID: org, LocationID: loc, ActorID: otherStaff}

	// 1. Mailchimp settings
	t.Run("MailchimpSettings", func(t *testing.T) {
		// Read default settings
		set, err := s.GetMailchimpSettings(ctx, scope)
		if err != nil {
			t.Fatal(err)
		}
		if set.APIKeyConfigured {
			t.Fatal("expected APIKeyConfigured false by default")
		}

		// Unauthorized read
		_, err = s.GetMailchimpSettings(ctx, unauthorizedScope)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}

		// Update settings
		updated, err := s.UpdateMailchimpSettings(ctx, scope, UpdateMailchimpSettingsInput{
			APIKeyConfigured: true,
			ServerPrefix:     "us21",
			DefaultListID:    "list_xyz",
			DoubleOptIn:      true,
			SyncEnabled:      true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !updated.APIKeyConfigured || updated.ServerPrefix != "us21" || updated.DefaultListID != "list_xyz" {
			t.Fatalf("unexpected updated settings: %+v", updated)
		}

		// Unauthorized update
		_, err = s.UpdateMailchimpSettings(ctx, unauthorizedScope, UpdateMailchimpSettingsInput{
			APIKeyConfigured: false,
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	// 2. Marketing Consents (CASL Compliance)
	t.Run("MarketingConsents", func(t *testing.T) {
		consent, err := s.RecordMarketingConsent(ctx, scope, RecordConsentInput{
			Email:         "lead@example.com",
			FirstName:     "Sam",
			LastName:      "Taylor",
			ConsentStatus: "opt_in",
			ConsentSource: "website_footer",
			ConsentText:   "I agree to receive promotional messages and detailing tips from Dapper Detailing.",
			IPAddress:     "192.168.1.100",
			UserAgent:     "Mozilla/5.0 Test",
		})
		if err != nil {
			t.Fatal(err)
		}
		if consent.Email != "lead@example.com" || consent.ConsentStatus != "opt_in" || consent.SyncStatus != "pending" {
			t.Fatalf("unexpected consent: %+v", consent)
		}
		if consent.MailchimpSubscriberHash == "" {
			t.Fatal("expected non-empty subscriber hash")
		}

		// List consents
		list, total, err := s.ListMarketingConsents(ctx, scope, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("expected 1 consent record, got total=%d, len=%d", total, len(list))
		}

		// Update sync status
		err = s.UpdateConsentSyncStatus(ctx, scope, consent.ID, "synced", consent.MailchimpSubscriberHash, "")
		if err != nil {
			t.Fatal(err)
		}

		// Opt out / unsubscribe update
		revoked, err := s.RecordMarketingConsent(ctx, scope, RecordConsentInput{
			Email:         "lead@example.com",
			ConsentStatus: "unsubscribed",
			ConsentSource: "customer_portal",
			ConsentText:   "Customer unsubscribed via account settings",
		})
		if err != nil {
			t.Fatal(err)
		}
		if revoked.ConsentStatus != "unsubscribed" || revoked.RevokedAt == nil {
			t.Fatalf("expected revoked_at to be set on unsubscribe, got %+v", revoked)
		}
	})

	// 3. Popup Form CRUD & Public Display Matching
	t.Run("PopupFormsAndSubmissions", func(t *testing.T) {
		form, err := s.CreatePopupForm(ctx, scope, CreatePopupFormInput{
			Name:        "Spring Promo Popup",
			Title:       "Get 15% Off Your First Detail",
			Headline:    "Exclusive Seasonal Offer",
			Description: "Subscribe to our VIP club and receive a 15% promo code immediately.",
			Fields: []PopupFormField{
				{Name: "email", Type: "email", Label: "Email Address", Required: true},
				{Name: "first_name", Type: "text", Label: "First Name", Required: false},
			},
			AudienceID:             "list_xyz",
			Tags:                   []string{"Spring2026", "WebLead"},
			DisplayDelaySeconds:    3,
			SelectedPages:          []string{"/", "/booking*"},
			RepeatDisplayLimitDays: 14,
			SuccessMessage:         "Thank you! Check your inbox for your 15% off coupon code.",
			Active:                 true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if form.ID == "" || form.Name != "Spring Promo Popup" {
			t.Fatalf("unexpected created form: %+v", form)
		}

		// Public active matching: test root path "/"
		rootPopups, err := s.GetPublicActivePopupForms(ctx, org, loc, "/")
		if err != nil {
			t.Fatal(err)
		}
		if len(rootPopups) != 1 {
			t.Fatalf("expected 1 popup matching '/', got %d", len(rootPopups))
		}

		// Public active matching: test wildcard "/booking/reserve"
		bookingPopups, err := s.GetPublicActivePopupForms(ctx, org, loc, "/booking/reserve")
		if err != nil {
			t.Fatal(err)
		}
		if len(bookingPopups) != 1 {
			t.Fatalf("expected 1 popup matching '/booking/reserve', got %d", len(bookingPopups))
		}

		// Public active matching: test non-matching path "/account"
		accountPopups, err := s.GetPublicActivePopupForms(ctx, org, loc, "/account")
		if err != nil {
			t.Fatal(err)
		}
		if len(accountPopups) != 0 {
			t.Fatalf("expected 0 popups matching '/account', got %d", len(accountPopups))
		}

		// Public submission
		sub, err := s.SubmitPublicPopup(ctx, form.ID, PublicPopupSubmissionInput{
			Email:     "visitor@autolovers.ca",
			FirstName: "Jordan",
			LastName:  "Bell",
			Metadata:  map[string]any{"source_url": "https://dapperdetailing.ca/"},
			IPAddress: "203.0.113.195",
			UserAgent: "Mozilla/5.0 Safari",
		})
		if err != nil {
			t.Fatal(err)
		}
		if sub.Email != "visitor@autolovers.ca" || sub.FirstName != "Jordan" {
			t.Fatalf("unexpected submission: %+v", sub)
		}

		// Verify automatic marketing consent generation from popup submission
		consents, _, err := s.ListMarketingConsents(ctx, scope, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		foundConsent := false
		for _, c := range consents {
			if c.Email == "visitor@autolovers.ca" && c.ConsentSource == "popup_form" && c.ConsentStatus == "opt_in" {
				foundConsent = true
				break
			}
		}
		if !foundConsent {
			t.Fatal("expected public popup submission to automatically generate opt-in marketing consent record")
		}

		// Staff listing submissions
		submissions, err := s.ListPopupSubmissions(ctx, scope, form.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(submissions) != 1 || submissions[0].Email != "visitor@autolovers.ca" {
			t.Fatalf("unexpected submissions list: %+v", submissions)
		}

		// Update form: toggle inactive
		updated, err := s.UpdatePopupForm(ctx, scope, form.ID, UpdatePopupFormInput{
			Name:                   form.Name,
			Title:                  form.Title,
			Description:            form.Description,
			Headline:               form.Headline,
			Fields:                 form.Fields,
			AudienceID:             form.AudienceID,
			Tags:                   form.Tags,
			DisplayDelaySeconds:    form.DisplayDelaySeconds,
			SelectedPages:          form.SelectedPages,
			RepeatDisplayLimitDays: form.RepeatDisplayLimitDays,
			SuccessMessage:         form.SuccessMessage,
			Active:                 false, // Inactive
		})
		if err != nil {
			t.Fatal(err)
		}
		if updated.Active {
			t.Fatal("expected active to be false")
		}

		// Verify inactive form is not returned to public
		noPopups, err := s.GetPublicActivePopupForms(ctx, org, loc, "/")
		if err != nil {
			t.Fatal(err)
		}
		if len(noPopups) != 0 {
			t.Fatalf("expected 0 active popups after deactivate, got %d", len(noPopups))
		}
	})
}
