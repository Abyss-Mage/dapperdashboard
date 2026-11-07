-- Printed quotation templates, kept apart from the invoice templates so quotes have their own title, terms, validity and look.
-- Same shape as invoice_print_templates and location_invoice_templates. No row means the quote starts from the invoice template.
CREATE TABLE quote_print_templates (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 config jsonb NOT NULL CHECK(jsonb_typeof(config)='object'),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE location_quote_templates (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 config jsonb NOT NULL CHECK (jsonb_typeof(config) = 'object'),
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_by uuid REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id, location_id),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id)
);
