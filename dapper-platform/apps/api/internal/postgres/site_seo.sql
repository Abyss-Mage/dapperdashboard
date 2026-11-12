-- Search settings for the public website (owner 2026-11-13): site-wide title, description, keywords, share image and search-console
-- verification, plus the same per page and a hide-from-search switch. Edited by people with access.manage.
ALTER TABLE public_sites ADD COLUMN seo jsonb NOT NULL DEFAULT '{}'::jsonb;
