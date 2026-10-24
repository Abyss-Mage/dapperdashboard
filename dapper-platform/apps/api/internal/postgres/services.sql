ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check
CHECK (action IN ('customers.read','customers.create','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','catalogue.read','catalogue.manage'));
CREATE TABLE catalog_services (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120),
 kind text NOT NULL CHECK(kind IN ('main','addon')),
 parent_id uuid,
 archived boolean NOT NULL DEFAULT false,
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 UNIQUE(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id),
 FOREIGN KEY(organization_id,location_id,parent_id) REFERENCES catalog_services(organization_id,location_id,id),
 CHECK((kind='main' AND parent_id IS NULL) OR (kind='addon' AND parent_id IS NOT NULL AND parent_id<>id))
);
CREATE INDEX catalog_parent_idx ON catalog_services(organization_id,location_id,parent_id) WHERE NOT archived;
CREATE TABLE catalog_service_revisions (
 service_id uuid NOT NULL REFERENCES catalog_services(id),
 version integer NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(service_id,version)
);
