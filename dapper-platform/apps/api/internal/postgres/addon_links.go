package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// An add-on is one record offered with many main services. addon_links.sql holds the link tables; parent_id is the add-on's primary
// main service and is always one of its links. The functions here read the links for lists and write them when a service is saved.

type linkQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readLinks(rows pgx.Rows, err error) (map[string][]string, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var addon, main string
		if err = rows.Scan(&addon, &main); err != nil {
			return nil, err
		}
		out[addon] = append(out[addon], main)
	}
	return out, rows.Err()
}

// centralAddonLinks maps each central add-on to the main services it is offered with.
func centralAddonLinks(ctx context.Context, q linkQuerier, org string) (map[string][]string, error) {
	return readLinks(q.Query(ctx, `SELECT l.addon_id::text,l.main_id::text FROM central_addon_parents l JOIN central_catalog_services m ON m.id=l.main_id WHERE l.organization_id=$1 ORDER BY m.name,l.main_id`, org))
}

// localAddonLinks does the same for one franchise's services.
func localAddonLinks(ctx context.Context, q linkQuerier, org, location string) (map[string][]string, error) {
	return readLinks(q.Query(ctx, `SELECT l.addon_id::text,l.main_id::text FROM catalog_addon_parents l JOIN catalog_services m ON m.id=l.main_id WHERE l.organization_id=$1 AND l.location_id=$2 ORDER BY m.name,l.main_id`, org, location))
}

// uniqueParents drops repeats and blanks and keeps the order given.
func uniqueParents(ids []string, self string) ([]string, bool) {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		if id == "" || id == self {
			return nil, false
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, len(out) > 0
}

// primaryParent keeps the current primary main service when it is still offered, otherwise takes the first of the new set.
func primaryParent(current *string, set []string) string {
	if current != nil {
		for _, id := range set {
			if id == *current {
				return id
			}
		}
	}
	return set[0]
}

// setCentralAddonLinks makes the add-on's links exactly the given main services (all must be active central main services).
func setCentralAddonLinks(ctx context.Context, tx pgx.Tx, org, addon string, mains []string) error {
	for _, id := range mains {
		var ok string
		err := tx.QueryRow(ctx, `SELECT id::text FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND kind='main' AND NOT archived FOR SHARE`, org, id).Scan(&ok)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCentralServiceInput
		}
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM central_addon_parents WHERE addon_id=$1 AND NOT (main_id = ANY($2::uuid[]))`, addon, mains); err != nil {
		return err
	}
	for _, id := range mains {
		if _, err := tx.Exec(ctx, `INSERT INTO central_addon_parents(organization_id,addon_id,main_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, org, addon, id); err != nil {
			return err
		}
	}
	return nil
}

// releaseCentralMain is run before a central main service is archived or deleted. Add-ons that are also offered with another active
// main service move their primary there and simply stop being offered with this one; an add-on offered with this one only still
// blocks it, as before.
func releaseCentralMain(ctx context.Context, tx pgx.Tx, org, main string) error {
	if _, err := tx.Exec(ctx, `UPDATE central_catalog_services a SET parent_id=(
  SELECT l.main_id FROM central_addon_parents l JOIN central_catalog_services m ON m.id=l.main_id AND NOT m.archived
  WHERE l.addon_id=a.id AND l.main_id<>$2 ORDER BY l.main_id LIMIT 1)
 WHERE a.organization_id=$1 AND a.parent_id=$2 AND a.kind='addon' AND NOT a.archived
  AND EXISTS(SELECT 1 FROM central_addon_parents l JOIN central_catalog_services m ON m.id=l.main_id AND NOT m.archived WHERE l.addon_id=a.id AND l.main_id<>$2)`, org, main); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM central_addon_parents l WHERE l.main_id=$2 AND l.organization_id=$1
  AND EXISTS(SELECT 1 FROM central_catalog_services a WHERE a.id=l.addon_id AND a.parent_id<>$2)`, org, main)
	return err
}

// setLocalAddonLinks makes the franchise add-on's links exactly the given main services (all must be active main services here).
func setLocalAddonLinks(ctx context.Context, tx pgx.Tx, scope Scope, addon string, mains []string) error {
	for _, id := range mains {
		var ok string
		err := tx.QueryRow(ctx, `SELECT id::text FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='main' AND NOT archived FOR SHARE`, scope.OrganizationID, scope.LocationID, id).Scan(&ok)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrServiceInput
		}
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM catalog_addon_parents WHERE addon_id=$1 AND NOT (main_id = ANY($2::uuid[]))`, addon, mains); err != nil {
		return err
	}
	for _, id := range mains {
		if _, err := tx.Exec(ctx, `INSERT INTO catalog_addon_parents(organization_id,location_id,addon_id,main_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, scope.OrganizationID, scope.LocationID, addon, id); err != nil {
			return err
		}
	}
	return nil
}

// releaseLocalMain: add-ons also offered with another main service stop being offered with this one (done when it is archived or deleted).
func releaseLocalMain(ctx context.Context, tx pgx.Tx, scope Scope, main string) error {
	_, err := tx.Exec(ctx, `DELETE FROM catalog_addon_parents l WHERE l.main_id=$3 AND l.organization_id=$1 AND l.location_id=$2
  AND EXISTS(SELECT 1 FROM catalog_services a WHERE a.id=l.addon_id AND a.parent_id<>$3)`, scope.OrganizationID, scope.LocationID, main)
	return err
}
