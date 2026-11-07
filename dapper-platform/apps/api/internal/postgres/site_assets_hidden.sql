-- Built-in pictures, logos and films the owner has taken off the website (the file stays; the website just stops showing it).
CREATE TABLE site_asset_hidden (
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 path text NOT NULL CHECK(path ~ '^[A-Za-z0-9][A-Za-z0-9/_.-]{1,200}$' AND position('..' in path)=0),
 PRIMARY KEY(organization_id, location_id, path)
);
