-- error404 #91 (2026-11-14): accepted, revoked or expired invitations can be taken off the Team list; the row stays as history.
ALTER TABLE member_invitations ADD COLUMN hidden_at timestamptz;
