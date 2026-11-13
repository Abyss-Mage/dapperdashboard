package postgres

import (
	"context"
	"strings"
)

// EmployeeFindCustomers lets workspace staff find a returning customer at their franchise by email (or phone when no email is typed),
// so the walk-in form can be filled in (error404 #100). At most five, and only with customers.read there.
func (s *Store) EmployeeFindCustomers(ctx context.Context, actor, query string) ([]Customer, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	if err = authorizeTransaction(ctx, tx, employeeScope(a), "customers.read"); err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	out := []Customer{}
	if len(q) < 3 {
		return out, nil
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,email,phone FROM customers WHERE organization_id=$1 AND location_id=$2 AND removed_at IS NULL
 AND CASE WHEN position('@' in $3)>0 OR $3 ~ '[a-z]' THEN position($3 in lower(email))>0
  ELSE length(regexp_replace($3,'\D','','g'))>=4 AND position(regexp_replace($3,'\D','','g') in regexp_replace(phone,'\D','','g'))>0 END
 ORDER BY lower(email)=$3 DESC,name LIMIT 5`, a.OrganizationID, a.LocationID, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Customer
		if err = rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
