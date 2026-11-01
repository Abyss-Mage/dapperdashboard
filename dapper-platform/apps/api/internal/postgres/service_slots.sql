-- Slots belong to each service: how many bookings of that service can run at the same time. Existing rules keep the number the
-- shared calendar allowed them before (calendar slots divided by the units the service used).
ALTER TABLE service_scheduling_rules ADD COLUMN slots integer NOT NULL DEFAULT 1 CHECK (slots BETWEEN 1 AND 1000);
UPDATE service_scheduling_rules r SET slots = LEAST(1000, GREATEST(1, s.capacity / r.units))
 FROM scheduling_resources s WHERE s.organization_id = r.organization_id AND s.location_id = r.location_id AND s.id = r.resource_id;
