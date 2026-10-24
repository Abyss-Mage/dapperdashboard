-- Work descriptions preserve creation-time catalogue names, without inventing
-- approved pricing, quantities, durations or tax rules.
ALTER TABLE jobs ADD COLUMN addons jsonb NOT NULL DEFAULT '[]'::jsonb
 CHECK (jsonb_typeof(addons) = 'array' AND jsonb_array_length(addons) <= 100);
