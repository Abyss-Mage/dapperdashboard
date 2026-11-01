package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrJobInput = errors.New("valid customer, service and job description required")
var ErrJobUnavailable = errors.New("job unavailable or already completed")
var ErrJobAddons = errors.New("active add-ons must belong to the selected main service")

type JobAddon struct {
	ServiceID string `json:"serviceId"`
	Name      string `json:"name"`
}

type Job struct {
	ID            string           `json:"id"`
	CustomerID    string           `json:"customerId"`
	ServiceName   string           `json:"serviceName"`
	ServiceID     *string          `json:"serviceId"`
	Description   string           `json:"description"`
	Status        string           `json:"status"`
	CompletedBy   *string          `json:"completedBy"`
	CompletedAt   *time.Time       `json:"completedAt"`
	Addons        []JobAddon       `json:"addons"`
	AddonIDs      []string         `json:"-"`
	ReservationID *string          `json:"reservationId,omitempty"`
	BookingID     *string          `json:"bookingId,omitempty"`
	Contributors  []JobContributor `json:"contributors"`
}

// Lock the explicit grant or scoped role assignment and active principal until the mutation commits.
// Concurrent grant deletion/role revocation/account disable serializes rather than racing it.
func authorizeTransaction(ctx context.Context, tx pgx.Tx, scope Scope, action string) error {
	if err := scope.validate(); err != nil {
		return err
	}

	// 1. Lock the active principal row FOR SHARE so concurrent deactivation
	// (UPDATE principals SET active=false ... FOR UPDATE) will block and serialize.
	var principalActive bool
	err := tx.QueryRow(ctx, `SELECT active FROM principals WHERE id=$1 FOR SHARE`, scope.ActorID).Scan(&principalActive)
	if errors.Is(err, pgx.ErrNoRows) || !principalActive {
		return ErrForbidden
	}
	if err != nil {
		return err
	}

	// 2. Lock the granting authority row FOR SHARE so concurrent grant deletion / assignment revocation blocks.
	// Check direct location grant:
	var dummy int
	err = tx.QueryRow(ctx, `
		SELECT 1 FROM location_grants g
		WHERE g.principal_id=$1 AND g.organization_id=$2 AND g.location_id=$3 AND g.action=$4
		FOR SHARE OF g
	`, scope.ActorID, scope.OrganizationID, scope.LocationID, action).Scan(&dummy)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Check location-scoped role assignment:
	var assignID string
	err = tx.QueryRow(ctx, `
		SELECT a.id::text FROM member_role_assignments a
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE a.principal_id=$1 AND a.organization_id=$2
		  AND a.scope_type='location' AND a.location_id=$3 AND rc.action=$4
		FOR SHARE OF a
		LIMIT 1
	`, scope.ActorID, scope.OrganizationID, scope.LocationID, action).Scan(&assignID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Check franchise-scoped role assignment:
	err = tx.QueryRow(ctx, `
		SELECT a.id::text FROM member_role_assignments a
		JOIN locations l ON l.organization_id=a.organization_id AND l.id=$3
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE a.principal_id=$1 AND a.organization_id=$2
		  AND a.scope_type='franchise' AND a.franchise_id=l.franchise_id AND rc.action=$4
		FOR SHARE OF a
		LIMIT 1
	`, scope.ActorID, scope.OrganizationID, scope.LocationID, action).Scan(&assignID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Check organization-scoped role assignment:
	err = tx.QueryRow(ctx, `
		SELECT a.id::text FROM member_role_assignments a
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE a.principal_id=$1 AND a.organization_id=$2
		  AND a.scope_type='organization' AND a.scope_id=$2 AND rc.action=$3
		FOR SHARE OF a
		LIMIT 1
	`, scope.ActorID, scope.OrganizationID, action).Scan(&assignID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	return ErrForbidden
}
func scanJob(row pgx.Row) (Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.CustomerID, &j.ServiceName, &j.Description, &j.Status, &j.CompletedBy, &j.CompletedAt, &j.ServiceID, &j.Addons, &j.ReservationID, &j.BookingID)
	return j, err
}

const jobColumns = `id::text,customer_id::text,service_name,description,status,completed_by::text,completed_at,service_id::text,addons,reservation_id::text,booking_id::text`

func (s *Store) CreateJob(ctx context.Context, scope Scope, input Job) (Job, error) {
	input.ServiceName = strings.TrimSpace(input.ServiceName)
	input.Description = strings.TrimSpace(input.Description)
	if input.CustomerID == "" || input.ServiceID == nil && (utf8.RuneCountInString(input.ServiceName) < 2 || utf8.RuneCountInString(input.ServiceName) > 120) || utf8.RuneCountInString(input.Description) > 2000 {
		return Job{}, ErrJobInput
	}
	if len(input.AddonIDs) > 100 || len(input.AddonIDs) > 0 && input.ServiceID == nil {
		return Job{}, ErrJobAddons
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "jobs.create"); err != nil {
		return Job{}, err
	}
	if input.ReservationID != nil {
		var resID string
		err = tx.QueryRow(ctx, `SELECT id::text FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND cancelled_at IS NULL FOR SHARE`, scope.OrganizationID, scope.LocationID, *input.ReservationID).Scan(&resID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrJobInput
		}
		if err != nil {
			return Job{}, err
		}
	}
	if input.ServiceID != nil {
		err = tx.QueryRow(ctx, `SELECT name FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='main' AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, *input.ServiceID).Scan(&input.ServiceName)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT name FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND kind='main' AND NOT archived FOR SHARE`, scope.OrganizationID, *input.ServiceID).Scan(&input.ServiceName)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrJobInput
		}
		if err != nil {
			return Job{}, err
		}
	}
	// Stable row-lock order prevents two selections from locking children in
	// opposite order. All names come from scoped catalogue rows, never input.
	ids := append([]string(nil), input.AddonIDs...)
	sort.Strings(ids)
	addons := []JobAddon{}
	for i, id := range ids {
		if i > 0 && id == ids[i-1] {
			return Job{}, ErrJobAddons
		}
		var addon JobAddon
		err = tx.QueryRow(ctx, `SELECT id::text,name FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='addon' AND catalog_addon_offered_with(id,$4) AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, id, *input.ServiceID).Scan(&addon.ServiceID, &addon.Name)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT id::text,name FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND kind='addon' AND central_addon_offered_with(id,$3) AND NOT archived FOR SHARE`, scope.OrganizationID, id, *input.ServiceID).Scan(&addon.ServiceID, &addon.Name)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrJobAddons
		}
		if err != nil {
			return Job{}, err
		}
		addons = append(addons, addon)
	}
	job, err := scanJob(tx.QueryRow(ctx, `INSERT INTO jobs(organization_id,location_id,customer_id,service_name,description,service_id,addons,reservation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+jobColumns, scope.OrganizationID, scope.LocationID, input.CustomerID, input.ServiceName, input.Description, input.ServiceID, addons, input.ReservationID))
	if err != nil {
		return Job{}, err
	}
	if err = jobAudit(ctx, tx, scope, "job.created", job.ID); err != nil {
		return Job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return job, nil
}
func jobAudit(ctx context.Context, tx pgx.Tx, scope Scope, action, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4,$5)`, scope.OrganizationID, scope.LocationID, scope.ActorID, action, id)
	return err
}
func (s *Store) Jobs(ctx context.Context, scope Scope) ([]Job, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "jobs.read"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+jobColumns+` FROM jobs WHERE organization_id=$1 AND location_id=$2 ORDER BY created_at,id LIMIT 100`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	jobs := []Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		jobs = append(jobs, job)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = loadJobContributors(ctx, tx, scope, jobs); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return jobs, nil
}
func (s *Store) CompleteJob(ctx context.Context, scope Scope, id string) (Job, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "jobs.complete"); err != nil {
		return Job{}, err
	}
	job, err := scanJob(tx.QueryRow(ctx, `UPDATE jobs SET status='completed',completed_by=$4,completed_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND status='available' RETURNING `+jobColumns, scope.OrganizationID, scope.LocationID, id, scope.ActorID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobUnavailable
	}
	if err != nil {
		return Job{}, err
	}
	if _, _, err = insertJobContributor(ctx, tx, scope, id); err != nil {
		return Job{}, err
	}
	completed := []Job{job}
	if err = loadJobContributors(ctx, tx, scope, completed); err != nil {
		return Job{}, err
	}
	job = completed[0]
	if err = jobAudit(ctx, tx, scope, "job.completed", id); err != nil {
		return Job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return job, nil
}
