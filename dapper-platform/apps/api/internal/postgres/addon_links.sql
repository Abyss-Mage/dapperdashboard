-- One add-on, offered with many main services.
--
-- Until now an add-on belonged to exactly one main service, so offering "Claybar" with 20 main services meant 20 separate add-on
-- rows. The add-on is now ONE row, and these link tables say which main services it is offered with. parent_id stays as the add-on's
-- primary main service (it keeps older checks and the central-to-franchise sync valid) and is always one of the links.

CREATE TABLE central_addon_parents (
 organization_id uuid NOT NULL,
 addon_id uuid NOT NULL,
 main_id uuid NOT NULL,
 PRIMARY KEY (addon_id, main_id),
 FOREIGN KEY (organization_id, addon_id) REFERENCES central_catalog_services(organization_id, id) ON DELETE CASCADE,
 FOREIGN KEY (organization_id, main_id) REFERENCES central_catalog_services(organization_id, id) ON DELETE CASCADE,
 CHECK (addon_id <> main_id)
);
CREATE INDEX central_addon_parents_main_idx ON central_addon_parents(main_id);

CREATE TABLE catalog_addon_parents (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 addon_id uuid NOT NULL,
 main_id uuid NOT NULL,
 PRIMARY KEY (addon_id, main_id),
 FOREIGN KEY (organization_id, location_id, addon_id) REFERENCES catalog_services(organization_id, location_id, id) ON DELETE CASCADE,
 FOREIGN KEY (organization_id, location_id, main_id) REFERENCES catalog_services(organization_id, location_id, id) ON DELETE CASCADE,
 CHECK (addon_id <> main_id)
);
CREATE INDEX catalog_addon_parents_main_idx ON catalog_addon_parents(main_id);

-- Duplicates folded into another add-on are archived and marked, so they stay out of the lists but old records still resolve.
ALTER TABLE central_catalog_services ADD COLUMN merged_into uuid;
ALTER TABLE catalog_services ADD COLUMN merged_into uuid;

-- Is this add-on offered with this main service? Used by every check that an add-on belongs to the chosen service.
CREATE FUNCTION catalog_addon_offered_with(addon uuid, main uuid) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM catalog_services WHERE id = addon AND kind = 'addon' AND parent_id = main)
     OR EXISTS(SELECT 1 FROM catalog_addon_parents WHERE addon_id = addon AND main_id = main)
$$;
CREATE FUNCTION central_addon_offered_with(addon uuid, main uuid) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM central_catalog_services WHERE id = addon AND kind = 'addon' AND parent_id = main)
     OR EXISTS(SELECT 1 FROM central_addon_parents WHERE addon_id = addon AND main_id = main)
$$;

-- An add-on's primary main service is always one of its links.
CREATE FUNCTION central_addon_primary_link() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO central_addon_parents(organization_id, addon_id, main_id) VALUES (NEW.organization_id, NEW.id, NEW.parent_id) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
CREATE TRIGGER central_addon_primary_link AFTER INSERT OR UPDATE OF parent_id ON central_catalog_services
 FOR EACH ROW WHEN (NEW.kind = 'addon' AND NEW.parent_id IS NOT NULL) EXECUTE FUNCTION central_addon_primary_link();

CREATE FUNCTION catalog_addon_primary_link() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO catalog_addon_parents(organization_id, location_id, addon_id, main_id) VALUES (NEW.organization_id, NEW.location_id, NEW.id, NEW.parent_id) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
CREATE TRIGGER catalog_addon_primary_link AFTER INSERT OR UPDATE OF parent_id ON catalog_services
 FOR EACH ROW WHEN (NEW.kind = 'addon' AND NEW.parent_id IS NOT NULL) EXECUTE FUNCTION catalog_addon_primary_link();

-- The franchise copies follow the central links.
CREATE FUNCTION central_addon_links_to_locations() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP = 'INSERT' THEN
  INSERT INTO catalog_addon_parents(organization_id, location_id, addon_id, main_id)
  SELECT a.organization_id, a.location_id, a.id, m.id
  FROM catalog_services a JOIN catalog_services m ON m.organization_id = a.organization_id AND m.location_id = a.location_id AND m.central_service_id = NEW.main_id
  WHERE a.central_service_id = NEW.addon_id AND a.organization_id = NEW.organization_id
  ON CONFLICT DO NOTHING;
  RETURN NEW;
 END IF;
 DELETE FROM catalog_addon_parents l USING catalog_services a, catalog_services m
 WHERE l.addon_id = a.id AND l.main_id = m.id AND a.central_service_id = OLD.addon_id AND m.central_service_id = OLD.main_id;
 RETURN OLD;
END $$;
CREATE TRIGGER central_addon_links_to_locations AFTER INSERT OR DELETE ON central_addon_parents
 FOR EACH ROW EXECUTE FUNCTION central_addon_links_to_locations();

