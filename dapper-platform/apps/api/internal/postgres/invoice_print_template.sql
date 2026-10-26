-- The printed invoice layout. One template per organization, edited by people who hold invoices.configure (Super Admin, Owner).
-- It controls wording and presentation only (title, labels, accent colour, notes, paper size); amounts, parties and numbering always
-- come from the invoice itself. No row means the built-in default template.
CREATE TABLE invoice_print_templates (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 config jsonb NOT NULL CHECK(jsonb_typeof(config)='object'),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
