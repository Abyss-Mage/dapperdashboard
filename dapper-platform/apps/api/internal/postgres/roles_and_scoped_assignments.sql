ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage',
 'attendance.read','attendance.manage',
 'bookings.read','bookings.manage',
 'invoices.read','invoices.manage','invoices.configure','payments.read',
 'sop.read','sop.manage','access.manage',
 'quotes.read','quotes.manage',
 'dealership.read','dealership.manage',
 'marketing.read','marketing.manage',
 'roles.manage','roles.assign'
));

CREATE TABLE roles (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 80),
 description text NOT NULL DEFAULT '' CHECK(length(description) <= 500),
 version int NOT NULL DEFAULT 1 CHECK(version >= 1),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(organization_id, name)
);

CREATE INDEX roles_organization_idx ON roles(organization_id, name);

CREATE TABLE role_capabilities (
 role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
 action text NOT NULL CHECK(action IN (
   'customers.read', 'customers.create', 'customer_accounts.manage', 'vehicles.read', 'vehicles.create',
   'jobs.read', 'jobs.create', 'jobs.complete', 'jobs.contribute',
   'catalogue.read', 'catalogue.manage', 'content.manage',
   'scheduling.manage', 'scheduling.reserve', 'scheduling.release', 'scheduling.read',
   'attendance.record', 'attendance.read', 'attendance.manage',
   'bookings.read', 'bookings.create', 'bookings.manage',
   'invoices.read', 'invoices.create', 'invoices.issue', 'invoices.manage',
   'payments.record', 'payments.read',
   'sop.read', 'sop.acknowledge', 'sop.manage',
   'quotes.read', 'quotes.create', 'quotes.manage', 'quotes.issue',
   'dealership.read', 'dealership.manage',
   'marketing.read', 'marketing.manage',
   'structure.read', 'structure.manage',
   'invoices.configure',
   'access.manage', 'roles.manage', 'roles.assign'
 )),
 PRIMARY KEY(role_id, action)
);

CREATE TABLE member_role_assignments (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
 principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE RESTRICT,
 role_id uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
 scope_type text NOT NULL CHECK(scope_type IN ('location', 'franchise', 'organization')),
 scope_id uuid NOT NULL,
 location_id uuid,
 franchise_id uuid,
 version int NOT NULL DEFAULT 1 CHECK(version >= 1),
 assigned_by uuid NOT NULL REFERENCES principals(id) ON DELETE RESTRICT,
 assigned_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(organization_id, principal_id) REFERENCES organization_memberships(organization_id, principal_id) ON DELETE RESTRICT,
 FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id) ON DELETE RESTRICT,
 FOREIGN KEY(organization_id, franchise_id) REFERENCES franchises(organization_id, id) ON DELETE RESTRICT,
 CONSTRAINT member_role_scope_consistency CHECK (
   (scope_type = 'location' AND location_id IS NOT NULL AND franchise_id IS NULL AND scope_id = location_id) OR
   (scope_type = 'franchise' AND franchise_id IS NOT NULL AND location_id IS NULL AND scope_id = franchise_id) OR
   (scope_type = 'organization' AND location_id IS NULL AND franchise_id IS NULL AND scope_id = organization_id)
 ),
 UNIQUE(organization_id, principal_id, role_id, scope_type, scope_id)
);

CREATE INDEX member_role_assignments_scope_idx ON member_role_assignments(organization_id, principal_id, scope_type, scope_id);

CREATE TABLE role_definition_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL REFERENCES organizations(id),
 actor_id uuid NOT NULL REFERENCES principals(id),
 role_id uuid NOT NULL,
 role_name text NOT NULL,
 action text NOT NULL CHECK(action IN ('created', 'updated', 'deleted')),
 version int NOT NULL,
 capabilities text[] NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE role_assignment_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL REFERENCES organizations(id),
 actor_id uuid NOT NULL REFERENCES principals(id),
 principal_id uuid NOT NULL REFERENCES principals(id),
 role_id uuid NOT NULL,
 scope_type text NOT NULL,
 scope_id uuid NOT NULL,
 action text NOT NULL CHECK(action IN ('assigned', 'revoked')),
 version int NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