-- A franchise copy made later (or a main service switched on later) picks up the links that already exist centrally.
CREATE FUNCTION new_local_copy_links() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.kind = 'addon' THEN
  INSERT INTO catalog_addon_parents(organization_id, location_id, addon_id, main_id)
  SELECT NEW.organization_id, NEW.location_id, NEW.id, m.id
  FROM central_addon_parents c JOIN catalog_services m ON m.organization_id = NEW.organization_id AND m.location_id = NEW.location_id AND m.central_service_id = c.main_id
  WHERE c.addon_id = NEW.central_service_id ON CONFLICT DO NOTHING;
 ELSE
  INSERT INTO catalog_addon_parents(organization_id, location_id, addon_id, main_id)
  SELECT NEW.organization_id, NEW.location_id, a.id, NEW.id
  FROM central_addon_parents c JOIN catalog_services a ON a.organization_id = NEW.organization_id AND a.location_id = NEW.location_id AND a.central_service_id = c.addon_id
  WHERE c.main_id = NEW.central_service_id ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER new_local_copy_links AFTER INSERT ON catalog_services
 FOR EACH ROW WHEN (NEW.central_service_id IS NOT NULL) EXECUTE FUNCTION new_local_copy_links();

-- When a central add-on's primary main service changes, its franchise copies move to the matching main service there.
CREATE FUNCTION central_primary_to_locations() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE catalog_services l SET parent_id = m.id
 FROM catalog_services m
 WHERE l.central_service_id = NEW.id AND m.organization_id = l.organization_id AND m.location_id = l.location_id AND m.central_service_id = NEW.parent_id AND NOT m.archived;
 RETURN NEW;
END $$;
CREATE TRIGGER central_primary_to_locations AFTER UPDATE OF parent_id ON central_catalog_services
 FOR EACH ROW WHEN (OLD.parent_id IS DISTINCT FROM NEW.parent_id AND NEW.kind = 'addon') EXECUTE FUNCTION central_primary_to_locations();

-- ---- Existing data: links from today's parents, then fold the per-main duplicates into one add-on each. ----

INSERT INTO central_addon_parents(organization_id, addon_id, main_id)
SELECT organization_id, id, parent_id FROM central_catalog_services WHERE kind = 'addon' AND parent_id IS NOT NULL ON CONFLICT DO NOTHING;
INSERT INTO catalog_addon_parents(organization_id, location_id, addon_id, main_id)
SELECT organization_id, location_id, id, parent_id FROM catalog_services WHERE kind = 'addon' AND parent_id IS NOT NULL ON CONFLICT DO NOTHING;

CREATE TEMP TABLE central_merge AS
SELECT id AS dup, canon FROM (
 SELECT id, first_value(id) OVER (PARTITION BY organization_id, lower(btrim(name)) ORDER BY created_at, id) AS canon
 FROM central_catalog_services WHERE kind = 'addon' AND NOT archived
) x WHERE id <> canon;

CREATE TEMP TABLE local_merge AS
SELECT d.id AS dup, c.id AS canon, d.organization_id, d.location_id
FROM central_merge cm
JOIN catalog_services d ON d.central_service_id = cm.dup AND NOT d.archived
JOIN catalog_services c ON c.central_service_id = cm.canon AND c.location_id = d.location_id AND NOT c.archived;

-- the surviving add-on is offered with every main service any duplicate was offered with
INSERT INTO central_addon_parents(organization_id, addon_id, main_id)
SELECT cc.organization_id, cm.canon, l.main_id
FROM central_merge cm JOIN central_addon_parents l ON l.addon_id = cm.dup JOIN central_catalog_services cc ON cc.id = cm.canon
ON CONFLICT DO NOTHING;
INSERT INTO catalog_addon_parents(organization_id, location_id, addon_id, main_id)
SELECT lm.organization_id, lm.location_id, lm.canon, l.main_id
FROM local_merge lm JOIN catalog_addon_parents l ON l.addon_id = lm.dup
ON CONFLICT DO NOTHING;

-- records that point at a duplicate now point at the surviving add-on
UPDATE bookings b SET addons = (
 SELECT COALESCE(jsonb_agg(CASE WHEN m.canon IS NULL THEN x.e ELSE jsonb_set(x.e, '{serviceId}', to_jsonb(m.canon::text)) END ORDER BY x.ord), '[]'::jsonb)
 FROM jsonb_array_elements(b.addons) WITH ORDINALITY x(e, ord) LEFT JOIN local_merge m ON m.dup::text = x.e->>'serviceId')
WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(b.addons) e JOIN local_merge m ON m.dup::text = e->>'serviceId');
UPDATE jobs j SET addons = (
 SELECT COALESCE(jsonb_agg(CASE WHEN m.canon IS NULL THEN x.e ELSE jsonb_set(x.e, '{serviceId}', to_jsonb(m.canon::text)) END ORDER BY x.ord), '[]'::jsonb)
 FROM jsonb_array_elements(j.addons) WITH ORDINALITY x(e, ord) LEFT JOIN local_merge m ON m.dup::text = x.e->>'serviceId')
WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(j.addons) e JOIN local_merge m ON m.dup::text = e->>'serviceId');
UPDATE service_scheduling_rules r SET addon_ids = (
 SELECT COALESCE(jsonb_agg(DISTINCT COALESCE(m.canon::text, x.e) ORDER BY COALESCE(m.canon::text, x.e)), '[]'::jsonb)
 FROM jsonb_array_elements_text(r.addon_ids) x(e) LEFT JOIN local_merge m ON m.dup::text = x.e)
WHERE EXISTS (SELECT 1 FROM jsonb_array_elements_text(r.addon_ids) e JOIN local_merge m ON m.dup::text = e);

UPDATE catalog_services s SET archived = true, merged_into = lm.canon FROM local_merge lm WHERE s.id = lm.dup;
UPDATE central_catalog_services s SET archived = true, merged_into = cm.canon FROM central_merge cm WHERE s.id = cm.dup;

DROP TABLE local_merge;
DROP TABLE central_merge;
