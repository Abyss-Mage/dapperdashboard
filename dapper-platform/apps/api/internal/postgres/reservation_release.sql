ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release'));
ALTER TABLE capacity_reservations ADD COLUMN cancelled_at timestamptz;
ALTER TABLE capacity_reservations ADD COLUMN cancelled_by uuid REFERENCES principals(id);
ALTER TABLE capacity_reservations ADD CONSTRAINT reservation_release_actor CHECK(
 (cancelled_at IS NULL AND cancelled_by IS NULL) OR (cancelled_at IS NOT NULL AND cancelled_by IS NOT NULL));
-- Preserve cancelled reservations and their request snapshots for audit/history.
CREATE INDEX capacity_reservations_active_idx ON capacity_reservations(organization_id,location_id,resource_id,occupied_start,occupied_end) WHERE cancelled_at IS NULL;
