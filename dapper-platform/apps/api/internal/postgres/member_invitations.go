package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInvitationInput       = errors.New("valid invitation email and explicit expiry required")
	ErrInvitationUnavailable = errors.New("invitation unavailable, expired or already used")
	ErrInvitationPending     = errors.New("a pending invitation already exists")
	ErrInvitationIdentity    = errors.New("sign in to the invited existing account; invitation cannot replace its password")
)

type MemberInvitation struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Email          string    `json:"email"`
	CreatedBy      string    `json:"createdBy"`
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
	Status         string    `json:"status"`
	DeliveryState  string    `json:"deliveryState"`
	// The role and franchise the member gets on accepting (empty when none was chosen).
	RoleName     string `json:"roleName"`
	LocationName string `json:"locationName"`
}

// InvitationRole is the role (at one franchise location) given to the member when they accept.
type InvitationRole struct{ RoleID, LocationID string }

// InvitationService is explicitly configured with the server's enrollment origin
// and encryption key. Raw enrollment tokens exist only in encrypted delivery
// payloads; list/create responses and audits never contain them.
type InvitationService struct {
	store  *Store
	origin string
	seal   cipher.AEAD
	slots  chan struct{}
}

func NewInvitationService(store *Store, origin string, key []byte) (*InvitationService, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || len(key) != 32 || store == nil {
		return nil, ErrInvitationInput
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	seal, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &InvitationService{store: store, origin: origin, seal: seal, slots: make(chan struct{}, 2)}, nil
}

const memberInvitationColumns = `i.id::text,i.organization_id::text,i.email,i.created_by::text,i.created_at,i.expires_at,i.status,d.state,COALESCE((SELECT r.name FROM roles r WHERE r.id=i.preset_role_id),''),COALESCE((SELECT l.name FROM locations l WHERE l.id=i.preset_location_id),'')`

func scanMemberInvitation(row pgx.Row) (MemberInvitation, error) {
	var i MemberInvitation
	err := row.Scan(&i.ID, &i.OrganizationID, &i.Email, &i.CreatedBy, &i.CreatedAt, &i.ExpiresAt, &i.Status, &i.DeliveryState, &i.RoleName, &i.LocationName)
	return i, err
}

func (s *InvitationService) Create(ctx context.Context, actor, org, email string, lifetime time.Duration) (MemberInvitation, error) {
	return s.CreateWithRole(ctx, actor, org, email, lifetime, nil)
}

// CreateWithRole queues an invitation; when preset is given, the new member gets that role at that franchise on accepting.
func (s *InvitationService) CreateWithRole(ctx context.Context, actor, org, email string, lifetime time.Duration, preset *InvitationRole) (MemberInvitation, error) {
	return s.create(ctx, actor, org, email, lifetime, preset, func(tx pgx.Tx) error { return accessManagementGrant(ctx, tx, actor, org) })
}

func (s *InvitationService) create(ctx context.Context, actor, org, email string, lifetime time.Duration, preset *InvitationRole, gate func(pgx.Tx) error) (MemberInvitation, error) {
	email, valid := normalizedEmail(email)
	if !valid || lifetime < 15*time.Minute || lifetime > 24*time.Hour {
		return MemberInvitation{}, ErrInvitationInput
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return MemberInvitation{}, err
	}
	defer clear(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256(raw)
	tx, err := s.store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return MemberInvitation{}, err
	}
	defer tx.Rollback(ctx)
	if err = gate(tx); err != nil {
		return MemberInvitation{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, org+"/invite/"+email); err != nil {
		return MemberInvitation{}, err
	}
	// Expired links are retired without relabeling them as accepted or delivered.
	if _, err = tx.Exec(ctx, `WITH retired AS (UPDATE member_invitations SET status='revoked' WHERE organization_id=$1 AND email=$2 AND status='pending' AND expires_at<=now() RETURNING id) INSERT INTO member_invitation_events(organization_id,invitation_id,actor_id,action) SELECT $1,id,$3,'revoked' FROM retired`, org, email, actor); err != nil {
		return MemberInvitation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE member_invitation_deliveries d SET state='cancelled' FROM member_invitations i WHERE d.invitation_id=i.id AND i.organization_id=$1 AND i.email=$2 AND i.status='revoked' AND d.state IN ('queued','failed')`, org, email); err != nil {
		return MemberInvitation{}, err
	}
	var pending bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_invitations WHERE organization_id=$1 AND email=$2 AND status='pending')`, org, email).Scan(&pending); err != nil {
		return MemberInvitation{}, err
	}
	if pending {
		return MemberInvitation{}, ErrInvitationPending
	}
	var roleID, locationID *string
	if preset != nil {
		// The role must be one of this organization's roles and the franchise one of its locations.
		var ok bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE id=$2::uuid AND organization_id=$1) AND EXISTS(SELECT 1 FROM locations WHERE id=$3::uuid AND organization_id=$1)`, org, preset.RoleID, preset.LocationID).Scan(&ok); err != nil {
			return MemberInvitation{}, err
		}
		if !ok {
			return MemberInvitation{}, ErrInvitationInput
		}
		roleID, locationID = &preset.RoleID, &preset.LocationID
	}
	var result MemberInvitation
	if err = tx.QueryRow(ctx, `INSERT INTO member_invitations(organization_id,email,token_hash,created_by,expires_at,preset_role_id,preset_location_id) VALUES($1,$2,$3,$4,now()+make_interval(secs=>$5),$6,$7) RETURNING id::text,organization_id::text,email,created_by::text,created_at,expires_at,status,COALESCE((SELECT name FROM roles WHERE id=$6),''),COALESCE((SELECT name FROM locations WHERE id=$7),'')`, org, email, digest[:], actor, lifetime.Seconds(), roleID, locationID).Scan(&result.ID, &result.OrganizationID, &result.Email, &result.CreatedBy, &result.CreatedAt, &result.ExpiresAt, &result.Status, &result.RoleName, &result.LocationName); err != nil {
		return MemberInvitation{}, err
	}
	var organizationName string
	if err = tx.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1`, org).Scan(&organizationName); err != nil {
		return MemberInvitation{}, err
	}
	payload, err := json.Marshal(map[string]string{"recipient": email, "organization": organizationName, "enrollmentURL": s.origin + "/enroll#invitation=" + token})
	if err != nil {
		return MemberInvitation{}, err
	}
	defer clear(payload)
	nonce := make([]byte, s.seal.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return MemberInvitation{}, err
	}
	sealed := s.seal.Seal(nonce, nonce, payload, []byte(org+"/"+result.ID))
	if _, err = tx.Exec(ctx, `INSERT INTO member_invitation_deliveries(organization_id,invitation_id,payload_ciphertext) VALUES($1,$2,$3)`, org, result.ID, sealed); err != nil {
		return MemberInvitation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_invitation_events(organization_id,invitation_id,actor_id,action) VALUES($1,$2,$3,'created')`, org, result.ID, actor); err != nil {
		return MemberInvitation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MemberInvitation{}, err
	}
	result.DeliveryState = "queued"
	return result, nil
}

func (s *InvitationService) List(ctx context.Context, actor, org string) ([]MemberInvitation, error) {
	tx, err := s.store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = accessManagementGrant(ctx, tx, actor, org); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+memberInvitationColumns+` FROM member_invitations i JOIN member_invitation_deliveries d ON d.invitation_id=i.id WHERE i.organization_id=$1 AND i.hidden_at IS NULL ORDER BY i.created_at DESC,i.id LIMIT 100`, org)
	if err != nil {
		return nil, err
	}
	result := []MemberInvitation{}
	for rows.Next() {
		invitation, e := scanMemberInvitation(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		result = append(result, invitation)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *InvitationService) Revoke(ctx context.Context, actor, org, id string) error {
	return s.revoke(ctx, actor, org, id, func(tx pgx.Tx) error { return accessManagementGrant(ctx, tx, actor, org) })
}

func (s *InvitationService) revoke(ctx context.Context, actor, org, id string, gate func(pgx.Tx) error) error {
	tx, err := s.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = gate(tx); err != nil {
		return err
	}
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM member_invitations WHERE organization_id=$1 AND id=$2 FOR UPDATE`, org, id).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return ErrInvitationUnavailable
	} else if err != nil {
		return err
	}
	if status == "accepted" {
		return ErrInvitationUnavailable
	}
	if status == "revoked" {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `UPDATE member_invitations SET status='revoked' WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE member_invitation_deliveries SET state='cancelled' WHERE invitation_id=$1 AND state IN ('queued','failed')`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_invitation_events(organization_id,invitation_id,actor_id,action) VALUES($1,$2,$3,'revoked')`, org, id, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type EmailSender interface {
	SendEmail(ctx context.Context, recipient, subject, bodyHTML, bodyText string) (providerReference string, err error)
}

type DeliveryResult struct {
	InvitationID      string `json:"invitationId"`
	OrganizationID    string `json:"organizationId"`
	Recipient         string `json:"recipient"`
	State             string `json:"state"`
	ProviderReference string `json:"providerReference,omitempty"`
	Error             string `json:"error,omitempty"`
}

// ProcessDeliveries sweeps queued invitation delivery payloads, decrypts their
// contents, and dispatches them via the provided EmailSender.
// Upon success, it writes the exact provider reference and timestamp required
// by database integrity constraints.
func (s *InvitationService) ProcessDeliveries(ctx context.Context, sender EmailSender, limit int) ([]DeliveryResult, error) {
	if sender == nil {
		return nil, ErrInvitationInput
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	tx, err := s.store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	type pendingDelivery struct {
		invitationID   string
		organizationID string
		ciphertext     []byte
		email          string
		status         string
		isExpired      bool
	}

	rows, err := tx.Query(ctx, `
		SELECT d.invitation_id::text, d.organization_id::text, d.payload_ciphertext, i.email, i.status, (i.expires_at <= now())
		FROM member_invitation_deliveries d
		JOIN member_invitations i ON i.id = d.invitation_id
		WHERE d.state = 'queued' OR (d.state = 'failed' AND d.send_failures < 6 AND d.retry_at <= now())
		ORDER BY d.created_at
		LIMIT $1
		FOR UPDATE OF d SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}

	var batch []pendingDelivery
	for rows.Next() {
		var pd pendingDelivery
		if err := rows.Scan(&pd.invitationID, &pd.organizationID, &pd.ciphertext, &pd.email, &pd.status, &pd.isExpired); err != nil {
			rows.Close()
			return nil, err
		}
		batch = append(batch, pd)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	var results []DeliveryResult
	for _, pd := range batch {
		if pd.status != "pending" || pd.isExpired {
			// Invitation is no longer eligible for delivery
			if _, err := tx.Exec(ctx, `UPDATE member_invitation_deliveries SET state='cancelled' WHERE invitation_id=$1`, pd.invitationID); err != nil {
				return nil, err
			}
			results = append(results, DeliveryResult{
				InvitationID:   pd.invitationID,
				OrganizationID: pd.organizationID,
				Recipient:      pd.email,
				State:          "cancelled",
			})
			continue
		}

		nonceSize := s.seal.NonceSize()
		if len(pd.ciphertext) <= nonceSize {
			if _, err := tx.Exec(ctx, `UPDATE member_invitation_deliveries SET state='failed' WHERE invitation_id=$1`, pd.invitationID); err != nil {
				return nil, err
			}
			results = append(results, DeliveryResult{
				InvitationID:   pd.invitationID,
				OrganizationID: pd.organizationID,
				Recipient:      pd.email,
				State:          "failed",
				Error:          "corrupted delivery payload",
			})
			continue
		}

		nonce := pd.ciphertext[:nonceSize]
		ciphertextData := pd.ciphertext[nonceSize:]
		plain, err := s.seal.Open(nil, nonce, ciphertextData, []byte(pd.organizationID+"/"+pd.invitationID))
		if err != nil {
			if _, err := tx.Exec(ctx, `UPDATE member_invitation_deliveries SET state='failed' WHERE invitation_id=$1`, pd.invitationID); err != nil {
				return nil, err
			}
			results = append(results, DeliveryResult{
				InvitationID:   pd.invitationID,
				OrganizationID: pd.organizationID,
				Recipient:      pd.email,
				State:          "failed",
				Error:          "decryption failed",
			})
			continue
		}

		var payload struct {
			Recipient     string `json:"recipient"`
			Organization  string `json:"organization"`
			EnrollmentURL string `json:"enrollmentURL"`
		}
		if err := json.Unmarshal(plain, &payload); err != nil {
			clear(plain)
			if _, err := tx.Exec(ctx, `UPDATE member_invitation_deliveries SET state='failed' WHERE invitation_id=$1`, pd.invitationID); err != nil {
				return nil, err
			}
			results = append(results, DeliveryResult{
				InvitationID:   pd.invitationID,
				OrganizationID: pd.organizationID,
				Recipient:      pd.email,
				State:          "failed",
				Error:          "invalid payload json",
			})
			continue
		}
		clear(plain)

		subject, bodyHTML, bodyText := s.store.RenderEmail(ctx, "invitation", map[string]string{"organization": payload.Organization, "link": payload.EnrollmentURL})

		providerRef, err := sender.SendEmail(ctx, payload.Recipient, subject, bodyHTML, bodyText)
		if err != nil || strings.TrimSpace(providerRef) == "" {
			errMsg := "send error"
			if err != nil {
				errMsg = err.Error()
			} else {
				errMsg = "missing provider reference"
			}
			if _, e := tx.Exec(ctx, `UPDATE member_invitation_deliveries SET state='failed', send_failures=send_failures+1, retry_at=now()+make_interval(mins => 15*(send_failures+1)) WHERE invitation_id=$1`, pd.invitationID); e != nil {
				return nil, e
			}
			results = append(results, DeliveryResult{
				InvitationID:   pd.invitationID,
				OrganizationID: pd.organizationID,
				Recipient:      pd.email,
				State:          "failed",
				Error:          errMsg,
			})
		} else {
			providerRef = strings.TrimSpace(providerRef)
			if _, e := tx.Exec(ctx, `UPDATE member_invitation_deliveries SET state='sent', provider_reference=$1, delivered_at=now() WHERE invitation_id=$2`, providerRef, pd.invitationID); e != nil {
				return nil, e
			}
			results = append(results, DeliveryResult{
				InvitationID:      pd.invitationID,
				OrganizationID:    pd.organizationID,
				Recipient:         pd.email,
				State:             "sent",
				ProviderReference: providerRef,
			})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

// Accept creates a credential only for a previously unregistered email. Existing
// identities require their current verified session and never accept a new hash.
// There is no automatic sign-in, role, location capability or customer link.
func (s *InvitationService) Accept(ctx context.Context, token, password, sessionToken string) error {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return ErrInvitationUnavailable
	}
	digest := sha256.Sum256(raw)
	clear(raw)
	var email string
	if err = s.store.pool.QueryRow(ctx, `SELECT email FROM member_invitations WHERE token_hash=$1 AND status='pending' AND expires_at>now()`, digest[:]).Scan(&email); errors.Is(err, pgx.ErrNoRows) {
		return ErrInvitationUnavailable
	} else if err != nil {
		return err
	}
	var actor string
	if sessionToken != "" {
		actor, err = s.store.SessionPrincipal(ctx, sessionToken)
		if err != nil {
			if !errors.Is(err, ErrSession) {
				return err
			}
			actor = ""
		}
	}
	var existing string
	err = s.store.pool.QueryRow(ctx, `SELECT principal_id::text FROM password_credentials WHERE email=$1`, email).Scan(&existing)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	hash := ""
	if existing != "" {
		if actor != existing || password != "" {
			return ErrInvitationIdentity
		}
	} else {
		if actor != "" {
			return ErrInvitationIdentity
		}
		if len(password) < 15 || len(password) > 1024 {
			return ErrInvitationInput
		}
		select {
		case s.slots <- struct{}{}:
			defer func() { <-s.slots }()
		default:
			return ErrAuthenticationBusy
		}
		hash, err = argon2id.CreateHash(password, passwordParams())
		if err != nil {
			return err
		}
	}
	tx, err := s.store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "member-enrollment/"+email); err != nil {
		return err
	}
	var id, org, creator, lockedEmail string
	var employeeAt *string
	err = tx.QueryRow(ctx, `SELECT i.id::text,i.organization_id::text,i.created_by::text,i.email,CASE WHEN lower(r.name)='employee' THEN i.preset_location_id::text END FROM member_invitations i LEFT JOIN roles r ON r.id=i.preset_role_id WHERE i.token_hash=$1 AND i.status='pending' AND i.expires_at>now() FOR UPDATE OF i`, digest[:]).Scan(&id, &org, &creator, &lockedEmail, &employeeAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvitationUnavailable
	}
	if err != nil {
		return err
	}
	if lockedEmail != email {
		return ErrInvitationUnavailable
	}
	// Whoever sent it must still be allowed to: an administrator, or for an Employee invitation the owner of that franchise.
	err = accessManagementGrant(ctx, tx, creator, org)
	if errors.Is(err, ErrForbidden) && employeeAt != nil {
		err = authorizeTransaction(ctx, tx, Scope{OrganizationID: org, LocationID: *employeeAt, ActorID: creator}, franchiseTeamAction)
	}
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			return ErrInvitationUnavailable
		}
		return err
	}
	var current string
	err = tx.QueryRow(ctx, `SELECT c.principal_id::text FROM password_credentials c JOIN principals p ON p.id=c.principal_id WHERE c.email=$1 AND p.active FOR SHARE OF c,p`, email).Scan(&current)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if current != "" {
		if current != actor || hash != "" || password != "" {
			return ErrInvitationIdentity
		}
		sessionRaw, e := base64.RawURLEncoding.DecodeString(sessionToken)
		if e != nil || len(sessionRaw) != 32 {
			return ErrInvitationIdentity
		}
		sessionHash := sha256.Sum256(sessionRaw)
		clear(sessionRaw)
		var sessionActor string
		if err = tx.QueryRow(ctx, `SELECT principal_id::text FROM auth_sessions WHERE token_hash=$1 AND principal_id=$2 AND revoked_at IS NULL AND expires_at>now() FOR SHARE`, sessionHash[:], current).Scan(&sessionActor); errors.Is(err, pgx.ErrNoRows) {
			return ErrInvitationIdentity
		} else if err != nil {
			return err
		}
		actor = current
	} else {
		if existing != "" || hash == "" {
			return ErrInvitationIdentity
		}
		// An inactive registered account is never replaced by an invitation.
		var occupied bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM password_credentials WHERE email=$1)`, email).Scan(&occupied); err != nil {
			return err
		}
		if occupied {
			return ErrInvitationIdentity
		}
		if err = tx.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('managed-enrollment:'||gen_random_uuid()::text) RETURNING id::text`).Scan(&actor); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO password_credentials(principal_id,email,password_hash) VALUES($1,$2,$3)`, actor, email, hash); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, org, actor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE member_invitations SET status='accepted',accepted_by=$2,accepted_at=now() WHERE id=$1`, id, actor); err != nil {
		return err
	}
	// The role chosen when inviting, at that franchise, assigned by the person who invited them.
	if _, err = tx.Exec(ctx, `WITH preset AS (SELECT preset_role_id AS role_id, preset_location_id AS location_id FROM member_invitations WHERE id=$1 AND preset_role_id IS NOT NULL),
 added AS (INSERT INTO member_role_assignments(organization_id,principal_id,role_id,scope_type,scope_id,location_id,version,assigned_by)
  SELECT $2,$3,p.role_id,'location',p.location_id,p.location_id,1,$4 FROM preset p
  WHERE NOT EXISTS(SELECT 1 FROM member_role_assignments a WHERE a.organization_id=$2 AND a.principal_id=$3 AND a.role_id=p.role_id AND a.scope_type='location' AND a.scope_id=p.location_id)
  RETURNING role_id, scope_id)
 INSERT INTO role_assignment_events(organization_id,actor_id,principal_id,role_id,scope_type,scope_id,action,version)
  SELECT $2,$4,$3,role_id,'location',scope_id,'assigned',1 FROM added`, id, org, actor, creator); err != nil {
		return err
	}
	// Someone invited as an Employee of a franchise can start work there straight away.
	if _, err = tx.Exec(ctx, `INSERT INTO employee_worksite_assignments(principal_id,organization_id,location_id,timezone,assigned_by)
 SELECT $2::uuid,i.organization_id,i.preset_location_id,COALESCE((SELECT r.timezone FROM scheduling_resources r WHERE r.organization_id=i.organization_id AND r.location_id=i.preset_location_id ORDER BY r.id LIMIT 1),'America/Toronto'),$3::uuid
 FROM member_invitations i JOIN roles ro ON ro.id=i.preset_role_id AND lower(ro.name)='employee' WHERE i.id=$1 ON CONFLICT (principal_id) DO NOTHING`, id, actor, creator); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE member_invitation_deliveries SET state='cancelled' WHERE invitation_id=$1 AND state IN ('queued','failed')`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO member_invitation_events(organization_id,invitation_id,actor_id,action) VALUES($1,$2,$3,'accepted')`, org, id, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
