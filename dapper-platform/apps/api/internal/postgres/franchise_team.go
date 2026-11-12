package postgres

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// FranchiseEmployee is one person with the Employee role at a franchise.
type FranchiseEmployee struct {
	PrincipalID string `json:"principalId"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Active      bool   `json:"active"`
}

// FranchiseInvitation is an employee invitation still waiting to be accepted.
type FranchiseInvitation struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	DeliveryState string    `json:"deliveryState"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// FranchiseTeam is what a franchise owner manages: their own employees and the invitations they have open.
type FranchiseTeam struct {
	Employees   []FranchiseEmployee   `json:"employees"`
	Invitations []FranchiseInvitation `json:"invitations"`
}

// A franchise owner is whoever can manage attendance at that franchise; the Owner and Super Admin hold it everywhere.
const franchiseTeamAction = "attendance.manage"

func (s *Store) FranchiseTeam(ctx context.Context, actor, org, loc string) (FranchiseTeam, error) {
	out := FranchiseTeam{Employees: []FranchiseEmployee{}, Invitations: []FranchiseInvitation{}}
	if err := s.authorize(ctx, Scope{OrganizationID: org, LocationID: loc, ActorID: actor}, franchiseTeamAction); err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT p.id::text,coalesce(NULLIF(btrim(mp.full_name),''),''),coalesce(c.email,''),coalesce(mp.phone,''),p.active FROM member_role_assignments a JOIN roles r ON r.id=a.role_id AND lower(r.name)='employee' JOIN principals p ON p.id=a.principal_id LEFT JOIN password_credentials c ON c.principal_id=p.id LEFT JOIN member_profiles mp ON mp.principal_id=p.id WHERE a.organization_id=$1 AND a.scope_type='location' AND a.scope_id=$2 ORDER BY 2,3 LIMIT 200`, org, loc)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var e FranchiseEmployee
		if err = rows.Scan(&e.PrincipalID, &e.Name, &e.Email, &e.Phone, &e.Active); err != nil {
			return out, err
		}
		out.Employees = append(out.Employees, e)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT i.id::text,i.email,coalesce(d.state,'queued'),i.expires_at FROM member_invitations i LEFT JOIN member_invitation_deliveries d ON d.invitation_id=i.id WHERE i.organization_id=$1 AND i.preset_location_id=$2 AND i.status='pending' AND i.expires_at>now() ORDER BY i.expires_at DESC LIMIT 100`, org, loc)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var i FranchiseInvitation
		if err = rows.Scan(&i.ID, &i.Email, &i.DeliveryState, &i.ExpiresAt); err != nil {
			return out, err
		}
		out.Invitations = append(out.Invitations, i)
	}
	return out, rows.Err()
}

// franchiseEmployeeGate allows a franchise owner to change only plain Employees of their own franchise, never anyone holding a wider role.
func franchiseEmployeeGate(ctx context.Context, tx pgx.Tx, actor, org, loc, member string) error {
	if err := authorizeTransaction(ctx, tx, Scope{OrganizationID: org, LocationID: loc, ActorID: actor}, franchiseTeamAction); err != nil {
		return err
	}
	var plain bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_role_assignments a JOIN roles r ON r.id=a.role_id WHERE a.organization_id=$1 AND a.principal_id=$2 AND a.scope_type='location' AND a.scope_id=$3 AND lower(r.name)='employee')
 AND NOT EXISTS(SELECT 1 FROM member_role_assignments a JOIN roles r ON r.id=a.role_id WHERE a.organization_id=$1 AND a.principal_id=$2 AND lower(r.name)<>'employee')
 AND NOT EXISTS(SELECT 1 FROM organization_grants g WHERE g.organization_id=$1 AND g.principal_id=$2)`, org, member, loc).Scan(&plain)
	if err != nil {
		return err
	}
	if !plain || actor == member {
		return ErrForbidden
	}
	return nil
}

// UpdateFranchiseEmployee corrects an employee's name and phone.
func (s *Store) UpdateFranchiseEmployee(ctx context.Context, actor, org, loc, member, name, phone string) error {
	name, phone = strings.TrimSpace(name), strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(phone)
	if n := utf8.RuneCountInString(name); n < 2 || n > 120 || (phone != "" && !employeePhone.MatchString(phone)) {
		return ErrProfileInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = franchiseEmployeeGate(ctx, tx, actor, org, loc, member); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_profiles(principal_id,full_name,phone) VALUES($1,$2,$3) ON CONFLICT(principal_id) DO UPDATE SET full_name=excluded.full_name,phone=excluded.phone,updated_at=clock_timestamp()`, member, name, phone); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_security_events(principal_id,action) VALUES($1,'profile_saved')`, member); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetFranchiseEmployeeActive removes an employee (they are signed out and cannot sign in; their history stays) or brings them back.
func (s *Store) SetFranchiseEmployeeActive(ctx context.Context, actor, org, loc, member string, active bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = franchiseEmployeeGate(ctx, tx, actor, org, loc, member); err != nil {
		return err
	}
	if err = setMemberActiveTx(ctx, tx, actor, org, member, active); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CreateEmployeeInvitation lets a franchise owner invite someone as an Employee of their own franchise. The role and franchise are fixed here,
// so it can never hand out more than that.
func (s *InvitationService) CreateEmployeeInvitation(ctx context.Context, actor, org, loc, email string) (MemberInvitation, error) {
	var role string
	err := s.store.pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE organization_id=$1 AND lower(name)='employee'`, org).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return MemberInvitation{}, ErrInvitationInput
	}
	if err != nil {
		return MemberInvitation{}, err
	}
	return s.create(ctx, actor, org, email, 24*time.Hour, &InvitationRole{RoleID: role, LocationID: loc}, func(tx pgx.Tx) error {
		return authorizeTransaction(ctx, tx, Scope{OrganizationID: org, LocationID: loc, ActorID: actor}, franchiseTeamAction)
	})
}

// RevokeEmployeeInvitation withdraws an invitation to this franchise only.
func (s *InvitationService) RevokeEmployeeInvitation(ctx context.Context, actor, org, loc, id string) error {
	return s.revoke(ctx, actor, org, id, func(tx pgx.Tx) error {
		if err := authorizeTransaction(ctx, tx, Scope{OrganizationID: org, LocationID: loc, ActorID: actor}, franchiseTeamAction); err != nil {
			return err
		}
		var here bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_invitations WHERE organization_id=$1 AND id=$2 AND preset_location_id=$3)`, org, id, loc).Scan(&here); err != nil {
			return err
		}
		if !here {
			return ErrForbidden
		}
		return nil
	})
}
