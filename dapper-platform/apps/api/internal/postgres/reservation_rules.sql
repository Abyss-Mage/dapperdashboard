ALTER TABLE service_scheduling_rules ADD CONSTRAINT scheduling_rule_resource_identity UNIQUE(organization_id,location_id,resource_id,id);
ALTER TABLE capacity_reservations ADD COLUMN rule_id uuid;
ALTER TABLE capacity_reservations ADD COLUMN rule_version integer;
ALTER TABLE capacity_reservations ADD COLUMN rule_snapshot jsonb;
ALTER TABLE capacity_reservations ADD CONSTRAINT reservation_rule_resource_scope
 FOREIGN KEY(organization_id,location_id,resource_id,rule_id) REFERENCES service_scheduling_rules(organization_id,location_id,resource_id,id);
ALTER TABLE capacity_reservations ADD CONSTRAINT reservation_rule_revision
 FOREIGN KEY(rule_id,rule_version) REFERENCES service_scheduling_revisions(rule_id,version);
ALTER TABLE capacity_reservations ADD CONSTRAINT reservation_rule_snapshot CHECK(
 (rule_id IS NULL AND rule_version IS NULL AND rule_snapshot IS NULL) OR
 (rule_id IS NOT NULL AND rule_version IS NOT NULL AND rule_snapshot IS NOT NULL AND jsonb_typeof(rule_snapshot)='object'));
