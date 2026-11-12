-- A new franchise starts from the bays and weekly hours of the organization's largest franchise, so it can take bookings at once. The owner changes them under Setup.
CREATE FUNCTION add_default_bays_to_location(org uuid, loc uuid) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM scheduling_resources WHERE organization_id=org AND location_id=loc) THEN RETURN; END IF;
 INSERT INTO scheduling_resources(organization_id,location_id,name,capacity,timezone,rules,weekly_hours)
 SELECT org,loc,r.name,r.capacity,r.timezone,r.rules,r.weekly_hours FROM scheduling_resources r
 WHERE r.organization_id=org AND r.location_id=(SELECT s.location_id FROM scheduling_resources s WHERE s.organization_id=org AND s.location_id<>loc GROUP BY s.location_id ORDER BY count(*) DESC,s.location_id LIMIT 1);
END $$;
-- Franchises already open catch up now, once. Their services become bookable when the service starts (SyncMissingServiceRules).
SELECT add_default_bays_to_location(organization_id,id) FROM locations;
