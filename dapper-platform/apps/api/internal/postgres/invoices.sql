ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','vehicles.read','vehicles.create',
 'jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage',
 'scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage',
 'bookings.read','bookings.create','bookings.manage',
 'invoices.read','invoices.create','invoices.manage','payments.record','payments.read'));

ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage',
 'attendance.read','attendance.manage',
 'bookings.read','bookings.manage',
 'invoices.read','invoices.manage','payments.read'));

CREATE TABLE invoices (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 booking_id uuid REFERENCES bookings(id),
 job_id uuid,
 invoice_number text NOT NULL CHECK(length(btrim(invoice_number)) BETWEEN 3 AND 60),
 status text NOT NULL CHECK(status IN ('draft','issued','partially_paid','paid','void')),
 currency text NOT NULL DEFAULT 'CAD',
 subtotal_cents bigint NOT NULL CHECK(subtotal_cents >= 0),
 tax_cents bigint NOT NULL CHECK(tax_cents >= 0),
 total_cents bigint NOT NULL CHECK(total_cents >= 0),
 paid_cents bigint NOT NULL DEFAULT 0 CHECK(paid_cents >= 0),
 due_cents bigint NOT NULL CHECK(due_cents >= 0),
 notes text NOT NULL DEFAULT '',
 issued_at timestamptz,
 due_at timestamptz,
 paid_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id),
 FOREIGN KEY(organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id),
 UNIQUE(organization_id,invoice_number),
 CHECK(paid_cents <= total_cents),
 CHECK(due_cents = total_cents - paid_cents)
);

CREATE INDEX invoices_scope_idx ON invoices(organization_id,location_id,created_at DESC);
CREATE INDEX invoices_customer_idx ON invoices(organization_id,location_id,customer_id);
CREATE INDEX invoices_status_idx ON invoices(organization_id,location_id,status);

CREATE TABLE invoice_items (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 invoice_id uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 description text NOT NULL CHECK(length(btrim(description)) BETWEEN 2 AND 255),
 quantity integer NOT NULL DEFAULT 1 CHECK(quantity > 0),
 unit_price_cents bigint NOT NULL CHECK(unit_price_cents >= 0),
 total_price_cents bigint NOT NULL CHECK(total_price_cents >= 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(total_price_cents = quantity * unit_price_cents)
);

CREATE INDEX invoice_items_invoice_idx ON invoice_items(invoice_id);

CREATE TABLE payments (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 invoice_id uuid NOT NULL REFERENCES invoices(id),
 customer_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 amount_cents bigint NOT NULL CHECK(amount_cents > 0),
 currency text NOT NULL DEFAULT 'CAD',
 method text NOT NULL CHECK(method IN ('cash','square_card','square_terminal','bank_transfer','cheque')),
 status text NOT NULL CHECK(status IN ('pending','completed','refunded','failed')),
 provider_reference text NOT NULL DEFAULT '',
 idempotency_key text NOT NULL DEFAULT '',
 notes text NOT NULL DEFAULT '',
 recorded_at timestamptz NOT NULL DEFAULT now(),
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id),
 FOREIGN KEY(organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id)
);

CREATE INDEX payments_invoice_idx ON payments(invoice_id,recorded_at);
CREATE INDEX payments_scope_idx ON payments(organization_id,location_id,recorded_at DESC);
CREATE UNIQUE INDEX payments_idempotency_idx ON payments(organization_id,location_id,idempotency_key) WHERE idempotency_key <> '';

CREATE TABLE invoice_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 invoice_id uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 from_status text,
 to_status text NOT NULL,
 notes text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX invoice_events_invoice_idx ON invoice_events(invoice_id,created_at);
