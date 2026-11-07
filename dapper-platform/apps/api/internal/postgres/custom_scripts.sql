-- Extra code snippets for the public website, added by the owner (like WPCode in WordPress). Unlike website_tracking this
-- stores code that runs in visitor browsers, so only people who can manage access may change it.
CREATE TABLE custom_scripts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 80),
 placement text NOT NULL CHECK (placement IN ('head','body')),
 code text NOT NULL CHECK (length(code) BETWEEN 1 AND 20000),
 paths text[] NOT NULL DEFAULT '{}' CHECK (cardinality(paths) <= 20),
 active boolean NOT NULL DEFAULT true,
 created_by uuid REFERENCES principals(id),
 updated_by uuid REFERENCES principals(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX custom_scripts_org_idx ON custom_scripts(organization_id, created_at);
