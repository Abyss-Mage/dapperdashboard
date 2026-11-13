-- One-time clean-up the owner approved (tester report #80, 2026-11-13): every booking still shown as Approved and dated up to and including
-- 13 November 2026 (Peterborough time) is marked completed. Each one gets a history entry; no emails go out (this is not a live change).
WITH done AS (
 UPDATE bookings SET status='completed'
 WHERE status IN ('confirmed','awaiting_payment','scheduled','in_progress')
   AND (booking_time AT TIME ZONE 'America/Toronto')::date <= DATE '2026-11-13'
 RETURNING id, organization_id, location_id
)
INSERT INTO booking_events(booking_id,organization_id,location_id,actor_id,from_status,to_status,notes)
SELECT id, organization_id, location_id, NULL, 'confirmed', 'completed', 'Marked completed in the clean-up of bookings up to 13 November 2026' FROM done;
