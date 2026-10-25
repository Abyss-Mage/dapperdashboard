package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ImportCentralService explicitly enables an organization service at one location.
// It does not infer mappings for old records or grant central editing authority.
func (s *Store) ImportCentralService(ctx context.Context, scope Scope, centralID string, version int) (CatalogService, error) {
	if centralID == "" || version < 1 {
		return CatalogService{}, ErrServiceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CatalogService{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "catalogue.manage"); err != nil {
		return CatalogService{}, err
	}
	if err = centralCatalogueGrant(ctx, tx, scope.ActorID, scope.OrganizationID, "catalogue.read"); err != nil {
		return CatalogService{}, err
	}
	var name, kind string
	var centralParent, parent *string
	var price *int64
	err = tx.QueryRow(ctx, `SELECT name,kind,parent_id::text,price_cents FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND version=$3 AND NOT archived FOR SHARE`, scope.OrganizationID, centralID, version).Scan(&name, &kind, &centralParent, &price)
	if errors.Is(err, pgx.ErrNoRows) {
		return CatalogService{}, ErrServiceConflict
	}
	if err != nil {
		return CatalogService{}, err
	}
	if centralParent != nil {
		var id string
		err = tx.QueryRow(ctx, `SELECT id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND central_service_id=$3 AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, *centralParent).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return CatalogService{}, ErrServiceInput
		}
		if err != nil {
			return CatalogService{}, err
		}
		parent = &id
	}
	service, err := scanService(tx.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind,parent_id,central_service_id,price_cents) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING RETURNING `+serviceColumns, scope.OrganizationID, scope.LocationID, name, kind, parent, centralID, price))
	if errors.Is(err, pgx.ErrNoRows) {
		// Central services are switched on at every franchise automatically, so it may already be here: return that one.
		existing, e := scanService(tx.QueryRow(ctx, `SELECT `+serviceColumns+` FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND central_service_id=$3 AND NOT archived`, scope.OrganizationID, scope.LocationID, centralID))
		if e != nil {
			return CatalogService{}, ErrServiceConflict
		}
		return existing, tx.Commit(ctx)
	}
	if err != nil {
		return service, err
	}
	if err = serviceRevision(ctx, tx, scope, service); err != nil {
		return service, err
	}
	return service, tx.Commit(ctx)
}
func centralLinkFailure(err error) error {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		switch pgerr.ConstraintName {
		case "central_link_identity":
			return ErrCentralServiceConflict
		case "central_link_children":
			return ErrCentralServiceHasChildren
		}
	}
	return err
}
