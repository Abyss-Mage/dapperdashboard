---
name: dapper-go-backend
description: Implement Dapper Go API and PostgreSQL changes, including scoped persistence, validation and migrations.
---

# Dapper Go backend
Read the project AGENTS.md, ADR-002 and the canonical API/database documents before changing a runtime contract. Active Go code is dapper-platform/apps/api; ERPNext/HRMS imports are reference sources, not Go services.

The JSON demo on 8120 and scoped PostgreSQL foundation on 8121 are separate modes. Never use the shared demo cookie as a fallback when a real session is absent. There is no public identity/session issuance flow yet.

Preserve migration checksums: add a version for schema changes. Customer/vehicle writes and audits share a transaction. Session issuance locks its principal against deactivation. Do not log DSNs, tokens or raw provider failures.

For real database tests use scripts/test-postgres.ps1 from the project root. It passes credentials privately and tests isolated schemas. pgx Config.ConnString retains the original string: mutating RuntimeParams does not update that string when opening another pool. Keep the test schema explicit in the new connection URL.

Use Go tests/vet appropriate to the change. A Windows sandbox rename/build failure is not evidence of an application bug; diagnose before changing storage semantics. No WSL.

References from this skill folder: ../../../Dapper Detailing Knowledge/03 Engineering/API_SPECIFICATION.md and DATABASE_SCHEMA.md. Update the vault with actual source/runtime evidence and remaining limits.
