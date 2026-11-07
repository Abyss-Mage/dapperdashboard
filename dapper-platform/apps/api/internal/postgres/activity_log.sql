-- Who signed in or tried to, and every change a signed-in person made: when, from where, on what device. Query strings and
-- request bodies are never stored, so no password, token or customer detail lands here.
CREATE TABLE activity_log (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 at timestamptz NOT NULL DEFAULT now(),
 principal_id uuid REFERENCES principals(id),
 email text NOT NULL DEFAULT '' CHECK (length(email) <= 254),
 kind text NOT NULL CHECK (kind IN ('sign_in','sign_in_failed','write')),
 method text NOT NULL DEFAULT '' CHECK (length(method) <= 10),
 path text NOT NULL DEFAULT '' CHECK (length(path) <= 300),
 status integer NOT NULL DEFAULT 0,
 ip text NOT NULL DEFAULT '' CHECK (length(ip) <= 64),
 user_agent text NOT NULL DEFAULT '' CHECK (length(user_agent) <= 300)
);
CREATE INDEX activity_log_at_idx ON activity_log(at DESC);
CREATE INDEX activity_log_principal_idx ON activity_log(principal_id, at DESC);
