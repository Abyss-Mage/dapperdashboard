-- Explicit action grants, with no assumed business role or inherited visibility.
-- This is a technical boundary; approved franchise roles remain a client decision.
CREATE TABLE principals (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 external_subject text NOT NULL UNIQUE,
 active boolean NOT NULL DEFAULT true
);
CREATE TABLE location_grants (
 principal_id uuid NOT NULL REFERENCES principals(id),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 action text NOT NULL CHECK (action IN ('customers.read','customers.create')),
 PRIMARY KEY (principal_id,organization_id,location_id,action),
 FOREIGN KEY (organization_id,location_id) REFERENCES locations(organization_id,id)
);
