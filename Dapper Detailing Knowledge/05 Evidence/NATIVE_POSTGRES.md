# Native PostgreSQL foundation evidence
Date: 2026-10-21. No WSL, containers, system services or legacy changes.

## Runtime provenance
- Vendor page: https://www.enterprisedb.com/download-postgresql-binaries, linked by https://www.postgresql.org/download/windows/.
- Official 17.11 Windows link resolved to https://get.enterprisedb.com/postgresql/postgresql-17.11-5-windows-x64-binaries.zip.
- Download SHA256: 80379B2C04D51C30225532E0AE04509899141E9957ED096FE749D7FD9DF8F82F. This is a locally computed artifact hash, not a separately published signature.
- postgres --version reports PostgreSQL 17.11. Executable Authenticode status was NotSigned; provenance is the official vendor HTTPS download.
- Runtime/data/logs remain ignored under .cache/postgresql. Listening only on 127.0.0.1:15432, SCRAM authentication. Generated credentials are Windows-user encrypted outside source/Obsidian. Initialization password file is removed after initdb.

## Source and passing runtime evidence
- Go pgx v5.11.0 pinned in go.mod/go.sum; internal/postgres owns the initial migration and parameterized scoped customer persistence.
- Explicit migration command: cmd/database, requires -migrate and DAPPER_DATABASE_URL. Opening the pool or starting the demo does not auto-migrate a database.
- Transaction lock serializes migration runners. Checksums reject changed migration history.
- Composite foreign keys reject mismatched organization/location/customer vehicle links. Customer insertion and audit insertion share a transaction.
- scripts/test-postgres.ps1 ran against real native PostgreSQL, creates a unique isolated test schema, removes only that test schema and keeps connection credentials out of command output.
- Tests passed before and after database stop/start: scoped customer read, cross-organization location rejection, vehicle owner link rejection, audit presence, audit failure rollback, repeatable migration and checksum mismatch rejection.
- scripts/start-postgres.ps1 originally waited on the long-lived descendant process; corrected to wait only on pg_ctl. Clean stop/start and subsequent tests pass.

## Limits
The HTTP demo remains JSON-backed. PostgreSQL is an internal foundation, not a claim of production application integration. Scope structs must come from verified authentication/authorization; these are not implemented or exposed as trusted request fields. There is no approved franchise role policy, database RLS, production least-privilege role, production backup/restore evidence or provider configuration yet. Test database owner privileges are for local synthetic integration tests only. Actual organization/legal mapping remains pending client approval.

Update 2026-10-22: migration 2 and internal authorization now require an active principal and explicit per-location customers.read/customers.create grant. Both repository methods recheck grants on each operation. Real tests pass for cross-organization scope selection, unknown actions, disabled principals and revoked grants. Authentication/session proof and production HTTP integration remain incomplete. Audit rollback test injects a NOT VALID check constraint in the isolated test schema to reject the next audit insertion without invalidating existing audit rows.

Migration 3 adds explicit vehicle read/create actions. Scoped vehicle create/list, general equipment description validation, same-scope ownership FK, audit insertion and revoked create-grant rejection passed the real database suite. No production HTTP identity/session integration claim.
