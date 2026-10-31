-- A franchise can have its own printed invoice template. Without one it uses the organization's template.
CREATE TABLE location_invoice_templates (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 config jsonb NOT NULL CHECK (jsonb_typeof(config) = 'object'),
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_by uuid REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id, location_id),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id)
);
