package postgres

import "context"

// PersonRow is one organization member as an administrator needs to see them: who they are, where they work and
// what is left of onboarding. It never carries secrets (no MFA secret, recovery codes or document contents).
type PersonRow struct {
	PrincipalID      string   `json:"principalId"`
	Email            string   `json:"email"`
	Active           bool     `json:"active"`
	FullName         string   `json:"fullName"`
	Phone            string   `json:"phone"`
	ProfileComplete  bool     `json:"profileComplete"`
	Administrator    bool     `json:"administrator"`
	MFAEnabled       bool     `json:"mfaEnabled"`
	MFASetupStarted  bool     `json:"mfaSetupStarted"`
	LocationID       string   `json:"locationId"`
	LocationName     string   `json:"locationName"`
	LocationCode     string   `json:"locationCode"`
	DealershipName   string   `json:"dealershipName"`
	Roles            []string `json:"roles"`
	DocumentsTotal   int      `json:"documentsTotal"`
	DocumentsAcked   int      `json:"documentsAcknowledged"`
	OnboardingStatus string   `json:"onboardingStatus"` // complete | incomplete | not_assigned | inactive
	Missing          []string `json:"missing"`          // details, mfa, documents, franchise
}

// PeopleRoster lists up to 200 members of the organization with onboarding state. Requires the same
// access-management read authority as the access screen.
func (s *Store) PeopleRoster(ctx context.Context, actor, org string) ([]PersonRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = accessReadGrant(ctx, tx, actor, org); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT p.id::text, COALESCE(c.email,''), p.active,
 COALESCE(mp.full_name,''), COALESCE(mp.phone,''), mp.principal_id IS NOT NULL,
 EXISTS(SELECT 1 FROM organization_grants og WHERE og.principal_id=p.id),
 COALESCE(mfa.enabled_at IS NOT NULL,false), mfa.principal_id IS NOT NULL,
 COALESCE(w.location_id::text,''), COALESCE(l.name,''), COALESCE(l.code,''), COALESCE(da.account_name,''),
 COALESCE((SELECT array_agg(DISTINCT r.name ORDER BY r.name) FROM member_role_assignments a JOIN roles r ON r.id=a.role_id WHERE a.organization_id=$1 AND a.principal_id=p.id),'{}'),
 (SELECT count(*) FROM sop_documents d WHERE d.organization_id=$1 AND NOT d.archived),
 (SELECT count(*) FROM sop_acknowledgments k JOIN sop_documents d ON d.id=k.sop_document_id AND NOT d.archived WHERE d.organization_id=$1 AND k.employee_id=p.id AND k.location_id=w.location_id AND k.training_track=$2)
FROM organization_memberships m
JOIN principals p ON p.id=m.principal_id
LEFT JOIN password_credentials c ON c.principal_id=p.id
LEFT JOIN member_profiles mp ON mp.principal_id=p.id
LEFT JOIN member_mfa mfa ON mfa.principal_id=p.id
LEFT JOIN employee_worksite_assignments w ON w.principal_id=p.id AND w.organization_id=m.organization_id
LEFT JOIN locations l ON l.id=w.location_id AND l.organization_id=m.organization_id
LEFT JOIN dealership_accounts da ON da.id=w.dealership_id AND da.organization_id=w.organization_id AND da.location_id=w.location_id
WHERE m.organization_id=$1 AND p.external_subject NOT LIKE 'system:%'
ORDER BY c.email, p.id LIMIT 200`, org, employeeAckTrack)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonRow{}
	mfaOn := s.MFAConfigured()
	for rows.Next() {
		var r PersonRow
		var hasMFARow bool
		if err = rows.Scan(&r.PrincipalID, &r.Email, &r.Active, &r.FullName, &r.Phone, &r.ProfileComplete, &r.Administrator,
			&r.MFAEnabled, &hasMFARow, &r.LocationID, &r.LocationName, &r.LocationCode, &r.DealershipName, &r.Roles,
			&r.DocumentsTotal, &r.DocumentsAcked); err != nil {
			return nil, err
		}
		r.MFASetupStarted = hasMFARow && !r.MFAEnabled
		r.Missing = []string{}
		switch {
		case !r.Active:
			r.OnboardingStatus = "inactive"
		case r.LocationID == "":
			r.OnboardingStatus = "not_assigned"
			r.Missing = append(r.Missing, "franchise")
		default:
			if !r.ProfileComplete {
				r.Missing = append(r.Missing, "details")
			}
			if mfaOn && r.Administrator && !r.MFAEnabled {
				r.Missing = append(r.Missing, "mfa")
			}
			if r.DocumentsAcked < r.DocumentsTotal {
				r.Missing = append(r.Missing, "documents")
			}
			r.OnboardingStatus = "complete"
			if len(r.Missing) > 0 {
				r.OnboardingStatus = "incomplete"
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
