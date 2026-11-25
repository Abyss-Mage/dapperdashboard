package postgres

import (
	"time"
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
	// A sale (error404 #133): the sale price and its last day (YYYY-MM-DD, Toronto; nil = until removed). NowPriceCents is the price
	// charged today: the sale while it runs, else the regular price (for a linked service, the central service's sale counts).
	SalePriceCents *int64  `json:"salePriceCents"`
	SaleEndsOn     *string `json:"saleEndsOn"`
	NowPriceCents *int64 `json:"nowPriceCents"`
	// SaleSet says whether the sale fields are part of this save; when false an edit keeps the stored sale.
	SaleSet bool `json:"-"`
	// KeepPrice leaves the stored price alone on an edit (the save did not include a price), so a bulk or text edit that only
	// changes names or times cannot erase a franchise's own prices (error404 #150).
	KeepPrice bool `json:"-"`
	// BookingGroup is the category the service is booked under (empty = from its name); a linked copy shows the central one (#166).
	BookingGroup string `json:"bookingGroup"`
	// GroupSet says whether BookingGroup is part of this save; when false an edit keeps the stored one.
	GroupSet bool `json:"-"`
}

const serviceColumns = `id::text,name,kind,parent_id::text,archived,version,central_service_id::text,price_cents,duration_minutes,sale_price_cents,sale_ends_on::text,` +
	// The price today, from this row's own values (the row just saved, in RETURNING) or its central service's sale.
	`CASE WHEN central_service_id IS NULL THEN sale_or_price(catalog_services.price_cents,catalog_services.sale_price_cents,catalog_services.sale_ends_on)
	 ELSE coalesce((SELECT sale_or_price(catalog_services.price_cents,cc.sale_price_cents,cc.sale_ends_on) FROM central_catalog_services cc WHERE cc.id=catalog_services.central_service_id),catalog_services.price_cents) END,` +
	`COALESCE((SELECT cc.booking_group FROM central_catalog_services cc WHERE cc.id=catalog_services.central_service_id),catalog_services.booking_group)`

