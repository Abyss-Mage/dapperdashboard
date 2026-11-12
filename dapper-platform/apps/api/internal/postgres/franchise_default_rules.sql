-- What makes a service bookable lives in its booking rule (how long it takes, on which bay), so a new franchise copies them from the largest
-- franchise, matching services by their central catalogue entry and bays by name.
CREATE FUNCTION add_default_rules_to_location(org uuid, loc uuid, actor uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE source uuid;
BEGIN
 IF EXISTS(SELECT 1 FROM service_scheduling_rules WHERE organization_id=org AND location_id=loc) THEN RETURN; END IF;
 SELECT location_id INTO source FROM service_scheduling_rules WHERE organization_id=org AND location_id<>loc GROUP BY location_id ORDER BY count(*) DESC,location_id LIMIT 1;
 IF source IS NULL THEN RETURN; END IF;
 WITH added AS (
  INSERT INTO service_scheduling_rules(organization_id,location_id,resource_id,service_id,addon_ids,duration_minutes,before_minutes,after_minutes,units,working_time,slots)
  SELECT org,loc,target_bay.id,target_service.id,'[]'::jsonb,rule.duration_minutes,rule.before_minutes,rule.after_minutes,rule.units,rule.working_time,rule.slots
  FROM service_scheduling_rules rule
  JOIN scheduling_resources bay ON bay.id=rule.resource_id
  JOIN scheduling_resources target_bay ON target_bay.organization_id=org AND target_bay.location_id=loc AND target_bay.name=bay.name
  JOIN catalog_services service ON service.id=rule.service_id AND service.central_service_id IS NOT NULL
  JOIN catalog_services target_service ON target_service.organization_id=org AND target_service.location_id=loc AND target_service.central_service_id=service.central_service_id AND NOT target_service.archived
  WHERE rule.organization_id=org AND rule.location_id=source AND rule.addon_ids='[]'::jsonb
  ON CONFLICT DO NOTHING RETURNING *)
 INSERT INTO service_scheduling_revisions(rule_id,version,snapshot,actor_id) SELECT a.id,a.version,to_jsonb(a),actor FROM added a;
END $$;
SELECT add_default_rules_to_location(l.organization_id,l.id,a.actor_id)
FROM locations l CROSS JOIN LATERAL (SELECT r.actor_id FROM central_catalog_service_revisions r JOIN central_catalog_services c ON c.id=r.service_id WHERE c.organization_id=l.organization_id ORDER BY r.created_at DESC LIMIT 1) a;
