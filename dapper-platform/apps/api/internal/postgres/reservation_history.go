package postgres

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

func reservationRevision(ctx context.Context, tx pgx.Tx, scope Scope, item CapacityReservation) error {
	snapshot, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO capacity_reservation_revisions(organization_id,location_id,reservation_id,version,snapshot,actor_id) VALUES($1,$2,$3,$4,$5,$6)`, scope.OrganizationID, scope.LocationID, item.ID, item.Version, snapshot, scope.ActorID)
	return err
}
