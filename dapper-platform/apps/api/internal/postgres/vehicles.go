package postgres

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

type Vehicle struct {
	ID                  string `json:"id"`
	CustomerID          string `json:"customerId"`
	Description         string `json:"description"`
	CategoryDescription string `json:"categoryDescription"`
	Identifier          string `json:"identifier"`
}

var ErrVehicle = errors.New("invalid vehicle description or customer link")

func (s *Store) CreateVehicle(ctx context.Context, scope Scope, input Vehicle) (Vehicle, error) {
	if err := s.authorize(ctx, scope, "vehicles.create"); err != nil {
		return Vehicle{}, err
	}
	input.Description = strings.TrimSpace(input.Description)
	input.CategoryDescription = strings.TrimSpace(input.CategoryDescription)
	input.Identifier = strings.TrimSpace(input.Identifier)
	size := utf8.RuneCountInString(input.Description)
	if input.CustomerID == "" || size < 2 || size > 120 || utf8.RuneCountInString(input.CategoryDescription) > 60 || utf8.RuneCountInString(input.Identifier) > 60 {
		return Vehicle{}, ErrVehicle
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Vehicle{}, err
	}
	defer tx.Rollback(ctx)
	var v Vehicle
	// The composite FK enforces customer ownership within this exact scope.
	err = tx.QueryRow(ctx, `INSERT INTO vehicles(organization_id,location_id,customer_id,description,category_description,identifier) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text,customer_id::text,description,category_description,identifier`, scope.OrganizationID, scope.LocationID, input.CustomerID, input.Description, input.CategoryDescription, input.Identifier).Scan(&v.ID, &v.CustomerID, &v.Description, &v.CategoryDescription, &v.Identifier)
	if err != nil {
		return Vehicle{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'vehicle.created',$4)`, scope.OrganizationID, scope.LocationID, scope.ActorID, v.ID); err != nil {
		return Vehicle{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Vehicle{}, err
	}
	return v, nil
}

func (s *Store) Vehicles(ctx context.Context, scope Scope) ([]Vehicle, error) {
	if err := s.authorize(ctx, scope, "vehicles.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,customer_id::text,description,category_description,identifier FROM vehicles WHERE organization_id=$1 AND location_id=$2 ORDER BY created_at,id LIMIT 100`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Vehicle{}
	for rows.Next() {
		var v Vehicle
		if err = rows.Scan(&v.ID, &v.CustomerID, &v.Description, &v.CategoryDescription, &v.Identifier); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
