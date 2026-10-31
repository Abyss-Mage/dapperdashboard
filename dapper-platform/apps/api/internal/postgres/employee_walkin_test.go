package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"testing"
	"time"
)

func TestEmployeeContactFormats(t *testing.T) {
	name, email, phone, err := normalizeEmployeeContact("  Real Name ", "person@EXAMPLE.CA", "+1 (416) 555-0123")
	if err != nil || name != "Real Name" || email != "person@example.ca" || phone != "+14165550123" {
		t.Fatal(name, email, phone, err)
	}
	for _, v := range [][3]string{{"A", "person@example.ca", "+14165550123"}, {"Name", "Name <a@example.ca>", "+14165550123"}, {"Name", "a@localhost", "+14165550123"}, {"Name", "a@example.ca", "4165550123"}, {"Name", "a@example.ca", "+00000000"}, {"Name", "a@example.ca", "+1abc4165550123"}, {"Name", "a@@example.ca", "+14165550123"}} {
		if _, _, _, e := normalizeEmployeeContact(v[0], v[1], v[2]); !errors.Is(e, ErrEmployeeContact) {
			t.Fatal("bad contact accepted", v, e)
		}
	}
	for _, v := range []string{"2026-03-08T02:30", "2026-11-01T01:30", "2026-02-30T10:00"} {
		if _, e := employeeLocalStart(v, "America/Toronto"); !errors.Is(e, ErrEmployeeLocalTime) {
			t.Fatal("invalid or ambiguous local time", v, e)
		}
	}
	if got, e := employeeLocalStart("2026-10-24T10:00", "America/Toronto"); e != nil || got.Format(time.RFC3339) != "2026-10-24T14:00:00Z" {
		t.Fatal(got, e)
	}
}
func verifyEmployeeWalkIn(t *testing.T, ctx context.Context, s *Store, owner Scope, actor, dealer string) {
	t.Helper()
	grant := func(principal, action string) {
		t.Helper()
		if _, e := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, principal, owner.OrganizationID, owner.LocationID, action); e != nil {
			t.Fatal(e)
		}
	}
	for _, action := range []string{"scheduling.manage", "catalogue.manage"} {
		grant(owner.ActorID, action)
	}
	grant(actor, "scheduling.reserve")
	z, _ := time.LoadLocation("America/Toronto")
	n := time.Now().In(z).AddDate(0, 0, 3)
	start := time.Date(n.Year(), n.Month(), n.Day(), 10, 0, 0, 0, z)
	resource, e := s.CreateSchedulingResource(ctx, owner, SchedulingResource{Name: "Synthetic employee walk-in bay", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}}}})
	if e != nil {
		t.Fatal(e)
	}
	main, e := s.SaveService(ctx, owner, CatalogService{Name: "Synthetic employee detail", Kind: "main"})
	if e != nil {
		t.Fatal(e)
	}
	rule, e := s.SaveSchedulingRule(ctx, owner, SchedulingRule{ResourceID: resource.ID, ServiceID: main.ID, DurationMinutes: 60, Units: 1})
	if e != nil {
		t.Fatal(e)
	}
	input := EmployeeWalkInInput{RequestKey: "synthetic-walkin-0001", Name: "Synthetic walk-in", Email: "employee-walkin@example.invalid", Phone: "+1 (416) 555-0100", ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: rule.Version, LocalStart: start.Format("2006-01-02T15:04")}
	if _, e = s.CreateEmployeeWalkIn(ctx, actor, input); !errors.Is(e, ErrForbidden) {
		t.Fatal("customer grant inferred", e)
	}
	grant(actor, "customers.create")
	options, e := s.EmployeeWalkInOptions(ctx, actor)
	if e != nil || len(options.Rules) == 0 {
		t.Fatal("scoped choices", e)
	}
	type result struct {
		b Booking
		e error
	}
	ch := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() { b, e := s.CreateEmployeeWalkIn(ctx, actor, input); ch <- result{b, e} }()
	}
	a, b := <-ch, <-ch
	if a.e != nil || b.e != nil || a.b.ID == "" || a.b.ID != b.b.ID || a.b.ReservationID == nil || a.b.Status != "confirmed" {
		t.Fatal("atomic retry", a, b)
	}
	changed := input
	changed.Email = "other@example.invalid"
	if _, e = s.CreateEmployeeWalkIn(ctx, actor, changed); !errors.Is(e, ErrBookingCapacityRequest) {
		t.Fatal("payload rebound", e)
	}
	changed = input
	changed.RequestKey = "synthetic-walkin-conflict"
	if _, e = s.CreateEmployeeWalkIn(ctx, actor, changed); !errors.Is(e, ErrReservationConflict) {
		t.Fatal("capacity doublebooked", e)
	}
	var customers int
	if e = s.pool.QueryRow(ctx, `SELECT count(*) FROM customers WHERE organization_id=$1 AND email='employee-walkin@example.invalid'`, owner.OrganizationID).Scan(&customers); e != nil || customers != 1 {
		t.Fatal("retry/conflict orphan customer", customers, e)
	}
	calendar, e := s.EmployeeCalendarBookings(ctx, actor, start.Format("2006-01-02"), start.AddDate(0, 0, 1).Format("2006-01-02"))
	if e != nil || len(calendar.Items) != 1 || calendar.Items[0].ID != a.b.ID {
		t.Fatal("calendar persisted dealership linkage", calendar, e)
	}
	if _, e = s.EmployeeCalendarBookings(ctx, actor, "2026-01-01", "2026-03-01"); !errors.Is(e, ErrEmployeeInput) {
		t.Fatal("unbounded calendar", e)
	}
	if _, e = s.pool.Exec(ctx, `CREATE FUNCTION reject_walkin_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic walkin rollback'; END $$; CREATE TRIGGER reject_walkin_audit BEFORE INSERT ON employee_workspace_events FOR EACH ROW EXECUTE FUNCTION reject_walkin_audit()`); e != nil {
		t.Fatal(e)
	}
	changed = input
	changed.RequestKey = "synthetic-walkin-rollback"
	changed.LocalStart = start.Add(3 * time.Hour).Format("2006-01-02T15:04")
	changed.Email = "rollback@example.invalid"
	if _, e = s.CreateEmployeeWalkIn(ctx, actor, changed); e == nil {
		t.Fatal("audit failed open")
	}
	var count int
	query := `SELECT count(*) FROM customers WHERE email='rollback@example.invalid'`
	if e = s.pool.QueryRow(ctx, query).Scan(&count); e != nil || count != 0 {
		t.Fatal("customer rollback", count, e)
	}
	if _, e = s.pool.Exec(ctx, `DROP TRIGGER reject_walkin_audit ON employee_workspace_events`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.CreateEmployeeWalkIn(ctx, actor, changed); e != nil {
		t.Fatal("allocation/receipt rollback prevented retry", e)
	}
}
