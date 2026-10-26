-- Credentials grant no organization/location access by themselves.
CREATE TABLE password_credentials (
 principal_id uuid PRIMARY KEY REFERENCES principals(id),
 email text NOT NULL UNIQUE CHECK (email=lower(btrim(email)) AND length(email)<=254),
 password_hash text NOT NULL CHECK (password_hash LIKE '$argon2id$%'),
 changed_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION invalidate_password_sessions() RETURNS trigger LANGUAGE plpgsql
SET search_path FROM CURRENT AS $$
BEGIN
 UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE principal_id=OLD.principal_id;
 RETURN NULL;
END;
$$;
CREATE TRIGGER password_sessions_changed AFTER UPDATE OR DELETE ON password_credentials
FOR EACH ROW EXECUTE FUNCTION invalidate_password_sessions();
