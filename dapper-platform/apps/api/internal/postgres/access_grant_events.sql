CREATE TABLE access_grant_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL,
 enabled boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE INDEX access_grant_events_scope_idx ON access_grant_events(organization_id,location_id,created_at,id);
