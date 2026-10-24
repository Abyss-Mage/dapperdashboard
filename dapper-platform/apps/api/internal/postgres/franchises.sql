CREATE TABLE organization_grants (
 principal_id uuid NOT NULL REFERENCES principals(id),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 action text NOT NULL CHECK(action IN ('structure.read','structure.manage')),
 PRIMARY KEY(principal_id,organization_id,action)
);
CREATE TABLE franchises (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120),
 UNIQUE(organization_id,id)
);
CREATE UNIQUE INDEX franchises_name_unique ON franchises(organization_id,lower(btrim(name)));
ALTER TABLE locations ADD COLUMN franchise_id uuid;
ALTER TABLE locations ADD CONSTRAINT location_franchise_scope
 FOREIGN KEY(organization_id,franchise_id) REFERENCES franchises(organization_id,id);
-- Existing locations remain unclassified until an approved mapping is supplied.
CREATE UNIQUE INDEX locations_franchise_name_unique ON locations(organization_id,franchise_id,lower(btrim(name))) WHERE franchise_id IS NOT NULL;
CREATE TABLE organization_audit_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL REFERENCES organizations(id),
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL,
 entity_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
