package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var (
	ErrMemberSelf      = errors.New("you cannot terminate your own account")
	ErrMemberLastAdmin = errors.New("at least one active administrator must remain")
	ErrMemberNotFound  = errors.New("team member not found")
)

// SetMemberActive terminates (active=false) or reinstates a team member. Termination blocks sign-in at once and ends every
// session; everything the person did stays in the records. It needs access.manage, cannot be used on yourself, and cannot
// remove the last active administrator.
func (s *Store) SetMemberActive(ctx context.Context, actor, org, member string, active bool) error {
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
	if err = setMemberActiveTx(ctx, tx, actor, org, member, active); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func setMemberActiveTx(ctx context.Context, tx pgx.Tx, actor, org, member string, active bool) error {
	var err error
	if !active && actor == member {
		return ErrMemberSelf
	}
	var found bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_memberships WHERE organization_id=$1 AND principal_id=$2)`, org, member).Scan(&found); err != nil {
		return err
	}
	if !found {
		return ErrMemberNotFound
	}
	if !active {
		var others int
		if err = tx.QueryRow(ctx, `SELECT count(DISTINCT p.id) FROM principals p JOIN organization_memberships m ON m.principal_id=p.id AND m.organization_id=$1
 WHERE p.active AND p.id<>$2 AND (EXISTS(SELECT 1 FROM organization_grants g WHERE g.principal_id=p.id AND g.organization_id=$1 AND g.action='access.manage')
  OR EXISTS(SELECT 1 FROM member_role_assignments a JOIN role_capabilities rc ON rc.role_id=a.role_id WHERE a.principal_id=p.id AND a.organization_id=$1 AND a.scope_type='organization' AND a.scope_id=$1 AND rc.action='access.manage'))`, org, member).Scan(&others); err != nil {
			return err
		}
		if others < 1 {
			return ErrMemberLastAdmin
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE principals SET active=$2 WHERE id=$1`, member, active); err != nil {
		return err
	}
	if !active {
		if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE principal_id=$1`, member); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM password_reset_tokens WHERE principal_id=$1 AND used_at IS NULL`, member); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	return nil
}
