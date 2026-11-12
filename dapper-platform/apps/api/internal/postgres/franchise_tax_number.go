package postgres

import "context"

// franchiseTaxNumber is the franchise's own GST/HST number, or empty when it uses the one on its business details. Business details can be
// shared by several outlets, so the number that belongs to one outlet is kept on the outlet.
func franchiseTaxNumber(ctx context.Context, q rowQuerier, org, loc string) (string, error) {
	var n string
	err := q.QueryRow(ctx, `SELECT tax_registration FROM locations WHERE organization_id=$1 AND id=$2`, org, loc).Scan(&n)
	return n, err
}
