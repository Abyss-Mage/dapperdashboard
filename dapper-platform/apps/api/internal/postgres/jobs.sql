ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check
CHECK (action IN ('customers.read','customers.create','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete'));

-- Available work orders have no fixed employee assignment. This does not
-- approve booking/payment transitions or infer availability from quote status.
CREATE TABLE jobs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 service_name text NOT NULL CHECK (length(btrim(service_name)) BETWEEN 2 AND 120),
 description text NOT NULL DEFAULT '' CHECK (length(description)<=2000),
 status text NOT NULL DEFAULT 'available' CHECK(status IN ('available','completed')),
 completed_by uuid REFERENCES principals(id),
 completed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id),
 CHECK ((status='available' AND completed_by IS NULL AND completed_at IS NULL) OR
        (status='completed' AND completed_by IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE INDEX jobs_scope_idx ON jobs(organization_id,location_id,status,created_at,id);
