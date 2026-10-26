ALTER TABLE location_grants DROP CONSTRAINT location_grants_action_check;
ALTER TABLE location_grants ADD CONSTRAINT location_grants_action_check CHECK(action IN (
  'customers.read','customers.create','vehicles.read','vehicles.create',
  'jobs.read','jobs.create','jobs.complete','jobs.contribute',
  'catalogue.read','catalogue.manage',
  'scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
  'attendance.record','attendance.read','attendance.manage',
  'bookings.read','bookings.create','bookings.manage',
  'invoices.read','invoices.create','invoices.manage','payments.record','payments.read',
  'sop.read','sop.acknowledge','sop.manage'));

ALTER TABLE organization_grants DROP CONSTRAINT organization_grants_action_check;
ALTER TABLE organization_grants ADD CONSTRAINT organization_grants_action_check CHECK(action IN (
  'structure.read','structure.manage','catalogue.read','catalogue.manage',
  'attendance.read','attendance.manage',
  'bookings.read','bookings.manage',
  'invoices.read','invoices.manage','payments.read',
  'sop.read','sop.manage'));

CREATE TABLE sop_documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id),
  title text NOT NULL CHECK(length(btrim(title)) BETWEEN 3 AND 150),
  category text NOT NULL CHECK(category IN ('safety', 'chemical', 'procedure', 'onboarding')),
  file_name text NOT NULL CHECK(length(btrim(file_name)) BETWEEN 3 AND 150),
  sha256_hash text NOT NULL CHECK(length(sha256_hash) = 64),
  version integer NOT NULL DEFAULT 1 CHECK(version > 0),
  page_count integer NOT NULL CHECK(page_count > 0),
  effective_date date NOT NULL DEFAULT CURRENT_DATE,
  archived boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(organization_id, file_name, version)
);

CREATE INDEX sop_documents_org_idx ON sop_documents(organization_id, archived, created_at DESC);

CREATE TABLE sop_acknowledgments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL,
  location_id uuid NOT NULL,
  sop_document_id uuid NOT NULL REFERENCES sop_documents(id),
  employee_id uuid NOT NULL REFERENCES principals(id),
  actor_id uuid NOT NULL REFERENCES principals(id),
  acknowledged_sha256 text NOT NULL CHECK(length(acknowledged_sha256) = 64),
  training_track text NOT NULL CHECK(training_track IN ('orientation', 'assisted', 'independent', 'certified')),
  notes text NOT NULL DEFAULT '',
  acknowledged_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id),
  UNIQUE(organization_id, location_id, sop_document_id, employee_id, training_track)
);

CREATE INDEX sop_acknowledgments_emp_idx ON sop_acknowledgments(organization_id, location_id, employee_id);
CREATE INDEX sop_acknowledgments_doc_idx ON sop_acknowledgments(sop_document_id);
