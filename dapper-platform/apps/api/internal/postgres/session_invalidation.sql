-- Account deactivation revokes existing sessions in the same transaction.
-- Re-enabling a principal cannot revive bearer tokens issued before disable.
CREATE FUNCTION revoke_disabled_principal_sessions() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path FROM CURRENT AS $$
BEGIN
 IF OLD.active AND NOT NEW.active THEN
  UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now())
  WHERE principal_id=NEW.id;
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER revoke_sessions_on_principal_disable
AFTER UPDATE OF active ON principals FOR EACH ROW
EXECUTE FUNCTION revoke_disabled_principal_sessions();
