-- How large the owner wants each dealership logo on the website. logo is a built-in file's path (partners/partner-1.png) or dealers/<id> for an uploaded dealer.
CREATE TABLE site_logo_sizes (
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 logo text NOT NULL CHECK(logo ~ '^[A-Za-z0-9][A-Za-z0-9/_.-]{1,200}$' AND position('..' in logo)=0),
 size text NOT NULL CHECK(size IN ('s','m','l','xl')),
 PRIMARY KEY(organization_id, location_id, logo)
);
