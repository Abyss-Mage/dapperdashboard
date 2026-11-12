// Package postgres is an internal persistence foundation, not an HTTP auth layer.
// Callers must obtain Scope from verified authorization, never request JSON.
package postgres

import (
	"sync"
	"dapper.local/platform/internal/square"
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

//go:embed schema.sql
var schema string

//go:embed authorization.sql
var authorizationSchema string

//go:embed vehicle_grants.sql
var vehicleGrantSchema string

//go:embed sessions.sql
var sessionSchema string

//go:embed session_invalidation.sql
var sessionInvalidationSchema string

//go:embed oidc_identities.sql
var oidcIdentitySchema string

//go:embed password_credentials.sql
var passwordCredentialSchema string

//go:embed jobs.sql
var jobSchema string

//go:embed services.sql
var serviceSchema string

//go:embed franchises.sql
var franchiseSchema string

//go:embed job_service_link.sql
var jobServiceSchema string

//go:embed job_addons.sql
var jobAddonSchema string

//go:embed reservations.sql
var reservationSchema string

//go:embed reservation_release.sql
var reservationReleaseSchema string

//go:embed scheduling_rules.sql
var schedulingRuleSchema string

//go:embed reservation_rules.sql
var reservationRuleSchema string

//go:embed reservation_history.sql
var reservationHistorySchema string

//go:embed scheduling_read.sql
var schedulingReadSchema string

//go:embed job_contributors.sql
var jobContributorSchema string

//go:embed central_catalogue.sql
var centralCatalogueSchema string

//go:embed attendance.sql
var attendanceSchema string

//go:embed bookings.sql
var bookingSchema string

//go:embed invoices.sql
var invoiceSchema string

//go:embed sop_documents.sql
var sopDocumentSchema string

//go:embed booking_vehicle_ownership.sql
var bookingVehicleOwnershipSchema string

//go:embed customer_accounts.sql
var customerAccountSchema string

//go:embed booking_job_requests.sql
var bookingJobRequestSchema string

//go:embed initial_administrator.sql
var initialAdministratorSchema string

//go:embed access_grant_events.sql
var accessGrantEventsSchema string

//go:embed invoice_issuance.sql
var invoiceIssuanceSchema string

//go:embed invoice_draft_revisions.sql
var invoiceDraftRevisionSchema string

//go:embed member_invitations.sql
var memberInvitationSchema string

//go:embed customer_account_management.sql
var customerAccountManagementSchema string

//go:embed booking_capacity_requests.sql
var bookingCapacityRequestSchema string

//go:embed booking_reschedule_requests.sql
var bookingRescheduleRequestSchema string

//go:embed booking_offers.sql
var bookingOfferSchema string

//go:embed customer_booking_requests.sql
var customerBookingRequestSchema string

//go:embed scoped_quotations.sql
var scopedQuotationSchema string

//go:embed dealership_b2b.sql
var dealershipB2BSchema string

//go:embed marketing_popups.sql
var marketingPopupsSchema string

//go:embed quote_templates.sql
var quoteTemplatesSchema string

//go:embed outlet_templates.sql
var outletTemplatesSchema string

//go:embed legacy_import_map.sql
var legacyImportMapSchema string

//go:embed legacy_billing.sql
var legacyBillingSchema string

//go:embed booking_numbers.sql
var bookingNumbersSchema string

//go:embed website_dealers.sql
var websiteDealersSchema string

//go:embed portal_profile.sql
var portalProfileSchema string

//go:embed window_tint_category.sql
var windowTintCategorySchema string

//go:embed email_templates.sql
var emailTemplatesSchema string

//go:embed site_assets.sql
var siteAssetsSchema string

//go:embed email_templates_more.sql
var emailTemplatesMoreSchema string

//go:embed site_assets_hidden.sql
var siteAssetsHiddenSchema string

//go:embed admin_alerts.sql
var adminAlertsSchema string

//go:embed payment_refund_accounting.sql
var paymentRefundAccountingSchema string

//go:embed public_content.sql
var publicContentSchema string

//go:embed roles_and_scoped_assignments.sql
var rolesAndScopedAssignmentsSchema string

//go:embed central_service_links.sql
var centralServiceLinksSchema string

//go:embed employee_upload_views.sql
var employeeUploadViewsSchema string

//go:embed employee_upload_booking.sql
var employeeUploadBookingSchema string

//go:embed employee_worksite_location.sql
var employeeWorksiteLocationSchema string

//go:embed franchise_site.sql
var franchiseSiteSchema string

//go:embed employee_upload_retention.sql
var employeeUploadRetentionSchema string

//go:embed account_security.sql
var accountSecuritySchema string

//go:embed scheduling_weekly_hours.sql
var schedulingWeeklyHoursSchema string

//go:embed franchise_taxes.sql
var franchiseTaxesSchema string

//go:embed scheduling_working_time.sql
var schedulingWorkingTimeSchema string

//go:embed booking_capacity_working_time.sql
var bookingCapacityWorkingTimeSchema string

//go:embed service_prices.sql
var servicePricesSchema string

//go:embed invoice_print_template.sql
var invoicePrintTemplateSchema string

//go:embed invoice_one_per_booking.sql
var invoiceOnePerBookingSchema string

//go:embed public_booking.sql
var publicBookingSchema string

//go:embed employee_uploads.sql
var employeeUploadsSchema string

//go:embed square_payment_intents.sql
var squarePaymentIntentsSchema string

//go:embed gps_tracking.sql
var gpsTrackingSchema string

//go:embed employee_workspace.sql
var employeeWorkspaceSchema string

//go:embed customer_unique_availability_review.sql
var customerUniqueAvailabilityReviewSchema string

//go:embed owner_wave_2026_10.sql
var ownerWave202610Schema string

//go:embed dashboard_batch_c.sql
var dashboardBatchCSchema string

//go:embed account_recovery.sql
var accountRecoverySchema string

//go:embed addon_links.sql
var addonLinksSchema string

//go:embed clean_status_notes.sql
var cleanStatusNotesSchema string

//go:embed employee_role.sql
var employeeRoleSchema string

//go:embed messaging_settings.sql
var messagingSettingsSchema string

//go:embed portfolio_dimensions.sql
var portfolioDimensionsSchema string

//go:embed service_slots.sql
var serviceSlotsSchema string

//go:embed custom_scripts.sql
var customScriptsSchema string

//go:embed website_tracking.sql
var websiteTrackingSchema string

//go:embed calendar_feeds.sql
var calendarFeedsSchema string

//go:embed location_invoice_templates.sql
var locationInvoiceTemplatesSchema string

//go:embed website_enquiries.sql
var websiteEnquiriesSchema string

//go:embed location_logos.sql
var locationLogosSchema string

//go:embed approve_pending_bookings.sql
var approvePendingBookingsSchema string

//go:embed activity_log.sql
var activityLogSchema string

//go:embed coupons.sql
var couponsSchema string

//go:embed invoice_item_order.sql
var invoiceItemOrderSchema string

//go:embed booking_email_delivery.sql
var bookingEmailDeliverySchema string

//go:embed site_logo_sizes.sql
var siteLogoSizesSchema string

//go:embed media_bin.sql
var mediaBinSchema string

//go:embed franchise_tax_number.sql
var franchiseTaxNumberSchema string

//go:embed quote_delete.sql
var quoteDeleteSchema string

//go:embed quote_email_template.sql
var quoteEmailTemplateSchema string

//go:embed document_numbering.sql
var documentNumberingSchema string

//go:embed star_package.sql
var starPackageSchema string

//go:embed franchise_central_catalogue.sql
var franchiseCentralCatalogueSchema string

//go:embed franchise_default_bays.sql
var franchiseDefaultBaysSchema string

//go:embed franchise_default_rules.sql
var franchiseDefaultRulesSchema string

//go:embed package_upsells.sql
var packageUpsellsSchema string

//go:embed shop_clock.sql
var shopClockSchema string

//go:embed dealer_invoices.sql
var dealerInvoicesSchema string

//go:embed dealer_invoices_v2.sql
var dealerInvoicesV2Schema string

//go:embed shop_login.sql
var shopLoginSchema string

//go:embed dealer_portal.sql
var dealerPortalSchema string

//go:embed franchise_sites_backfill.sql
var franchiseSitesBackfillSchema string

//go:embed location_square_apps.sql
var locationSquareAppsSchema string

//go:embed location_square_keys.sql
var locationSquareKeysSchema string

//go:embed site_seo.sql
var siteSEOSchema string

//go:embed site_banner.sql
var siteBannerSchema string

//go:embed invitation_delivery_retry.sql
var invitationDeliveryRetrySchema string

//go:embed franchise_owner_no_catalogue.sql
var franchiseOwnerNoCatalogueSchema string

type migration struct {
	version int
	sql     string
}

func migrations() []migration {
	return []migration{{1, schema}, {2, authorizationSchema}, {3, vehicleGrantSchema}, {4, sessionSchema}, {5, sessionInvalidationSchema}, {6, oidcIdentitySchema}, {7, passwordCredentialSchema}, {8, jobSchema}, {9, serviceSchema}, {10, franchiseSchema}, {11, jobServiceSchema}, {12, jobAddonSchema}, {13, reservationSchema}, {14, reservationReleaseSchema}, {15, schedulingRuleSchema}, {16, reservationRuleSchema}, {17, reservationHistorySchema}, {18, schedulingReadSchema}, {19, jobContributorSchema}, {20, centralCatalogueSchema}, {21, attendanceSchema}, {22, bookingSchema}, {23, invoiceSchema}, {24, sopDocumentSchema}, {25, bookingVehicleOwnershipSchema}, {26, customerAccountSchema}, {27, bookingJobRequestSchema}, {28, initialAdministratorSchema}, {29, accessGrantEventsSchema}, {30, invoiceIssuanceSchema}, {31, invoiceDraftRevisionSchema}, {32, memberInvitationSchema}, {33, customerAccountManagementSchema}, {34, bookingCapacityRequestSchema}, {35, bookingRescheduleRequestSchema}, {36, bookingOfferSchema}, {37, customerBookingRequestSchema}, {38, scopedQuotationSchema}, {39, dealershipB2BSchema}, {40, marketingPopupsSchema}, {41, paymentRefundAccountingSchema}, {43, publicContentSchema}, {44, rolesAndScopedAssignmentsSchema}, {45, centralServiceLinksSchema}, {46, employeeWorkspaceSchema}, {47, gpsTrackingSchema}, {48, squarePaymentIntentsSchema}, {49, employeeUploadsSchema}, {50, employeeUploadViewsSchema}, {51, employeeUploadBookingSchema}, {52, employeeWorksiteLocationSchema}, {53, franchiseSiteSchema}, {54, employeeUploadRetentionSchema}, {55, accountSecuritySchema}, {56, schedulingWeeklyHoursSchema}, {57, franchiseTaxesSchema}, {58, schedulingWorkingTimeSchema}, {59, bookingCapacityWorkingTimeSchema}, {60, servicePricesSchema}, {61, invoicePrintTemplateSchema}, {62, invoiceOnePerBookingSchema}, {63, publicBookingSchema}, {64, customerUniqueAvailabilityReviewSchema}, {65, ownerWave202610Schema}, {66, dashboardBatchCSchema}, {67, accountRecoverySchema}, {68, websiteTrackingSchema}, {69, calendarFeedsSchema}, {70, locationInvoiceTemplatesSchema}, {71, websiteEnquiriesSchema}, {72, locationLogosSchema}, {73, approvePendingBookingsSchema}, {74, activityLogSchema}, {75, couponsSchema}, {76, customScriptsSchema}, {77, serviceSlotsSchema}, {78, portfolioDimensionsSchema}, {79, messagingSettingsSchema}, {80, employeeRoleSchema}, {81, cleanStatusNotesSchema}, {82, addonLinksSchema}, {83, quoteTemplatesSchema}, {84, outletTemplatesSchema}, {85, legacyImportMapSchema}, {86, legacyBillingSchema}, {87, bookingNumbersSchema}, {88, websiteDealersSchema}, {89, portalProfileSchema}, {90, windowTintCategorySchema}, {91, emailTemplatesSchema}, {92, siteAssetsSchema}, {93, emailTemplatesMoreSchema}, {94, siteAssetsHiddenSchema}, {95, adminAlertsSchema}, {96, invoiceItemOrderSchema}, {97, bookingEmailDeliverySchema}, {98, siteLogoSizesSchema}, {99, mediaBinSchema}, {100, franchiseTaxNumberSchema}, {101, quoteDeleteSchema}, {102, quoteEmailTemplateSchema}, {103, documentNumberingSchema}, {104, starPackageSchema}, {105, franchiseCentralCatalogueSchema}, {106, franchiseDefaultBaysSchema}, {107, franchiseDefaultRulesSchema}, {108, franchiseOwnerNoCatalogueSchema}, {109, invitationDeliveryRetrySchema}, {110, siteBannerSchema}, {111, packageUpsellsSchema}, {112, shopClockSchema}, {113, dealerInvoicesSchema}, {114, dealerInvoicesV2Schema}, {115, shopLoginSchema}, {116, dealerPortalSchema}, {117, franchiseSitesBackfillSchema}, {118, locationSquareAppsSchema}, {119, locationSquareKeysSchema}, {120, siteSEOSchema}}
}

// ValidateSchema is read-only. Runtime startup never applies migrations.
func (s *Store) ValidateSchema(ctx context.Context) error {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM dapper_schema_migrations`).Scan(&count); err != nil {
		return errors.New("schema migration verification required")
	}
	if count != len(migrations()) {
		return errors.New("schema version mismatch")
	}
	for _, m := range migrations() {
		var checksum string
		if err := s.pool.QueryRow(ctx, `SELECT checksum FROM dapper_schema_migrations WHERE version=$1`, m.version).Scan(&checksum); err != nil || checksum != fmt.Sprintf("%x", sha256.Sum256([]byte(m.sql))) {
			return errors.New("schema checksum verification failed")
		}
	}
	return nil
}

type Store struct {
	pool           *pgxpool.Pool
	squareResolver SquareCredentialResolver
	squareOAuth    map[string]square.OAuthApp
	squareMu       sync.RWMutex
	// Public identifiers only (see SetSquareApplicationIDs).
	squareApplicationIDs map[string]string
	uploadDir            string
	sopDir               string
	mfaKey               []byte
	phoneVerifier        PhoneVerifier
	bookingHook          func(BookingEmailEvent)
}
type Scope struct{ OrganizationID, LocationID, ActorID string }
type Customer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

var ErrScope = errors.New("verified organization, location and actor scope required")
var ErrForbidden = errors.New("scope or action not granted")

func Open(ctx context.Context, dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("database configuration required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	config.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, errors.New("could not initialize database pool")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("database unavailable")
	}
	return &Store{pool: pool}, nil
}
func (s *Store) Close() { s.pool.Close() }

// Migrate is explicit; opening the API never silently changes a schema.
// A transaction lock serializes runners and a checksum detects edited history.
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(761421031)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS dapper_schema_migrations(version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	for _, current := range migrations() {
		checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(current.sql)))
		var previous string
		err = tx.QueryRow(ctx, `SELECT checksum FROM dapper_schema_migrations WHERE version=$1`, current.version).Scan(&previous)
		if err == nil {
			if previous != checksum {
				return errors.New("migration checksum changed; add a new migration")
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err = tx.Exec(ctx, current.sql); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO dapper_schema_migrations(version,checksum) VALUES($1,$2)`, current.version, checksum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (scope Scope) validate() error {
	if scope.OrganizationID == "" || scope.LocationID == "" || scope.ActorID == "" {
		return ErrScope
	}
	return nil
}
func (s *Store) CreateCustomer(ctx context.Context, scope Scope, name, email, phone string) (Customer, error) {
	if err := s.authorize(ctx, scope, "customers.create"); err != nil {
		return Customer{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Customer{}, err
	}
	defer tx.Rollback(ctx)
	var c Customer
	err = tx.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,$3,$4,$5) RETURNING id::text,name,email,phone`, scope.OrganizationID, scope.LocationID, name, email, phone).Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
	if err != nil {
		return Customer{}, customerUniqueness(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'customer.created',$4)`, scope.OrganizationID, scope.LocationID, scope.ActorID, c.ID); err != nil {
		return Customer{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Customer{}, err
	}
	return c, nil
}
func (s *Store) Customers(ctx context.Context, scope Scope) ([]Customer, error) {
	if err := s.authorize(ctx, scope, "customers.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,name,email,phone FROM customers WHERE organization_id=$1 AND location_id=$2 AND removed_at IS NULL ORDER BY created_at,id LIMIT 1000`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Customer{}
	for rows.Next() {
		var c Customer
		if err = rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
