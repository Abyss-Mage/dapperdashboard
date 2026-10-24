-- Website metadata references the operational catalogue; no duplicate service names/prices.
ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
 'customers.read','customers.create','customer_accounts.manage','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage','bookings.read','bookings.create','bookings.manage',
 'invoices.read','invoices.create','invoices.issue','invoices.manage','payments.record','payments.read',
 'sop.read','sop.acknowledge','sop.manage','quotes.read','quotes.create','quotes.manage','quotes.issue',
 'dealership.read','dealership.manage','marketing.read','marketing.manage','content.manage'));
CREATE TABLE public_sites (
 organization_id uuid NOT NULL, location_id uuid NOT NULL, slug text NOT NULL UNIQUE CHECK(slug ~ '^[a-z0-9][a-z0-9-]{1,59}$'),
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 120), published boolean NOT NULL DEFAULT false,
 version integer NOT NULL DEFAULT 1 CHECK(version>0), PRIMARY KEY(organization_id,location_id),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE TABLE service_publications (
 organization_id uuid NOT NULL, location_id uuid NOT NULL, service_id uuid NOT NULL,
 category text NOT NULL CHECK(category IN ('2-wheeler','hatchback','sedan','suv','pickup','van','ceramic','ppf','aircraft','boat')),
 summary text NOT NULL CHECK(length(btrim(summary)) BETWEEN 1 AND 2000),
 published boolean NOT NULL, version integer NOT NULL DEFAULT 1 CHECK(version>0),
 PRIMARY KEY(organization_id,location_id,service_id),
 FOREIGN KEY(organization_id,location_id) REFERENCES public_sites(organization_id,location_id),
 FOREIGN KEY(organization_id,location_id,service_id) REFERENCES catalog_services(organization_id,location_id,id)
);
CREATE TABLE content_media (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL, location_id uuid NOT NULL,
 content_type text NOT NULL CHECK(content_type IN ('image/jpeg','image/png')),
 data bytea NOT NULL CHECK(octet_length(data) BETWEEN 1 AND 6291456),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE TABLE portfolio_images (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL, location_id uuid NOT NULL, media_id uuid NOT NULL,
 title text NOT NULL CHECK(length(btrim(title)) BETWEEN 2 AND 120), alt text NOT NULL CHECK(length(btrim(alt)) BETWEEN 2 AND 300),
 category text NOT NULL CHECK(category IN ('detailing','ceramic','ppf','other')), sort_order integer NOT NULL CHECK(sort_order BETWEEN 0 AND 10000),
 published boolean NOT NULL, version integer NOT NULL DEFAULT 1 CHECK(version>0),
 FOREIGN KEY(organization_id,location_id) REFERENCES public_sites(organization_id,location_id),
 FOREIGN KEY(organization_id,location_id,media_id) REFERENCES content_media(organization_id,location_id,id)
);
CREATE TABLE content_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, organization_id uuid NOT NULL, location_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id), action text NOT NULL, entity_id text NOT NULL, snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
