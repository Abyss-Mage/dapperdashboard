-- Marketing and analytics tags for the public website. Only the identifiers are stored, never scripts: the site builds the
-- tags itself, so nothing an administrator pastes can run as arbitrary code.
CREATE TABLE website_tracking (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 gtm_id text NOT NULL DEFAULT '' CHECK (gtm_id = '' OR gtm_id ~ '^GTM-[A-Z0-9]{4,12}$'),
 meta_pixel_id text NOT NULL DEFAULT '' CHECK (meta_pixel_id = '' OR meta_pixel_id ~ '^[0-9]{8,20}$'),
 mailchimp_script_url text NOT NULL DEFAULT '' CHECK (mailchimp_script_url = '' OR (length(mailchimp_script_url) <= 600 AND mailchimp_script_url ~ '^https://(chimpstatic\.com|[a-z0-9-]+\.list-manage\.com)/[A-Za-z0-9._~:/?#@!$&()*+,;=%-]+$')),
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_by uuid REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
