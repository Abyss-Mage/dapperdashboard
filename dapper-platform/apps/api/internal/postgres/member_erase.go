package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ErrMemberElsewhere: the person also belongs to another organization, so their sign-in and profile cannot be erased from this one.
var ErrMemberElsewhere = errors.New("the person belongs to another organization")

// EraseMember deletes a team member completely (owner 2026-11-14, error404 #118, "Everything personal"). First they are removed from the
// team as RemoveMember (sign-in blocked, last administrator and yourself protected). Then their sign-in, password, two-factor, sessions,
// reset links, calendar feeds, shop clock PIN, customer-portal links, invitations, availability, uploaded photos (files too), their own
// clock-in history with its GPS points, and their security history are erased; the profile is renamed "Deleted user" with no phone, and
// the activity log keeps the entries without their email, address or browser. Bookings, invoices, payments and work records stay and
// show "Deleted user". Owner and administrators only (access.manage), after typing the person's email (or name if they have none).
func (s *Store) EraseMember(ctx context.Context, actor, org, member, typed string) error {
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
	var email, name string
	var elsewhere bool
	err = tx.QueryRow(ctx, `SELECT coalesce(c.email,''), coalesce(mp.full_name,''),
	 EXISTS(SELECT 1 FROM organization_memberships o WHERE o.principal_id=m.principal_id AND o.organization_id<>m.organization_id)
	 FROM organization_memberships m LEFT JOIN password_credentials c ON c.principal_id=m.principal_id LEFT JOIN member_profiles mp ON mp.principal_id=m.principal_id
	 WHERE m.organization_id=$1 AND m.principal_id::text=$2`, org, member).Scan(&email, &name, &elsewhere)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMemberNotFound
	}
	if err != nil {
		return err
	}
	want := email
	if want == "" {
		want = name
	}
	if want == "" || !strings.EqualFold(strings.TrimSpace(typed), strings.TrimSpace(want)) {
		return ErrPurgeConfirm
	}
	if elsewhere {
		return ErrMemberElsewhere
	}
	if err = removeMemberTx(ctx, tx, actor, org, member); err != nil {
		return err
	}
	var keys []string
	if err = tx.QueryRow(ctx, `SELECT coalesce(array_agg(f.storage_key),'{}') FROM employee_upload_photos f JOIN employee_upload_posts p ON p.id=f.post_id WHERE p.principal_id::text=$1`, member).Scan(&keys); err != nil {
		return err
	}
	steps := []string{
		`DELETE FROM password_credentials WHERE principal_id::text=$1`,
		`DELETE FROM password_reset_tokens WHERE principal_id::text=$1`,
		`DELETE FROM auth_sessions WHERE principal_id::text=$1`,
		`DELETE FROM oidc_identities WHERE principal_id::text=$1`,
		`DELETE FROM member_mfa_recovery_codes WHERE principal_id::text=$1`,
		`DELETE FROM member_mfa WHERE principal_id::text=$1`,
		`DELETE FROM member_security_events WHERE principal_id::text=$1`,
		`DELETE FROM calendar_feeds WHERE principal_id::text=$1`,
		`DELETE FROM employee_clock_pins WHERE principal_id::text=$1`,
		`DELETE FROM customer_account_links WHERE principal_id::text=$1`,
		`DELETE FROM employee_availability_submissions WHERE principal_id::text=$1`,
		`DELETE FROM employee_upload_posts WHERE principal_id::text=$1`, // photos go with it (ON DELETE CASCADE)
		`DELETE FROM gps_samples WHERE principal_id::text=$1`,
		`DELETE FROM gps_shift_state WHERE principal_id::text=$1`,
		`DELETE FROM employee_attendance_events WHERE principal_id::text=$1`,
		`DELETE FROM employee_attendance WHERE principal_id::text=$1`,
		`UPDATE member_invitations SET email='',hidden_at=coalesce(hidden_at,now()),status=CASE WHEN status='pending' THEN 'revoked' ELSE status END
		 WHERE (accepted_by::text=$1 OR ($2<>'' AND email=$2))`,
		`UPDATE activity_log SET email='',ip='',user_agent='' WHERE principal_id::text=$1 OR ($2<>'' AND lower(email)=$2)`,
		`UPDATE member_profiles SET full_name='Deleted user',phone='',updated_at=clock_timestamp() WHERE principal_id::text=$1`,
		`INSERT INTO member_profiles(principal_id,full_name,phone) SELECT id,'Deleted user','' FROM principals WHERE id::text=$1 ON CONFLICT DO NOTHING`,
		`UPDATE principals SET external_subject='deleted:'||id::text,active=false WHERE id::text=$1`,
	}
	for _, q := range steps {
		args := []any{member}
		if strings.Contains(q, "$2") {
			args = append(args, strings.ToLower(email))
		}
		if _, err = tx.Exec(ctx, q, args...); err != nil {
			return fmt.Errorf("erase stopped at %q: %w", strings.Fields(q)[2], err)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,actor_id,action,entity_id) VALUES($1,$2,'member.erased',$3)`, org, actor, member); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if s.uploadDir != "" {
		for _, key := range keys {
			_ = os.Remove(filepath.Join(s.uploadDir, key))
		}
	}
	return nil
}
