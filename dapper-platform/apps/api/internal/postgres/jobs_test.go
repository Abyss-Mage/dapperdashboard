package postgres

import (
	"context"
	"sync"
	"testing"
)

func verifyJobPool(t *testing.T, ctx context.Context, s *Store, a, b Scope, customerID string) {
	t.Helper()
	for _, scope := range []Scope{a, b} {
		for _, action := range []string{"jobs.read", "jobs.create", "jobs.complete"} {
			if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, scope.ActorID, scope.OrganizationID, scope.LocationID, action); err != nil {
				t.Fatal(err)
			}
		}
	}
	job, err := s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceName: "Synthetic main detail"})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "available" || job.CompletedBy != nil {
		t.Fatal("new job preassigned")
	}
	if _, err = s.CreateJob(ctx, b, Job{CustomerID: customerID, ServiceName: "Foreign owner"}); err == nil {
		t.Fatal("cross-scope customer accepted")
	}
	if items, err := s.Jobs(ctx, b); err != nil || len(items) != 0 {
		t.Fatal("cross-scope list leaked")
	}
	if _, err = s.CompleteJob(ctx, b, job.ID); err != ErrJobUnavailable {
		t.Fatal("cross-scope completion accepted")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.CompleteJob(ctx, a, job.ID); results <- err }()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if err == ErrJobUnavailable {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("completion not exclusive")
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE entity_id=$1 AND actor_id=$2 AND action='job.completed'`, job.ID, a.ActorID).Scan(&count); err != nil || count != 1 {
		t.Fatal("completion audit mismatch")
	}
	jobs, err := s.Jobs(ctx, a)
	if err != nil || len(jobs) != 1 || jobs[0].CompletedBy == nil || *jobs[0].CompletedBy != a.ActorID {
		t.Fatal("performer not persisted")
	}
	another, err := s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceName: "Audit rollback fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_job_audit_failure CHECK(action<>'job.completed') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteJob(ctx, a, another.ID); err == nil {
		t.Fatal("audit failure committed completion")
	}
	var status string
	if err = s.pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id=$1`, another.ID).Scan(&status); err != nil || status != "available" {
		t.Fatal("audit failure did not roll back job")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_job_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='jobs.complete'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteJob(ctx, a, another.ID); err != ErrForbidden {
		t.Fatal("revoked completion grant retained access")
	}
}
