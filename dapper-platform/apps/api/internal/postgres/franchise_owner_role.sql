-- error404 #89 (2026-11-14): the Franchise Owner role (owner request 2026-10-30) for every organization that does not have it; a fresh
-- install had only Super Admin and Employee. Same capabilities as FranchiseOwnerRoleCapabilities (franchise_owner_role.go).
WITH made AS (
 INSERT INTO roles(organization_id,name,description,version)
 SELECT o.id,'Franchise Owner','Runs the franchises it is assigned to: bookings, customers, work, staff time, invoices, quotes and marketing. No organization-wide settings, people/roles management, central catalogue or documents.',1
 FROM organizations o WHERE NOT EXISTS(SELECT 1 FROM roles r WHERE r.organization_id=o.id AND r.name='Franchise Owner')
 RETURNING id)
INSERT INTO role_capabilities(role_id,action)
SELECT made.id, c FROM made CROSS JOIN unnest(ARRAY['customers.read','customers.create','customer_accounts.manage','vehicles.read','vehicles.create','jobs.read','jobs.create','jobs.complete','jobs.contribute','catalogue.read','catalogue.manage','scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read','attendance.record','attendance.read','attendance.manage','bookings.read','bookings.create','bookings.manage','invoices.read','invoices.create','invoices.issue','invoices.manage','payments.record','payments.read','sop.read','sop.acknowledge','quotes.read','quotes.create','quotes.manage','quotes.issue','dealership.read','dealership.manage','marketing.read','marketing.manage']) AS c;
