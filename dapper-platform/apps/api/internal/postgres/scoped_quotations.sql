ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','customer_accounts.manage','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage','bookings.read','bookings.create','bookings.manage',
 'invoices.read','invoices.create','invoices.issue','invoices.manage','payments.record','payments.read',
 'sop.read','sop.acknowledge','sop.manage',
 'quotes.read','quotes.create','quotes.manage','quotes.issue'));

ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage',
 'attendance.read','attendance.manage',
 'bookings.read','bookings.manage',
 'invoices.read','invoices.manage','invoices.configure','payments.read',
 'sop.read','sop.manage','access.manage',
 'quotes.read','quotes.manage'));

CREATE TABLE scoped_quotations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 request_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 vehicle_id uuid NOT NULL,
 currency text NOT NULL CHECK(currency='CAD'),
 status text NOT NULL CHECK(status IN ('draft','issued','accepted','declined','expired','superseded','fulfilled')),
 version integer NOT NULL DEFAULT 1 CHECK(version > 0),
 subtotal_cents bigint NOT NULL CHECK(subtotal_cents >= 0 AND subtotal_cents <= 9000000000000),
 tax_cents bigint NOT NULL CHECK(tax_cents >= 0 AND tax_cents <= 9000000000000),
 deposit_cents bigint NOT NULL CHECK(deposit_cents >= 0 AND deposit_cents <= 18000000000000),
 total_cents bigint GENERATED ALWAYS AS (subtotal_cents + tax_cents) STORED,
 reference_number text NOT NULL CHECK(length(reference_number) BETWEEN 6 AND 30),
 internal_notes text NOT NULL DEFAULT '' CHECK(length(internal_notes) <= 2000),
 customer_note text NOT NULL DEFAULT '' CHECK(length(customer_note) <= 2000),
 expires_at timestamptz,
 issued_at timestamptz,
 issued_by uuid REFERENCES principals(id),
 decided_at timestamptz,
 booking_id uuid,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id, location_id, id),
 UNIQUE(organization_id, location_id, reference_number),
 FOREIGN KEY(organization_id, location_id, request_id) REFERENCES customer_booking_requests(organization_id, location_id, id),
 FOREIGN KEY(organization_id, location_id, customer_id) REFERENCES customers(organization_id, location_id, id),
 FOREIGN KEY(organization_id, location_id, customer_id, vehicle_id) REFERENCES vehicles(organization_id, location_id, customer_id, id),
 CHECK(deposit_cents <= total_cents),
 CHECK(status <> 'issued' OR (issued_at IS NOT NULL AND issued_by IS NOT NULL)),
 CHECK(status NOT IN ('accepted','declined','fulfilled') OR decided_at IS NOT NULL)
);

CREATE TABLE scoped_quotation_items (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 quotation_id uuid NOT NULL REFERENCES scoped_quotations(id),
 sort_order integer NOT NULL DEFAULT 0,
 description text NOT NULL CHECK(length(btrim(description)) BETWEEN 1 AND 500),
 quantity integer NOT NULL DEFAULT 1 CHECK(quantity BETWEEN 1 AND 999),
 unit_price_cents bigint NOT NULL CHECK(unit_price_cents >= 0 AND unit_price_cents <= 9000000000000),
 service_id uuid,
 service_name_snapshot text,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX scoped_quotation_items_quotation ON scoped_quotation_items(quotation_id, sort_order, id);

CREATE TABLE scoped_quotation_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 quotation_id uuid NOT NULL REFERENCES scoped_quotations(id),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL CHECK(action IN ('created','edited','issued','accepted','declined','expired','superseded','fulfilled')),
 note text NOT NULL DEFAULT '' CHECK(length(note) <= 500),
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id, location_id, quotation_id) REFERENCES scoped_quotations(organization_id, location_id, id)
);
CREATE INDEX scoped_quotation_events_quotation ON scoped_quotation_events(quotation_id, created_at, id);

CREATE TABLE scoped_quotation_acceptances (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 quotation_id uuid NOT NULL REFERENCES scoped_quotations(id),
 quotation_version integer NOT NULL CHECK(quotation_version > 0),
 customer_id uuid NOT NULL,
 content_sha256 bytea NOT NULL CHECK(octet_length(content_sha256) = 32),
 actor_id uuid NOT NULL REFERENCES principals(id),
 acceptance_key text NOT NULL CHECK(length(acceptance_key) BETWEEN 16 AND 128),
 terms_reviewed boolean NOT NULL CHECK(terms_reviewed),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(quotation_id, quotation_version),
 UNIQUE(actor_id, acceptance_key)
);

CREATE TABLE quote_fulfillment_receipts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 request_id uuid NOT NULL UNIQUE,
 quotation_id uuid NOT NULL UNIQUE,
 booking_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 request_key text NOT NULL,
 request_hash bytea NOT NULL CHECK(octet_length(request_hash) = 32),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(actor_id, request_key),
 FOREIGN KEY(organization_id, location_id, request_id) REFERENCES customer_booking_requests(organization_id, location_id, id),
 FOREIGN KEY(organization_id, location_id, quotation_id) REFERENCES scoped_quotations(organization_id, location_id, id)
);

CREATE FUNCTION protect_quotation_immutable_record() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Quotation audit and acceptance records are immutable' USING ERRCODE='23514'; END $$;

CREATE TRIGGER scoped_quotation_acceptance_immutable
 BEFORE UPDATE OR DELETE ON scoped_quotation_acceptances
 FOR EACH ROW EXECUTE FUNCTION protect_quotation_immutable_record();

CREATE TRIGGER scoped_quotation_event_immutable
 BEFORE UPDATE OR DELETE ON scoped_quotation_events
 FOR EACH ROW EXECUTE FUNCTION protect_quotation_immutable_record();

CREATE TRIGGER quote_fulfillment_receipt_immutable
 BEFORE UPDATE OR DELETE ON quote_fulfillment_receipts
 FOR EACH ROW EXECUTE FUNCTION protect_quotation_immutable_record();

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS quotation_id uuid REFERENCES scoped_quotations(id);

CREATE INDEX scoped_quotations_location_history ON scoped_quotations(organization_id, location_id, created_at DESC, id);
CREATE INDEX scoped_quotations_request ON scoped_quotations(request_id, status, created_at DESC);
CREATE INDEX scoped_quotations_customer ON scoped_quotations(organization_id, location_id, customer_id, created_at DESC);
