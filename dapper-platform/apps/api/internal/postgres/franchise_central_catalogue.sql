-- A central service reaches a franchise when the service is saved, so a franchise opened afterwards needs the whole catalogue copied in.
CREATE FUNCTION add_central_catalogue_to_location(org uuid, loc uuid, actor uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE central central_catalog_services; local catalog_services; parent_local uuid;
BEGIN
 FOR central IN SELECT * FROM central_catalog_services WHERE organization_id=org AND NOT archived ORDER BY (kind='addon'),id LOOP
  CONTINUE WHEN EXISTS(SELECT 1 FROM catalog_services c WHERE c.organization_id=org AND c.location_id=loc AND c.central_service_id=central.id);
  parent_local := NULL;
  IF central.kind='addon' THEN
   SELECT id INTO parent_local FROM catalog_services WHERE organization_id=org AND location_id=loc AND central_service_id=central.parent_id AND NOT archived;
   CONTINUE WHEN parent_local IS NULL;
  END IF;
  INSERT INTO catalog_services(organization_id,location_id,name,kind,parent_id,central_service_id,price_cents,duration_minutes)
   VALUES(org,loc,central.name,central.kind,parent_local,central.id,central.price_cents,central.duration_minutes) RETURNING * INTO local;
  INSERT INTO catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES(local.id,local.version,actor,
   jsonb_build_object('id',local.id,'name',local.name,'kind',local.kind,'parentId',local.parent_id,'archived',local.archived,'version',local.version,'centralServiceId',central.id,'priceCents',local.price_cents,'durationMinutes',local.duration_minutes));
  INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES(org,loc,actor,'service.central_service_added',local.id);
 END LOOP;
END $$;
-- Franchises already open catch up now, recorded against whoever last changed the central catalogue.
SELECT add_central_catalogue_to_location(l.organization_id,l.id,a.actor_id)
FROM locations l CROSS JOIN LATERAL (SELECT r.actor_id FROM central_catalog_service_revisions r JOIN central_catalog_services c ON c.id=r.service_id WHERE c.organization_id=l.organization_id ORDER BY r.created_at DESC LIMIT 1) a;
