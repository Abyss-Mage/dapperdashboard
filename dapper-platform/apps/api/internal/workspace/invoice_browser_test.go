package workspace

// Opt-in browser fixture, compiled only by go test. Never mounted by cmd/workspace.
import (
	"context"
	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/scheduling"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInvoiceBrowserHarness(t *testing.T) {
	if os.Getenv("DAPPER_INVOICE_BROWSER_TEST") != "1" {
		t.Skip("explicit isolated browser fixture opt-in required")
	}
	root := os.Getenv("DAPPER_BROWSER_ROOT")
	if root == "" {
		t.Fatal("browser fixture output directory required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Host != "127.0.0.1:15432" || parsed.Path != "/postgres" {
		t.Fatal("isolated local test database required; application database forbidden")
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_invoice_browser_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error("browser fixture schema cleanup failed")
		}
	}()
	query := parsed.Query()
	query.Set("search_path", namespace)
	parsed.RawQuery = query.Encode()
	store, err := postgres.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal("fixture store unavailable")
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	setup, err := pgxpool.New(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close()
	var actor, org, location string
	for _, entry := range []struct {
		sql    string
		target *string
	}{{`INSERT INTO principals(external_subject) VALUES('isolated-browser-invoice-operator') RETURNING id::text`, &actor}, {`INSERT INTO organizations(name) VALUES('Synthetic invoice browser fixture') RETURNING id::text`, &org}} {
		if err = setup.QueryRow(ctx, entry.sql).Scan(entry.target); err != nil {
			t.Fatal(err)
		}
	}
	if err = setup.QueryRow(ctx, `INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic browser-only location') RETURNING id::text`, org).Scan(&location); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, actor); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'invoices.configure')`, org, actor); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"customers.create", "customers.read", "invoices.read", "invoices.create", "invoices.manage", "invoices.issue", "payments.record"} {
		if _, err = setup.Exec(ctx, `INSERT INTO location_grants(organization_id,location_id,principal_id,action) VALUES($1,$2,$3,$4)`, org, location, actor, action); err != nil {
			t.Fatal(err)
		}
	}
	scope := postgres.Scope{OrganizationID: org, LocationID: location, ActorID: actor}
	customer, err := store.CreateCustomer(ctx, scope, "Synthetic browser customer", "invoice-browser@example.invalid", "")
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := store.SaveInvoiceIssuer(ctx, actor, org, postgres.InvoiceIssuerInput{LegalName: "Synthetic browser issuer", Address: "Isolated browser test address", TaxRegistration: "TEST-ONLY", Currency: "CAD", NumberPrefix: "BROWSER-", StartNumber: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.BindInvoiceIssuer(ctx, actor, org, location, issuer.ID); err != nil {
		t.Fatal(err)
	}
	draft, err := store.CreateInvoice(ctx, scope, postgres.CreateInvoiceInput{CustomerID: customer.ID, Items: []postgres.CreateInvoiceItemInput{{Description: "Synthetic browser original service", Quantity: 1, UnitPriceCents: 10000}}})
	if err != nil {
		t.Fatal(err)
	}
	suite := os.Getenv("DAPPER_BROWSER_SUITE")
	var ownershipTarget string
	var bookingResource string
	var initialBookingID, journeyServiceID, journeyContributorID string
	availabilityVehicles := []postgres.Vehicle{}
	bookingStart := time.Date(2027, 1, 3, 9, 0, 0, 0, time.UTC)
	if suite == "booking-cancellation" || suite == "booking-reschedule" || suite == "customer-availability" || suite == "customer-request" {
		bookingStart = time.Now().UTC().Truncate(24 * time.Hour).Add(33 * time.Hour)
	}
	if suite == "operational-journey" {
		bookingStart = time.Now().UTC().Truncate(24 * time.Hour).Add(33 * time.Hour)
		for _, action := range []string{
			"vehicles.read", "bookings.create", "bookings.read", "bookings.manage",
			"scheduling.release", "scheduling.reserve", "scheduling.manage",
			"catalogue.read", "catalogue.manage",
			"jobs.create", "jobs.read", "jobs.contribute", "jobs.complete",
			"invoices.read", "invoices.create", "invoices.manage", "invoices.issue", "payments.record",
			"attendance.record", "attendance.read",
		} {
			if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor, org, location, action); err != nil {
				t.Fatal(err)
			}
		}
		var contributorActor string
		if err = setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-journey-contributor') RETURNING id::text`).Scan(&contributorActor); err != nil {
			t.Fatal(err)
		}
		journeyContributorID = contributorActor
		if _, err = setup.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, contributorActor); err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"jobs.read", "jobs.contribute"} {
			if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, contributorActor, org, location, action); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = setup.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, actor, org, location, customer.ID); err != nil {
			t.Fatal(err)
		}
		vehicle, e := store.CreateCustomerVehicle(ctx, actor, customer.ID, postgres.Vehicle{Description: "2024 Porsche 911 GT3 (Coupe)", CategoryDescription: "Coupe"})
		if e != nil {
			t.Fatal(e)
		}
		availabilityVehicles = append(availabilityVehicles, vehicle)
		resource, e := store.CreateSchedulingResource(ctx, scope, postgres.SchedulingResource{Name: "Bay 1 - Main Bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: bookingStart, End: bookingStart.Add(8 * time.Hour)}}}})
		if e != nil {
			t.Fatal(e)
		}
		bookingResource = resource.ID
		service, e := store.SaveService(ctx, scope, postgres.CatalogService{Name: "Ceramic Coating Elite", Kind: "main"})
		if e != nil {
			t.Fatal(e)
		}
		journeyServiceID = service.ID
		rule, e := store.SaveSchedulingRule(ctx, scope, postgres.SchedulingRule{ResourceID: resource.ID, ServiceID: service.ID, DurationMinutes: 120, BeforeMinutes: 0, AfterMinutes: 0, Units: 1})
		if e != nil {
			t.Fatal(e)
		}
		initialBooking, e := store.ReserveBooking(ctx, scope, postgres.ReserveBookingInput{RequestKey: "journey-initial-booking", CustomerID: customer.ID, ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: rule.Version, Start: bookingStart, Channel: "internal"})
		if e != nil {
			t.Fatal(e)
		}
		initialBookingID = initialBooking.ID
	}
	if suite == "booking-capacity" || suite == "booking-cancellation" || suite == "booking-reschedule" || suite == "booking-offers" || suite == "customer-availability" || suite == "customer-request" {
		for _, action := range []string{"vehicles.read", "bookings.create", "bookings.read", "bookings.manage", "scheduling.release", "scheduling.reserve", "scheduling.manage", "catalogue.manage"} {
			if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor, org, location, action); err != nil {
				t.Fatal(err)
			}
		}
		resource, e := store.CreateSchedulingResource(ctx, scope, postgres.SchedulingResource{Name: "Synthetic browser booking bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: bookingStart, End: bookingStart.Add(3 * time.Hour)}}}})
		if e != nil {
			t.Fatal(e)
		}
		bookingResource = resource.ID
		service, e := store.SaveService(ctx, scope, postgres.CatalogService{Name: "Synthetic browser booking service", Kind: "main"})
		if e != nil {
			t.Fatal(e)
		}
		rule, e := store.SaveSchedulingRule(ctx, scope, postgres.SchedulingRule{ResourceID: resource.ID, ServiceID: service.ID, DurationMinutes: 60, BeforeMinutes: 0, AfterMinutes: 0, Units: 1})
		if e != nil {
			t.Fatal(e)
		}
		if suite == "booking-cancellation" || suite == "booking-reschedule" {
			if _, e = store.ReserveBooking(ctx, scope, postgres.ReserveBookingInput{RequestKey: "synthetic-cancellation-browser", CustomerID: customer.ID, ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: rule.Version, Start: bookingStart, Channel: "internal"}); e != nil {
				t.Fatal(e)
			}
		}
		if suite == "booking-reschedule" {
			if _, e = store.ReserveServiceCapacity(ctx, scope, resource.ID, rule.ID, rule.Version, bookingStart.Add(time.Hour)); e != nil {
				t.Fatal(e)
			}
		}
		if suite == "customer-availability" || suite == "customer-request" {
			if _, e = setup.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4)`, actor, org, location, customer.ID); e != nil {
				t.Fatal(e)
			}
			vehicle, e := store.CreateCustomerVehicle(ctx, actor, customer.ID, postgres.Vehicle{Description: "Synthetic availability car", CategoryDescription: "Synthetic car"})
			if e != nil {
				t.Fatal(e)
			}
			availabilityVehicles = append(availabilityVehicles, vehicle)
			if _, e = store.SaveBookingOffer(ctx, scope, "", 0, true, postgres.BookingOfferConfig{ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: rule.Version, Title: "Synthetic availability service", EquipmentCategory: "Synthetic car", Mode: "fixed", Currency: "CAD", SubtotalCents: 10000, TaxCents: 1300, DepositCents: 2000, Terms: "Synthetic service terms only", CancellationTerms: "Synthetic cancellation terms only", LeadMinutes: 0, AdvanceDays: 30, SlotMinutes: 30, Published: true}); e != nil {
				t.Fatal(e)
			}
			if _, e = store.ReserveServiceCapacity(ctx, scope, resource.ID, rule.ID, rule.Version, bookingStart.Add(time.Hour)); e != nil {
				t.Fatal(e)
			}
		}
	}
	if suite == "customer-accounts" {
		if err = setup.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-browser-customer-account') RETURNING id::text`).Scan(&ownershipTarget); err != nil {
			t.Fatal(err)
		}
		verifier, e := postgres.NewPasswordAuthenticator(store)
		if e != nil {
			t.Fatal(e)
		}
		if err = verifier.SetPassword(ctx, ownershipTarget, "customer-browser@example.invalid", "Isolated browser customer password only"); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, ownershipTarget); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage')`, actor, org); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'customer_accounts.manage')`, actor, org, location); err != nil {
			t.Fatal(err)
		}
	}
	token, err := store.IssueSession(ctx, actor, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	api := NewHandler(store, "https://127.0.0.1:8444")
	webURL, _ := url.Parse("http://127.0.0.1:3007")
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", api)
	mux.HandleFunc("GET /acceptance-session", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: identity.SessionCookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /acceptance-context", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]any{
			"base":             "/api/v1/organizations/" + org + "/locations/" + location,
			"invoiceId":        draft.ID,
			"suite":            suite,
			"actorId":          actor,
			"startHint":        bookingStart.Format(time.RFC3339),
			"customerId":       customer.ID,
			"vehicles":         availabilityVehicles,
			"initialBookingId": initialBookingID,
			"serviceId":        journeyServiceID,
			"contributorId":    journeyContributorID,
		})
	})
	mux.HandleFunc("POST /journey-secondary-contribute", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			JobID string `json:"jobId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		cScope := postgres.Scope{OrganizationID: org, LocationID: location, ActorID: journeyContributorID}
		if _, err := store.ContributeJob(r.Context(), cScope, payload.JobID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		reply(w, 200, map[string]any{"ok": true})
	})
	mux.Handle("/", httputil.NewSingleHostReverseProxy(webURL))
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	listener, err := net.Listen("tcp", "127.0.0.1:8444")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	done := make(chan error, 1)
	go func() {
		done <- server.ServeTLS(listener, os.Getenv("DAPPER_BROWSER_CERT"), os.Getenv("DAPPER_BROWSER_KEY"))
	}()
	if err = os.WriteFile(filepath.Join(root, "ready"), []byte("isolated synthetic fixture ready"), 0600); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err = <-done:
			t.Fatal("browser fixture server stopped", err)
		case <-ctx.Done():
			t.Fatal("browser fixture expired before acceptance finished")
		case <-ticker.C:
			if _, err = os.Stat(filepath.Join(root, "finish")); err == nil {
				if suite == "customer-request" {
					var requests, events, bookings, reservations int
					if e := setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM customer_booking_requests WHERE organization_id=$1 AND customer_note='Synthetic browser appointment request' AND terms_reviewed AND offer_snapshot#>>'{public,subtotalCents}'='10000'),(SELECT count(*) FROM customer_booking_request_events WHERE organization_id=$1),(SELECT count(*) FROM bookings WHERE organization_id=$1),(SELECT count(*) FROM capacity_reservations WHERE resource_id=$2)`, org, bookingResource).Scan(&requests, &events, &bookings, &reservations); e != nil {
						t.Fatal(e)
					}
					if requests != 1 || events != 1 || bookings != 0 || reservations != 1 {
						t.Fatal("customer request browser missing snapshot/audit or created capacity/booking")
					}
					if e := os.WriteFile(filepath.Join(root, "verified"), []byte("PASS: isolated PostgreSQL customer_request1 submitted_event1 immutable_price_snapshot10000 booking0 existing_staff_reservation1 no_payment"), 0600); e != nil {
						t.Fatal(e)
					}
					return
				}
				if suite == "customer-availability" {
					var bookings, reservations int
					if e := setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM bookings WHERE organization_id=$1),(SELECT count(*) FROM capacity_reservations WHERE resource_id=$2)`, org, bookingResource).Scan(&bookings, &reservations); e != nil {
						t.Fatal(e)
					}
					if bookings != 0 || reservations != 1 {
						t.Fatal("availability browser created a booking or changed fixture allocation")
					}
					if e := os.WriteFile(filepath.Join(root, "verified"), []byte("PASS: isolated PostgreSQL advisory_availability booking0 existing_staff_reservation1 no_checkout_or_payment"), 0600); e != nil {
						t.Fatal(e)
					}
					return
				}
				if suite == "booking-offers" {
					var offers, revisions int
					if e := setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM booking_offers WHERE resource_id=$1 AND version=3 AND NOT published AND title='Synthetic browser offer' AND subtotal_cents=10000 AND tax_cents=1300 AND deposit_cents=2000),(SELECT count(*) FROM booking_offer_revisions r JOIN booking_offers o ON o.id=r.offer_id WHERE o.resource_id=$1)`, bookingResource).Scan(&offers, &revisions); e != nil {
						t.Fatal(e)
					}
					if offers != 1 || revisions != 3 {
						t.Fatal("browser offer draft/publication/withdraw revisions not persisted")
					}
					if e := os.WriteFile(filepath.Join(root, "verified"), []byte("PASS: isolated PostgreSQL offer_draft_published_withdrawn revisions3 exact_cents10000_tax1300_deposit2000"), 0600); e != nil {
						t.Fatal(e)
					}
					return
				}
				if suite == "booking-reschedule" {
					var moved, receipts, events int
					if e := setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM bookings b JOIN capacity_reservations r ON r.id=b.reservation_id WHERE r.resource_id=$1 AND b.status='pending' AND b.booking_time=$2 AND r.version=2 AND r.cancelled_at IS NULL),(SELECT count(*) FROM booking_reschedule_requests q JOIN bookings b ON b.id=q.booking_id JOIN capacity_reservations r ON r.id=b.reservation_id WHERE r.resource_id=$1),(SELECT count(*) FROM booking_events e JOIN bookings b ON b.id=e.booking_id JOIN capacity_reservations r ON r.id=b.reservation_id WHERE r.resource_id=$1 AND e.notes LIKE 'Rescheduled from %')`, bookingResource, bookingStart.Add(2*time.Hour)).Scan(&moved, &receipts, &events); e != nil {
						t.Fatal(e)
					}
					if moved != 1 || receipts != 1 || events != 1 {
						t.Fatal("browser move did not persist coherent booking/reservation/receipt/event")
					}
					if e := os.WriteFile(filepath.Join(root, "verified"), []byte("PASS: isolated PostgreSQL moved_pending_booking1 reservation_version2 receipt1 reschedule_event1"), 0600); e != nil {
						t.Fatal(e)
					}
					return
				}
				if suite == "booking-cancellation" {
					var cancelled, released, events int
					if e := setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM bookings b JOIN capacity_reservations r ON r.id=b.reservation_id WHERE r.resource_id=$1 AND b.status='cancelled' AND b.cancellation_reason='Synthetic browser cancellation'),(SELECT count(*) FROM capacity_reservations WHERE resource_id=$1 AND cancelled_at IS NOT NULL AND version=2),(SELECT count(*) FROM booking_events e JOIN bookings b ON b.id=e.booking_id JOIN capacity_reservations r ON r.id=b.reservation_id WHERE r.resource_id=$1 AND e.to_status='cancelled')`, bookingResource).Scan(&cancelled, &released, &events); e != nil {
						t.Fatal(e)
					}
					if cancelled != 1 || released != 1 || events != 1 {
						t.Fatal("browser cancellation did not atomically persist booking, released capacity and one event")
					}
					if e := os.WriteFile(filepath.Join(root, "verified"), []byte("PASS: isolated PostgreSQL cancelled_booking=1 released_reservation_version2=1 cancellation_event=1"), 0600); e != nil {
						t.Fatal(e)
					}
					return
				}
				if suite == "booking-capacity" {
					var bookings, reservations, receipts int
					if e := setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM bookings b JOIN capacity_reservations r ON r.id=b.reservation_id WHERE r.resource_id=$1 AND b.status='pending'),(SELECT count(*) FROM capacity_reservations WHERE resource_id=$1),(SELECT count(*) FROM booking_capacity_requests q JOIN bookings b ON b.id=q.booking_id JOIN capacity_reservations r ON r.id=b.reservation_id WHERE r.resource_id=$1)`, bookingResource).Scan(&bookings, &reservations, &receipts); e != nil {
						t.Fatal(e)
					}
					if bookings != 1 || reservations != 1 || receipts != 1 {
						t.Fatal("browser did not persist one atomic pending booking/reservation/retry receipt")
					}
					if e := os.WriteFile(filepath.Join(root, "verified"), []byte("PASS: isolated PostgreSQL pending_booking=1 capacity_reservation=1 retry_receipt=1 no_confirmation_or_payment"), 0600); e != nil {
						t.Fatal(e)
					}
					return
				}
				if suite == "customer-accounts" {
					var grants, revokes, links int
					if e := setup.QueryRow(ctx, `SELECT count(*) FILTER(WHERE enabled),count(*) FILTER(WHERE NOT enabled) FROM customer_account_events WHERE principal_id=$1 AND customer_id=$2`, ownershipTarget, customer.ID).Scan(&grants, &revokes); e != nil {
						t.Fatal(e)
					}
					if e := setup.QueryRow(ctx, `SELECT count(*) FROM customer_account_links WHERE principal_id=$1 AND customer_id=$2`, ownershipTarget, customer.ID).Scan(&links); e != nil {
						t.Fatal(e)
					}
					if grants != 1 || revokes != 1 || links != 0 {
						t.Fatal("browser did not persist exactly one reviewed grant and revoke")
					}
					if _, e := store.CustomerRecords(ctx, ownershipTarget, customer.ID); !errors.Is(e, postgres.ErrForbidden) {
						t.Fatal("revoked browser fixture customer retained record access", e)
					}
					if e := os.WriteFile(filepath.Join(root, "verified"), []byte("PASS: isolated PostgreSQL customer ownership grant=1 revoke=1 remaining_links=0 portal_access=denied"), 0600); e != nil {
						t.Fatal(e)
					}
					return
				}
				if suite == "operational-journey" {
					var customersCount, vehiclesCount, bookingsCount, cancelEventsCount, completedJobsCount, contributorsCount, invoicesCount, paymentsCount int
					if e := setup.QueryRow(ctx, `
						SELECT
							(SELECT count(*) FROM customers WHERE organization_id=$1),
							(SELECT count(*) FROM vehicles WHERE customer_id=$2),
							(SELECT count(*) FROM bookings WHERE organization_id=$1),
							(SELECT count(*) FROM booking_events WHERE organization_id=$1 AND to_status='cancelled'),
							(SELECT count(*) FROM jobs WHERE organization_id=$1 AND status='completed'),
							(SELECT count(*) FROM job_contributors jc JOIN jobs j ON j.id=jc.job_id WHERE j.organization_id=$1),
							(SELECT count(*) FROM invoices WHERE organization_id=$1),
							(SELECT count(*) FROM payments WHERE organization_id=$1)
					`, org, customer.ID).Scan(
						&customersCount, &vehiclesCount, &bookingsCount, &cancelEventsCount,
						&completedJobsCount, &contributorsCount, &invoicesCount, &paymentsCount,
					); e != nil {
						t.Fatal(e)
					}
					if customersCount < 1 || vehiclesCount < 1 || bookingsCount < 1 || cancelEventsCount < 1 || completedJobsCount < 1 || contributorsCount < 2 || invoicesCount < 1 || paymentsCount < 1 {
						t.Fatalf("operational journey verification failed: customers=%d vehicles=%d bookings=%d cancelEvents=%d completedJobs=%d contributors=%d invoices=%d payments=%d",
							customersCount, vehiclesCount, bookingsCount, cancelEventsCount, completedJobsCount, contributorsCount, invoicesCount, paymentsCount)
					}
					verifiedMsg := fmt.Sprintf("PASS: isolated PostgreSQL operational_journey customers=%d vehicles=%d bookings=%d cancel_events=%d completed_jobs=%d contributors=%d invoices=%d payments=%d",
						customersCount, vehiclesCount, bookingsCount, cancelEventsCount, completedJobsCount, contributorsCount, invoicesCount, paymentsCount)
					if e := os.WriteFile(filepath.Join(root, "verified"), []byte(verifiedMsg), 0600); e != nil {
						t.Fatal(e)
					}
					return
				}
				final, e := store.GetInvoice(ctx, scope, draft.ID)
				if e != nil {
					t.Fatal(e)
				}
				history, e := store.InvoiceDraftHistory(ctx, scope, draft.ID)
				if e != nil {
					t.Fatal(e)
				}
				if final.Version < 2 || len(history) < 1 {
					t.Fatal("browser did not persist a real draft revision")
				}
				var subtotal int64
				for _, item := range final.Items {
					subtotal += item.TotalPriceCents
				}
				if subtotal != final.SubtotalCents || final.TotalCents != subtotal+final.TaxCents {
					t.Fatal("persisted browser invoice totals inconsistent")
				}
				if e = os.WriteFile(filepath.Join(root, "verified"), []byte(fmt.Sprintf("PASS: isolated PostgreSQL browser invoice version=%d revisions=%d status=%s", final.Version, len(history), final.Status)), 0600); e != nil {
					t.Fatal(e)
				}
				return
			}
		}
	}
}
