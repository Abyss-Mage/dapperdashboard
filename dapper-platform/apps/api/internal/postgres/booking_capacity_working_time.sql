-- The booking/reservation consistency check assumed (a) the occupied span is exactly start + duration + after and (b) a booking carries
-- exactly the add-ons of its rule. Two features break those assumptions on purpose:
--  * working-time rules (migration 58): the occupied span runs across closed hours, so it may be LONGER than start + duration + after;
--  * a base rule (no add-ons of its own) may be booked with any of its service's add-ons (they add no time).
-- Everything else stays exact.
CREATE OR REPLACE FUNCTION validate_booking_capacity(booking_uuid uuid) RETURNS void LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
DECLARE b bookings%ROWTYPE; r capacity_reservations%ROWTYPE; working boolean; rule_addons text[];
BEGIN
 SELECT * INTO b FROM bookings WHERE id=booking_uuid;
 IF NOT FOUND OR b.reservation_id IS NULL THEN RETURN; END IF;
 SELECT * INTO r FROM capacity_reservations WHERE id=b.reservation_id FOR SHARE;
 IF NOT FOUND OR r.rule_snapshot IS NULL THEN
  RAISE EXCEPTION 'Booking and capacity reservation must remain consistent' USING ERRCODE='23514',CONSTRAINT='booking_capacity_consistency';
 END IF;
 working := COALESCE((r.rule_snapshot->>'workingTime')::boolean,false);
 rule_addons := ARRAY(SELECT jsonb_array_elements_text(COALESCE(NULLIF(r.rule_snapshot->'addonIds','null'::jsonb),'[]'::jsonb)) ORDER BY 1);
 IF r.organization_id<>b.organization_id OR r.location_id<>b.location_id
  OR (r.request->>'Start')::timestamptz IS DISTINCT FROM b.booking_time
  OR (r.rule_snapshot->>'serviceId') IS DISTINCT FROM b.service_id::text
  OR r.occupied_start IS DISTINCT FROM b.booking_time-make_interval(mins=>(r.rule_snapshot->>'beforeMinutes')::integer)
  OR (NOT working AND r.occupied_end IS DISTINCT FROM b.booking_time+make_interval(mins=>((r.rule_snapshot->>'durationMinutes')::integer+(r.rule_snapshot->>'afterMinutes')::integer)))
  OR (working AND r.occupied_end < b.booking_time+make_interval(mins=>((r.rule_snapshot->>'durationMinutes')::integer+(r.rule_snapshot->>'afterMinutes')::integer)))
  OR r.units IS DISTINCT FROM (r.rule_snapshot->>'units')::integer
  OR (cardinality(rule_addons)>0 AND rule_addons IS DISTINCT FROM ARRAY(SELECT element->>'serviceId' FROM jsonb_array_elements(b.addons) element ORDER BY 1))
  OR (b.status<>'cancelled' AND r.cancelled_at IS NOT NULL)
 THEN RAISE EXCEPTION 'Booking and capacity reservation must remain consistent' USING ERRCODE='23514',CONSTRAINT='booking_capacity_consistency';
 END IF;
END $$;
