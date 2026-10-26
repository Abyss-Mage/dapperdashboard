-- Phone checks for returning customers on the public booking pop-up. A code is sent to the phone; only after the right code is
-- entered does the site show the saved name and email for that number. Codes are stored hashed and expire after 10 minutes.
CREATE TABLE phone_verifications (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 phone_digits text NOT NULL CHECK(phone_digits ~ '^[0-9]{7,15}$'),
 code_hash bytea NOT NULL,
 token_hash bytea,
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
 expires_at timestamptz NOT NULL,
 verified_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE INDEX phone_verifications_lookup ON phone_verifications(organization_id,location_id,phone_digits,created_at DESC);
-- Online bookings made through the website are recorded with the channel "online" like any other online booking.
