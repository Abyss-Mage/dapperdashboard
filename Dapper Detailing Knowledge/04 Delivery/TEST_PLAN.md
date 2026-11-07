# Test plan
Source/import checks verify provenance, licenses and immutable source equality. They do not prove runtime behavior. ADR-002 replaces the old Frappe installation baseline with native Go/Next acceptance; upstream suites remain reference evidence only.

## Current automated suites
- apps/api: customer validation/session/origin/Host/duplicate/persistence; vehicle owner/field/override/duplicate/persistence; sourced catalogue approval/provenance/session boundary tests.
- apps/api/internal/postgres: missing scope rejection; optional real native database integration via scripts/test-postgres.ps1. Without DAPPER_TEST_DATABASE_URL the database test is explicitly skipped.
- integrations/wordpress/tests/exporter-test.php: isolated admin capability, same-origin, GET-only, pagination, public-only content/media, explicit SQL column projection and exact decimal/no-cache boundaries. This is not live WordPress testing.
- apps/web: production build includes TypeScript checking. Browser acceptance covers fictional customer creation, preserved duplicate errors, keyboard error focus/Escape/return focus, linked equipment creation, reload/restart persistence and 390x844 layout/navigation. This is not full accessibility certification.

## Remaining acceptance
Anonymous/insufficient roles, membership revocation, two-franchise CRUD/search/export/file/report isolation; concurrent capacity/rescheduling; duration/buffer/holiday/DST policies; quote/payment bypass prevention; Square signatures/idempotency/reconciliation; invoice exact totals/snapshots/corrections; employee jobs/attendance/break rules; GPS consent/access; provider retries; migration dry-run reconciliation; backup restoration; deployment/handoff.

Only record passed evidence for actual Dapper executions. See IMPLEMENTATION_STATUS and NATIVE_POSTGRES for scope and limitations.
