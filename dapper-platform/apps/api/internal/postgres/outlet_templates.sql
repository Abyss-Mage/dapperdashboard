-- Two printed invoice templates: one for brand-owned outlets and one for franchisee outlets.
-- The existing organization template (invoice_print_templates) stays the brand-owned one. Franchisee outlets get their own,
-- stored beside it in the same shape; until it is saved they print with the brand-owned template.
-- Each outlet is one or the other; every existing outlet starts as brand-owned.
ALTER TABLE locations ADD COLUMN ownership text NOT NULL DEFAULT 'brand' CHECK (ownership IN ('brand','franchise'));

CREATE TABLE franchise_print_templates (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 config jsonb NOT NULL CHECK(jsonb_typeof(config)='object'),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
