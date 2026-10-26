-- A price on a catalogue service: whole cents in CAD, entered WITHOUT tax (tax is added from the franchise's tax settings).
-- Central services carry the price; linked franchise services receive it with every central revision, like the name.
-- NULL means no price has been set.
ALTER TABLE central_catalog_services ADD COLUMN price_cents bigint CHECK(price_cents IS NULL OR price_cents BETWEEN 0 AND 100000000000);
ALTER TABLE catalog_services ADD COLUMN price_cents bigint CHECK(price_cents IS NULL OR price_cents BETWEEN 0 AND 100000000000);
CREATE OR REPLACE FUNCTION sync_central_service_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE central central_catalog_services; local catalog_services;
BEGIN
 SELECT * INTO STRICT central FROM central_catalog_services WHERE id=NEW.service_id;
 FOR local IN SELECT * FROM catalog_services WHERE central_service_id=central.id ORDER BY location_id,id FOR UPDATE LOOP
  IF local.kind<>central.kind OR
   (central.kind='addon' AND NOT EXISTS(SELECT 1 FROM catalog_services parent WHERE parent.id=local.parent_id AND parent.central_service_id=central.parent_id)) THEN
   RAISE EXCEPTION 'Linked service identity cannot change' USING ERRCODE='23514',CONSTRAINT='central_link_identity';
  END IF;
  IF central.archived AND EXISTS(SELECT 1 FROM catalog_services child WHERE child.parent_id=local.id AND NOT child.archived) THEN
   RAISE EXCEPTION 'Linked service has active location add-ons' USING ERRCODE='23514',CONSTRAINT='central_link_children';
  END IF;
  UPDATE catalog_services SET name=central.name,price_cents=central.price_cents,archived=archived OR central.archived,version=version+1 WHERE id=local.id RETURNING * INTO local;
  INSERT INTO catalog_service_revisions(service_id,version,actor_id,snapshot) VALUES(local.id,local.version,NEW.actor_id,
   jsonb_build_object('id',local.id,'name',local.name,'kind',local.kind,'parentId',local.parent_id,'archived',local.archived,'version',local.version,'centralServiceId',central.id,'priceCents',local.price_cents));
  INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES(local.organization_id,local.location_id,NEW.actor_id,'service.central_revision_applied',local.id);
 END LOOP;
 RETURN NEW;
END $$;
-- Linked services already enabled at franchises pick the price up with the next central revision.
