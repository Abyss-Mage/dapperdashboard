package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type JobContributor struct {
	PrincipalID string    `json:"principalId"`
	StartedAt   time.Time `json:"startedAt"`
}

// ContributeJob records only the verified actor choosing available work.
// Repeating a contribution is idempotent; completed work cannot gain contributors.
func (s *Store) ContributeJob(ctx context.Context, scope Scope, id string) (JobContributor, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return JobContributor{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "jobs.contribute"); err != nil {
		return JobContributor{}, err
	}
	var job string
	err = tx.QueryRow(ctx, `SELECT id::text FROM jobs WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND status='available' FOR UPDATE`, scope.OrganizationID, scope.LocationID, id).Scan(&job)
	if errors.Is(err, pgx.ErrNoRows) {
		return JobContributor{}, ErrJobUnavailable
	}
	if err != nil {
		return JobContributor{}, err
	}
	result, inserted, err := insertJobContributor(ctx, tx, scope, id)
	if err != nil {
		return JobContributor{}, err
	}
	if inserted {
		if err = jobAudit(ctx, tx, scope, "job.contributed", id); err != nil {
			return JobContributor{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return JobContributor{}, err
	}
	return result, nil
}
func insertJobContributor(ctx context.Context, tx pgx.Tx, scope Scope, id string) (JobContributor, bool, error) {
	result := JobContributor{PrincipalID: scope.ActorID}
	err := tx.QueryRow(ctx, `INSERT INTO job_contributors(organization_id,location_id,job_id,principal_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING started_at`, scope.OrganizationID, scope.LocationID, id, scope.ActorID).Scan(&result.StartedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT started_at FROM job_contributors WHERE organization_id=$1 AND location_id=$2 AND job_id=$3 AND principal_id=$4`, scope.OrganizationID, scope.LocationID, id, scope.ActorID).Scan(&result.StartedAt)
		return result, false, err
	}
	return result, true, err
}
func loadJobContributors(ctx context.Context, tx pgx.Tx, scope Scope, jobs []Job) error {
	indices := map[string]int{}
	ids := []string{}
	for i := range jobs {
		indices[jobs[i].ID] = i
		ids = append(ids, jobs[i].ID)
		jobs[i].Contributors = []JobContributor{}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT job_id::text,principal_id::text,started_at FROM job_contributors WHERE organization_id=$1 AND location_id=$2 AND job_id::text=ANY($3::text[]) ORDER BY started_at,principal_id`, scope.OrganizationID, scope.LocationID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var item JobContributor
		if err = rows.Scan(&id, &item.PrincipalID, &item.StartedAt); err != nil {
			return err
		}
		index := indices[id]
		jobs[index].Contributors = append(jobs[index].Contributors, item)
	}
	return rows.Err()
}
