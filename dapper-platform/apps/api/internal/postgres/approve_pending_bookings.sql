-- Every booking is Approved by default. Bookings still waiting from before that rule are approved, and the history says so.
WITH approved AS (
  UPDATE bookings SET status='confirmed', updated_at=now() WHERE status='pending' RETURNING id, organization_id, location_id
)
INSERT INTO booking_events(booking_id,organization_id,location_id,actor_id,from_status,to_status,notes)
SELECT id, organization_id, location_id, NULL, 'pending', 'confirmed', 'Approved automatically: every booking is approved by default' FROM approved;
