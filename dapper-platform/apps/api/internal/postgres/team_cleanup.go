package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrInvitationStillOpen: a pending invitation that has not expired must be revoked before it can be removed from the list.
var ErrInvitationStillOpen = errors.New("the invitation is still open")

// HideMemberInvitation takes an accepted, revoked or expired invitation off the Team list (error404 #91). An expired pending one is
// revoked at the same time so it cannot block a new invitation to that address. The row stays as history. Needs access.manage.
func (s *Store) HideMemberInvitation(ctx context.Context, actor, org, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = accessManagementGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	var status string
	var expired bool
	err = tx.QueryRow(ctx, `SELECT status,expires_at<now() FROM member_invitations WHERE organization_id=$1 AND id::text=$2 FOR UPDATE`, org, id).Scan(&status, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvitationUnavailable
	}
	if err != nil {
		return err
	}
	if status == "pending" && !expired {
		return ErrInvitationStillOpen
	}
	if _, err = tx.Exec(ctx, `UPDATE member_invitations SET hidden_at=now(),status=CASE WHEN status='pending' THEN 'revoked' ELSE status END WHERE organization_id=$1 AND id::text=$2`, org, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemoveMember takes a person off the team for good (error404 #90): they are terminated (sign-in blocked, sessions ended, the last
// administrator and yourself protected, as SetMemberActive), then their roles, grants, workspace assignment and membership are removed,
// so they leave the Team and Employees lists. What they did stays in the records under their name. Needs access.manage.
func (s *Store) RemoveMember(ctx context.Context, actor, org, member string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ok, err := actorHasOrgCapability(ctx, tx, actor, org, "access.manage")
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if err = setMemberActiveTx(ctx, tx, actor, org, member, false); err != nil {
		return err
	}
	steps := []string{
		`INSERT INTO role_assignment_events(organization_id,actor_id,principal_id,role_id,scope_type,scope_id,action,version) SELECT organization_id,$3,principal_id,role_id,scope_type,scope_id,'revoked',version FROM member_role_assignments WHERE organization_id=$1 AND principal_id=$2`,
		`DELETE FROM member_role_assignments WHERE organization_id=$1 AND principal_id=$2 AND $3::uuid IS NOT NULL`,
		`DELETE FROM location_grants WHERE organization_id=$1 AND principal_id=$2 AND $3::uuid IS NOT NULL`,
		`DELETE FROM organization_grants WHERE organization_id=$1 AND principal_id=$2 AND $3::uuid IS NOT NULL`,
		`DELETE FROM employee_worksite_assignments WHERE organization_id=$1 AND principal_id=$2 AND $3::uuid IS NOT NULL`,
		`DELETE FROM organization_memberships WHERE organization_id=$1 AND principal_id=$2 AND $3::uuid IS NOT NULL`,
	}
	for _, q := range steps {
		if _, err = tx.Exec(ctx, q, org, member, actor); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
