package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"unicode/utf8"
)

var ErrCentralServiceInput = errors.New("valid central service name, kind and parent required")
var ErrCentralServiceConflict = errors.New("central service edit conflict; current version required")
var ErrCentralServiceHasChildren = errors.New("cannot archive central service while active add-on services reference it")

type CentralService struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	ParentID   *string `json:"parentId"`
	// ParentIDs are every main service an add-on is offered with (ParentID is the primary one). Empty for a main service.
	ParentIDs  []string `json:"parentIds"`
	Archived   bool    `json:"archived"`
	Version    int     `json:"version"`
	CreatedAt  string  `json:"createdAt"`
	PriceCents *int64  `json:"priceCents"` // CAD cents before tax; nil = not set
	// DurationMinutes is how long the service takes to complete; nil = not set.
	DurationMinutes *int `json:"durationMinutes"`
}

type CentralServiceInput struct {
	ID       string
	Name     string
	Kind     string
	ParentID *string
	// ParentIDs are the main services an add-on is offered with; it replaces ParentID when given.
	ParentIDs []string
	Version  int
	// PriceSet says whether PriceCents is part of this save. When false an edit leaves the stored price alone.
	PriceSet   bool
	PriceCents *int64
	// DurationSet says whether DurationMinutes is part of this save; when false an edit leaves the stored duration alone.
	DurationSet     bool
	DurationMinutes *int
}

const centralServiceColumns = `id::text,name,kind,parent_id::text,archived,version,to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),price_cents,duration_minutes`

func scanCentralService(row pgx.Row) (CentralService, error) {
	var s CentralService
	err := row.Scan(&s.ID, &s.Name, &s.Kind, &s.ParentID, &s.Archived, &s.Version, &s.CreatedAt, &s.PriceCents, &s.DurationMinutes)
	return s, err
}

func centralCatalogueGrant(ctx context.Context, tx pgx.Tx, actor, org, action string) error {
	if actor == "" || org == "" {
		return ErrScope
	}
	if action != "catalogue.read" && action != "catalogue.manage" {
		return ErrForbidden
	}
	var authorized bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(
		-- 1. Direct organization grant
		SELECT 1 FROM organization_grants g
		JOIN principals p ON p.id=g.principal_id
		WHERE p.active AND g.principal_id=$1 AND g.organization_id=$2 AND g.action=$3

		UNION ALL

		-- 2. Scoped role assignment at organization scope
		SELECT 1 FROM member_role_assignments a
		JOIN principals p ON p.id=a.principal_id
		JOIN role_capabilities rc ON rc.role_id=a.role_id
		WHERE p.active AND a.principal_id=$1 AND a.organization_id=$2
		  AND a.scope_type='organization' AND a.scope_id=$2 AND rc.action=$3
	)`, actor, org, action).Scan(&authorized)
	if err != nil {
		return err
	}
	if !authorized {
		return ErrForbidden
	}
	return nil
}

func centralCatalogueAudit(ctx context.Context, tx pgx.Tx, actor, org, action, entityID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4)`, org, actor, action, entityID)
	return err
}

