-- Explicit employee assignment is independent of location permissions. No seeds.
CREATE TABLE employee_worksite_assignments (
 principal_id uuid PRIMARY KEY REFERENCES principals(id),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 dealership_id uuid NOT NULL,
 timezone text NOT NULL CHECK(length(timezone) BETWEEN 1 AND 100),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 assigned_by uuid NOT NULL REFERENCES principals(id),
 assigned_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,principal_id) REFERENCES organization_memberships(organization_id,principal_id),
 FOREIGN KEY(organization_id,location_id,dealership_id) REFERENCES dealership_accounts(organization_id,location_id,id)
);
CREATE TABLE employee_availability_submissions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 dealership_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 week_start date NOT NULL CHECK(extract(isodow FROM week_start)=1),
 timezone text NOT NULL,
 intervals jsonb NOT NULL CHECK(jsonb_typeof(intervals)='array'),
 notes text NOT NULL DEFAULT '' CHECK(length(notes)<=1000),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 submitted_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,location_id,dealership_id,principal_id,week_start),
 FOREIGN KEY(organization_id,location_id,dealership_id) REFERENCES dealership_accounts(organization_id,location_id,id)
);
-- Only bookings explicitly attributed by an administrator are employee-visible.
ALTER TABLE bookings ADD CONSTRAINT bookings_employee_scope_unique UNIQUE(organization_id,location_id,id);
CREATE TABLE employee_booking_worksites (
 booking_id uuid PRIMARY KEY,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 dealership_id uuid NOT NULL,
 linked_by uuid NOT NULL REFERENCES principals(id),
 linked_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id,booking_id) REFERENCES bookings(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id,dealership_id) REFERENCES dealership_accounts(organization_id,location_id,id)
);
CREATE TABLE employee_workspace_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL REFERENCES organizations(id),
 actor_id uuid NOT NULL REFERENCES principals(id),
 entity_id uuid NOT NULL,
 action text NOT NULL,
 detail jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION protect_employee_workspace_event() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Employee assignment and availability history is immutable' USING ERRCODE='23514'; END $$;
CREATE TRIGGER employee_workspace_events_immutable BEFORE UPDATE OR DELETE ON employee_workspace_events
 FOR EACH ROW EXECUTE FUNCTION protect_employee_workspace_event();
