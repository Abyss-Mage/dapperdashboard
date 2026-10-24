ALTER TABLE invoices ADD COLUMN version integer NOT NULL DEFAULT 1 CHECK(version>0);

CREATE FUNCTION advance_invoice_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.version := OLD.version + 1;
 RETURN NEW;
END $$;
CREATE TRIGGER advance_invoice_version BEFORE UPDATE ON invoices FOR EACH ROW EXECUTE FUNCTION advance_invoice_version();

CREATE TABLE invoice_draft_revisions (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 invoice_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 from_version integer NOT NULL CHECK(from_version>0),
 to_version integer NOT NULL CHECK(to_version=from_version+1),
 before_snapshot jsonb NOT NULL CHECK(jsonb_typeof(before_snapshot)='object'),
 after_snapshot jsonb NOT NULL CHECK(jsonb_typeof(after_snapshot)='object'),
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id,invoice_id) REFERENCES invoices(organization_id,location_id,id),
 UNIQUE(invoice_id,to_version)
);
