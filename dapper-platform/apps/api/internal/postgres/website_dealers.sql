-- The dealerships shown on the website (a logo and a name each), managed from the dashboard with the same publish rules as portfolio photos:
-- nothing appears until it is published, and publishing needs the owner's confirmation that the dealer's logo may be shown.
CREATE TABLE website_dealers (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL, location_id uuid NOT NULL, media_id uuid NOT NULL,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120), sort_order integer NOT NULL CHECK(sort_order BETWEEN 0 AND 10000),
 published boolean NOT NULL, version integer NOT NULL DEFAULT 1 CHECK(version>0),
 FOREIGN KEY(organization_id,location_id) REFERENCES public_sites(organization_id,location_id),
 FOREIGN KEY(organization_id,location_id,media_id) REFERENCES content_media(organization_id,location_id,id)
);
CREATE INDEX website_dealers_site_idx ON website_dealers(organization_id,location_id,sort_order);
