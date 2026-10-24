# Migration plan
SOW 13 requires agreed historical/future BookingPress booking migration and reconciliation. The read-only catalogue reference slice is not booking migration or cutover acceptance.

Verified live export after user-installed exporter update: 25 published pages, 60 eligible public media metadata records, 79 services and 12 categories; one page per resource. The plugin excludes customers, bookings, payments, users, credentials, options and private content. No live records/settings/code were changed by the agent. Raw captures stay ignored in .cache/wordpress-export. Only sanitized counts/hashes and allowlisted public catalogue derivatives enter source/Obsidian.

scripts/build_bookingpress_reference.py verifies capture hashes against wordpress-export-evidence.json, complete page counts, unique original IDs and explicit field sets. It retains decimal strings and duration values/units. Three one-minute durations are flagged. No silent category reassignment, currency inference, duration repair, business approval or transactional write occurs.

Required booking migration remains blocked on an explicitly scoped authorized source export, timezone/status/ownership/payment mappings and retention decisions. Preserve source system and original IDs, batch hashes, source timestamps and unmapped values. Never invent missing customers, historical payments or status transitions.

Implement repeatable dry runs, per-row errors, deterministic mapping, idempotent batches, count reconciliation and sampled historical/future bookings. Validate corresponding customer/service/date/status/payment references and future scheduling behavior. Assess employee/attendance/dealership/invoice history separately; it is not automatically in migration scope.

Final migration, backups, rollback/cutover window and legacy retirement require client approval and documented reconciliation. Existing systems remain untouched. Exporter manifest version text currently lags the plugin header; successful live endpoint behavior is verified, but the metadata issue remains to fix in a future reviewed package.
