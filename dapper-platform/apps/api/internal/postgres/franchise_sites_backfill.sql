-- error404 #43 (2026-11-12): franchises added after setup had no website entry, so the booking page never listed them. Like the first
-- franchise, each franchise gets a published entry named after it; its slug is its code.
INSERT INTO public_sites(organization_id, location_id, slug, name, published)
SELECT l.organization_id, l.id, regexp_replace(lower(l.code), '[^a-z0-9-]', '-', 'g'), l.name, true
FROM locations l
WHERE NOT EXISTS (SELECT 1 FROM public_sites ps WHERE ps.organization_id=l.organization_id AND ps.location_id=l.id)
  AND regexp_replace(lower(coalesce(l.code,'')), '[^a-z0-9-]', '-', 'g') ~ '^[a-z0-9][a-z0-9-]{1,59}$'
  AND length(btrim(l.name)) BETWEEN 2 AND 120
ON CONFLICT DO NOTHING;

-- error404 #41 (2026-11-12): a team member's phone is optional (empty, or an international number as before).
ALTER TABLE member_profiles DROP CONSTRAINT member_profiles_phone_check;
ALTER TABLE member_profiles ADD CONSTRAINT member_profiles_phone_check CHECK (phone = '' OR phone ~ '^\+[1-9][0-9]{6,14}$');

-- error404 #44 (2026-11-12): when the dealership first opened an invoice in its portal (shown as "Viewed"; owner views do not count).
ALTER TABLE invoices ADD COLUMN viewed_at timestamptz;
