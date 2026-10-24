ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage','attendance.read','attendance.manage',
 'bookings.read','bookings.manage','invoices.read','invoices.manage','invoices.configure','payments.read','sop.read','sop.manage','access.manage'));
ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage','bookings.read','bookings.create','bookings.manage',
 'invoices.read','invoices.create','invoices.issue','invoices.manage','payments.record','payments.read','sop.read','sop.acknowledge','sop.manage'));

CREATE TABLE invoice_issuers (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 legal_name text NOT NULL CHECK(length(btrim(legal_name)) BETWEEN 2 AND 200),
 address text NOT NULL CHECK(length(btrim(address)) BETWEEN 5 AND 1000),
 tax_registration text NOT NULL CHECK(length(tax_registration)<=100),
 currency text NOT NULL CHECK(currency='CAD'),
 number_prefix text NOT NULL CHECK(number_prefix ~ '^[A-Z0-9-]{1,24}$'),
 next_number bigint NOT NULL CHECK(next_number>0 AND next_number<9223372036854775807),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 updated_by uuid NOT NULL REFERENCES principals(id),
 UNIQUE(organization_id,id), UNIQUE(organization_id,number_prefix)
);
CREATE TABLE location_invoice_issuers (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 issuer_id uuid NOT NULL,
 PRIMARY KEY(organization_id,location_id),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id),
 FOREIGN KEY(organization_id,issuer_id) REFERENCES invoice_issuers(organization_id,id)
);
CREATE TABLE invoice_issuer_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL REFERENCES organizations(id),
 issuer_id uuid NOT NULL REFERENCES invoice_issuers(id),
 location_id uuid,
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL,
 snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE TABLE organization_grant_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL REFERENCES organizations(id),
 principal_id uuid NOT NULL REFERENCES principals(id),
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL, enabled boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE invoices ADD COLUMN issuer_id uuid;
ALTER TABLE invoices ADD COLUMN issuer_snapshot jsonb;
ALTER TABLE invoices ADD COLUMN customer_snapshot jsonb;
ALTER TABLE invoices ADD COLUMN issued_by uuid REFERENCES principals(id);
ALTER TABLE invoices ADD CONSTRAINT invoice_issuer_scope FOREIGN KEY(organization_id,issuer_id) REFERENCES invoice_issuers(organization_id,id);
ALTER TABLE invoices ADD CONSTRAINT invoice_snapshot_types CHECK(
 (issuer_snapshot IS NULL OR jsonb_typeof(issuer_snapshot)='object') AND
 (customer_snapshot IS NULL OR jsonb_typeof(customer_snapshot)='object'));
ALTER TABLE invoices ADD CONSTRAINT invoices_scoped_identity_key UNIQUE(organization_id,location_id,id);
ALTER TABLE invoices ADD CONSTRAINT invoices_scoped_customer_key UNIQUE(organization_id,location_id,customer_id,id);
ALTER TABLE jobs ADD CONSTRAINT jobs_scoped_customer_key UNIQUE(organization_id,location_id,customer_id,id);
ALTER TABLE invoices ADD CONSTRAINT invoices_booking_owner FOREIGN KEY(organization_id,location_id,customer_id,booking_id) REFERENCES bookings(organization_id,location_id,customer_id,id);
ALTER TABLE invoices ADD CONSTRAINT invoices_job_owner FOREIGN KEY(organization_id,location_id,customer_id,job_id) REFERENCES jobs(organization_id,location_id,customer_id,id);
ALTER TABLE invoice_items ADD CONSTRAINT invoice_items_owner FOREIGN KEY(organization_id,location_id,invoice_id) REFERENCES invoices(organization_id,location_id,id);
ALTER TABLE invoice_events ADD CONSTRAINT invoice_events_owner FOREIGN KEY(organization_id,location_id,invoice_id) REFERENCES invoices(organization_id,location_id,id);
ALTER TABLE payments ADD CONSTRAINT payments_invoice_owner FOREIGN KEY(organization_id,location_id,customer_id,invoice_id) REFERENCES invoices(organization_id,location_id,customer_id,id);

CREATE FUNCTION protect_issued_invoice() RETURNS trigger LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  IF OLD.status<>'draft' THEN RAISE EXCEPTION 'Issued invoices must be retained'; END IF;
  RETURN OLD;
 END IF;
 IF TG_OP='INSERT' OR (OLD.status='draft' AND NEW.status IN ('issued','partially_paid','paid')) THEN
  IF NEW.status IN ('issued','partially_paid','paid') AND
   (NEW.issuer_id IS NULL OR NEW.issuer_snapshot IS NULL OR NEW.customer_snapshot IS NULL OR NEW.issued_by IS NULL OR NEW.issued_at IS NULL) THEN
   RAISE EXCEPTION 'Explicit invoice issuance snapshot required';
  END IF;
 END IF;
 IF TG_OP='UPDATE' AND OLD.status<>'draft' AND ROW(OLD.organization_id,OLD.location_id,OLD.customer_id,OLD.booking_id,OLD.job_id,OLD.invoice_number,OLD.currency,OLD.subtotal_cents,OLD.tax_cents,OLD.total_cents,OLD.notes,OLD.issuer_id,OLD.issuer_snapshot,OLD.customer_snapshot,OLD.issued_by,OLD.issued_at)
  IS DISTINCT FROM ROW(NEW.organization_id,NEW.location_id,NEW.customer_id,NEW.booking_id,NEW.job_id,NEW.invoice_number,NEW.currency,NEW.subtotal_cents,NEW.tax_cents,NEW.total_cents,NEW.notes,NEW.issuer_id,NEW.issuer_snapshot,NEW.customer_snapshot,NEW.issued_by,NEW.issued_at) THEN
  RAISE EXCEPTION 'Issued invoice details are immutable';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER protect_issued_invoice BEFORE INSERT OR UPDATE OR DELETE ON invoices FOR EACH ROW EXECUTE FUNCTION protect_issued_invoice();

CREATE FUNCTION protect_issued_invoice_items() RETURNS trigger LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
DECLARE invoice_status text;
BEGIN
 IF TG_OP<>'INSERT' THEN
  SELECT status INTO invoice_status FROM invoices WHERE id=OLD.invoice_id FOR SHARE;
  IF invoice_status<>'draft' THEN RAISE EXCEPTION 'Issued invoice items are immutable'; END IF;
 END IF;
 IF TG_OP<>'DELETE' THEN
  SELECT status INTO invoice_status FROM invoices WHERE id=NEW.invoice_id FOR SHARE;
  IF invoice_status<>'draft' THEN RAISE EXCEPTION 'Issued invoice items are immutable'; END IF;
  RETURN NEW;
 END IF;
 RETURN OLD;
END $$;
CREATE TRIGGER protect_issued_invoice_items BEFORE INSERT OR UPDATE OR DELETE ON invoice_items FOR EACH ROW EXECUTE FUNCTION protect_issued_invoice_items();
