ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','vehicles.read','vehicles.create',
 'jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage',
 'scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage',
 'bookings.read','bookings.create','bookings.manage'));

ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage',
 'attendance.read','attendance.manage',
 'bookings.read','bookings.manage'));

CREATE TABLE bookings (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 vehicle_id uuid REFERENCES vehicles(id),
 service_id uuid,
 service_name text NOT NULL CHECK(length(btrim(service_name)) BETWEEN 2 AND 120),
 addons jsonb NOT NULL DEFAULT '[]'::jsonb,
 reservation_id uuid,
 channel text NOT NULL CHECK(channel IN ('online','internal','phone','walk_in','dealership')),
 status text NOT NULL CHECK(status IN ('pending','awaiting_payment','confirmed','scheduled','in_progress','completed','cancelled')),
 booking_time timestamptz NOT NULL,
 notes text NOT NULL DEFAULT '',
 cancellation_reason text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id),
 FOREIGN KEY(organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id,reservation_id) REFERENCES capacity_reservations(organization_id,location_id,id)
);

CREATE INDEX bookings_scope_idx ON bookings(organization_id,location_id,booking_time);
CREATE INDEX bookings_customer_idx ON bookings(organization_id,location_id,customer_id);
CREATE INDEX bookings_status_idx ON bookings(organization_id,location_id,status);

CREATE TABLE booking_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 actor_id uuid REFERENCES principals(id),
 from_status text,
 to_status text NOT NULL,
 notes text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX booking_events_booking_idx ON booking_events(booking_id,created_at);

ALTER TABLE jobs ADD COLUMN booking_id uuid REFERENCES bookings(id);
CREATE INDEX jobs_booking_idx ON jobs(booking_id) WHERE booking_id IS NOT NULL;
