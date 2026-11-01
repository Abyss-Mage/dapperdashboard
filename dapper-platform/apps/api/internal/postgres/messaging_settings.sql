-- Outgoing email (SMTP) and text message (Twilio) settings, entered in the dashboard by people who manage access. Passwords and the
-- Twilio Auth Token are sealed with the server key and never returned. One row per organization; each half is optional.
CREATE TABLE messaging_settings (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 smtp_host text NOT NULL DEFAULT '' CHECK (length(smtp_host) <= 255),
 smtp_port integer NOT NULL DEFAULT 587 CHECK (smtp_port BETWEEN 1 AND 65535),
 smtp_username text NOT NULL DEFAULT '' CHECK (length(smtp_username) <= 255),
 smtp_password_sealed bytea,
 smtp_from_address text NOT NULL DEFAULT '' CHECK (length(smtp_from_address) <= 254),
 smtp_from_name text NOT NULL DEFAULT '' CHECK (length(smtp_from_name) <= 120),
 twilio_account_sid text NOT NULL DEFAULT '' CHECK (twilio_account_sid = '' OR twilio_account_sid ~ '^AC[0-9a-fA-F]{32}$'),
 twilio_auth_token_sealed bytea,
 twilio_from text NOT NULL DEFAULT '' CHECK (twilio_from = '' OR twilio_from ~ '^(\+[1-9][0-9]{7,14}|MG[0-9a-fA-F]{32})$'),
 updated_by uuid REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
