package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrBookingJobRequest = errors.New("booking work-order request key conflicts with an earlier request")

// CreateBookingJob snapshots server-owned booking fields atomically. Retrying
// an identical request key returns the original job, including after completion.
// No booking status transition, payment bypass, or invoice is generated here.
func (s *Store) CreateBookingJob(ctx context.Context, scope Scope, bookingID, key, description string) (Job, error) {
	description = strings.TrimSpace(description)
	if bookingID == "" || len(key) < 16 || len(key) > 128 || utf8.RuneCountInString(description) > 2000 {
		return Job{}, ErrJobInput
	}
	fingerprint := sha256.Sum256([]byte(bookingID + "\x00" + description))
	hash := hex.EncodeToString(fingerprint[:])
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "jobs.create"); err != nil {
		return Job{}, err
	}
	if err = authorizeTransaction(ctx, tx, scope, "bookings.read"); err != nil {
		return Job{}, err
	}
	// Serialize duplicate keys, including different bookings submitted with one key.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.OrganizationID+"/"+scope.LocationID+"/"+key); err != nil {
		return Job{}, err
	}
	var existingID, existingHash string
	err = tx.QueryRow(ctx, `SELECT job_id::text,request_hash FROM booking_job_requests WHERE organization_id=$1 AND location_id=$2 AND request_key=$3`, scope.OrganizationID, scope.LocationID, key).Scan(&existingID, &existingHash)
	if err == nil {
		if hash != existingHash {
			return Job{}, ErrBookingJobRequest
		}
		job, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, existingID))
		if err != nil {
			return Job{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Job{}, err
		}
		return job, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, err
	}
	var source Job
	previewReservation, lockErr := lockBookingAllocation(ctx, tx, scope, bookingID)
	if lockErr != nil {
		return Job{}, lockErr
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT customer_id::text,service_id::text,service_name,addons,reservation_id::text,status FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, bookingID).Scan(&source.CustomerID, &source.ServiceID, &source.ServiceName, &source.Addons, &source.ReservationID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrBookingNotFound
	}
	if err != nil {
		return Job{}, err
	}
	if status == "cancelled" || status == "completed" {
		return Job{}, ErrJobUnavailable
	}
	if (previewReservation == nil) != (source.ReservationID == nil) || previewReservation != nil && *previewReservation != *source.ReservationID {
		return Job{}, ErrReservationConflict
	}
	if source.ReservationID != nil {
		var id string
		err = tx.QueryRow(ctx, `SELECT id::text FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND cancelled_at IS NULL FOR SHARE`, scope.OrganizationID, scope.LocationID, *source.ReservationID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrJobInput
		}
		if err != nil {
			return Job{}, err
		}
	}
	job, err := scanJob(tx.QueryRow(ctx, `INSERT INTO jobs(organization_id,location_id,customer_id,service_id,service_name,addons,reservation_id,booking_id,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+jobColumns, scope.OrganizationID, scope.LocationID, source.CustomerID, source.ServiceID, source.ServiceName, source.Addons, source.ReservationID, bookingID, description))
	if err != nil {
		return Job{}, err
	}
	if err = jobAudit(ctx, tx, scope, "job.created", job.ID); err != nil {
		return Job{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_job_requests(organization_id,location_id,request_key,request_hash,booking_id,job_id) VALUES($1,$2,$3,$4,$5,$6)`, scope.OrganizationID, scope.LocationID, key, hash, bookingID, job.ID); err != nil {
		return Job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return job, nil
}
