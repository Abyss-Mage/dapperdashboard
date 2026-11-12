-- One package in each website category can be the star package.
ALTER TABLE service_publications ADD COLUMN starred boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX service_publications_one_star ON service_publications(organization_id, location_id, category) WHERE starred;
