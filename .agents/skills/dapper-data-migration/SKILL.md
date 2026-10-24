---
name: dapper-data-migration
description: Assess and map Dapper BookingPress exports with provenance, repeatability and reconciliation safeguards.
---

# Dapper data migration
Read canonical MIGRATION_PLAN.md and wordpress-export-evidence.json. Current authorized exporter scope is published content/media and allowlisted service/category fields; customers, bookings, payments, users, credentials and private content are excluded.

Raw captured JSON stays under ignored .cache/wordpress-export. Do not copy whole page bodies or testimonial/customer identities into source or Obsidian. scripts/build_bookingpress_reference.py verifies hashes/complete counts/explicit fields and produces only the public catalogue derivative.

Preserve original IDs, decimal strings and raw duration units. Surface unmapped categories and suspicious legacy values rather than approving, reassigning or normalizing them silently. The current 79-service/12-category derivative is not a transactional catalogue or completed booking migration.

For an agreed booking import, require scoped source data, mappings, repeatable dry runs, row errors, deterministic source IDs, batch hashes, count reconciliation and representative historical/future workflow checks. Never invent missing payment/history records.

Leave legacy systems untouched. Final migration/cutover/retirement follows the user's authorization and approved reconciliation plan; this skill grants no new live access or export scope. No credentials in files, logs or vault notes. No WSL.
