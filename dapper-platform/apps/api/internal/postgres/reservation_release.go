package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

// ReleaseCapacity preserves a cancellation receipt and frees technical capacity.
// A future booking adapter must separately enforce approved cancellation and
// refund eligibility. Possessing scheduling.reserve does not grant release.
func (s *Store) ReleaseCapacity(ctx context.Context, scope Scope, resourceID, id string, version int) (CapacityReservation, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return CapacityReservation{}, err
	}
	defer tx.Rollback(ctx)
	result, err := s.releaseCapacityInTransaction(ctx, tx, scope, resourceID, id, version)
	if err != nil {
		return CapacityReservation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CapacityReservation{}, reservationCommitError(err)
	}
	return result, nil
}

func (s *Store) releaseCapacityInTransaction(ctx context.Context, tx pgx.Tx, scope Scope, resourceID, id string, version int) (CapacityReservation, error) {
	if resourceID == "" || id == "" || version < 1 {
		return CapacityReservation{}, ErrReservationInput
	}
	var err error
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.release"); err != nil {
		return CapacityReservation{}, err
	}
	var locked string
	err = tx.QueryRow(ctx, `SELECT id::text FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, resourceID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return CapacityReservation{}, ErrReservationConflict
	}
	if err != nil {
		return CapacityReservation{}, err
	}
	result := CapacityReservation{ResourceID: resourceID}
	err = tx.QueryRow(ctx, `UPDATE capacity_reservations SET cancelled_at=now(),cancelled_by=$6,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND id=$4 AND version=$5 AND cancelled_at IS NULL RETURNING id::text,request,occupied_start,occupied_end,version,cancelled_at,cancelled_by::text,rule_snapshot`, scope.OrganizationID, scope.LocationID, resourceID, id, version, scope.ActorID).Scan(&result.ID, &result.Request, &result.Occupied.Start, &result.Occupied.End, &result.Version, &result.CancelledAt, &result.CancelledBy, &result.Rule)
	if errors.Is(err, pgx.ErrNoRows) {
		return CapacityReservation{}, ErrReservationConflict
	}
	if err != nil {
		return CapacityReservation{}, err
	}
	if err = reservationRevision(ctx, tx, scope, result); err != nil {
		return CapacityReservation{}, err
	}
	if err = jobAudit(ctx, tx, scope, "scheduling.released", id); err != nil {
		return CapacityReservation{}, err
	}
	return result, nil
}
