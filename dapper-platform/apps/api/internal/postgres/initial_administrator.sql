ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
 'structure.read','structure.manage','catalogue.read','catalogue.manage',
 'attendance.read','attendance.manage','bookings.read','bookings.manage',
 'invoices.read','invoices.manage','payments.read','sop.read','sop.manage','access.manage'));

CREATE TABLE organization_memberships (
 organization_id uuid NOT NULL REFERENCES organizations(id),
 principal_id uuid NOT NULL REFERENCES principals(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,principal_id)
);
