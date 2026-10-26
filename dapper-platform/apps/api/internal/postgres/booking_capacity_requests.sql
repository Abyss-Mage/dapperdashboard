CREATE UNIQUE INDEX bookings_reservation_identity ON bookings(reservation_id) WHERE reservation_id IS NOT NULL;
ALTER TABLE bookings ADD CONSTRAINT bookings_scoped_identity UNIQUE(organization_id,location_id,id);
CREATE TABLE booking_capacity_requests (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 request_key text NOT NULL CHECK(length(request_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 actor_id uuid NOT NULL REFERENCES principals(id),
 booking_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,location_id,request_key),
 FOREIGN KEY(organization_id,location_id,booking_id) REFERENCES bookings(organization_id,location_id,id)
);

CREATE FUNCTION validate_booking_capacity(booking_uuid uuid) RETURNS void LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
DECLARE b bookings%ROWTYPE; r capacity_reservations%ROWTYPE;
BEGIN
 SELECT * INTO b FROM bookings WHERE id=booking_uuid;
 IF NOT FOUND OR b.reservation_id IS NULL THEN RETURN; END IF;
 SELECT * INTO r FROM capacity_reservations WHERE id=b.reservation_id FOR SHARE;
 IF NOT FOUND OR r.organization_id<>b.organization_id OR r.location_id<>b.location_id OR r.rule_snapshot IS NULL
  OR (r.request->>'Start')::timestamptz IS DISTINCT FROM b.booking_time
  OR (r.rule_snapshot->>'serviceId') IS DISTINCT FROM b.service_id::text
  OR r.occupied_start IS DISTINCT FROM b.booking_time-make_interval(mins=>(r.rule_snapshot->>'beforeMinutes')::integer)
  OR r.occupied_end IS DISTINCT FROM b.booking_time+make_interval(mins=>((r.rule_snapshot->>'durationMinutes')::integer+(r.rule_snapshot->>'afterMinutes')::integer))
  OR r.units IS DISTINCT FROM (r.rule_snapshot->>'units')::integer
  OR ARRAY(SELECT jsonb_array_elements_text(COALESCE(NULLIF(r.rule_snapshot->'addonIds','null'::jsonb),'[]'::jsonb)) ORDER BY 1)
     IS DISTINCT FROM ARRAY(SELECT element->>'serviceId' FROM jsonb_array_elements(b.addons) element ORDER BY 1)
  OR (b.status<>'cancelled' AND r.cancelled_at IS NOT NULL)
 THEN RAISE EXCEPTION 'Booking and capacity reservation must remain consistent' USING ERRCODE='23514',CONSTRAINT='booking_capacity_consistency';
 END IF;
END $$;
DO $$ DECLARE booking_uuid uuid; BEGIN
 FOR booking_uuid IN SELECT id FROM bookings WHERE reservation_id IS NOT NULL LOOP PERFORM validate_booking_capacity(booking_uuid); END LOOP;
END $$;
CREATE FUNCTION check_booking_capacity_binding() RETURNS trigger LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
BEGIN
 PERFORM validate_booking_capacity(NEW.id);
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER check_booking_capacity_binding AFTER INSERT OR UPDATE ON bookings
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_booking_capacity_binding();
CREATE FUNCTION check_reservation_booking_binding() RETURNS trigger LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
DECLARE booking_uuid uuid;
BEGIN
 FOR booking_uuid IN SELECT id FROM bookings WHERE reservation_id=NEW.id LOOP
  PERFORM validate_booking_capacity(booking_uuid);
 END LOOP;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER check_reservation_booking_binding AFTER UPDATE ON capacity_reservations
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_reservation_booking_binding();
