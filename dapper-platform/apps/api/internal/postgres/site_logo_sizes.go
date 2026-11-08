package postgres

import "context"

// SetLogoSize records how large one dealership logo is shown on the website.
func (s *Store) SetLogoSize(ctx context.Context, scope Scope, logo, size string) error {
	logo, ok := cleanAssetPath(logo)
	if !ok || (size != "s" && size != "m" && size != "l" && size != "xl") {
		return ErrSiteAsset
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "content.manage"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO site_logo_sizes(organization_id,location_id,logo,size) VALUES($1,$2,$3,$4) ON CONFLICT(organization_id,location_id,logo) DO UPDATE SET size=$4`, scope.OrganizationID, scope.LocationID, logo, size); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) logoSizes(ctx context.Context, q string, args ...any) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var logo, size string
		if err = rows.Scan(&logo, &size); err != nil {
			return nil, err
		}
		out[logo] = size
	}
	return out, rows.Err()
}

// LogoSizes is this location's chosen sizes, for the dashboard.
func (s *Store) LogoSizes(ctx context.Context, scope Scope) (map[string]string, error) {
	return s.logoSizes(ctx, `SELECT logo,size FROM site_logo_sizes WHERE organization_id=$1 AND location_id=$2`, scope.OrganizationID, scope.LocationID)
}

// PublicLogoSizes is the main website's chosen sizes.
func (s *Store) PublicLogoSizes(ctx context.Context) (map[string]string, error) {
	return s.logoSizes(ctx, `SELECT a.logo,a.size FROM site_logo_sizes a`+onMainSite, mainSiteSlug())
}
