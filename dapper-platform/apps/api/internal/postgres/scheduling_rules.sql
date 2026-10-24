-- Exact main/add-on combinations have explicit scheduling inputs. No duration
-- or resource is inferred from imported service names or SOP labour estimates.
CREATE TABLE service_scheduling_rules (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,location_id uuid NOT NULL,
 resource_id uuid NOT NULL,service_id uuid NOT NULL,
 addon_ids jsonb NOT NULL CHECK(jsonb_typeof(addon_ids)='array' AND jsonb_array_length(addon_ids)<=100),
 duration_minutes integer NOT NULL CHECK(duration_minutes BETWEEN 1 AND 10080),
 before_minutes integer NOT NULL CHECK(before_minutes BETWEEN 0 AND 10080),
 after_minutes integer NOT NULL CHECK(after_minutes BETWEEN 0 AND 10080),
 units integer NOT NULL CHECK(units BETWEEN 1 AND 1000000),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 UNIQUE(organization_id,location_id,id),
 UNIQUE(organization_id,location_id,resource_id,service_id,addon_ids),
 FOREIGN KEY(organization_id,location_id,resource_id) REFERENCES scheduling_resources(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id,service_id) REFERENCES catalog_services(organization_id,location_id,id)
);
CREATE TABLE service_scheduling_revisions (
 rule_id uuid NOT NULL REFERENCES service_scheduling_rules(id),
 version integer NOT NULL, snapshot jsonb NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(rule_id,version)
);
