package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrTaxInput = errors.New("valid tax name, rate and settings required")
var ErrTaxConflict = errors.New("tax rate not found, already exists or was changed by someone else")

type TaxRate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	RateBP  int    `json:"rateBasisPoints"` // 1300 = 13.00%
	Active  bool   `json:"active"`
	Version int    `json:"version"`
}
type TaxSettings struct {
	PricesIncludeTax bool    `json:"pricesIncludeTax"`
	Rounding         *string `json:"rounding"` // nil until someone chooses it
	Version          int     `json:"version"`
}
type FranchiseTaxes struct {
	Rates    []TaxRate   `json:"rates"`
	Settings TaxSettings `json:"settings"`
}

func taxEvent(ctx context.Context, tx pgx.Tx, scope Scope, action, id string, snapshot any) error {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO franchise_tax_events(organization_id,location_id,actor_id,action,entity_id,snapshot) VALUES($1,$2,$3,$4,$5,$6)`, scope.OrganizationID, scope.LocationID, scope.ActorID, action, id, b)
	return err
}

func validTax(name string, bp int) (string, bool) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	return name, n >= 2 && n <= 60 && bp >= 0 && bp <= 5000
}

// FranchiseTaxes reads the franchise's tax rates and settings. Reading needs invoices.read; changing needs invoices.configure.
func (s *Store) FranchiseTaxes(ctx context.Context, scope Scope) (FranchiseTaxes, error) {
	out := FranchiseTaxes{Rates: []TaxRate{}, Settings: TaxSettings{Version: 0}}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.read"); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,rate_bp,active,version FROM franchise_tax_rates WHERE organization_id=$1 AND location_id=$2 ORDER BY lower(name)`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var r TaxRate
		if err = rows.Scan(&r.ID, &r.Name, &r.RateBP, &r.Active, &r.Version); err != nil {
			rows.Close()
			return out, err
		}
		out.Rates = append(out.Rates, r)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `SELECT prices_include_tax,rounding,version FROM franchise_tax_settings WHERE organization_id=$1 AND location_id=$2`, scope.OrganizationID, scope.LocationID).Scan(&out.Settings.PricesIncludeTax, &out.Settings.Rounding, &out.Settings.Version)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (s *Store) CreateTaxRate(ctx context.Context, scope Scope, name string, bp int) (TaxRate, error) {
	name, ok := validTax(name, bp)
	if !ok {
		return TaxRate{}, ErrTaxInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TaxRate{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.configure"); err != nil {
		return TaxRate{}, err
	}
	r := TaxRate{Name: name, RateBP: bp, Active: true, Version: 1}
	err = tx.QueryRow(ctx, `INSERT INTO franchise_tax_rates(organization_id,location_id,name,rate_bp) VALUES($1,$2,$3,$4) RETURNING id::text`, scope.OrganizationID, scope.LocationID, name, bp).Scan(&r.ID)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return TaxRate{}, ErrTaxConflict
	}
	if err != nil {
		return TaxRate{}, err
	}
	if err = taxEvent(ctx, tx, scope, "rate_created", r.ID, r); err != nil {
		return TaxRate{}, err
	}
	return r, tx.Commit(ctx)
}

func (s *Store) UpdateTaxRate(ctx context.Context, scope Scope, id, name string, bp int, active bool, version int) (TaxRate, error) {
	name, ok := validTax(name, bp)
	if !ok || version < 1 {
		return TaxRate{}, ErrTaxInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TaxRate{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.configure"); err != nil {
		return TaxRate{}, err
	}
	r := TaxRate{ID: id, Name: name, RateBP: bp, Active: active, Version: version + 1}
	tag, err := tx.Exec(ctx, `UPDATE franchise_tax_rates SET name=$4,rate_bp=$5,active=$6,version=version+1,updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND version=$7`, scope.OrganizationID, scope.LocationID, id, name, bp, active, version)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return TaxRate{}, ErrTaxConflict
	}
	if err != nil {
		return TaxRate{}, err
	}
	if tag.RowsAffected() != 1 {
		return TaxRate{}, ErrTaxConflict
	}
	if err = taxEvent(ctx, tx, scope, "rate_updated", id, r); err != nil {
		return TaxRate{}, err
	}
	return r, tx.Commit(ctx)
}

func (s *Store) DeleteTaxRate(ctx context.Context, scope Scope, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.configure"); err != nil {
		return err
	}
	var r TaxRate
	err = tx.QueryRow(ctx, `DELETE FROM franchise_tax_rates WHERE organization_id=$1 AND location_id=$2 AND id=$3 RETURNING id::text,name,rate_bp,active,version`, scope.OrganizationID, scope.LocationID, id).Scan(&r.ID, &r.Name, &r.RateBP, &r.Active, &r.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTaxConflict
	}
	if err != nil {
		return err
	}
	if err = taxEvent(ctx, tx, scope, "rate_deleted", id, r); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SaveTaxSettings(ctx context.Context, scope Scope, includeTax bool, rounding string, version int) (TaxSettings, error) {
	if rounding != "" && rounding != "half_up" && rounding != "half_even" && rounding != "down" || version < 0 {
		return TaxSettings{}, ErrTaxInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TaxSettings{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "invoices.configure"); err != nil {
		return TaxSettings{}, err
	}
	var current int
	err = tx.QueryRow(ctx, `SELECT version FROM franchise_tax_settings WHERE organization_id=$1 AND location_id=$2 FOR UPDATE`, scope.OrganizationID, scope.LocationID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		current = 0
	} else if err != nil {
		return TaxSettings{}, err
	}
	if current != version {
		return TaxSettings{}, ErrTaxConflict
	}
	var r *string
	if rounding != "" {
		r = &rounding
	}
	out := TaxSettings{PricesIncludeTax: includeTax, Rounding: r, Version: current + 1}
	_, err = tx.Exec(ctx, `INSERT INTO franchise_tax_settings(organization_id,location_id,prices_include_tax,rounding,version) VALUES($1,$2,$3,$4,$5)
 ON CONFLICT(organization_id,location_id) DO UPDATE SET prices_include_tax=excluded.prices_include_tax,rounding=excluded.rounding,version=excluded.version,updated_at=now()`, scope.OrganizationID, scope.LocationID, includeTax, r, out.Version)
	if err != nil {
		return TaxSettings{}, err
	}
	if err = taxEvent(ctx, tx, scope, "settings_saved", scope.LocationID, out); err != nil {
		return TaxSettings{}, err
	}
	return out, tx.Commit(ctx)
}
