-- Approved photo retention (user, 2026-10-24): dealership key tags are kept up to 30 days; customer-car inspection
-- photos are kept permanently. Tags carry an explicit expiry; cars have none. A purge removes expired posts and records
-- what was removed (no photo content, no names).
ALTER TABLE employee_upload_posts ADD COLUMN retention_until timestamptz;
UPDATE employee_upload_posts SET retention_until=created_at+interval '30 days' WHERE kind='dealership_tag';
ALTER TABLE employee_upload_posts ADD CONSTRAINT employee_upload_posts_retention CHECK((kind='dealership_tag')=(retention_until IS NOT NULL));
CREATE INDEX employee_upload_posts_expiry_idx ON employee_upload_posts(retention_until) WHERE retention_until IS NOT NULL;
CREATE TABLE employee_upload_purges (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 post_id uuid NOT NULL,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 kind text NOT NULL,
 photo_count integer NOT NULL,
 posted_at timestamptz NOT NULL,
 purged_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 reason text NOT NULL DEFAULT 'retention_30_days'
);
CREATE INDEX employee_upload_purges_idx ON employee_upload_purges(organization_id,location_id,purged_at DESC);
