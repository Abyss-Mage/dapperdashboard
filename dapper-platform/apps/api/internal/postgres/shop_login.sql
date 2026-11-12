-- The shop iPad, second round (error404 #20, owner decisions 2026-11-11): one iPad per franchise, signed in with the franchise's own shop
-- username and password (set by the owner; it opens only the worker list, never a dashboard). A worker taps their name and types their
-- 6-digit PIN; that opens their own employee dashboard on the iPad with Clock in / Break / Clock out on top, until they tap Done or leave it idle.
CREATE TABLE shop_logins (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 username text NOT NULL CHECK (username ~ '^[a-z0-9][a-z0-9._-]{2,39}$'),
 password_hash text NOT NULL,
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id, location_id),
 UNIQUE (username),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id)
);
-- The iPad's own sign-in; only the token's hash is kept. Changing or removing the shop login ends them.
CREATE TABLE shop_sessions (
 token_hash bytea PRIMARY KEY,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 revoked_at timestamptz,
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id)
);
-- A worker's dashboard session opened from the shop iPad remembers which franchise's iPad opened it (for the clock buttons there).
ALTER TABLE auth_sessions ADD COLUMN shop_location_id uuid;

-- PINs are now 6 digits and checked against the person tapped, so two people may not need different PINs; the 4-digit PINs set before
-- today no longer fit and are cleared (the owner sets new ones).
ALTER TABLE employee_clock_pins DROP CONSTRAINT employee_clock_pins_organization_id_location_id_pin_hash_key;
DELETE FROM employee_clock_pins;
