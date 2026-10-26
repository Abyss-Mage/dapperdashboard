ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','customer_accounts.manage','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage','bookings.read','bookings.create','bookings.manage',
 'invoices.read','invoices.create','invoices.issue','invoices.manage','payments.record','payments.read',
 'sop.read','sop.acknowledge','sop.manage',
 'quotes.read','quotes.create','quotes.manage','quotes.issue',
 'dealership.read','dealership.manage',
 'marketing.read','marketing.manage'));

ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage',
 'attendance.read','attendance.manage',
 'bookings.read','bookings.manage',
 'invoices.read','invoices.manage','invoices.configure','payments.read',
 'sop.read','sop.manage','access.manage',
 'quotes.read','quotes.manage',
 'dealership.read','dealership.manage',
 'marketing.read','marketing.manage'));

CREATE TABLE mailchimp_settings (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 api_key_configured boolean NOT NULL DEFAULT false,
 server_prefix text NOT NULL DEFAULT '',
 default_list_id text NOT NULL DEFAULT '',
 double_opt_in boolean NOT NULL DEFAULT false,
 sync_enabled boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id),
 UNIQUE(organization_id, location_id)
);

CREATE TABLE marketing_consents (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 customer_id uuid,
 email text NOT NULL CHECK(email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
 first_name text NOT NULL DEFAULT '',
 last_name text NOT NULL DEFAULT '',
 consent_status text NOT NULL CHECK(consent_status IN ('opt_in', 'opt_out', 'unsubscribed')),
 consent_source text NOT NULL CHECK(consent_source IN ('popup_form', 'website_footer', 'customer_portal', 'booking_checkout', 'manual_staff')),
 consent_text text NOT NULL CHECK(length(consent_text) > 0),
 ip_address text NOT NULL DEFAULT '',
 user_agent text NOT NULL DEFAULT '',
 consented_at timestamptz NOT NULL DEFAULT now(),
 revoked_at timestamptz,
 sync_status text NOT NULL DEFAULT 'pending' CHECK(sync_status IN ('pending', 'synced', 'failed')),
 mailchimp_subscriber_hash text NOT NULL DEFAULT '',
 last_synced_at timestamptz,
 last_sync_error text NOT NULL DEFAULT '',
 FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id),
 UNIQUE(organization_id, location_id, email)
);

CREATE TABLE popup_forms (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120),
 title text NOT NULL CHECK(length(btrim(title)) BETWEEN 2 AND 160),
 description text NOT NULL DEFAULT '',
 headline text NOT NULL DEFAULT '',
 fields jsonb NOT NULL DEFAULT '[]'::jsonb,
 audience_id text NOT NULL DEFAULT '',
 tags jsonb NOT NULL DEFAULT '[]'::jsonb,
 display_delay_seconds integer NOT NULL DEFAULT 5 CHECK(display_delay_seconds >= 0 AND display_delay_seconds <= 300),
 selected_pages jsonb NOT NULL DEFAULT '["/"]'::jsonb,
 repeat_display_limit_days integer NOT NULL DEFAULT 7 CHECK(repeat_display_limit_days >= 0 AND repeat_display_limit_days <= 365),
 success_message text NOT NULL DEFAULT 'Thank you for subscribing!' CHECK(length(success_message) > 0),
 active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id),
 UNIQUE(organization_id, location_id, id)
);

CREATE TABLE popup_submissions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 form_id uuid NOT NULL REFERENCES popup_forms(id) ON DELETE CASCADE,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 email text NOT NULL CHECK(email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
 first_name text NOT NULL DEFAULT '',
 last_name text NOT NULL DEFAULT '',
 metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
 ip_address text NOT NULL DEFAULT '',
 user_agent text NOT NULL DEFAULT '',
 submitted_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id)
);

CREATE INDEX idx_popup_forms_lookup ON popup_forms(organization_id, location_id, active);
CREATE INDEX idx_marketing_consents_sync ON marketing_consents(organization_id, location_id, sync_status);
CREATE INDEX idx_popup_submissions_form ON popup_submissions(form_id, submitted_at DESC);
