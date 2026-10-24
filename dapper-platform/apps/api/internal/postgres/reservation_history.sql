ALTER TABLE capacity_reservations ADD CONSTRAINT reservation_scoped_identity UNIQUE(organization_id,location_id,id);
CREATE TABLE capacity_reservation_revisions (
 organization_id uuid NOT NULL,location_id uuid NOT NULL,reservation_id uuid NOT NULL,
 version integer NOT NULL CHECK(version>0),snapshot jsonb NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(reservation_id,version),
 FOREIGN KEY(organization_id,location_id,reservation_id) REFERENCES capacity_reservations(organization_id,location_id,id)
);
-- Capture future writes only. Do not fabricate historical actors or snapshots
-- for reservations created before this migration.
