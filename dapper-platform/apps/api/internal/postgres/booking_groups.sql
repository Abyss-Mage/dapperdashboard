-- error404 #166 (owner 2026-11-23): the booking category a service is listed under when customers and staff book (Detailing, Ceramic
-- coating, Paint protection film, Window tinting or one the owner names, such as "Tire services"). Empty = worked out from the name, as
-- before. A franchise copy of a central service uses the central service's category.
ALTER TABLE central_catalog_services ADD COLUMN booking_group text NOT NULL DEFAULT '' CHECK (length(booking_group) <= 40);
ALTER TABLE catalog_services ADD COLUMN booking_group text NOT NULL DEFAULT '' CHECK (length(booking_group) <= 40);
