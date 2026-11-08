-- The media library's Bin: what the owner removes waits here until it is restored or deleted forever.
ALTER TABLE portfolio_images ADD COLUMN binned_at timestamptz;
ALTER TABLE website_dealers ADD COLUMN binned_at timestamptz;
ALTER TABLE site_asset_hidden ADD COLUMN removed_at timestamptz NOT NULL DEFAULT now();
-- Deleted forever from the Bin. A built-in file is part of the website's own code, so it stays hidden rather than being erased.
ALTER TABLE site_asset_hidden ADD COLUMN purged boolean NOT NULL DEFAULT false;
