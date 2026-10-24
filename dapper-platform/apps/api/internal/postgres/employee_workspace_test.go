package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestEmployeeAvailabilityValidation(t *testing.T) {
	for _, items := range [][]AvailabilityInterval{
		{{Day: 7, Start: "09:00", End: "10:00"}}, {{Day: 0, Start: "10:00", End: "09:00"}}, {{Day: 0, Start: "9:00", End: "10:00"}},
		{{Day: 0, Start: "09:00", End: "12:00"}, {Day: 0, Start: "11:00", End: "13:00"}},
	} {
		if !errors.Is(validateAvailability(items), ErrEmployeeInput) {
			t.Fatal("invalid availability accepted", items)
		}
	}
	if err := validateAvailability([]AvailabilityInterval{{Day: 0, Start: "09:00", End: "12:00"}, {Day: 0, Start: "12:00", End: "13:00"}, {Day: 6, Start: "09:00", End: "10:00"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := nextEmployeeWeek(time.Date(2026, 10, 25, 23, 0, 0, 0, time.UTC), "America/Toronto"); err != nil || got != "2026-10-26" {
		t.Fatal("next Monday", got, err)
	}
	if got, err := nextEmployeeWeek(time.Date(2026, 10, 26, 15, 0, 0, 0, time.UTC), "America/Toronto"); err != nil || got != "2026-11-02" {
		t.Fatal("next week not current Monday", got, err)
	}
}

func TestEmployeeWorkspacePersistence(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	namespace := fmt.Sprintf("dapper_employee_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = root.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 20*time.Second)
		defer done()
		if _, err := root.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config := root.pool.Config()
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
	id := func(query string, args ...any) string {
		t.Helper()
		var v string
		if err := pool.QueryRow(ctx, query, args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	org := id(`INSERT INTO organizations(name) VALUES('Synthetic employee organization') RETURNING id::text`)
	otherOrg := id(`INSERT INTO organizations(name) VALUES('Synthetic foreign organization') RETURNING id::text`)
	loc := id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic shared worksite') RETURNING id::text`, org)
	otherLoc := id(`INSERT INTO locations(organization_id,name) VALUES($1,'Synthetic foreign worksite') RETURNING id::text`, otherOrg)
	owner := id(`INSERT INTO principals(external_subject) VALUES('synthetic:employee-owner') RETURNING id::text`)
	employee := id(`INSERT INTO principals(external_subject) VALUES('synthetic:employee') RETURNING id::text`)
	foreign := id(`INSERT INTO principals(external_subject) VALUES('synthetic:foreign-employee') RETURNING id::text`)
	exec(`INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2),($1,$3),($4,$5)`, org, owner, employee, otherOrg, foreign)
	exec(`INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'access.manage')`, org, owner)
	dealer := func(o, l, code string) string {
		customer := id(`INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic dealer customer','dealer@example.invalid','+15555550100') RETURNING id::text`, o, l)
		return id(`INSERT INTO dealership_accounts(organization_id,location_id,customer_id,account_name,account_code,account_type,contact_name,contact_email) VALUES($1,$2,$3,'Synthetic dealership',$4,'dealership','Synthetic contact','dealer@example.invalid') RETURNING id::text`, o, l, customer, code)
	}
	a, b, c := dealer(org, loc, "SYN-A"), dealer(org, loc, "SYN-B"), dealer(otherOrg, otherLoc, "SYN-C")
	if d, err := s.EmployeeWorkspace(ctx, employee); err != nil || d.Assignment != nil {
		t.Fatal("no inferred assignment", d, err)
	}
	if _, err = s.AssignEmployeeWorksite(ctx, employee, org, employee, loc, a, "America/Toronto", 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("self assignment", err)
	}
	if _, err = s.AssignEmployeeWorksite(ctx, owner, org, foreign, loc, a, "America/Toronto", 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign membership", err)
	}
	if _, err = s.AssignEmployeeWorksite(ctx, owner, org, employee, otherLoc, c, "America/Toronto", 0); !errors.Is(err, ErrEmployeeInput) {
		t.Fatal("cross organization dealer", err)
	}
	if _, err = s.AssignEmployeeWorksite(ctx, owner, org, employee, loc, a, "not/a-zone", 0); !errors.Is(err, ErrEmployeeInput) {
		t.Fatal("timezone", err)
	}
	assignment, err := s.AssignEmployeeWorksite(ctx, owner, org, employee, loc, a, "America/Toronto", 0)
	if err != nil || assignment.Version != 1 {
		t.Fatal("assign", assignment, err)
	}
	if d, err := s.EmployeeWorkspace(ctx, employee); err != nil || len(d.Actions) != 0 || d.Assignment.DealershipID != a {
		t.Fatal("assignment added capabilities", d, err)
	}
	if _, err = s.EmployeeAttendanceAction(ctx, employee, "clock-in", nil); !errors.Is(err, ErrForbidden) {
		t.Fatal("attendance permission inferred", err)
	}
	if _, err = s.AssignEmployeeWorksite(ctx, owner, org, employee, loc, b, "America/Toronto", 0); !errors.Is(err, ErrEmployeeVersion) {
		t.Fatal("stale assignment", err)
	}
	for _, p := range []string{owner, employee} {
		for _, action := range []string{"attendance.record", "bookings.read", "bookings.create", "invoices.create", "invoices.read", "payments.record", "invoices.issue"} {
			exec(`INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, p, org, loc, action)
		}
	}
	week, _ := nextEmployeeWeek(time.Now(), assignment.Timezone)
	av := EmployeeAvailability{WeekStart: week, Intervals: []AvailabilityInterval{{Day: 0, Start: "08:00", End: "12:00"}}, Notes: "Synthetic availability only"}
	saved, err := s.SubmitEmployeeAvailability(ctx, employee, av)
	if err != nil || saved.Version != 1 || saved.SubmittedAt == nil {
		t.Fatal("persist availability", saved, err)
	}
	if _, err = s.SubmitEmployeeAvailability(ctx, employee, av); !errors.Is(err, ErrEmployeeVersion) {
		t.Fatal("availability optimistic concurrency", err)
	}
	desk, err := s.EmployeeWorkspace(ctx, employee)
	if err != nil || desk.Availability.Version != 1 || len(desk.Availability.Intervals) != 1 {
		t.Fatal("availability reload", desk, err)
	}
	review, err := s.EmployeeWorksiteAdministration(ctx, owner, org)
	if err != nil || len(review.Submissions) != 1 || len(review.Assignments) != 1 {
		t.Fatal("supervisor inbox", review, err)
	}
	if _, err = s.EmployeeWorksiteAdministration(ctx, employee, org); !errors.Is(err, ErrForbidden) {
		t.Fatal("employee review others", err)
	}
	exec(`CREATE FUNCTION reject_employee_test_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic audit rollback'; END $$`)
	exec(`CREATE TRIGGER reject_employee_test_audit BEFORE INSERT ON employee_workspace_events FOR EACH ROW EXECUTE FUNCTION reject_employee_test_audit()`)
	saved.Notes = "Should rollback"
	if _, err = s.SubmitEmployeeAvailability(ctx, employee, saved); err == nil {
		t.Fatal("audit failure committed")
	}
	exec(`DROP TRIGGER reject_employee_test_audit ON employee_workspace_events`)
	desk, err = s.EmployeeWorkspace(ctx, employee)
	if err != nil || desk.Availability.Version != 1 || desk.Availability.Notes == saved.Notes {
		t.Fatal("audit rollback lost", err)
	}
	shift, err := s.EmployeeAttendanceAction(ctx, employee, "clock-in", nil)
	if err != nil || shift.Status != "clocked_in" {
		t.Fatal("employee clock in", shift, err)
	}
	if _, err = s.AssignEmployeeWorksite(ctx, owner, org, employee, loc, b, "America/Toronto", 1); !errors.Is(err, ErrEmployeeActiveShift) {
		t.Fatal("active worksite changed", err)
	}
	if _, err = s.EmployeeAttendanceAction(ctx, employee, "start-break", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EmployeeAttendanceAction(ctx, employee, "end-break", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EmployeeAttendanceAction(ctx, employee, "clock-out", nil); err != nil {
		t.Fatal(err)
	}
	customer := id(`INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic walk-in','walkin@example.invalid','+15555550101') RETURNING id::text`, org, loc)
	scope := Scope{ActorID: owner, OrganizationID: org, LocationID: loc}
	makeBooking := func(name string) Booking {
		t.Helper()
		v, e := s.CreateBooking(ctx, scope, Booking{CustomerID: customer, ServiceName: name, Channel: "walk_in", BookingTime: time.Now().Add(time.Hour)})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	visible, hidden, unlinked := makeBooking("Synthetic A booking"), makeBooking("Synthetic B booking"), makeBooking("Synthetic unlinked booking")
	if err = s.LinkEmployeeBooking(ctx, owner, org, loc, a, visible.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.LinkEmployeeBooking(ctx, owner, org, loc, b, hidden.ID); err != nil {
		t.Fatal(err)
	}
	desk, err = s.EmployeeWorkspace(ctx, employee)
	if err != nil || len(desk.Bookings) != 1 || desk.Bookings[0].ID != visible.ID {
		t.Fatal("dealership leakage or missing linked booking", desk.Bookings, err)
	}
	if _, err = s.EmployeeBookingInvoices(ctx, employee, hidden.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Fatal("foreign dealer invoice access", err)
	}
	if _, err = s.EmployeeBookingInvoices(ctx, employee, unlinked.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Fatal("unlinked invoice access", err)
	}
	invoice, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: customer, BookingID: &visible.ID, InvoiceNumber: "SYNTHETIC-EMP-1", Items: []CreateInvoiceItemInput{{Description: "Synthetic service", Quantity: 1, UnitPriceCents: 2500}}})
	if err != nil {
		t.Fatal(err)
	}
	invoice = issueInvoiceForTest(t, ctx, s, scope, invoice)
	receipt, paid, err := s.EmployeeCashReceipt(ctx, employee, visible.ID, RecordPaymentInput{InvoiceID: invoice.ID, AmountCents: 2500, IdempotencyKey: "synthetic-employee-cash-1"})
	if err != nil || paid.DueCents != 0 {
		t.Fatal("cash", paid, err)
	}
	duplicate, _, err := s.EmployeeCashReceipt(ctx, employee, visible.ID, RecordPaymentInput{InvoiceID: invoice.ID, AmountCents: 2500, IdempotencyKey: "synthetic-employee-cash-1"})
	if err != nil || receipt.ID != duplicate.ID {
		t.Fatal("cash duplicated", err)
	}
	items, err := s.EmployeeBookingInvoices(ctx, employee, visible.ID)
	if err != nil || len(items) != 1 || items[0].DueCents != 0 {
		t.Fatal("paid invoice reload", items, err)
	}
	// Card payments reuse the same dealership/booking/invoice proof as cash receipts.
	if _, err = s.EmployeeCardInvoiceScope(ctx, employee, hidden.ID, invoice.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Fatal("card scope reached a foreign dealership booking", err)
	}
	if _, err = s.EmployeeCardInvoiceScope(ctx, employee, unlinked.ID, invoice.ID); !errors.Is(err, ErrBookingNotFound) {
		t.Fatal("card scope reached an unlinked booking", err)
	}
	if _, err = s.EmployeeCardInvoiceScope(ctx, employee, visible.ID, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, ErrInvoiceNotFound) {
		t.Fatal("card scope accepted an invoice that is not on the booking", err)
	}
	cardScope, err := s.EmployeeCardInvoiceScope(ctx, employee, visible.ID, invoice.ID)
	if err != nil || cardScope.OrganizationID != org || cardScope.LocationID != loc || cardScope.ActorID != employee {
		t.Fatal("card scope was not the assigned worksite", cardScope, err)
	}
	if _, err = s.EmployeeSquareScope(ctx, employee); err != nil {
		t.Fatal("employee card scope", err)
	}
	exec(`DELETE FROM location_grants WHERE principal_id=$1 AND action='payments.record'`, employee)
	if _, err = s.EmployeeCardInvoiceScope(ctx, employee, visible.ID, invoice.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked card access", err)
	}
	if _, err = s.EmployeeSquareScope(ctx, employee); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked card scope", err)
	}
	if _, _, err = s.EmployeeCashReceipt(ctx, employee, visible.ID, RecordPaymentInput{InvoiceID: invoice.ID, AmountCents: 2500, IdempotencyKey: "synthetic-employee-cash-1"}); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked cash access", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE employee_workspace_events SET action='changed'`); err == nil {
		t.Fatal("audit mutable")
	}
	verifyEmployeeUploads(t, ctx, s, employee, scope, visible.ID, hidden.ID, unlinked.ID, a, b)
	verifyEmployeeDocuments(t, ctx, s, employee, owner, org)
	verifyEmployeeWalkIn(t, ctx, s, scope, employee, a)
	// Assignment is by franchise site: the dealership is optional and must be a partner dealership account.
	siteOnly := id(`INSERT INTO principals(external_subject) VALUES('synthetic:site-only-employee') RETURNING id::text`)
	exec(`INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, org, siteOnly)
	exec(`INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'bookings.read')`, siteOnly, org, loc)
	internalAccount := id(`WITH c AS (INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic internal customer','internal2@example.invalid','+15555550102') RETURNING id) INSERT INTO dealership_accounts(organization_id,location_id,customer_id,account_name,account_code,account_type,contact_name,contact_email) SELECT $1,$2,id,'Synthetic Internal Worksite','SYN-INT2','commercial','Synthetic contact','internal2@example.invalid' FROM c RETURNING id::text`, org, loc)
	if _, err = s.AssignEmployeeWorksite(ctx, owner, org, siteOnly, loc, internalAccount, "America/Toronto", 0); !errors.Is(err, ErrEmployeeInput) {
		t.Fatal("an internal account was accepted as the employee's dealership", err)
	}
	if _, err = s.AssignEmployeeWorksite(ctx, owner, org, siteOnly, otherLoc, "", "America/Toronto", 0); !errors.Is(err, ErrEmployeeInput) {
		t.Fatal("a site in another organization was accepted", err)
	}
	siteAssignment, err := s.AssignEmployeeWorksite(ctx, owner, org, siteOnly, loc, "", "America/Toronto", 0)
	if err != nil || siteAssignment.LocationID != loc || siteAssignment.DealershipID != "" || siteAssignment.DealershipName != "" {
		t.Fatal("assignment by site without a dealership failed", siteAssignment, err)
	}
	siteBooking := makeBooking("Synthetic site-level booking")
	if err = s.LinkEmployeeBooking(ctx, owner, org, loc, "", siteBooking.ID); err != nil {
		t.Fatal("site-level booking link", err)
	}
	if d, e := s.EmployeeWorkspace(ctx, siteOnly); e != nil || d.Assignment == nil || len(d.Bookings) != 1 || d.Bookings[0].ID != siteBooking.ID {
		t.Fatal("a site-only employee must see exactly the site-level bookings", d.Bookings, e)
	}
	if d, e := s.EmployeeWorkspace(ctx, employee); e != nil || d.Assignment == nil || d.Assignment.DealershipID != a {
		t.Fatal("dealership-focused employee lost their dealership", e)
	} else {
		seen := map[string]bool{}
		for _, b := range d.Bookings {
			seen[b.ID] = true
		}
		if !seen[siteBooking.ID] || !seen[visible.ID] || seen[hidden.ID] {
			t.Fatal("dealership-focused employee must see site-level plus own-dealership bookings only", seen)
		}
	}
	if _, err = s.EmployeeBookingInvoices(ctx, siteOnly, visible.ID); !errors.Is(err, ErrBookingNotFound) && !errors.Is(err, ErrForbidden) {
		t.Fatal("site-only employee reached a dealership-linked booking", err)
	}
	// Availability is one submission per person per week at the site, with or without a dealership.
	if _, err = s.SubmitEmployeeAvailability(ctx, siteOnly, EmployeeAvailability{WeekStart: siteAssignmentWeek(t, siteOnly, s, ctx), Intervals: []AvailabilityInterval{}, Version: 0}); err != nil {
		t.Fatal("availability without a dealership", err)
	}
	exec(`UPDATE principals SET active=false WHERE id=$1`, employee)
	if d, err := s.EmployeeWorkspace(ctx, employee); err != nil || d.Assignment != nil {
		t.Fatal("inactive principal retained worksite", err)
	}
}

func siteAssignmentWeek(t *testing.T, actor string, s *Store, ctx context.Context) string {
	t.Helper()
	d, err := s.EmployeeWorkspace(ctx, actor)
	if err != nil || d.Availability == nil {
		t.Fatal("no availability week for the site-only employee", err)
	}
	return d.Availability.WeekStart
}
