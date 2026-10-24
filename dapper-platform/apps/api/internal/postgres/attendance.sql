ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','vehicles.read','vehicles.create',
 'jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage',
 'scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage'));

ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage',
 'attendance.read','attendance.manage'));

CREATE TABLE employee_attendance (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 status text NOT NULL CHECK (status IN ('clocked_in','on_break','clocked_out')),
 clock_in_at timestamptz NOT NULL,
 clock_out_at timestamptz,
 current_break_start timestamptz,
 total_break_minutes integer NOT NULL DEFAULT 0 CHECK (total_break_minutes >= 0),
 notes text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id),
 CHECK(clock_out_at IS NULL OR clock_out_at >= clock_in_at),
 CHECK((status = 'clocked_out' AND clock_out_at IS NOT NULL AND current_break_start IS NULL) OR
       (status = 'clocked_in' AND clock_out_at IS NULL AND current_break_start IS NULL) OR
       (status = 'on_break' AND clock_out_at IS NULL AND current_break_start IS NOT NULL))
);

CREATE UNIQUE INDEX employee_attendance_active_idx ON employee_attendance(organization_id,location_id,principal_id)
 WHERE clock_out_at IS NULL;

CREATE INDEX employee_attendance_scope_idx ON employee_attendance(organization_id,location_id,principal_id,clock_in_at);

CREATE TABLE employee_attendance_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 attendance_id uuid NOT NULL REFERENCES employee_attendance(id) ON DELETE CASCADE,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 actor_id uuid NOT NULL REFERENCES principals(id),
 event_type text NOT NULL CHECK (event_type IN ('clock_in','start_break','end_break','clock_out')),
 recorded_at timestamptz NOT NULL DEFAULT now(),
 notes text NOT NULL DEFAULT ''
);

CREATE INDEX employee_attendance_events_idx ON employee_attendance_events(attendance_id,recorded_at);
