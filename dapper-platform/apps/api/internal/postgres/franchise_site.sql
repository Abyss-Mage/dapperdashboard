-- A franchise IS a site: one record that carries the franchise code and (optional) address. The separate "location"
-- concept is retired from the product. Physically the site row stays in `locations` because grants, bookings,
-- scheduling, invoices and GPS are all keyed on it; the `franchises` row is kept as a 1:1 shadow so franchise-scoped
-- role assignments keep working. Codes are nullable only so rows created before this migration can be backfilled
-- explicitly; the application requires a code for every new franchise.
ALTER TABLE locations ADD COLUMN code text;
ALTER TABLE locations ADD COLUMN address_street text NOT NULL DEFAULT '';
ALTER TABLE locations ADD COLUMN address_city text NOT NULL DEFAULT '';
ALTER TABLE locations ADD COLUMN address_region text NOT NULL DEFAULT '';
ALTER TABLE locations ADD COLUMN address_postal_code text NOT NULL DEFAULT '';
ALTER TABLE locations ADD COLUMN address_country text NOT NULL DEFAULT '';
ALTER TABLE locations ADD CONSTRAINT locations_code_format CHECK(code IS NULL OR code ~ '^[A-Z0-9-]{2,12}$');
ALTER TABLE locations ADD CONSTRAINT locations_address_length CHECK(length(address_street)<=200 AND length(address_city)<=100 AND length(address_region)<=100 AND length(address_postal_code)<=20 AND length(address_country)<=100);
CREATE UNIQUE INDEX locations_code_unique ON locations(organization_id,code) WHERE code IS NOT NULL;
