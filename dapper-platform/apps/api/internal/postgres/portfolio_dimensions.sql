-- Photo sizes, so the website can hold each frame's shape before the picture loads. Older uploads stay 0 (unknown).
ALTER TABLE content_media ADD COLUMN width integer NOT NULL DEFAULT 0 CHECK (width >= 0);
ALTER TABLE content_media ADD COLUMN height integer NOT NULL DEFAULT 0 CHECK (height >= 0);
