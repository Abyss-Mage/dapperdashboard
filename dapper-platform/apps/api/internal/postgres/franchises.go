package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrStructureInput = errors.New("valid franchise code, name and address required")
var ErrFranchiseCode = errors.New("that franchise code is already used in this organization")
var ErrFranchiseInUse = errors.New("cannot delete a franchise that has operational records or active role assignments")

// ErrLocationInUse is kept as an alias so older callers keep compiling; the product has no separate "location".
var ErrLocationInUse = ErrFranchiseInUse

type DeleteResult struct {
	Permanent bool   `json:"permanent"`
	Archived  bool   `json:"archived"`
	Message   string `json:"message"`
}

type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// FranchiseInput is what an administrator supplies. The address is entirely optional.
type FranchiseInput struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Address Address `json:"address"`
}

// Franchise is one franchise site. ID is the site identifier used everywhere else in the system (grants, bookings,
// scheduling, GPS); FranchiseID is the internal 1:1 grouping row that franchise-scoped role assignments point at.
type Franchise struct {
	ID          string  `json:"id"`
	FranchiseID *string `json:"franchiseId"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Address     Address `json:"address"`
}

// Location is a read-only compatibility view of a franchise for screens that still list sites.
type Location struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Code        string  `json:"code"`
	FranchiseID *string `json:"franchiseId"`
}

var franchiseCodePattern = regexp.MustCompile(`^[A-Z0-9-]{2,12}$`)

func structureGrant(ctx context.Context, tx pgx.Tx, actor, org, action string) error {
	if actor == "" || org == "" {
		return ErrScope
	}
	if action != "structure.read" && action != "structure.manage" {
		return ErrForbidden
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT p.id::text FROM principals p JOIN organization_grants g ON g.principal_id=p.id WHERE p.id=$1 AND p.active AND g.organization_id=$2 AND g.action=$3 FOR SHARE OF p,g`, actor, org, action).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	return err
}
func structureAudit(ctx context.Context, tx pgx.Tx, actor, org, action, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,actor_id,action,entity_id) VALUES($1,$2,$3,$4)`, org, actor, action, id)
	return err
}
func validStructureName(name string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	return n >= 2 && n <= 120
}

// normalizeFranchise trims, upper-cases the code and validates lengths. Every address field is optional.
func normalizeFranchise(in FranchiseInput) (FranchiseInput, error) {
	in.Code, in.Name = strings.ToUpper(strings.TrimSpace(in.Code)), strings.TrimSpace(in.Name)
	a := &in.Address
	a.Street, a.City, a.Region, a.PostalCode, a.Country = strings.TrimSpace(a.Street), strings.TrimSpace(a.City), strings.TrimSpace(a.Region), strings.TrimSpace(a.PostalCode), strings.TrimSpace(a.Country)
	if !franchiseCodePattern.MatchString(in.Code) || !validStructureName(in.Name) ||
		utf8.RuneCountInString(a.Street) > 200 || utf8.RuneCountInString(a.City) > 100 || utf8.RuneCountInString(a.Region) > 100 || utf8.RuneCountInString(a.PostalCode) > 20 || utf8.RuneCountInString(a.Country) > 100 {
		return in, ErrStructureInput
	}
	return in, nil
}

func franchiseWriteError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && strings.Contains(pg.ConstraintName, "code") {
		return ErrFranchiseCode
	}
	return err
}

const franchiseColumns = `l.id::text,l.franchise_id::text,COALESCE(l.code,''),l.name,l.address_street,l.address_city,l.address_region,l.address_postal_code,l.address_country`

func scanFranchise(row pgx.Row) (Franchise, error) {
	var f Franchise
	err := row.Scan(&f.ID, &f.FranchiseID, &f.Code, &f.Name, &f.Address.Street, &f.Address.City, &f.Address.Region, &f.Address.PostalCode, &f.Address.Country)
	return f, err
}

// CreateFranchise creates one franchise site with its code and optional address in a single audited transaction.
func (s *Store) CreateFranchise(ctx context.Context, actor, org string, in FranchiseInput) (Franchise, error) {
	in, err := normalizeFranchise(in)
	if err != nil {
		return Franchise{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Franchise{}, err
	}
	defer tx.Rollback(ctx)
	if err = structureGrant(ctx, tx, actor, org, "structure.manage"); err != nil {
		return Franchise{}, err
	}
	var shadow string
	if err = tx.QueryRow(ctx, `INSERT INTO franchises(organization_id,name) VALUES($1,$2) RETURNING id::text`, org, in.Name).Scan(&shadow); err != nil {
		return Franchise{}, err
	}
	result, err := scanFranchise(tx.QueryRow(ctx, `WITH l AS (INSERT INTO locations(organization_id,franchise_id,name,code,address_street,address_city,address_region,address_postal_code,address_country) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *) SELECT `+franchiseColumns+` FROM l`,
		org, shadow, in.Name, in.Code, in.Address.Street, in.Address.City, in.Address.Region, in.Address.PostalCode, in.Address.Country))
	if err != nil {
		return Franchise{}, franchiseWriteError(err)
	}
	if err = structureAudit(ctx, tx, actor, org, "franchise.created", result.ID); err != nil {
		return Franchise{}, err
	}
	return result, tx.Commit(ctx)
}

// Structure lists the organization's franchise sites.
func (s *Store) Structure(ctx context.Context, actor, org string) ([]Franchise, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = structureGrant(ctx, tx, actor, org, "structure.read"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+franchiseColumns+` FROM locations l WHERE l.organization_id=$1 ORDER BY l.name,l.id LIMIT 100`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Franchise{}
	for rows.Next() {
		f, e := scanFranchise(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// UpdateFranchise edits a franchise by its site id. Sites created before franchises existed gain their 1:1 grouping
// row here, so every site ends up as a normal franchise.
func (s *Store) UpdateFranchise(ctx context.Context, actor, org, id string, in FranchiseInput) (Franchise, error) {
	in, err := normalizeFranchise(in)
	if err != nil || id == "" {
		return Franchise{}, ErrStructureInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Franchise{}, err
	}
	defer tx.Rollback(ctx)
	if err = structureGrant(ctx, tx, actor, org, "structure.manage"); err != nil {
		return Franchise{}, err
	}
	var shadow *string
	err = tx.QueryRow(ctx, `SELECT franchise_id::text FROM locations WHERE organization_id=$1 AND id=$2 FOR UPDATE`, org, id).Scan(&shadow)
	if errors.Is(err, pgx.ErrNoRows) {
		return Franchise{}, ErrStructureInput
	}
	if err != nil {
		return Franchise{}, err
	}
	if shadow == nil {
		var created string
		if err = tx.QueryRow(ctx, `INSERT INTO franchises(organization_id,name) VALUES($1,$2) RETURNING id::text`, org, in.Name).Scan(&created); err != nil {
			return Franchise{}, err
		}
		shadow = &created
	} else if _, err = tx.Exec(ctx, `UPDATE franchises SET name=$3 WHERE organization_id=$1 AND id=$2`, org, *shadow, in.Name); err != nil {
		return Franchise{}, err
	}
	result, err := scanFranchise(tx.QueryRow(ctx, `WITH l AS (UPDATE locations SET franchise_id=$3,name=$4,code=$5,address_street=$6,address_city=$7,address_region=$8,address_postal_code=$9,address_country=$10 WHERE organization_id=$1 AND id=$2 RETURNING *) SELECT `+franchiseColumns+` FROM l`,
		org, id, *shadow, in.Name, in.Code, in.Address.Street, in.Address.City, in.Address.Region, in.Address.PostalCode, in.Address.Country))
	if err != nil {
		return Franchise{}, franchiseWriteError(err)
	}
	if err = structureAudit(ctx, tx, actor, org, "franchise.updated", result.ID); err != nil {
		return Franchise{}, err
	}
	return result, tx.Commit(ctx)
}

// DeleteFranchise permanently deletes an unused franchise site (and its grouping row). Any operational record,
// attendance, photo post, or franchise-scoped role assignment blocks it.
func (s *Store) DeleteFranchise(ctx context.Context, actor, org, id string) (DeleteResult, error) {
	if id == "" {
		return DeleteResult{}, ErrStructureInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeleteResult{}, err
	}
	defer tx.Rollback(ctx)
	if err = structureGrant(ctx, tx, actor, org, "structure.manage"); err != nil {
		return DeleteResult{}, err
	}
	var shadow *string
	err = tx.QueryRow(ctx, `SELECT franchise_id::text FROM locations WHERE organization_id=$1 AND id=$2 FOR UPDATE`, org, id).Scan(&shadow)
	if errors.Is(err, pgx.ErrNoRows) {
		return DeleteResult{}, ErrStructureInput
	}
	if err != nil {
		return DeleteResult{}, err
	}
	var inUse bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM jobs WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM bookings WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM invoices WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM employee_attendance WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM employee_upload_posts WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM employee_worksite_assignments WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM employee_availability_submissions WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM employee_booking_worksites WHERE organization_id=$1 AND location_id=$2
		UNION ALL SELECT 1 FROM member_role_assignments WHERE organization_id=$1 AND franchise_id=$3
	)`, org, id, shadow).Scan(&inUse)
	if err != nil {
		return DeleteResult{}, err
	}
	if inUse {
		return DeleteResult{}, ErrFranchiseInUse
	}
	// Configuration that belongs only to the site goes with it.
	for _, q := range []string{
		`DELETE FROM gps_settings_events WHERE organization_id=$1 AND location_id=$2`,
		`DELETE FROM gps_settings WHERE organization_id=$1 AND location_id=$2`,
		// Grant history for a site that no longer exists goes with it; the deletion itself stays in organization_audit_events.
		`DELETE FROM access_grant_events WHERE organization_id=$1 AND location_id=$2`,
		`DELETE FROM location_grants WHERE organization_id=$1 AND location_id=$2`,
		`DELETE FROM member_role_assignments WHERE organization_id=$1 AND location_id=$2`,
	} {
		if _, err = tx.Exec(ctx, q, org, id); err != nil {
			return DeleteResult{}, err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM locations WHERE organization_id=$1 AND id=$2`, org, id); err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23503" {
			return DeleteResult{}, ErrFranchiseInUse
		}
		return DeleteResult{}, err
	}
	if shadow != nil {
		if _, err = tx.Exec(ctx, `DELETE FROM franchises WHERE organization_id=$1 AND id=$2`, org, *shadow); err != nil {
			return DeleteResult{}, err
		}
	}
	if err = structureAudit(ctx, tx, actor, org, "franchise.deleted", id); err != nil {
		return DeleteResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{Permanent: true, Archived: false, Message: "Unused franchise permanently deleted."}, nil
}
