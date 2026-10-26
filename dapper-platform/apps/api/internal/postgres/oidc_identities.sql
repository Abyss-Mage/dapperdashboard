-- Identity mappings are provisioned by an authorized administrative process.
-- A successful provider login must never create its own principal or grants.
CREATE TABLE oidc_identities (
 issuer text NOT NULL,
 subject text NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 PRIMARY KEY (issuer,subject)
);
