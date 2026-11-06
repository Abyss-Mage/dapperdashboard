-- Window tint becomes a website category of its own (its three packages already exist in the catalogue).
ALTER TABLE service_publications DROP CONSTRAINT service_publications_category_check;
ALTER TABLE service_publications ADD CONSTRAINT service_publications_category_check CHECK (category IN ('2-wheeler','hatchback','sedan','suv','pickup','van','ceramic','ppf','aircraft','boat','window-tint'));
