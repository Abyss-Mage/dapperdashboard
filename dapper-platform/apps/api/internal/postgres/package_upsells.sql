-- Upgrade offers in the booking flow: when a customer picks one package (by its name without the vehicle, e.g. "VIP Interior Detail"),
-- the flow offers another ("Luxury Full Body Detail") for the same vehicle, with the owner's pitch and what the upgrade adds.
CREATE TABLE package_upsells (
 organization_id uuid NOT NULL REFERENCES organizations(id),
 from_pack text NOT NULL CHECK(length(btrim(from_pack)) BETWEEN 2 AND 120),
 to_pack text NOT NULL CHECK(length(btrim(to_pack)) BETWEEN 2 AND 120 AND lower(btrim(to_pack))<>lower(btrim(from_pack))),
 pitch text NOT NULL DEFAULT '' CHECK(length(pitch) <= 200),
 perks jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(perks)='array' AND jsonb_array_length(perks) <= 6),
 active boolean NOT NULL DEFAULT true,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id, from_pack)
);
CREATE UNIQUE INDEX package_upsells_one_per_pack ON package_upsells(organization_id, lower(btrim(from_pack)));
