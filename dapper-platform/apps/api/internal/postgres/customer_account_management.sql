ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','customer_accounts.manage','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage','bookings.read','bookings.create','bookings.manage',
 'invoices.read','invoices.create','invoices.issue','invoices.manage','payments.record','payments.read','sop.read','sop.acknowledge','sop.manage'));

CREATE TABLE customer_account_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 actor_id uuid NOT NULL REFERENCES principals(id),
 enabled boolean NOT NULL,
 ownership_reviewed boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(NOT enabled OR ownership_reviewed),
 FOREIGN KEY(organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id)
);
CREATE INDEX customer_account_events_scope ON customer_account_events(organization_id,location_id,customer_id,created_at,id);
