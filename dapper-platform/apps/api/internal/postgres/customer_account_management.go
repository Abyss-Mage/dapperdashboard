package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrCustomerAccountReview = errors.New("explicit customer account ownership review required")

type ManagedCustomerAccountLink struct {
	CustomerID   string    `json:"customerId"`
	PrincipalID  string    `json:"principalId"`
	CustomerName string    `json:"customerName"`
	Email        string    `json:"email"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"createdAt"`
}
type CustomerAccountManagement struct {
	Customers []CustomerAccountChoice      `json:"customers"`
	Members   []AccessMember               `json:"members"`
	Links     []ManagedCustomerAccountLink `json:"links"`
	Limit     int                          `json:"limit"`
}
type CustomerAccountChoice struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func customerAccountManagementGrant(ctx context.Context, tx pgx.Tx, scope Scope) error {
	if err := authorizeTransaction(ctx, tx, scope, "customer_accounts.manage"); err != nil {
		return err
	}
	return accessManagementGrant(ctx, tx, scope.ActorID, scope.OrganizationID)
}

func (s *Store) ManagedCustomerAccounts(ctx context.Context, scope Scope) (CustomerAccountManagement, error) {
	result := CustomerAccountManagement{Customers: []CustomerAccountChoice{}, Members: []AccessMember{}, Links: []ManagedCustomerAccountLink{}, Limit: 100}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if err = customerAccountManagementGrant(ctx, tx, scope); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,email FROM customers WHERE organization_id=$1 AND location_id=$2 AND removed_at IS NULL ORDER BY name,id LIMIT 100`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var customer CustomerAccountChoice
		if err = rows.Scan(&customer.ID, &customer.Name, &customer.Email); err != nil {
			rows.Close()
			return result, err
		}
		result.Customers = append(result.Customers, customer)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = tx.Query(ctx, `SELECT p.id::text,c.email,p.active FROM organization_memberships m JOIN principals p ON p.id=m.principal_id JOIN password_credentials c ON c.principal_id=p.id WHERE m.organization_id=$1 ORDER BY c.email,p.id LIMIT 100`, scope.OrganizationID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var member AccessMember
		if err = rows.Scan(&member.PrincipalID, &member.Email, &member.Active); err != nil {
			rows.Close()
			return result, err
		}
		result.Members = append(result.Members, member)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = tx.Query(ctx, `SELECT a.customer_id::text,a.principal_id::text,c.name,coalesce(pc.email,''),p.active,a.created_at FROM customer_account_links a JOIN customers c ON (c.organization_id,c.location_id,c.id)=(a.organization_id,a.location_id,a.customer_id) JOIN principals p ON p.id=a.principal_id LEFT JOIN password_credentials pc ON pc.principal_id=p.id WHERE a.organization_id=$1 AND a.location_id=$2 ORDER BY c.name,a.customer_id,a.principal_id LIMIT 100`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var link ManagedCustomerAccountLink
		if err = rows.Scan(&link.CustomerID, &link.PrincipalID, &link.CustomerName, &link.Email, &link.Active, &link.CreatedAt); err != nil {
			rows.Close()
			return result, err
		}
		result.Links = append(result.Links, link)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) SetCustomerAccountLink(ctx context.Context, scope Scope, customer, principal string, enabled, ownershipReviewed bool) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = customerAccountManagementGrant(ctx, tx, scope); err != nil {
		return err
	}
	if enabled && !ownershipReviewed {
		return ErrCustomerAccountReview
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.OrganizationID+"/"+scope.LocationID+"/customer-account/"+customer+"/"+principal); err != nil {
		return err
	}
	var found string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, scope.OrganizationID, scope.LocationID, customer).Scan(&found); errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	} else if err != nil {
		return err
	}
	var changed int64
	if enabled {
		// Enrollment and organization membership are explicit prerequisites. Matching
		// customer email, a staff role or caller-supplied identity is never evidence.
		if err = tx.QueryRow(ctx, `SELECT p.id::text FROM principals p JOIN organization_memberships m ON m.principal_id=p.id AND m.organization_id=$2 JOIN password_credentials c ON c.principal_id=p.id WHERE p.id=$1 AND p.active FOR SHARE OF p,m,c`, principal, scope.OrganizationID).Scan(&found); errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		} else if err != nil {
			return err
		}
		tag, e := tx.Exec(ctx, `INSERT INTO customer_account_links(principal_id,organization_id,location_id,customer_id) VALUES($1,$2,$3,$4) ON CONFLICT(principal_id,customer_id) DO NOTHING`, principal, scope.OrganizationID, scope.LocationID, customer)
		if e != nil {
			return e
		}
		changed = tag.RowsAffected()
	} else {
		// Revocation remains possible after the target's membership or credential was
		// removed. Never require the old owner to be eligible for a new grant.
		tag, e := tx.Exec(ctx, `DELETE FROM customer_account_links WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 AND principal_id=$4`, scope.OrganizationID, scope.LocationID, customer, principal)
		if e != nil {
			return e
		}
		changed = tag.RowsAffected()
	}
	if changed > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO customer_account_events(organization_id,location_id,customer_id,principal_id,actor_id,enabled,ownership_reviewed) VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.OrganizationID, scope.LocationID, customer, principal, scope.ActorID, enabled, enabled && ownershipReviewed); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
