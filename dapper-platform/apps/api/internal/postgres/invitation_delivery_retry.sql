-- A send the mail server refused is tried again later, a limited number of times. Rows that failed for any other reason keep 0 and are left alone.
ALTER TABLE member_invitation_deliveries ADD COLUMN send_failures integer NOT NULL DEFAULT 0, ADD COLUMN retry_at timestamptz;