func (s *Store) ListCentralServices(ctx context.Context, actor, org string) ([]CentralService, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = centralCatalogueGrant(ctx, tx, actor, org, "catalogue.read"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+centralServiceColumns+` FROM central_catalog_services WHERE organization_id=$1 AND merged_into IS NULL ORDER BY name,id LIMIT 1000`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []CentralService
	for rows.Next() {
		svc, scanErr := scanCentralService(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		list = append(list, svc)
	}
	rows.Close()
	links, err := centralAddonLinks(ctx, tx, org)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Kind == "addon" {
			list[i].ParentIDs = links[list[i].ID]
			if len(list[i].ParentIDs) == 0 && list[i].ParentID != nil {
				list[i].ParentIDs = []string{*list[i].ParentID}
			}
		}
	}
	if list == nil {
		list = []CentralService{}
	}
	return list, nil
}

func (s *Store) SaveCentralService(ctx context.Context, actor, org string, input CentralServiceInput) (CentralService, error) {
	input.Name = strings.TrimSpace(input.Name)
	if utf8.RuneCountInString(input.Name) < 2 || utf8.RuneCountInString(input.Name) > 120 {
		return CentralService{}, ErrCentralServiceInput
	}
	if input.Kind != "main" && input.Kind != "addon" {
		return CentralService{}, ErrCentralServiceInput
	}
	if input.PriceSet && input.PriceCents != nil && (*input.PriceCents < 0 || *input.PriceCents > 100000000000) {
		return CentralService{}, ErrCentralServiceInput
	}
	if input.DurationSet && input.DurationMinutes != nil && (*input.DurationMinutes < 1 || *input.DurationMinutes > 10080) {
		return CentralService{}, ErrCentralServiceInput
	}
	if input.Kind == "main" && (input.ParentID != nil || len(input.ParentIDs) > 0) {
		return CentralService{}, ErrCentralServiceInput
	}
	if input.Kind == "addon" {
		ids := input.ParentIDs
		if len(ids) == 0 && input.ParentID != nil {
			ids = []string{*input.ParentID}
		}
		set, ok := uniqueParents(ids, input.ID)
		if !ok {
			return CentralService{}, ErrCentralServiceInput
		}
		input.ParentIDs = set
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CentralService{}, err
	}
	defer tx.Rollback(ctx)
	if err = centralCatalogueGrant(ctx, tx, actor, org, "catalogue.manage"); err != nil {
		return CentralService{}, err
	}
	var currentParent *string
	if input.ID != "" {
		var version int
		err = tx.QueryRow(ctx, `SELECT version,parent_id::text FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND NOT archived FOR UPDATE`, org, input.ID).Scan(&version, &currentParent)
		if errors.Is(err, pgx.ErrNoRows) || version != input.Version {
			return CentralService{}, ErrCentralServiceConflict
		}
		if err != nil {
			return CentralService{}, err
		}
		if input.Kind == "addon" {
			var children bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM central_catalog_services WHERE parent_id=$1 AND NOT archived)`, input.ID).Scan(&children)
			if err != nil {
				return CentralService{}, err
			}
			if children {
				return CentralService{}, ErrCentralServiceInput
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
		err = tx.QueryRow(ctx, `SELECT id::text FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND kind='main' AND NOT archived FOR SHARE`, org, *input.ParentID).Scan(&parent)
		if errors.Is(err, pgx.ErrNoRows) {
			return CentralService{}, ErrCentralServiceInput
		}
		if err != nil {
			return CentralService{}, err
		}
	}
	var svc CentralService
	var action string
	if input.ID == "" {
		action = "central_service.created"
		svc, err = scanCentralService(tx.QueryRow(ctx, `INSERT INTO central_catalog_services(organization_id,name,kind,parent_id,price_cents,duration_minutes) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+centralServiceColumns, org, input.Name, input.Kind, input.ParentID, input.PriceCents, input.DurationMinutes))
	} else {
		action = "central_service.updated"
		svc, err = scanCentralService(tx.QueryRow(ctx, `UPDATE central_catalog_services SET name=$3,kind=$4,parent_id=$5,price_cents=CASE WHEN $6 THEN $7 ELSE price_cents END,duration_minutes=CASE WHEN $8 THEN $9 ELSE duration_minutes END,version=version+1 WHERE organization_id=$1 AND id=$2 RETURNING `+centralServiceColumns, org, input.ID, input.Name, input.Kind, input.ParentID, input.PriceSet, input.PriceCents, input.DurationSet, input.DurationMinutes))
	}
	if err != nil {
		return CentralService{}, err
	}
	if svc.Kind == "addon" {
		if err = setCentralAddonLinks(ctx, tx, org, svc.ID, input.ParentIDs); err != nil {
			return CentralService{}, err
		}
		svc.ParentIDs = input.ParentIDs
	} else if _, err = tx.Exec(ctx, `DELETE FROM central_addon_parents WHERE addon_id=$1`, svc.ID); err != nil {
		return CentralService{}, err
	}
	snapshot, _ := json.Marshal(svc)
	if _, err = tx.Exec(ctx, `INSERT INTO central_catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES($1,$2,$3,$4)`, svc.ID, svc.Version, actor, snapshot); err != nil {
		return CentralService{}, centralLinkFailure(err)
	}
	if err = centralCatalogueAudit(ctx, tx, actor, org, action, svc.ID); err != nil {
		return CentralService{}, err
	}
	if err = syncServiceDurationRules(ctx, tx, org, "", actor); err != nil {
		return CentralService{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CentralService{}, err
	}
	return svc, nil
}

