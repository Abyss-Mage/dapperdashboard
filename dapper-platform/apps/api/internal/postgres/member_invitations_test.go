package postgres

import (
	"strings"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"
)

func verifyMemberInvitations(t *testing.T, ctx context.Context, store *Store, a, b Scope) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	service, err := NewInvitationService(store, "https://127.0.0.1:8443", key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewInvitationService(store, "http://127.0.0.1:8443", key); err == nil {
		t.Fatal("unencrypted enrollment origin accepted")
	}
	if _, err = NewInvitationService(store, "https://127.0.0.1:8443", nil); err == nil {
		t.Fatal("missing encryption key accepted")
	}
	// A franchise owner invites Employees to their own franchise only; someone from elsewhere cannot invite or withdraw there.
	if _, err = store.pool.Exec(ctx, `INSERT INTO roles(organization_id,name) VALUES($1,'Employee') ON CONFLICT DO NOTHING`, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateEmployeeInvitation(ctx, b.ActorID, a.OrganizationID, a.LocationID, "crew@example.invalid"); !errors.Is(err, ErrForbidden) {
		t.Fatal("outsider invited into another franchise", err)
	}
	if _, err = store.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'attendance.manage') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	crew, err := service.CreateEmployeeInvitation(ctx, a.ActorID, a.OrganizationID, a.LocationID, "crew@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	team, err := store.FranchiseTeam(ctx, a.ActorID, a.OrganizationID, a.LocationID)
	if err != nil || len(team.Invitations) != 1 || team.Invitations[0].Email != "crew@example.invalid" {
		t.Fatalf("franchise team: %v %+v", err, team)
	}
	if _, err = store.FranchiseTeam(ctx, b.ActorID, a.OrganizationID, a.LocationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("outsider read another franchise's team", err)
	}
	if err = service.RevokeEmployeeInvitation(ctx, b.ActorID, a.OrganizationID, a.LocationID, crew.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("outsider withdrew another franchise's invitation", err)
	}
	if err = service.RevokeEmployeeInvitation(ctx, a.ActorID, a.OrganizationID, a.LocationID, crew.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, a.ActorID, a.OrganizationID, "invited@example.invalid", time.Hour); !errors.Is(err, ErrForbidden) {
		t.Fatal("invitation implied admin grant", err)
	}
	for _, scope := range []Scope{a, b} {
		if _, err = store.pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, scope.OrganizationID, scope.ActorID); err != nil {
			t.Fatal(err)
		}
		if _, err = store.pool.Exec(ctx, `INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'access.manage') ON CONFLICT DO NOTHING`, scope.OrganizationID, scope.ActorID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = service.List(ctx, b.ActorID, a.OrganizationID); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign invitation list accepted", err)
	}
	if _, err = service.Create(ctx, a.ActorID, a.OrganizationID, "bad", time.Hour); !errors.Is(err, ErrInvitationInput) {
		t.Fatal("invalid recipient accepted", err)
	}
	if _, err = service.Create(ctx, a.ActorID, a.OrganizationID, "invited@example.invalid", time.Second); !errors.Is(err, ErrInvitationInput) {
		t.Fatal("implicit/invalid expiry accepted", err)
	}
	secret := func(inv MemberInvitation) string {
		var ciphertext []byte
		if e := store.pool.QueryRow(ctx, `SELECT payload_ciphertext FROM member_invitation_deliveries WHERE invitation_id=$1`, inv.ID).Scan(&ciphertext); e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(ciphertext, []byte("invitation=")) || bytes.Contains(ciphertext, []byte(inv.Email)) {
			t.Fatal("delivery persisted plaintext secret or recipient")
		}
		size := service.seal.NonceSize()
		plain, e := service.seal.Open(nil, ciphertext[:size], ciphertext[size:], []byte(inv.OrganizationID+"/"+inv.ID))
		if e != nil {
			t.Fatal(e)
		}
		defer clear(plain)
		var payload map[string]string
		if e = json.Unmarshal(plain, &payload); e != nil {
			t.Fatal(e)
		}
		parsed, e := url.Parse(payload["enrollmentURL"])
		if e != nil || parsed.RawQuery != "" || parsed.Path != "/enroll" {
			t.Fatal("enrollment link leaks token outside fragment", e)
		}
		fragment, e := url.ParseQuery(parsed.Fragment)
		if e != nil {
			t.Fatal(e)
		}
		token := fragment.Get("invitation")
		if len(token) != 43 {
			t.Fatal("invalid token entropy/encoding")
		}
		metadata, e := json.Marshal(inv)
		if e != nil || bytes.Contains(metadata, []byte(token)) {
			t.Fatal("invitation response exposed raw token")
		}
		if _, e = service.seal.Open(nil, ciphertext[:size], ciphertext[size:], []byte("foreign-organization/"+inv.ID)); e == nil {
			t.Fatal("ciphertext accepted wrong organization binding")
		}
		return token
	}
	if _, err = store.pool.Exec(ctx, `ALTER TABLE member_invitation_events ADD CONSTRAINT isolated_create_failure CHECK(action<>'created') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, a.ActorID, a.OrganizationID, "creation-fails@example.invalid", time.Hour); err == nil {
		t.Fatal("creation audit failure committed")
	}
	var creationCount int
	if err = store.pool.QueryRow(ctx, `SELECT count(*) FROM member_invitations WHERE email='creation-fails@example.invalid'`).Scan(&creationCount); err != nil || creationCount != 0 {
		t.Fatal("failed create retained invitation", err)
	}
	if _, err = store.pool.Exec(ctx, `ALTER TABLE member_invitation_events DROP CONSTRAINT isolated_create_failure`); err != nil {
		t.Fatal(err)
	}
	createResults := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, e := service.Create(ctx, a.ActorID, a.OrganizationID, "concurrent-create@example.invalid", time.Hour)
			createResults <- e
		}()
	}
	created, pending := 0, 0
	for i := 0; i < 2; i++ {
		e := <-createResults
		if e == nil {
			created++
		} else if errors.Is(e, ErrInvitationPending) {
			pending++
		} else {
			t.Fatal("unexpected concurrent creation result", e)
		}
	}
	if created != 1 || pending != 1 {
		t.Fatal("duplicate concurrent invitation queue created")
	}
	invite, err := service.Create(ctx, a.ActorID, a.OrganizationID, " Invited.Member@example.invalid ", time.Hour)
	if err != nil || invite.DeliveryState != "queued" || invite.Email != "invited.member@example.invalid" {
		t.Fatal("invitation queue failed", err)
	}
	token := secret(invite)
	if _, err = service.Create(ctx, a.ActorID, a.OrganizationID, invite.Email, time.Hour); !errors.Is(err, ErrInvitationPending) {
		t.Fatal("duplicate pending invitation accepted", err)
	}
	password := "Isolated invitation password only"
	// An invitation sent by a franchise owner, who is not an organization administrator, can be accepted and starts the employee at that franchise.
	var franchiseOwner, role string
	if err = store.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic-franchise-owner') RETURNING id::text`).Scan(&franchiseOwner); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO organization_memberships(organization_id,principal_id) VALUES($2,$1)`,
		`INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'attendance.manage')`,
	} {
		args := []any{franchiseOwner, a.OrganizationID}
		if strings.Contains(q, "$3") {
			args = append(args, a.LocationID)
		}
		if _, err = store.pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE organization_id=$1 AND lower(name)='employee'`, a.OrganizationID).Scan(&role); err != nil {
		t.Fatal(err)
	}
	crewInvite, err := service.CreateEmployeeInvitation(ctx, franchiseOwner, a.OrganizationID, a.LocationID, "crew.accepts@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Accept(ctx, secret(crewInvite), password, ""); err != nil {
		t.Fatal("a franchise owner's invitation must be acceptable:", err)
	}
	var assigned, worksite int
	if err = store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM member_role_assignments a JOIN password_credentials c ON c.principal_id=a.principal_id WHERE c.email='crew.accepts@example.invalid' AND a.role_id=$1 AND a.scope_id=$2),(SELECT count(*) FROM employee_worksite_assignments w JOIN password_credentials c ON c.principal_id=w.principal_id WHERE c.email='crew.accepts@example.invalid' AND w.location_id=$2)`, role, a.LocationID).Scan(&assigned, &worksite); err != nil || assigned != 1 || worksite != 1 {
		t.Fatalf("employee not started at the franchise: %v role %d worksite %d", err, assigned, worksite)
	}
	// The franchise owner can correct and remove their own employee; someone elsewhere cannot, and nobody with a wider role can be touched.
	var crewMember string
	if err = store.pool.QueryRow(ctx, `SELECT principal_id::text FROM password_credentials WHERE email='crew.accepts@example.invalid'`).Scan(&crewMember); err != nil {
		t.Fatal(err)
	}
	if err = store.SetFranchiseEmployeeActive(ctx, b.ActorID, a.OrganizationID, a.LocationID, crewMember, false); !errors.Is(err, ErrForbidden) {
		t.Fatal("outsider removed another franchise's employee", err)
	}
	if err = store.UpdateFranchiseEmployee(ctx, franchiseOwner, a.OrganizationID, a.LocationID, crewMember, "Synthetic Crew", "+1 705 555 0142"); err != nil {
		t.Fatal(err)
	}
	if err = store.SetFranchiseEmployeeActive(ctx, franchiseOwner, a.OrganizationID, a.LocationID, crewMember, false); err != nil {
		t.Fatal(err)
	}
	listed, err := store.FranchiseTeam(ctx, franchiseOwner, a.OrganizationID, a.LocationID)
	if err != nil || len(listed.Employees) != 1 || listed.Employees[0].Active || listed.Employees[0].Name != "Synthetic Crew" || listed.Employees[0].Phone != "+17055550142" {
		t.Fatalf("franchise employee after edit and removal: %v %+v", err, listed.Employees)
	}
	if err = store.SetFranchiseEmployeeActive(ctx, franchiseOwner, a.OrganizationID, a.LocationID, a.ActorID, false); !errors.Is(err, ErrForbidden) {
		t.Fatal("franchise owner removed someone who is not their employee", err)
	}
	// Once they stop being that franchise's owner, an invitation they sent no longer works.
	staleCrew, err := service.CreateEmployeeInvitation(ctx, franchiseOwner, a.OrganizationID, a.LocationID, "crew.late@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1`, franchiseOwner); err != nil {
		t.Fatal(err)
	}
	if err = service.Accept(ctx, secret(staleCrew), password, ""); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatal("an invitation from someone who lost the franchise was accepted:", err)
	}
	if err = service.Accept(ctx, "invalid", password, ""); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatal("unknown token accepted", err)
	}
	if _, err = store.pool.Exec(ctx, `ALTER TABLE member_invitation_events ADD CONSTRAINT isolated_accept_failure CHECK(action<>'accepted') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err = service.Accept(ctx, token, password, ""); err == nil {
		t.Fatal("accept audit failure committed")
	}
	var count int
	if err = store.pool.QueryRow(ctx, `SELECT count(*) FROM password_credentials WHERE email=$1`, invite.Email).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed acceptance persisted credential", err)
	}
	if _, err = store.pool.Exec(ctx, `ALTER TABLE member_invitation_events DROP CONSTRAINT isolated_accept_failure`); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- service.Accept(ctx, token, password, "obsolete-session-cookie") }()
	}
	wins := 0
	used := 0
	for i := 0; i < 2; i++ {
		e := <-results
		if e == nil {
			wins++
		} else if errors.Is(e, ErrInvitationUnavailable) {
			used++
		} else {
			t.Fatal("unexpected enrollment race result", e)
		}
	}
	if wins != 1 || used != 1 {
		t.Fatal("single-use enrollment not enforced")
	}
	authenticator, err := NewPasswordAuthenticator(store)
	if err != nil {
		t.Fatal(err)
	}
	session, err := authenticator.SignIn(ctx, invite.Email, password, time.Hour)
	if err != nil {
		t.Fatal("enrolled credential cannot sign in", err)
	}
	member, err := store.SessionPrincipal(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM location_grants WHERE principal_id=$1)+(SELECT count(*) FROM organization_grants WHERE principal_id=$1)+(SELECT count(*) FROM customer_account_links WHERE principal_id=$1)`, member).Scan(&count); err != nil || count != 0 {
		t.Fatal("enrollment created implicit permissions/customer link", err)
	}
	if err = store.pool.QueryRow(ctx, `SELECT count(*) FROM organization_memberships WHERE organization_id=$1 AND principal_id=$2`, a.OrganizationID, member).Scan(&count); err != nil || count != 1 {
		t.Fatal("explicit invited membership missing", err)
	}
	existing, err := service.Create(ctx, b.ActorID, b.OrganizationID, invite.Email, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	existingToken := secret(existing)
	if err = service.Accept(ctx, existingToken, "Replacement password is forbidden", ""); !errors.Is(err, ErrInvitationIdentity) {
		t.Fatal("invitation could reset existing identity", err)
	}
	if err = service.Accept(ctx, existingToken, "", session); err != nil {
		t.Fatal("existing verified identity could not accept", err)
	}
	if _, err = authenticator.SignIn(ctx, invite.Email, password, time.Hour); err != nil {
		t.Fatal("existing account password was changed", err)
	}
	revoked, err := service.Create(ctx, a.ActorID, a.OrganizationID, "revoked@example.invalid", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	revokedToken := secret(revoked)
	if err = service.Revoke(ctx, b.ActorID, a.OrganizationID, revoked.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign revocation accepted", err)
	}
	if err = service.Revoke(ctx, a.ActorID, a.OrganizationID, revoked.ID); err != nil {
		t.Fatal(err)
	}
	if err = service.Accept(ctx, revokedToken, password, ""); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatal("revoked invitation accepted", err)
	}
	expired, err := service.Create(ctx, a.ActorID, a.OrganizationID, "expired@example.invalid", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	expiredToken := secret(expired)
	if _, err = store.pool.Exec(ctx, `UPDATE member_invitations SET expires_at=created_at+interval '1 microsecond' WHERE id=$1`, expired.ID); err != nil {
		t.Fatal(err)
	}
	if err = service.Accept(ctx, expiredToken, password, ""); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatal("expired invitation accepted", err)
	}
	if _, err = service.Create(ctx, a.ActorID, a.OrganizationID, expired.Email, time.Hour); err != nil {
		t.Fatal("expired invitation prevented explicit replacement", err)
	}
	stale, err := service.Create(ctx, a.ActorID, a.OrganizationID, "creator-revoked@example.invalid", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	staleToken := secret(stale)
	if _, err = store.pool.Exec(ctx, `DELETE FROM organization_grants WHERE principal_id=$1 AND action='access.manage'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if err = service.Accept(ctx, staleToken, password, ""); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatal("revoked inviter still enrolled member", err)
	}

	// Restore access.manage grant to test ProcessDeliveries
	if _, err = store.pool.Exec(ctx, `INSERT INTO organization_grants(organization_id,principal_id,action) VALUES($1,$2,'access.manage') ON CONFLICT DO NOTHING`, a.OrganizationID, a.ActorID); err != nil {
		t.Fatal(err)
	}

	// ProcessDeliveries validation
	if _, err = service.ProcessDeliveries(ctx, nil, 10); !errors.Is(err, ErrInvitationInput) {
		t.Fatal("nil sender accepted", err)
	}

	// Successful delivery dispatch
	toSend, err := service.Create(ctx, a.ActorID, a.OrganizationID, "delivery-success@example.invalid", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mockSuccess := &testEmailSender{ref: "email-prov-ref-001"}
	deliveryResults, err := service.ProcessDeliveries(ctx, mockSuccess, 10)
	if err != nil {
		t.Fatalf("ProcessDeliveries failed: %v", err)
	}
	var foundSuccess bool
	for _, res := range deliveryResults {
		if res.InvitationID == toSend.ID {
			foundSuccess = true
			if res.State != "sent" || res.ProviderReference != "email-prov-ref-001" {
				t.Fatalf("unexpected delivery result: %+v", res)
			}
		}
	}
	if !foundSuccess {
		t.Fatal("delivery for toSend invitation not found in results")
	}

	// Verify database row for successful delivery
	var dbState, dbRef string
	var deliveredAt *time.Time
	err = store.pool.QueryRow(ctx, `SELECT state, provider_reference, delivered_at FROM member_invitation_deliveries WHERE invitation_id=$1`, toSend.ID).Scan(&dbState, &dbRef, &deliveredAt)
	if err != nil || dbState != "sent" || dbRef != "email-prov-ref-001" || deliveredAt == nil {
		t.Fatalf("database delivery row invalid: state=%s, ref=%s, deliveredAt=%v, err=%v", dbState, dbRef, deliveredAt, err)
	}

	// Failed delivery dispatch
	toFail, err := service.Create(ctx, a.ActorID, a.OrganizationID, "delivery-fail@example.invalid", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mockFail := &testEmailSender{err: errors.New("upstream provider connection timeout")}
	failResults, err := service.ProcessDeliveries(ctx, mockFail, 10)
	if err != nil {
		t.Fatalf("ProcessDeliveries with failing sender returned error: %v", err)
	}
	var foundFail bool
	for _, res := range failResults {
		if res.InvitationID == toFail.ID {
			foundFail = true
			if res.State != "failed" || !bytes.Contains([]byte(res.Error), []byte("upstream provider connection timeout")) {
				t.Fatalf("unexpected failing delivery result: %+v", res)
			}
		}
	}
	if !foundFail {
		t.Fatal("failing delivery result not found")
	}
	var failDbState string
	err = store.pool.QueryRow(ctx, `SELECT state FROM member_invitation_deliveries WHERE invitation_id=$1`, toFail.ID).Scan(&failDbState)
	if err != nil || failDbState != "failed" {
		t.Fatalf("expected state 'failed', got %s (err: %v)", failDbState, err)
	}
	// A refused send waits its turn, then goes out again.
	retrySender := &testEmailSender{ref: "email-prov-ref-002"}
	if early, e := service.ProcessDeliveries(ctx, retrySender, 10); e != nil || len(early) != 0 {
		t.Fatalf("a failed send was retried before its time: %v %+v", e, early)
	}
	if _, err = store.pool.Exec(ctx, `UPDATE member_invitation_deliveries SET retry_at=now()-interval '1 second' WHERE invitation_id=$1`, toFail.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProcessDeliveries(ctx, retrySender, 10); err != nil {
		t.Fatal(err)
	}
	if err = store.pool.QueryRow(ctx, `SELECT state FROM member_invitation_deliveries WHERE invitation_id=$1`, toFail.ID).Scan(&failDbState); err != nil || failDbState != "sent" {
		t.Fatalf("expected the retry to send, got %s (err: %v)", failDbState, err)
	}
}

type testEmailSender struct {
	ref  string
	err  error
	sent []string
}

func (m *testEmailSender) SendEmail(ctx context.Context, recipient, subject, bodyHTML, bodyText string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	m.sent = append(m.sent, recipient)
	return m.ref, nil
}
