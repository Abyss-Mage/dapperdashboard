ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve'));
-- Explicit dated resource windows, not approved recurring business hours.
CREATE TABLE scheduling_resources (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120),
 capacity integer NOT NULL CHECK(capacity BETWEEN 1 AND 1000000),
 timezone text NOT NULL,
 rules jsonb NOT NULL CHECK(jsonb_typeof(rules)='object'),
 UNIQUE(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE TABLE capacity_reservations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL, location_id uuid NOT NULL, resource_id uuid NOT NULL,
 occupied_start timestamptz NOT NULL, occupied_end timestamptz NOT NULL,
 units integer NOT NULL CHECK(units BETWEEN 1 AND 1000000),
 request jsonb NOT NULL CHECK(jsonb_typeof(request)='object'),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(occupied_start<occupied_end),
 FOREIGN KEY(organization_id,location_id,resource_id) REFERENCES scheduling_resources(organization_id,location_id,id)
);
CREATE INDEX capacity_reservations_scope_idx ON capacity_reservations(organization_id,location_id,resource_id,occupied_start,occupied_end);
