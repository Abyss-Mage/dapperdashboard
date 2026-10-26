ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read'));
ALTER TABLE jobs ADD CONSTRAINT job_scoped_identity UNIQUE(organization_id,location_id,id);
CREATE TABLE job_contributors (
 organization_id uuid NOT NULL,location_id uuid NOT NULL,job_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),started_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(job_id,principal_id),
 FOREIGN KEY(organization_id,location_id,job_id) REFERENCES jobs(organization_id,location_id,id)
);
-- No fixed assignment or fabricated historical contributor backfill.
