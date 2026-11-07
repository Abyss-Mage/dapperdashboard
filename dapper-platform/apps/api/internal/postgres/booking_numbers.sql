-- Every booking has a short number, one sequence per organization. Bookings imported from the old BookingPress site keep the
-- ID that site showed (so staff and customers see the same number); every other booking, and every new one, follows on in order.
ALTER TABLE bookings ADD COLUMN number bigint;

UPDATE bookings b SET number = CASE WHEN m.source_id ~ '^[0-9]+$' THEN m.source_id::bigint END
FROM legacy_import_map m
WHERE m.target_id=b.id AND m.source_system='bookingpress' AND m.source_kind='booking';

WITH ranked AS (SELECT id, organization_id, row_number() OVER (PARTITION BY organization_id ORDER BY created_at, id) AS rn FROM bookings WHERE number IS NULL),
 base AS (SELECT organization_id, COALESCE(max(number),0) AS highest FROM bookings GROUP BY organization_id)
UPDATE bookings b SET number = base.highest + ranked.rn FROM ranked JOIN base ON base.organization_id=ranked.organization_id WHERE b.id=ranked.id;

CREATE TABLE booking_counters (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 next_number bigint NOT NULL CHECK(next_number > 0)
);
INSERT INTO booking_counters(organization_id,next_number) SELECT organization_id, max(number)+1 FROM bookings GROUP BY organization_id;

-- The updates above queue deferred foreign-key checks; settle them before the table is altered again.
SET CONSTRAINTS ALL IMMEDIATE;
ALTER TABLE bookings ALTER COLUMN number SET NOT NULL;
CREATE UNIQUE INDEX bookings_number_key ON bookings(organization_id,number);

CREATE FUNCTION assign_booking_number() RETURNS trigger LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
BEGIN
 IF NEW.number IS NULL THEN
  INSERT INTO booking_counters(organization_id,next_number) VALUES(NEW.organization_id,2)
  ON CONFLICT (organization_id) DO UPDATE SET next_number=booking_counters.next_number+1
  RETURNING next_number-1 INTO NEW.number;
 ELSE
  INSERT INTO booking_counters(organization_id,next_number) VALUES(NEW.organization_id,NEW.number+1)
  ON CONFLICT (organization_id) DO UPDATE SET next_number=GREATEST(booking_counters.next_number,NEW.number+1);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER assign_booking_number BEFORE INSERT ON bookings FOR EACH ROW EXECUTE FUNCTION assign_booking_number();
