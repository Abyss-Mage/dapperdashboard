-- Pictures and films the owner uploaded to replace (or add to) the website's own files. A row whose path matches a built-in file under
-- /site/assets/ is served instead of that file; paths under films/ are extra films shown on Home and Portfolio. Deleting a row brings the original back.
CREATE TABLE site_assets (
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 path text NOT NULL CHECK(path ~ '^[A-Za-z0-9][A-Za-z0-9/_.-]{1,200}$' AND position('..' in path)=0),
 content_type text NOT NULL CHECK(content_type IN ('image/jpeg','image/png','image/webp','video/mp4','video/webm')),
 data bytea NOT NULL CHECK(length(data) BETWEEN 1 AND 26214400),
 updated_by uuid, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id, location_id, path)
);
