package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"unicode/utf8"
)

var ErrServiceInput = errors.New("valid service name, kind and active main parent required")
var ErrServiceConflict = errors.New("service changed, unavailable or has active add-ons")

type CatalogService struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Kind             string  `json:"kind"`
	ParentID         *string `json:"parentId"`
	// ParentIDs are every main service an add-on is offered with (ParentID is the primary one). Empty for a main service.
	ParentIDs []string `json:"parentIds"`
	Archived         bool    `json:"archived"`
	Version          int     `json:"version"`
	CentralServiceID *string `json:"centralServiceId"`
	PriceCents       *int64  `json:"priceCents"` // CAD cents before tax; comes from the central service when linked
	// DurationMinutes is how long the service takes to complete (nil = not set); comes from the central service when linked.
	DurationMinutes *int `json:"durationMinutes"`
}

const serviceColumns = `id::text,name,kind,parent_id::text,archived,version,central_service_id::text,price_cents,duration_minutes`

func scanService(row pgx.Row) (CatalogService, error) {
	var service CatalogService
	err := row.Scan(&service.ID, &service.Name, &service.Kind, &service.ParentID, &service.Archived, &service.Version, &service.CentralServiceID, &service.PriceCents, &service.DurationMinutes)
	return service, err
}
func (s *Store) Services(ctx context.Context, scope Scope) ([]CatalogService, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "catalogue.read"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+serviceColumns+` FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND merged_into IS NULL ORDER BY name,id LIMIT 1000`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	result := []CatalogService{}
	for rows.Next() {
		service, err := scanService(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, service)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	links, err := localAddonLinks(ctx, tx, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	for i := range result {
		if result[i].Kind == "addon" {
			result[i].ParentIDs = links[result[i].ID]
			if len(result[i].ParentIDs) == 0 && result[i].ParentID != nil {
				result[i].ParentIDs = []string{*result[i].ParentID}
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// SaveService preserves every revision; IDs/version/archived cannot be used to
// bypass scope. Prices and duration approval are not inferred from source data.
func (s *Store) SaveService(ctx context.Context, scope Scope, input CatalogService) (CatalogService, error) {
	input.Name = strings.TrimSpace(input.Name)
	if n := utf8.RuneCountInString(input.Name); n < 2 || n > 120 {
		return CatalogService{}, ErrServiceInput
	}
	if input.DurationMinutes != nil && (*input.DurationMinutes < 1 || *input.DurationMinutes > 10080) {
		return CatalogService{}, ErrServiceInput
	}
	if input.Kind != "main" && input.Kind != "addon" || input.Kind == "main" && (input.ParentID != nil || len(input.ParentIDs) > 0) {
		return CatalogService{}, ErrServiceInput
	}
	if input.Kind == "addon" {
		ids := input.ParentIDs
		if len(ids) == 0 && input.ParentID != nil {
			ids = []string{*input.ParentID}
		}
		set, ok := uniqueParents(ids, input.ID)
		if !ok {
			return CatalogService{}, ErrServiceInput
		}
		input.ParentIDs = set
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CatalogService{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "catalogue.manage"); err != nil {
		return CatalogService{}, err
	}
	var currentParent *string
	if input.ID != "" {
		var version int
		err = tx.QueryRow(ctx, `SELECT version,parent_id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND NOT archived FOR UPDATE`, scope.OrganizationID, scope.LocationID, input.ID).Scan(&version, &currentParent)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && version != input.Version {
			return CatalogService{}, ErrServiceConflict
		}
		if err != nil {
			return CatalogService{}, err
		}
		var linked bool
		if err = tx.QueryRow(ctx, `SELECT central_service_id IS NOT NULL FROM catalog_services WHERE id=$1`, input.ID).Scan(&linked); err != nil {
			return CatalogService{}, err
		}
		if linked {
			return CatalogService{}, ErrServiceConflict
		}
		if input.Kind != "main" {
			var children bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_services WHERE parent_id=$1 AND NOT archived)`, input.ID).Scan(&children)
			if err != nil {
				return CatalogService{}, err
			}
			if children {
				return CatalogService{}, ErrServiceConflict
			}
		}
	}
	input.ParentID = nil
	if input.Kind == "addon" {
		primary := primaryParent(currentParent, input.ParentIDs)
		input.ParentID = &primary
	}
	if input.ParentID != nil {
		var parent string
		err = tx.QueryRow(ctx, `SELECT id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='main' AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, *input.ParentID).Scan(&parent)
		if errors.Is(err, pgx.ErrNoRows) {
			return CatalogService{}, ErrServiceInput
		}
		if err != nil {
			return CatalogService{}, err
		}
	}
	var service CatalogService
	if input.ID == "" {
		service, err = scanService(tx.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind,parent_id,duration_minutes) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+serviceColumns, scope.OrganizationID, scope.LocationID, input.Name, input.Kind, input.ParentID, input.DurationMinutes))
	} else {
		service, err = scanService(tx.QueryRow(ctx, `UPDATE catalog_services SET name=$4,kind=$5,parent_id=$6,duration_minutes=$7,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND id=$3 RETURNING `+serviceColumns, scope.OrganizationID, scope.LocationID, input.ID, input.Name, input.Kind, input.ParentID, input.DurationMinutes))
	}
	if err != nil {
		return CatalogService{}, err
	}
	if service.Kind == "addon" {
		if err = setLocalAddonLinks(ctx, tx, scope, service.ID, input.ParentIDs); err != nil {
			return CatalogService{}, err
		}
		service.ParentIDs = input.ParentIDs
	} else if _, err = tx.Exec(ctx, `DELETE FROM catalog_addon_parents WHERE addon_id=$1`, service.ID); err != nil {
		return CatalogService{}, err
	}
	if err = serviceRevision(ctx, tx, scope, service); err != nil {
		return CatalogService{}, err
	}
	if err = syncServiceDurationRules(ctx, tx, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return CatalogService{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CatalogService{}, err
	}
	return service, nil
}
func serviceRevision(ctx context.Context, tx pgx.Tx, scope Scope, service CatalogService) error {
	snapshot, err := json.Marshal(service)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES($1,$2,$3,$4)`, service.ID, service.Version, scope.ActorID, snapshot)
	if err != nil {
		return err
	}
	return jobAudit(ctx, tx, scope, "service.revised", service.ID)
}
func (s *Store) ArchiveService(ctx context.Context, scope Scope, id string, version int) (CatalogService, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CatalogService{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "catalogue.manage"); err != nil {
		return CatalogService{}, err
	}
	current, err := scanService(tx.QueryRow(ctx, `SELECT `+serviceColumns+` FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (current.Archived || current.Version != version) {
		return CatalogService{}, ErrServiceConflict
	}
	if err != nil {
		return CatalogService{}, err
	}
	if err = releaseLocalMain(ctx, tx, scope, id); err != nil {
		return CatalogService{}, err
	}
	var children bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_services WHERE parent_id=$1 AND NOT archived)`, id).Scan(&children)
	if err != nil {
		return CatalogService{}, err
	}
	if children {
		return CatalogService{}, ErrServiceConflict
	}
	current, err = scanService(tx.QueryRow(ctx, `UPDATE catalog_services SET archived=true,version=version+1 WHERE id=$1 RETURNING `+serviceColumns, id))
	if err != nil {
		return CatalogService{}, err
	}
	if err = serviceRevision(ctx, tx, scope, current); err != nil {
		return CatalogService{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CatalogService{}, err
	}
	return current, nil
}

func (s *Store) DeleteService(ctx context.Context, scope Scope, id string, version int) (DeleteResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeleteResult{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "catalogue.manage"); err != nil {
		return DeleteResult{}, err
	}
	current, err := scanService(tx.QueryRow(ctx, `SELECT `+serviceColumns+` FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id))
	if errors.Is(err, pgx.ErrNoRows) || (version > 0 && current.Version != version) {
		return DeleteResult{}, ErrServiceConflict
	}
	if err != nil {
		return DeleteResult{}, err
	}
	if err = releaseLocalMain(ctx, tx, scope, id); err != nil {
		return DeleteResult{}, err
	}
	var children bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_services WHERE parent_id=$1 AND NOT archived)`, id).Scan(&children)
	if err != nil {
		return DeleteResult{}, err
	}
	if children {
		return DeleteResult{}, ErrServiceConflict
	}

	var referenced bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM service_publications WHERE organization_id=$1 AND location_id=$2 AND service_id=$3
		UNION ALL
		SELECT 1 FROM jobs WHERE organization_id=$1 AND location_id=$2 AND service_id=$3
		UNION ALL
		SELECT 1 FROM bookings WHERE organization_id=$1 AND location_id=$2 AND service_id=$3
		UNION ALL
		SELECT 1 FROM scoped_quotation_items qi
		JOIN scoped_quotations q ON q.id=qi.quotation_id
		WHERE q.organization_id=$1 AND q.location_id=$2 AND qi.service_id=$3
	)`, scope.OrganizationID, scope.LocationID, id).Scan(&referenced)
	if err != nil {
		return DeleteResult{}, err
	}

	if referenced {
		current, err = scanService(tx.QueryRow(ctx, `UPDATE catalog_services SET archived=true,version=version+1 WHERE id=$1 RETURNING `+serviceColumns, id))
		if err != nil {
			return DeleteResult{}, err
		}
		if err = serviceRevision(ctx, tx, scope, current); err != nil {
			return DeleteResult{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return DeleteResult{}, err
		}
		return DeleteResult{Permanent: false, Archived: true, Message: "Service removed from active catalogue; historical records and revisions retained."}, nil
	}

	if _, err = tx.Exec(ctx, `DELETE FROM catalog_service_revisions WHERE service_id=$1`, id); err != nil {
		return DeleteResult{}, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, id)
	if err != nil {
		return DeleteResult{}, err
	}
	if tag.RowsAffected() == 0 {
		return DeleteResult{}, ErrServiceConflict
	}
	if err = jobAudit(ctx, tx, scope, "service.permanently_deleted", id); err != nil {
		return DeleteResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{Permanent: true, Archived: false, Message: "Service permanently deleted from catalogue."}, nil
}
