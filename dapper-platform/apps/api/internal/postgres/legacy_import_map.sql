-- Provenance for records imported from the legacy BookingPress site: which source record became which Dapper record, in which
-- batch. A re-run finds the mapping and skips what is already imported, and a batch can be reviewed or removed by its id.
CREATE TABLE legacy_import_map (
 source_system text NOT NULL,
 source_kind text NOT NULL,
 source_id text NOT NULL,
 organization_id uuid NOT NULL REFERENCES organizations(id),
 target_id uuid NOT NULL,
 batch text NOT NULL,
 note text NOT NULL DEFAULT '',
 imported_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (source_system, source_kind, source_id, organization_id)
);
CREATE INDEX legacy_import_map_batch_idx ON legacy_import_map(batch, source_kind);
