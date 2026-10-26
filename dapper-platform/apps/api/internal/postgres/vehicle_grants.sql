ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check
 CHECK (action IN ('customers.read','customers.create','vehicles.read','vehicles.create'));
