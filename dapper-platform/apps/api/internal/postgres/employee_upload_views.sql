-- Audit of supervisor/administrator access to employees' photo posts. Append-only. Post/photo identifiers are
-- deliberately not foreign keys so a future approved retention process can delete photos without erasing the audit.
CREATE TABLE employee_upload_views (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 post_id uuid NOT NULL,
 photo_id uuid NOT NULL,
 owner_id uuid NOT NULL REFERENCES principals(id),
 viewer_id uuid NOT NULL REFERENCES principals(id),
 viewed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE INDEX employee_upload_views_scope_idx ON employee_upload_views(organization_id,location_id,viewed_at DESC);
CREATE FUNCTION protect_employee_upload_views() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'employee upload view history is append-only'; END $$;
CREATE TRIGGER employee_upload_views_immutable BEFORE UPDATE OR DELETE ON employee_upload_views FOR EACH ROW EXECUTE FUNCTION protect_employee_upload_views();
