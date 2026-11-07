-- Enquiries sent from the public website's contact form. Staff read them in the dashboard and reply by phone or email.
CREATE TABLE website_enquiries (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 name text NOT NULL CHECK (length(btrim(name)) BETWEEN 2 AND 100),
 email text NOT NULL DEFAULT '' CHECK (length(email) <= 254),
 phone text NOT NULL DEFAULT '' CHECK (length(phone) <= 40),
 vehicle text NOT NULL DEFAULT '' CHECK (length(vehicle) <= 120),
 service text NOT NULL DEFAULT '' CHECK (length(service) <= 80),
 message text NOT NULL DEFAULT '' CHECK (length(message) <= 3000),
 status text NOT NULL DEFAULT 'new' CHECK (status IN ('new','read','archived')),
 created_at timestamptz NOT NULL DEFAULT now(),
 handled_by uuid REFERENCES principals(id),
 handled_at timestamptz,
 CHECK (email <> '' OR phone <> '')
);
CREATE INDEX website_enquiries_org_status_idx ON website_enquiries(organization_id, status, created_at DESC);
