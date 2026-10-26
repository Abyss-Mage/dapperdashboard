-- Explicit verified enrollment only. Matching email never grants ownership.
-- No automatic backfill, public linking route, or staff-grant fallback.
CREATE TABLE customer_account_links (
 principal_id uuid NOT NULL REFERENCES principals(id),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (principal_id,customer_id),
 FOREIGN KEY (organization_id,location_id,customer_id)
  REFERENCES customers(organization_id,location_id,id)
);
CREATE INDEX customer_account_owner_idx ON customer_account_links(organization_id,location_id,customer_id);
