package postgres

import (
	"context"
	"testing"
)

func verifyJobContributors(t *testing.T, ctx context.Context, s *Store, a, b Scope, customerID string) {
	t.Helper()
	var colleagueID string
	if err := s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:contributor') RETURNING id::text`).Scan(&colleagueID); err != nil {
		t.Fatal(err)
	}
	colleague := Scope{OrganizationID: a.OrganizationID, LocationID: a.LocationID, ActorID: colleagueID}
	for _, scope := range []Scope{a, b, colleague} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'jobs.contribute')`, scope.ActorID, scope.OrganizationID, scope.LocationID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'jobs.complete') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	job, err := s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceName: "Synthetic shared work"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.ContributeJob(ctx, a, job.ID)
	if err != nil || first.PrincipalID != a.ActorID {
		t.Fatal("session contributor missing", err)
	}
	again, err := s.ContributeJob(ctx, a, job.ID)
	if err != nil || !again.StartedAt.Equal(first.StartedAt) {
		t.Fatal("contribution retry not idempotent", err)
	}
	if _, err = s.ContributeJob(ctx, b, job.ID); err != ErrJobUnavailable {
		t.Fatal("foreign job contribution accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_contribution_audit_failure CHECK(action<>'job.contributed') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ContributeJob(ctx, colleague, job.ID); err == nil {
		t.Fatal("contribution committed without audit")
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM job_contributors WHERE job_id=$1`, job.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("failed contribution audit left roster row", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_contribution_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ContributeJob(ctx, colleague, job.ID); err != nil {
		t.Fatal(err)
	}
	completed, err := s.CompleteJob(ctx, a, job.ID)
	if err != nil || completed.Status != "completed" || len(completed.Contributors) != 2 {
		t.Fatal("multi-contributor immediate completion failed", err)
	}
	if _, err = s.ContributeJob(ctx, colleague, job.ID); err != ErrJobUnavailable {
		t.Fatal("completed roster changed", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE entity_id=$1 AND action='job.contributed'`, job.ID).Scan(&count); err != nil || count != 2 {
		t.Fatal("duplicate/missing contribution audit", err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='jobs.contribute'`, colleagueID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ContributeJob(ctx, colleague, job.ID); err != ErrForbidden {
		t.Fatal("revoked contributor grant accepted", err)
	}
}