func (s *Store) ArchiveCentralService(ctx context.Context, actor, org, id string, version int) (CentralService, error) {
	if actor == "" || org == "" || id == "" || version <= 0 {
		return CentralService{}, ErrCentralServiceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CentralService{}, err
	}
	defer tx.Rollback(ctx)
	if err = centralCatalogueGrant(ctx, tx, actor, org, "catalogue.manage"); err != nil {
		return CentralService{}, err
	}
	current, err := scanCentralService(tx.QueryRow(ctx, `SELECT `+centralServiceColumns+` FROM central_catalog_services WHERE organization_id=$1 AND id=$2 FOR UPDATE`, org, id))
	if errors.Is(err, pgx.ErrNoRows) || current.Archived || current.Version != version {
		return CentralService{}, ErrCentralServiceConflict
	}
	if err != nil {
		return CentralService{}, err
	}
	if err = releaseCentralMain(ctx, tx, org, id); err != nil {
		return CentralService{}, err
	}
	var children bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM central_catalog_services WHERE parent_id=$1 AND NOT archived)`, id).Scan(&children)
	if err != nil {
		return CentralService{}, err
	}
	if children {
		return CentralService{}, ErrCentralServiceHasChildren
	}
	current, err = scanCentralService(tx.QueryRow(ctx, `UPDATE central_catalog_services SET archived=true,version=version+1 WHERE id=$1 RETURNING `+centralServiceColumns, id))
	if err != nil {
		return CentralService{}, err
	}
	snapshot, _ := json.Marshal(current)
	if _, err = tx.Exec(ctx, `INSERT INTO central_catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES($1,$2,$3,$4)`, current.ID, current.Version, actor, snapshot); err != nil {
		return CentralService{}, centralLinkFailure(err)
	}
	if err = centralCatalogueAudit(ctx, tx, actor, org, "central_service.archived", current.ID); err != nil {
		return CentralService{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CentralService{}, err
	}
	return current, nil
}

// RestoreCentralService brings an archived service back into the catalogue, and its franchise copies with it (archiving it had archived
// them). An add-on comes back only while its main service is active.
func (s *Store) RestoreCentralService(ctx context.Context, actor, org, id string, version int) (CentralService, error) {
	if actor == "" || org == "" || id == "" || version <= 0 {
		return CentralService{}, ErrCentralServiceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CentralService{}, err
	}
	defer tx.Rollback(ctx)
	if err = centralCatalogueGrant(ctx, tx, actor, org, "catalogue.manage"); err != nil {
		return CentralService{}, err
	}
	current, err := scanCentralService(tx.QueryRow(ctx, `SELECT `+centralServiceColumns+` FROM central_catalog_services WHERE organization_id=$1 AND id=$2 FOR UPDATE`, org, id))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!current.Archived || current.Version != version) {
		return CentralService{}, ErrCentralServiceConflict
	}
	if err != nil {
		return CentralService{}, err
	}
	if current.ParentID != nil {
		var parentActive bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND NOT archived)`, org, *current.ParentID).Scan(&parentActive); err != nil {
			return CentralService{}, err
		}
		if !parentActive {
			return CentralService{}, ErrCentralServiceConflict
		}
	}
	// The franchise copies come back too; the revision below then syncs them as usual.
	if _, err = tx.Exec(ctx, `UPDATE catalog_services SET archived=false WHERE central_service_id=$1 AND archived`, id); err != nil {
		return CentralService{}, err
	}
	current, err = scanCentralService(tx.QueryRow(ctx, `UPDATE central_catalog_services SET archived=false,version=version+1 WHERE id=$1 RETURNING `+centralServiceColumns, id))
	if err != nil {
		return CentralService{}, err
	}
	snapshot, _ := json.Marshal(current)
	if _, err = tx.Exec(ctx, `INSERT INTO central_catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES($1,$2,$3,$4)`, current.ID, current.Version, actor, snapshot); err != nil {
		return CentralService{}, centralLinkFailure(err)
	}
	if err = centralCatalogueAudit(ctx, tx, actor, org, "central_service.restored", current.ID); err != nil {
		return CentralService{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CentralService{}, err
	}
	return current, nil
}

