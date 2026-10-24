-- On-shift GPS. Collection is OFF until an administrator enables it per location.
-- Reuses attendance.record (employee samples), attendance.read (viewers) and
-- attendance.manage (settings); no new roles or grants are created.
CREATE TABLE gps_settings (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 sample_interval_seconds integer NOT NULL DEFAULT 300 CHECK(sample_interval_seconds BETWEEN 60 AND 3600),
 retention_days integer NOT NULL DEFAULT 30 CHECK(retention_days BETWEEN 1 AND 365),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(organization_id,location_id),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE TABLE gps_settings_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 version integer NOT NULL,
 enabled boolean NOT NULL,
 sample_interval_seconds integer NOT NULL,
 retention_days integer NOT NULL,
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE TABLE gps_samples (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 attendance_id uuid NOT NULL REFERENCES employee_attendance(id) ON DELETE CASCADE,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 latitude double precision NOT NULL CHECK(latitude BETWEEN -90 AND 90),
 longitude double precision NOT NULL CHECK(longitude BETWEEN -180 AND 180),
 accuracy_meters real CHECK(accuracy_meters IS NULL OR accuracy_meters BETWEEN 0 AND 100000),
 captured_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(attendance_id,captured_at),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE INDEX gps_samples_location_received_idx ON gps_samples(organization_id,location_id,received_at);
CREATE INDEX gps_samples_shift_idx ON gps_samples(attendance_id,captured_at);
CREATE TABLE gps_shift_state (
 attendance_id uuid PRIMARY KEY REFERENCES employee_attendance(id) ON DELETE CASCADE,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 state text NOT NULL CHECK(state IN ('active','permission_denied','permission_lost','unsupported','stopped')),
 last_sample_at timestamptz,
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
