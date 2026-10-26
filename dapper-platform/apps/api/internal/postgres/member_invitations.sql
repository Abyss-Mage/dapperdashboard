CREATE TABLE member_invitations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 email text NOT NULL CHECK(email=lower(btrim(email)) AND length(email)<=254),
 token_hash bytea NOT NULL UNIQUE CHECK(octet_length(token_hash)=32),
 created_by uuid NOT NULL REFERENCES principals(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL CHECK(expires_at>created_at),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','accepted','revoked')),
 accepted_by uuid REFERENCES principals(id),
 accepted_at timestamptz,
 UNIQUE(organization_id,id),
 CHECK((status='accepted' AND accepted_by IS NOT NULL AND accepted_at IS NOT NULL) OR (status<>'accepted' AND accepted_by IS NULL AND accepted_at IS NULL))
);
CREATE UNIQUE INDEX member_invitation_pending_email ON member_invitations(organization_id,email) WHERE status='pending';

CREATE TABLE member_invitation_deliveries (
 invitation_id uuid PRIMARY KEY,
 organization_id uuid NOT NULL,
 payload_ciphertext bytea NOT NULL CHECK(octet_length(payload_ciphertext)>28),
 state text NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','sent','failed','cancelled')),
 provider_reference text,
 delivered_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,invitation_id) REFERENCES member_invitations(organization_id,id),
 CHECK(state<>'sent' OR (provider_reference IS NOT NULL AND length(provider_reference)>0 AND delivered_at IS NOT NULL))
);

CREATE TABLE member_invitation_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL,
 invitation_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL CHECK(action IN ('created','revoked','accepted')),
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,invitation_id) REFERENCES member_invitations(organization_id,id)
);
