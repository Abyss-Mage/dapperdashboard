ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','customer_accounts.manage','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage','bookings.read','bookings.create','bookings.manage',
 'invoices.read','invoices.create','invoices.issue','invoices.manage','payments.record','payments.read',
 'sop.read','sop.acknowledge','sop.manage',
 'quotes.read','quotes.create','quotes.manage','quotes.issue',
 'dealership.read','dealership.manage'));

ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage',
 'attendance.read','attendance.manage',
 'bookings.read','bookings.manage',
 'invoices.read','invoices.manage','invoices.configure','payments.read',
 'sop.read','sop.manage','access.manage',
 'quotes.read','quotes.manage',
 'dealership.read','dealership.manage'));

CREATE TABLE dealership_accounts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 account_name text NOT NULL CHECK(length(btrim(account_name)) BETWEEN 2 AND 160),
 account_code text NOT NULL CHECK(account_code ~ '^[A-Z0-9-]{2,32}$'),
 account_type text NOT NULL CHECK(account_type IN ('dealership', 'fleet', 'commercial')),
 contact_name text NOT NULL CHECK(length(btrim(contact_name)) BETWEEN 2 AND 120),
 contact_email text NOT NULL CHECK(contact_email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
 contact_phone text NOT NULL DEFAULT '' CHECK(length(contact_phone) <= 40),
 billing_terms_days integer NOT NULL DEFAULT 30 CHECK(billing_terms_days >= 0 AND billing_terms_days <= 180),
 tax_exempt boolean NOT NULL DEFAULT false,
 active boolean NOT NULL DEFAULT true,
 notes text NOT NULL DEFAULT '' CHECK(length(notes) <= 2000),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id, location_id, customer_id) REFERENCES customers(organization_id, location_id, id),
 FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id),
 UNIQUE(organization_id, location_id, account_code),
 UNIQUE(organization_id, location_id, id)
);

CREATE TABLE dealership_tags (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 dealership_id uuid NOT NULL,
 tag_name text NOT NULL CHECK(length(btrim(tag_name)) BETWEEN 1 AND 60),
 tag_color text NOT NULL DEFAULT '#0066cc' CHECK(tag_color ~ '^#[0-9a-fA-F]{6}$'),
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id, location_id, dealership_id) REFERENCES dealership_accounts(organization_id, location_id, id) ON DELETE CASCADE,
 UNIQUE(dealership_id, tag_name)
);

CREATE TABLE dealership_fleet_intake (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 dealership_id uuid NOT NULL,
 vehicle_id uuid NOT NULL,
 stock_number text NOT NULL CHECK(length(btrim(stock_number)) BETWEEN 1 AND 60),
 vin text NOT NULL DEFAULT '' CHECK(length(vin) <= 30),
 po_number text NOT NULL DEFAULT '' CHECK(length(po_number) <= 60),
 lot_location text NOT NULL DEFAULT '' CHECK(length(lot_location) <= 60),
 service_id uuid REFERENCES catalog_services(id),
 service_name text NOT NULL CHECK(length(btrim(service_name)) BETWEEN 2 AND 120),
 status text NOT NULL DEFAULT 'intake' CHECK(status IN ('intake', 'queued', 'in_progress', 'quality_checked', 'completed', 'released')),
 job_id uuid REFERENCES jobs(id),
 assigned_principal_id uuid REFERENCES principals(id),
 intake_notes text NOT NULL DEFAULT '' CHECK(length(intake_notes) <= 2000),
 received_at timestamptz NOT NULL DEFAULT now(),
 completed_at timestamptz,
 released_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id, location_id, dealership_id) REFERENCES dealership_accounts(organization_id, location_id, id),
 FOREIGN KEY(vehicle_id) REFERENCES vehicles(id),
 UNIQUE(dealership_id, stock_number),
 UNIQUE(organization_id, location_id, id)
);

CREATE TABLE dealership_intake_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 intake_id uuid NOT NULL,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL CHECK(action IN ('intake_received', 'status_changed', 'assigned', 'job_linked', 'released')),
 from_status text,
 to_status text,
 notes text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id, location_id, intake_id) REFERENCES dealership_fleet_intake(organization_id, location_id, id) ON DELETE CASCADE
);
