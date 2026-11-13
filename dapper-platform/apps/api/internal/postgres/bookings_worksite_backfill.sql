-- error404 #88 (2026-11-14): bookings brought in by the legacy import after owner_wave_2026_10 had no staff worksite row, so the employee
-- workspace calendar and agenda did not show them. Same backfill: every booking is this location's work unless linked otherwise.
INSERT INTO employee_booking_worksites(booking_id,organization_id,location_id,dealership_id,linked_by)
SELECT b.id,b.organization_id,b.location_id,NULL,
 COALESCE((SELECT be.actor_id FROM booking_events be WHERE be.booking_id=b.id AND be.actor_id IS NOT NULL ORDER BY be.created_at LIMIT 1),
          (SELECT m.principal_id FROM organization_memberships m WHERE m.organization_id=b.organization_id ORDER BY m.principal_id LIMIT 1))
FROM bookings b
WHERE NOT EXISTS(SELECT 1 FROM employee_booking_worksites w WHERE w.booking_id=b.id)
 AND COALESCE((SELECT be.actor_id FROM booking_events be WHERE be.booking_id=b.id AND be.actor_id IS NOT NULL ORDER BY be.created_at LIMIT 1),
          (SELECT m.principal_id FROM organization_memberships m WHERE m.organization_id=b.organization_id ORDER BY m.principal_id LIMIT 1)) IS NOT NULL;

