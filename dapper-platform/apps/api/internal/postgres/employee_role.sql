-- The built-in Employee role (workspace staff) for every business that does not have one yet. Setup creates it for new businesses.
INSERT INTO roles(organization_id, name, description, version)
SELECT o.id, 'Employee', 'Workspace staff. Signs in to the employee workspace only; the dashboard sends them there. Owners can adjust what the role may do under Team > Roles.', 1
FROM organizations o
WHERE NOT EXISTS (SELECT 1 FROM roles r WHERE r.organization_id = o.id AND r.name = 'Employee');

INSERT INTO role_capabilities(role_id, action)
SELECT r.id, c.action
FROM roles r
CROSS JOIN unnest(ARRAY[
 'attendance.record','attendance.read',
 'bookings.read','bookings.create','bookings.manage',
 'customers.read','customers.create','vehicles.read','vehicles.create',
 'invoices.read','invoices.create','invoices.issue','payments.record','payments.read',
 'scheduling.read','scheduling.reserve','catalogue.read',
 'jobs.read','jobs.contribute',
 'sop.read','sop.acknowledge']) AS c(action)
WHERE r.name = 'Employee' AND r.description LIKE 'Workspace staff.%'
  AND NOT EXISTS (SELECT 1 FROM role_capabilities x WHERE x.role_id = r.id);
