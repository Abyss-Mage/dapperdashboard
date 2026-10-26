package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"testing"
	"time"
)

func verifySchedulingRules(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	var main, child string
	if err := s.pool.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind) VALUES($1,$2,'Synthetic rule main','main') RETURNING id::text`, a.OrganizationID, a.LocationID).Scan(&main); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind,parent_id) VALUES($1,$2,'Synthetic rule add-on','addon',$3) RETURNING id::text`, a.OrganizationID, a.LocationID, main).Scan(&child); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	resource, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Synthetic rule resource", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 2, Open: []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}}}})
	if err != nil {
		t.Fatal(err)
	}
	input := SchedulingRule{ResourceID: resource.ID, ServiceID: main, AddonIDs: []string{child}, DurationMinutes: 60, BeforeMinutes: 10, AfterMinutes: 15, Units: 1}
	rule, err := s.SaveSchedulingRule(ctx, a, input)
	if err != nil || rule.Version != 1 || len(rule.AddonIDs) != 1 {
		t.Fatal("explicit combination rule failed", err)
	}
	if _, err = s.SaveSchedulingRule(ctx, a, input); err != ErrSchedulingRuleConflict {
		t.Fatal("duplicate combination accepted", err)
	}
	if _, err = s.SaveSchedulingRule(ctx, b, input); err != ErrSchedulingRule {
		t.Fatal("foreign resource rule accepted", err)
	}
	duplicate := input
	duplicate.AddonIDs = []string{child, child}
	if _, err = s.SaveSchedulingRule(ctx, a, duplicate); err != ErrSchedulingRule {
		t.Fatal("duplicate add-ons accepted", err)
	}
	standalone := input
	standalone.ServiceID = child
	standalone.AddonIDs = nil
	if _, err = s.SaveSchedulingRule(ctx, a, standalone); err != ErrSchedulingRule {
		t.Fatal("standalone add-on rule accepted", err)
	}
	tooMany := input
	tooMany.Units = 3
	if _, err = s.SaveSchedulingRule(ctx, a, tooMany); err != ErrSchedulingRule {
		t.Fatal("rule exceeds resource capacity", err)
	}
	edited := rule
	edited.DurationMinutes = 90
	updated, err := s.SaveSchedulingRule(ctx, a, edited)
	if err != nil || updated.Version != 2 {
		t.Fatal("rule revision failed", err)
	}
	if _, err = s.SaveSchedulingRule(ctx, a, edited); err != ErrSchedulingRuleConflict {
		t.Fatal("stale revision accepted", err)
	}
	for _, action := range []string{"scheduling.reserve", "scheduling.release"} {
		if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID, action); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.ReserveServiceCapacity(ctx, a, resource.ID, rule.ID, 1, start.Add(30*time.Minute)); err != ErrSchedulingRuleConflict {
		t.Fatal("stale rule used for reservation", err)
	}
	reserved, err := s.ReserveServiceCapacity(ctx, a, resource.ID, rule.ID, 2, start.Add(30*time.Minute))
	if err != nil || reserved.Request.Duration != 90*time.Minute || reserved.Request.Before != 10*time.Minute || reserved.Request.After != 15*time.Minute || reserved.Request.Units != 1 || reserved.Rule == nil || reserved.Rule.Version != 2 {
		t.Fatal("server rule timing not applied", err)
	}
	if _, err = s.RescheduleCapacity(ctx, a, resource.ID, reserved.ID, 1, scheduling.Request{Start: start.Add(4 * time.Hour), Duration: time.Minute, Units: 1}); err != ErrReservationInput {
		t.Fatal("generic move bypassed configured rule", err)
	}
	changedIdentity := updated
	changedIdentity.AddonIDs = nil
	if _, err = s.SaveSchedulingRule(ctx, a, changedIdentity); err != ErrSchedulingRuleConflict {
		t.Fatal("rule combination identity silently changed", err)
	}
	var originalDuration, revisions int
	if err = s.pool.QueryRow(ctx, `SELECT (snapshot->>'durationMinutes')::int FROM service_scheduling_revisions WHERE rule_id=$1 AND version=1`, rule.ID).Scan(&originalDuration); err != nil || originalDuration != 60 {
		t.Fatal("historical rule snapshot lost", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_rule_audit_failure CHECK(action<>'scheduling.rule.revised') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	attempted := updated
	attempted.DurationMinutes = 120
	if _, err = s.SaveSchedulingRule(ctx, a, attempted); err == nil {
		t.Fatal("rule committed without audit")
	}
	if err = s.pool.QueryRow(ctx, `SELECT version FROM service_scheduling_rules WHERE id=$1`, rule.ID).Scan(&revisions); err != nil || revisions != 2 {
		t.Fatal("audit failure did not roll back rule", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM service_scheduling_revisions WHERE rule_id=$1`, rule.ID).Scan(&revisions); err != nil || revisions != 2 {
		t.Fatal("audit failure left revision behind", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_rule_audit_failure`); err != nil {
		t.Fatal(err)
	}
	later := updated
	later.DurationMinutes = 120
	newRule, err := s.SaveSchedulingRule(ctx, a, later)
	if err != nil || newRule.Version != 3 {
		t.Fatal("new rule revision failed", err)
	}
	var appliedMinutes int
	if err = s.pool.QueryRow(ctx, `SELECT (rule_snapshot->>'durationMinutes')::int FROM capacity_reservations WHERE id=$1`, reserved.ID).Scan(&appliedMinutes); err != nil || appliedMinutes != 90 {
		t.Fatal("later rule edit changed reservation snapshot", err)
	}
	if _, err = s.RescheduleServiceCapacity(ctx, a, resource.ID, rule.ID, reserved.ID, 2, 1, start.Add(4*time.Hour)); err != ErrSchedulingRuleConflict {
		t.Fatal("stale rule move accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_bound_move_failure CHECK(action<>'scheduling.rescheduled') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RescheduleServiceCapacity(ctx, a, resource.ID, rule.ID, reserved.ID, 3, 1, start.Add(4*time.Hour)); err == nil {
		t.Fatal("rule move committed without audit")
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM capacity_reservation_revisions WHERE reservation_id=$1`, reserved.ID).Scan(&revisions); err != nil || revisions != 1 {
		t.Fatal("failed move left history revision", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_bound_move_failure`); err != nil {
		t.Fatal(err)
	}
	moved, err := s.RescheduleServiceCapacity(ctx, a, resource.ID, rule.ID, reserved.ID, 3, 1, start.Add(4*time.Hour))
	if err != nil || moved.Version != 2 || moved.Rule == nil || moved.Rule.Version != 3 || moved.Request.Duration != 120*time.Minute {
		t.Fatal("rule-aware move did not resolve current timing", err)
	}
	if _, err = s.RescheduleServiceCapacity(ctx, a, resource.ID, rule.ID, reserved.ID, 3, 1, start.Add(3*time.Hour)); err != ErrReservationConflict {
		t.Fatal("stale reservation move accepted", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT (snapshot->'Rule'->>'durationMinutes')::int FROM capacity_reservation_revisions WHERE reservation_id=$1 AND version=1`, reserved.ID).Scan(&appliedMinutes); err != nil || appliedMinutes != 90 {
		t.Fatal("move rewrote prior reservation snapshot", err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE catalog_services SET archived=true WHERE id=$1`, child); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSchedulingRule(ctx, a, updated); err != ErrSchedulingRule {
		t.Fatal("archived combination member accepted", err)
	}
	if _, err = s.ReserveServiceCapacity(ctx, a, resource.ID, rule.ID, 3, start.Add(4*time.Hour)); err != ErrSchedulingRule {
		t.Fatal("archived member rule used for new reservation", err)
	}
	released, err := s.ReleaseCapacity(ctx, a, resource.ID, reserved.ID, 2)
	if err != nil || released.Rule == nil || released.Rule.Version != 3 || released.Rule.DurationMinutes != 120 {
		t.Fatal("release lost applied rule receipt", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM capacity_reservation_revisions WHERE reservation_id=$1`, reserved.ID).Scan(&revisions); err != nil || revisions != 3 {
		t.Fatal("create/move/release history incomplete", err)
	}
}
