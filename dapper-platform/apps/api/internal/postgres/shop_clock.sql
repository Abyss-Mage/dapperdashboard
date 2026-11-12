-- The shop clock (owner decision 2026-11-11): staff punch in, out and take breaks on a shared iPad in the shop with a 4-digit PIN the owner
-- sets, instead of clocking in on their own phones with location tracking. A PIN is unique within its franchise; only its hash is kept.
CREATE TABLE employee_clock_pins (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 pin_hash bytea NOT NULL,
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id, location_id, principal_id),
 UNIQUE(organization_id, location_id, pin_hash),
 FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id)
);
-- Location tracking is retired with the personal clock-in; past samples stay in the records until their retention runs out.
UPDATE gps_settings SET enabled=false WHERE enabled;
