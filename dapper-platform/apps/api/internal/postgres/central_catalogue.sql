ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check
 CHECK (action IN ('structure.read','structure.manage','catalogue.read','catalogue.manage'));

CREATE TABLE central_catalog_services (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120),
 kind text NOT NULL CHECK(kind IN ('main','addon')),
 parent_id uuid,
 archived boolean NOT NULL DEFAULT false,
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,parent_id) REFERENCES central_catalog_services(organization_id,id),
 CHECK((kind='main' AND parent_id IS NULL) OR (kind='addon' AND parent_id IS NOT NULL AND parent_id<>id))
);

CREATE INDEX central_catalog_parent_idx ON central_catalog_services(organization_id,parent_id) WHERE NOT archived;

CREATE TABLE central_catalog_service_revisions (
 service_id uuid NOT NULL REFERENCES central_catalog_services(id),
 version integer NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(service_id,version)
);

ALTER TABLE jobs ADD COLUMN reservation_id uuid;
ALTER TABLE jobs ADD CONSTRAINT jobs_reservation_scope_fk
 FOREIGN KEY(organization_id,location_id,reservation_id)
 REFERENCES capacity_reservations(organization_id,location_id,id);

CREATE INDEX jobs_reservation_idx ON jobs(organization_id,location_id,reservation_id) WHERE reservation_id IS NOT NULL;