func (s *Store) DeleteCentralService(ctx context.Context, actor, org, id string, version int) (DeleteResult, error) {
	if actor == "" || org == "" || id == "" {
		return DeleteResult{}, ErrCentralServiceInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeleteResult{}, err
	}
	defer tx.Rollback(ctx)
	if err = centralCatalogueGrant(ctx, tx, actor, org, "catalogue.manage"); err != nil {
		return DeleteResult{}, err
	}
	current, err := scanCentralService(tx.QueryRow(ctx, `SELECT `+centralServiceColumns+` FROM central_catalog_services WHERE organization_id=$1 AND id=$2 FOR UPDATE`, org, id))
	if errors.Is(err, pgx.ErrNoRows) || (version > 0 && current.Version != version) {
		return DeleteResult{}, ErrCentralServiceConflict
	}
	if err != nil {
		return DeleteResult{}, err
	}
	if err = releaseCentralMain(ctx, tx, org, id); err != nil {
		return DeleteResult{}, err
	}
	var children bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM central_catalog_services WHERE parent_id=$1 AND NOT archived)`, id).Scan(&children)
	if err != nil {
		return DeleteResult{}, err
	}
	if children {
		return DeleteResult{}, ErrCentralServiceHasChildren
	}

	// Franchise copies that were switched on automatically and never used go with it; if any copy is used (bookings, jobs,
	// invoices, rules...), everything is kept and the service is archived instead.
	if _, err = tx.Exec(ctx, `SAVEPOINT unused_copies`); err != nil {
		return DeleteResult{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_service_revisions WHERE service_id IN (SELECT id FROM catalog_services WHERE central_service_id=$1)`, id); err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM catalog_services WHERE central_service_id=$1`, id)
	}
	if err != nil {
		if _, e := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT unused_copies`); e != nil {
			return DeleteResult{}, e
		}
	}
	var linked bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_services WHERE central_service_id=$1) OR EXISTS(SELECT 1 FROM central_catalog_services WHERE parent_id=$1)`, id).Scan(&linked)
	if err != nil {
		return DeleteResult{}, err
	}

	if linked {
		current, err = scanCentralService(tx.QueryRow(ctx, `UPDATE central_catalog_services SET archived=true,version=version+1 WHERE id=$1 RETURNING `+centralServiceColumns, id))
		if err != nil {
			return DeleteResult{}, err
		}
		snapshot, _ := json.Marshal(current)
		if _, err = tx.Exec(ctx, `INSERT INTO central_catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES($1,$2,$3,$4)`, current.ID, current.Version, actor, snapshot); err != nil {
			return DeleteResult{}, centralLinkFailure(err)
		}
		if err = centralCatalogueAudit(ctx, tx, actor, org, "central_service.deleted", current.ID); err != nil {
			return DeleteResult{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return DeleteResult{}, err
		}
		return DeleteResult{Permanent: false, Archived: true, Message: "Central service removed from active catalogue; linked location services retain historical records."}, nil
	}

	if _, err = tx.Exec(ctx, `DELETE FROM central_catalog_service_revisions WHERE service_id=$1`, id); err != nil {
		return DeleteResult{}, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM central_catalog_services WHERE organization_id=$1 AND id=$2`, org, id)
	if err != nil {
		return DeleteResult{}, err
	}
	if tag.RowsAffected() == 0 {
		return DeleteResult{}, ErrCentralServiceConflict
	}
	if err = centralCatalogueAudit(ctx, tx, actor, org, "central_service.permanently_deleted", id); err != nil {
		return DeleteResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{Permanent: true, Archived: false, Message: "Central service permanently deleted from catalogue."}, nil
}
