package workspace

// Opt-in, test-only server. Never registered by the application executable.
import (
	"context"
	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/scheduling"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Cookies are port-independent. Never replace the user's real application session.
const employeeReviewCookie = "dapper_employee_review"

type employeeReviewWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *employeeReviewWriter) WriteHeader(status int) {
	if !w.wrote {
		values := w.Header().Values("Set-Cookie")
		w.Header().Del("Set-Cookie")
		for _, v := range values {
			w.Header().Add("Set-Cookie", strings.Replace(v, identity.SessionCookieName+"=", employeeReviewCookie+"=", 1))
		}
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *employeeReviewWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}

func TestEmployeeBrowserHarness(t *testing.T) {
	if os.Getenv("DAPPER_EMPLOYEE_BROWSER_TEST") != "1" {
		t.Skip("explicit isolated employee browser opt-in required")
	}
	rootPath := os.Getenv("DAPPER_EMPLOYEE_BROWSER_ROOT")
	if rootPath == "" {
		t.Fatal("fixture directory required")
	}
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	u, err := url.Parse(dsn)
	if err != nil || u.Host != "127.0.0.1:15432" || u.Path != "/postgres" {
		t.Fatal("isolated postgres database required; application database forbidden")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Minute)
	defer cancel()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	namespace := fmt.Sprintf("dapper_employee_browser_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = root.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if _, err := root.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	q := u.Query()
	q.Set("search_path", namespace)
	u.RawQuery = q.Encode()
	store, err := postgres.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	config := root.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	setup, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close()
	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		if err := setup.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := setup.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	org := id(`INSERT INTO organizations(name) VALUES('Synthetic employee review organization') RETURNING id::text`)
	loc := id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic review worksite') RETURNING id::text`, org)
	owner := id(`INSERT INTO principals(external_subject) VALUES('synthetic:employee-browser-owner') RETURNING id::text`)
	employee := id(`INSERT INTO principals(external_subject) VALUES('synthetic:employee-browser-person') RETURNING id::text`)
	exec(`INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2),($1,$3)`, org, owner, employee)
	exec(`INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'access.manage'),($1,$2,'invoices.configure')`, org, owner)
	for _, p := range []string{owner, employee} {
		for _, a := range []string{"attendance.record", "bookings.read", "bookings.create", "customers.create", "scheduling.reserve", "scheduling.manage", "catalogue.read", "catalogue.manage", "invoices.create", "invoices.issue", "invoices.read", "payments.record"} {
			exec(`INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, p, org, loc, a)
		}
	}
	scope := postgres.Scope{ActorID: owner, OrganizationID: org, LocationID: loc}
	customer := id(`INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic review customer','review@example.invalid','+15555550123') RETURNING id::text`, org, loc)
	dealer := id(`INSERT INTO dealership_accounts(organization_id,location_id,customer_id,account_name,account_code,account_type,contact_name,contact_email) VALUES($1,$2,$3,'Synthetic review dealership','REVIEW','dealership','Synthetic coordinator','review@example.invalid') RETURNING id::text`, org, loc, customer)
	if _, err = store.AssignEmployeeWorksite(ctx, owner, org, employee, loc, dealer, "America/Toronto", 0); err != nil {
		t.Fatal(err)
	}
	service, err := store.SaveService(ctx, scope, postgres.CatalogService{Name: "Synthetic exterior detail", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveService(ctx, scope, postgres.CatalogService{Name: "Synthetic interior add-on", Kind: "addon", ParentID: &service.ID}); err != nil {
		t.Fatal(err)
	}
	zone, _ := time.LoadLocation("America/Toronto")
	next := time.Now().In(zone).AddDate(0, 0, 1)
	open := time.Date(next.Year(), next.Month(), next.Day(), 8, 0, 0, 0, zone)
	resource, e := store.CreateSchedulingResource(ctx, scope, postgres.SchedulingResource{Name: "Synthetic review bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: open, End: open.Add(10 * time.Hour)}}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = store.SaveSchedulingRule(ctx, scope, postgres.SchedulingRule{ResourceID: resource.ID, ServiceID: service.ID, DurationMinutes: 60, Units: 1}); e != nil {
		t.Fatal(e)
	}
	booking, err := store.CreateBooking(ctx, scope, postgres.Booking{CustomerID: customer, ServiceID: &service.ID, Channel: "walk_in", BookingTime: time.Now().Add(30 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.LinkEmployeeBooking(ctx, owner, org, loc, dealer, booking.ID); err != nil {
		t.Fatal(err)
	}
	inv, err := store.CreateInvoice(ctx, scope, postgres.CreateInvoiceInput{CustomerID: customer, BookingID: &booking.ID, InvoiceNumber: "SYNTHETIC-REVIEW", Items: []postgres.CreateInvoiceItemInput{{Description: service.Name, Quantity: 1, UnitPriceCents: 2500}}})
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := store.SaveInvoiceIssuer(ctx, owner, org, postgres.InvoiceIssuerInput{LegalName: "Synthetic review issuer", Address: "Synthetic review address", Currency: "CAD", NumberPrefix: "REVIEW-", StartNumber: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.BindInvoiceIssuer(ctx, owner, org, loc, issuer.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.IssueInvoice(ctx, scope, inv.ID); err != nil {
		t.Fatal(err)
	}
	token, err := store.IssueSession(ctx, employee, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	api := NewHandler(store, "https://127.0.0.1:8445")
	mux.Handle("/api/v1/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clone := r.Clone(r.Context())
		clone.Header = r.Header.Clone()
		clone.Header.Del("Cookie")
		if cookie, err := r.Cookie(employeeReviewCookie); err == nil {
			clone.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: cookie.Value})
		}
		api.ServeHTTP(&employeeReviewWriter{ResponseWriter: w}, clone)
	}))
	mux.HandleFunc("GET /acceptance-session", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: employeeReviewCookie, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		http.Redirect(w, r, "/workspace", http.StatusSeeOther)
	})
	webURL, _ := url.Parse("http://127.0.0.1:4176")
	mux.Handle("/", httputil.NewSingleHostReverseProxy(webURL))
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:8445")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- server.ServeTLS(listener, os.Getenv("DAPPER_BROWSER_CERT"), os.Getenv("DAPPER_BROWSER_KEY"))
	}()
	if err = os.WriteFile(filepath.Join(rootPath, "ready"), []byte("Synthetic isolated employee fixture ready"), 0600); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal("isolated review expired")
		case <-ticker.C:
			if _, err = os.Stat(filepath.Join(rootPath, "finish")); err != nil {
				continue
			}
			var shifts, submissions, payments, walkins int
			var due int64
			if err = setup.QueryRow(ctx, `SELECT count(*) FROM employee_attendance WHERE principal_id=$1 AND status='clocked_out'`, employee).Scan(&shifts); err != nil {
				t.Fatal(err)
			}
			if err = setup.QueryRow(ctx, `SELECT count(*) FROM employee_availability_submissions WHERE principal_id=$1`, employee).Scan(&submissions); err != nil {
				t.Fatal(err)
			}
			if err = setup.QueryRow(ctx, `SELECT count(*) FROM payments WHERE actor_id=$1 AND method='cash'`, employee).Scan(&payments); err != nil {
				t.Fatal(err)
			}
			if err = setup.QueryRow(ctx, `SELECT due_cents FROM invoices WHERE id=$1`, inv.ID).Scan(&due); err != nil {
				t.Fatal(err)
			}
			if err = setup.QueryRow(ctx, `SELECT count(*) FROM bookings b JOIN employee_booking_worksites w ON w.booking_id=b.id JOIN customers c ON c.id=b.customer_id WHERE c.email='walkin-browser@example.invalid' AND b.reservation_id IS NOT NULL AND w.dealership_id=$1`, dealer).Scan(&walkins); err != nil {
				t.Fatal(err)
			}
			if shifts != 1 || submissions != 1 || payments != 1 || due != 0 || walkins != 1 {
				t.Fatalf("employee browser persistence incomplete: closedShifts=%d availability=%d payments=%d due=%d", shifts, submissions, payments, due)
			}
			if err = os.WriteFile(filepath.Join(rootPath, "verified"), []byte("PASS synthetic isolated employee closedShift1 availability1 cashReceipt1 invoiceDue0 walkinCapacity1 dealershipLink1; no OTP/provider/GPS operation"), 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
}
