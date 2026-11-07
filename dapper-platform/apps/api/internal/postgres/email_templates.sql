-- The owner's wording for the emails the system sends. A row replaces the built-in wording for that email; deleting it brings the built-in wording back.
CREATE TABLE email_templates (
 organization_id uuid NOT NULL, template_key text NOT NULL CHECK(template_key IN ('password_reset','invitation','booking_confirmation')),
 subject text NOT NULL CHECK(length(btrim(subject)) BETWEEN 1 AND 200), body text NOT NULL CHECK(length(btrim(body)) BETWEEN 1 AND 5000),
 updated_by uuid, updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id, template_key)
);