func scanService(row pgx.Row) (CatalogService, error) {
	var service CatalogService
	err := row.Scan(&service.ID, &service.Name, &service.Kind, &service.ParentID, &service.Archived, &service.Version, &service.CentralServiceID, &service.PriceCents, &service.DurationMinutes, &service.SalePriceCents, &service.SaleEndsOn, &service.NowPriceCents, &service.BookingGroup)
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
	// A franchise's own service carries its own price (error404 #112 #113); linked ones take it from the central catalogue.
	if input.PriceCents != nil && (*input.PriceCents < 0 || *input.PriceCents > 100_000_000) {
		return CatalogService{}, ErrServiceInput
	}
	if input.DurationMinutes != nil && (*input.DurationMinutes < 1 || *input.DurationMinutes > 10080) {
		return CatalogService{}, ErrServiceInput
	}
	if !validSaleEnd(input.SaleEndsOn) {
		return CatalogService{}, ErrServiceInput
	}
	if input.BookingGroup = strings.TrimSpace(input.BookingGroup); !validBookingGroup(input.BookingGroup) {
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
		service, err = scanService(tx.QueryRow(ctx, `INSERT INTO catalog_services(organization_id,location_id,name,kind,parent_id,duration_minutes,price_cents,sale_price_cents,sale_ends_on,booking_group) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::date,$10) RETURNING `+serviceColumns, scope.OrganizationID, scope.LocationID, input.Name, input.Kind, input.ParentID, input.DurationMinutes, input.PriceCents, input.SalePriceCents, input.SaleEndsOn, input.BookingGroup))
	} else {
		service, err = scanService(tx.QueryRow(ctx, `UPDATE catalog_services SET name=$4,kind=$5,parent_id=$6,duration_minutes=$7,price_cents=CASE WHEN $12 THEN price_cents ELSE $8 END,
		 sale_price_cents=CASE WHEN $9 THEN $10 ELSE sale_price_cents END,sale_ends_on=CASE WHEN $9 THEN $11::date ELSE sale_ends_on END,
		 booking_group=CASE WHEN $13 THEN $14 ELSE booking_group END,version=version+1
		 WHERE organization_id=$1 AND location_id=$2 AND id=$3 RETURNING `+serviceColumns, scope.OrganizationID, scope.LocationID, input.ID, input.Name, input.Kind, input.ParentID, input.DurationMinutes, input.PriceCents, input.SaleSet, input.SalePriceCents, input.SaleEndsOn, input.KeepPrice, input.GroupSet, input.BookingGroup))
	}
	if err != nil {
		return CatalogService{}, err
	}
	if !saleBelowPrice(service.SalePriceCents, service.PriceCents) {
		return CatalogService{}, ErrServiceInput
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
	if err = archiveOnlyLocalAddons(ctx, tx, scope, id); err != nil {
		return CatalogService{}, err
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

// RestoreService brings an archived franchise service back. A copy of a central service waits for the central one to be restored, and an
// add-on comes back only while its main service is active.
func (s *Store) RestoreService(ctx context.Context, scope Scope, id string, version int) (CatalogService, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CatalogService{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "catalogue.manage"); err != nil {
		return CatalogService{}, err
	}
	current, err := scanService(tx.QueryRow(ctx, `SELECT `+serviceColumns+` FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!current.Archived || current.Version != version) {
		return CatalogService{}, ErrServiceConflict
	}
	if err != nil {
		return CatalogService{}, err
	}
	var blocked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_services s LEFT JOIN central_catalog_services c ON c.id=s.central_service_id LEFT JOIN catalog_services p ON p.id=s.parent_id
 WHERE s.id=$1 AND (c.archived OR p.archived))`, id).Scan(&blocked); err != nil {
		return CatalogService{}, err
	}
	if blocked {
		return CatalogService{}, ErrServiceConflict
	}
	current, err = scanService(tx.QueryRow(ctx, `UPDATE catalog_services SET archived=false,version=version+1 WHERE id=$1 RETURNING `+serviceColumns, id))
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
	if err = archiveOnlyLocalAddons(ctx, tx, scope, id); err != nil {
		return DeleteResult{}, err
	}

	// A website listing alone does not keep a service; it goes with it (error404 #161).
	var referenced bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM catalog_services WHERE parent_id=$3
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

	// The time to complete is kept as a scheduling rule; an unused service takes its rules with it (error404, 2026-11-14). A rule that
	// holds a reservation or offer still blocks the delete.
	for _, q := range []string{
		`DELETE FROM service_scheduling_revisions WHERE rule_id IN (SELECT id FROM service_scheduling_rules WHERE service_id=$1)`,
		`DELETE FROM service_scheduling_rules WHERE service_id=$1`,
		`DELETE FROM service_publications WHERE service_id=$1`,
		`DELETE FROM catalog_service_revisions WHERE service_id=$1`,
	} {
		if _, err = tx.Exec(ctx, q, id); err != nil {
			return DeleteResult{}, err
		}
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

// saleBelowPrice: a sale needs a regular price and must be lower than it.
func saleBelowPrice(sale, price *int64) bool {
	return sale == nil || (price != nil && *sale >= 0 && *sale < *price)
}

// validSaleEnd accepts no date or a calendar date (YYYY-MM-DD).
func validSaleEnd(v *string) bool {
	if v == nil {
		return true
	}
	_, err := time.Parse("2006-01-02", *v)
	return err == nil
}

// archiveOnlyLocalAddons archives the active add-ons offered only with this main service (others were moved to another main by
// releaseLocalMain), so archiving or deleting the main is not refused (owner, error404 #160). Each keeps a revision.
func archiveOnlyLocalAddons(ctx context.Context, tx pgx.Tx, scope Scope, main string) error {
	rows, err := tx.Query(ctx, `UPDATE catalog_services SET archived=true,version=version+1 WHERE organization_id=$1 AND location_id=$2 AND parent_id=$3 AND NOT archived RETURNING `+serviceColumns, scope.OrganizationID, scope.LocationID, main)
	if err != nil {
		return err
	}
	var done []CatalogService
	for rows.Next() {
		svc, e := scanService(rows)
		if e != nil {
			rows.Close()
			return e
		}
		done = append(done, svc)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, svc := range done {
		if err = serviceRevision(ctx, tx, scope, svc); err != nil {
			return err
		}
	}
	return nil
}

// validBookingGroup: empty (worked out from the name) or 2 to 40 characters.
func validBookingGroup(g string) bool {
	n := utf8.RuneCountInString(g)
	return n == 0 || n >= 2 && n <= 40 && !strings.ContainsAny(g, "\r\n<>")
}
