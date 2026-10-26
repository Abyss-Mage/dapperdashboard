-- Customers: one customer per email and per phone number at a franchise, and a "removed" state for customers whose issued
-- invoices or payments must be kept for tax records (they disappear from lists and look-ups, the financial records stay).
ALTER TABLE customers ADD COLUMN IF NOT EXISTS removed_at timestamptz;
CREATE UNIQUE INDEX IF NOT EXISTS customers_unique_email ON customers(organization_id, location_id, lower(btrim(email)))
  WHERE removed_at IS NULL AND btrim(email) <> '';
CREATE UNIQUE INDEX IF NOT EXISTS customers_unique_phone ON customers(organization_id, location_id, regexp_replace(phone, '\D', '', 'g'))
  WHERE removed_at IS NULL AND regexp_replace(phone, '\D', '', 'g') <> '';

-- Staff availability is a request: the franchise owner or the Owner/Super Admin approves or rejects it.
-- Changing a submission sends it back for review.
ALTER TABLE employee_availability_submissions
  ADD COLUMN IF NOT EXISTS review_status text NOT NULL DEFAULT 'submitted' CHECK (review_status IN ('submitted', 'approved', 'rejected')),
  ADD COLUMN IF NOT EXISTS reviewed_by uuid REFERENCES principals(id),
  ADD COLUMN IF NOT EXISTS reviewed_at timestamptz,
  ADD COLUMN IF NOT EXISTS review_note text NOT NULL DEFAULT '' CHECK (length(review_note) <= 500);

-- An invitation can carry the role the new member gets at one franchise; it is assigned when they accept.
ALTER TABLE member_invitations
  ADD COLUMN IF NOT EXISTS preset_role_id uuid REFERENCES roles(id),
  ADD COLUMN IF NOT EXISTS preset_location_id uuid,
  ADD CONSTRAINT member_invitations_preset_pair CHECK ((preset_role_id IS NULL) = (preset_location_id IS NULL));

-- Square: each franchise connects its own Square account (OAuth). Tokens are stored encrypted; a binding that points at
-- credential_reference 'oauth:<location id>' charges with that franchise's own connection.
CREATE TABLE IF NOT EXISTS square_oauth_connections (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 environment text NOT NULL CHECK (environment IN ('sandbox', 'production')),
 merchant_id text NOT NULL CHECK (length(merchant_id) BETWEEN 1 AND 128),
 access_token_sealed bytea NOT NULL,
 refresh_token_sealed bytea NOT NULL,
 expires_at timestamptz NOT NULL,
 refreshed_at timestamptz NOT NULL DEFAULT now(),
 connected_by uuid NOT NULL REFERENCES principals(id),
 connected_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id, location_id, environment),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id)
);
-- One-time "state" values for the round trip to Square's approval page (CSRF protection), kept as hashes, valid 15 minutes.
CREATE TABLE IF NOT EXISTS square_oauth_states (
 state_hash bytea PRIMARY KEY,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 environment text NOT NULL CHECK (environment IN ('sandbox', 'production')),
 actor_id uuid NOT NULL REFERENCES principals(id),
 expires_at timestamptz NOT NULL
);

-- Invoice Ninja: dealership (B2B) invoices can be sent to the organization's Invoice Ninja account. The API token is stored
-- encrypted. Each Dapper invoice is sent at most once; the Invoice Ninja id and number are kept.
CREATE TABLE IF NOT EXISTS invoice_ninja_settings (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 base_url text NOT NULL CHECK (base_url ~ '^https://[^\s/]+(/[^\s]*)?$' AND length(base_url) <= 200),
 token_sealed bytea NOT NULL,
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS invoice_ninja_exports (
 invoice_id uuid PRIMARY KEY REFERENCES invoices(id),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 ninja_client_id text NOT NULL,
 ninja_invoice_id text NOT NULL,
 ninja_number text NOT NULL DEFAULT '',
 exported_by uuid NOT NULL REFERENCES principals(id),
 exported_at timestamptz NOT NULL DEFAULT now()
);

-- Central catalogue is the source for every franchise: a central change (name, price, archive) applies to all linked franchise
-- services, and a new central service is switched on at every franchise. Franchises keep their own extra services alongside.
CREATE OR REPLACE FUNCTION sync_central_service_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE central central_catalog_services; local catalog_services; loc record; parent_local uuid;
BEGIN
 SELECT * INTO STRICT central FROM central_catalog_services WHERE id=NEW.service_id;
 FOR local IN SELECT * FROM catalog_services WHERE central_service_id=central.id ORDER BY location_id,id FOR UPDATE LOOP
  IF local.kind<>central.kind OR
   (central.kind='addon' AND NOT EXISTS(SELECT 1 FROM catalog_services parent WHERE parent.id=local.parent_id AND parent.central_service_id=central.parent_id)) THEN
   RAISE EXCEPTION 'Linked service identity cannot change' USING ERRCODE='23514',CONSTRAINT='central_link_identity';
  END IF;
  IF central.archived AND EXISTS(SELECT 1 FROM catalog_services child WHERE child.parent_id=local.id AND NOT child.archived AND child.central_service_id IS NULL) THEN
   RAISE EXCEPTION 'Linked service has active location add-ons' USING ERRCODE='23514',CONSTRAINT='central_link_children';
  END IF;
  UPDATE catalog_services SET name=central.name,price_cents=central.price_cents,archived=archived OR central.archived,version=version+1 WHERE id=local.id RETURNING * INTO local;
  INSERT INTO catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES(local.id,local.version,NEW.actor_id,
   jsonb_build_object('id',local.id,'name',local.name,'kind',local.kind,'parentId',local.parent_id,'archived',local.archived,'version',local.version,'centralServiceId',central.id,'priceCents',local.price_cents));
  INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES(local.organization_id,local.location_id,NEW.actor_id,'service.central_revision_applied',local.id);
 END LOOP;
 IF NOT central.archived THEN
  FOR loc IN SELECT l.id FROM locations l WHERE l.organization_id=central.organization_id
    AND NOT EXISTS(SELECT 1 FROM catalog_services c WHERE c.organization_id=l.organization_id AND c.location_id=l.id AND c.central_service_id=central.id) LOOP
   parent_local := NULL;
   IF central.kind='addon' THEN
    SELECT id INTO parent_local FROM catalog_services WHERE organization_id=central.organization_id AND location_id=loc.id AND central_service_id=central.parent_id AND NOT archived;
    CONTINUE WHEN parent_local IS NULL;
   END IF;
   INSERT INTO catalog_services(organization_id,location_id,name,kind,parent_id,central_service_id,price_cents)
    VALUES(central.organization_id,loc.id,central.name,central.kind,parent_local,central.id,central.price_cents) RETURNING * INTO local;
   INSERT INTO catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES(local.id,local.version,NEW.actor_id,
    jsonb_build_object('id',local.id,'name',local.name,'kind',local.kind,'parentId',local.parent_id,'archived',local.archived,'version',local.version,'centralServiceId',central.id,'priceCents',local.price_cents));
   INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES(local.organization_id,loc.id,NEW.actor_id,'service.central_service_added',local.id);
  END LOOP;
 END IF;
 RETURN NEW;
END $$;
