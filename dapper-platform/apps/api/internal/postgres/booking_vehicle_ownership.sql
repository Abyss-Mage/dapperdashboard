-- Optional equipment must belong to the booking customer in the exact scope.
ALTER TABLE vehicles ADD CONSTRAINT vehicles_scoped_owner_key
 UNIQUE (organization_id, location_id, customer_id, id);
ALTER TABLE bookings DROP CONSTRAINT bookings_vehicle_id_fkey;
ALTER TABLE bookings ADD CONSTRAINT bookings_vehicle_owner_fkey
 FOREIGN KEY (organization_id, location_id, customer_id, vehicle_id)
 REFERENCES vehicles (organization_id, location_id, customer_id, id);
