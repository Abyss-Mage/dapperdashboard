-- 1. Every booking is visible to the staff at its location. Staff screens read employee_booking_worksites; a row with no
--    dealership means "this location's work". New bookings get this row when they are created (code); this backfills older ones.
INSERT INTO employee_booking_worksites(booking_id,organization_id,location_id,dealership_id,linked_by)
SELECT b.id,b.organization_id,b.location_id,NULL,
 COALESCE((SELECT be.actor_id FROM booking_events be WHERE be.booking_id=b.id AND be.actor_id IS NOT NULL ORDER BY be.created_at LIMIT 1),
          (SELECT m.principal_id FROM organization_memberships m WHERE m.organization_id=b.organization_id ORDER BY m.principal_id LIMIT 1))
FROM bookings b
WHERE NOT EXISTS(SELECT 1 FROM employee_booking_worksites w WHERE w.booking_id=b.id)
 AND COALESCE((SELECT be.actor_id FROM booking_events be WHERE be.booking_id=b.id AND be.actor_id IS NOT NULL ORDER BY be.created_at LIMIT 1),
          (SELECT m.principal_id FROM organization_memberships m WHERE m.organization_id=b.organization_id ORDER BY m.principal_id LIMIT 1)) IS NOT NULL;

-- 2. Dealerships: a per-car rate (CAD cents before tax; NULL = not set) and several contact people. The account keeps its
--    original contact columns as the primary (first) contact so existing screens and exports keep working.
ALTER TABLE dealership_accounts ADD COLUMN per_car_rate_cents bigint CHECK(per_car_rate_cents IS NULL OR per_car_rate_cents BETWEEN 0 AND 100000000000);
CREATE TABLE dealership_contacts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 dealership_id uuid NOT NULL,
 position integer NOT NULL DEFAULT 0,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120),
 email text NOT NULL DEFAULT '' CHECK(email='' OR email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
 phone text NOT NULL DEFAULT '' CHECK(length(phone)<=40),
 FOREIGN KEY(organization_id,location_id,dealership_id) REFERENCES dealership_accounts(organization_id,location_id,id) ON DELETE CASCADE
);
CREATE INDEX dealership_contacts_dealer_idx ON dealership_contacts(dealership_id,position);
INSERT INTO dealership_contacts(organization_id,location_id,dealership_id,position,name,email,phone)
SELECT organization_id,location_id,id,0,contact_name,contact_email,contact_phone FROM dealership_accounts;

-- 3. Franchise contact people (a franchise is a row in locations).
CREATE TABLE location_contacts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 position integer NOT NULL DEFAULT 0,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120),
 email text NOT NULL DEFAULT '' CHECK(email='' OR email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
 phone text NOT NULL DEFAULT '' CHECK(length(phone)<=40),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id) ON DELETE CASCADE
);
CREATE INDEX location_contacts_location_idx ON location_contacts(location_id,position);

-- 4. Time to complete a service, in minutes (NULL = not set). Central services carry it and linked franchise services receive it
--    with every central revision, like the name and the price. The application keeps a booking rule in step with it.
ALTER TABLE central_catalog_services ADD COLUMN duration_minutes integer CHECK(duration_minutes IS NULL OR duration_minutes BETWEEN 1 AND 10080);
ALTER TABLE catalog_services ADD COLUMN duration_minutes integer CHECK(duration_minutes IS NULL OR duration_minutes BETWEEN 1 AND 10080);
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
  UPDATE catalog_services SET name=central.name,price_cents=central.price_cents,duration_minutes=central.duration_minutes,archived=archived OR central.archived,version=version+1 WHERE id=local.id RETURNING * INTO local;
  INSERT INTO catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES(local.id,local.version,NEW.actor_id,
   jsonb_build_object('id',local.id,'name',local.name,'kind',local.kind,'parentId',local.parent_id,'archived',local.archived,'version',local.version,'centralServiceId',central.id,'priceCents',local.price_cents,'durationMinutes',local.duration_minutes));
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
   INSERT INTO catalog_services(organization_id,location_id,name,kind,parent_id,central_service_id,price_cents,duration_minutes)
    VALUES(central.organization_id,loc.id,central.name,central.kind,parent_local,central.id,central.price_cents,central.duration_minutes) RETURNING * INTO local;
   INSERT INTO catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES(local.id,local.version,NEW.actor_id,
    jsonb_build_object('id',local.id,'name',local.name,'kind',local.kind,'parentId',local.parent_id,'archived',local.archived,'version',local.version,'centralServiceId',central.id,'priceCents',local.price_cents,'durationMinutes',local.duration_minutes));
   INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES(local.organization_id,loc.id,NEW.actor_id,'service.central_service_added',local.id);
  END LOOP;
 END IF;
 RETURN NEW;
END $$;

-- 5. Square application keys (Application ID and secret) for test and live, entered by the owner instead of set on the server.
--    The secret is sealed with the server key (AES-256-GCM) and is never returned by the API.
CREATE TABLE square_app_credentials (
 environment text PRIMARY KEY CHECK(environment IN ('sandbox','production')),
 application_id text NOT NULL CHECK(length(btrim(application_id)) BETWEEN 6 AND 200),
 secret_sealed bytea NOT NULL CHECK(octet_length(secret_sealed)>28),
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
