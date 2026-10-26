-- Customer-car inspection posts belong to a booking (customer and dealership come from the booking, not typed text).
-- Dealership tags may name any active dealership account at the location; dealerships are separate from locations.
ALTER TABLE employee_upload_posts ADD COLUMN booking_id uuid;
ALTER TABLE employee_upload_posts ADD CONSTRAINT employee_upload_posts_booking_fk FOREIGN KEY(organization_id,location_id,booking_id) REFERENCES bookings(organization_id,location_id,id);
ALTER TABLE employee_upload_posts ADD CONSTRAINT employee_upload_posts_booking_kind CHECK((kind='customer_car')=(booking_id IS NOT NULL));
CREATE INDEX employee_upload_posts_booking_idx ON employee_upload_posts(organization_id,location_id,booking_id) WHERE booking_id IS NOT NULL;
