ALTER TABLE jobs ADD COLUMN service_id uuid;
ALTER TABLE jobs ADD CONSTRAINT job_service_scope
 FOREIGN KEY(organization_id,location_id,service_id) REFERENCES catalog_services(organization_id,location_id,id);
-- Existing/manual work orders preserve their original name snapshot. New
-- catalogue-linked orders retain both source identity and creation-time name.
