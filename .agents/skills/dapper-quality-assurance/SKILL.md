---
name: dapper-quality-assurance
description: Verify Dapper changes with native Go/PostgreSQL, frontend build and browser acceptance evidence.
---

# Dapper quality assurance
Read canonical TEST_PLAN.md and IMPLEMENTATION_STATUS.md. Distinguish imported source, static checks, local runtime, live provider and production acceptance. Never copy upstream historical test reports as current Dapper results.

Native commands: Go tests/vet in apps/api; scripts/test-postgres.ps1 for real PostgreSQL tests; npm run build in apps/web; PHP exporter boundary suite; scripts/test_bookingpress_reference.py for source-data import boundaries. Database tests skipped without their environment are not live database evidence.

Use the documented browser UI tool for actual layout, keyboard, field-error and persisted-action checks. A successful build does not prove mobile fit or functional persistence. Test meaningful failure and authorization paths appropriate to each change; avoid tests that only repeat implementation wording.

A Windows sandbox filesystem/network denial needs diagnosis and an authorized native retry, not WSL or speculative business logic changes. Preserve unrelated servers and operate Dapper with its recorded process scripts.

Record changed files/hashes, exact passing checks, failures, blockers and next steps in the vault. Check quota before substantive continuation and stop/checkpoint at the user's threshold; do not claim completion to fit the budget.
