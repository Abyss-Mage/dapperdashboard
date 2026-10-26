# Dapper application and reference sources

The active application is Go/PostgreSQL in `apps/api` and Next.js in `apps/web`. See [application operations](APPLICATION.md) and ADR-005. Production-grade development is the current goal; complete production acceptance is still unfinished.

Pinned ERPNext, HRMS and Frappe imports under `apps/` are reference sources, not the runtime. Preserve their licenses/provenance and do not claim Python modules execute within Go.

The canonical requirements and adaptation decisions live in [the Obsidian vault](<../Dapper Detailing Knowledge/00 Home.md>). See [source provenance](../upstream-lock.json) for immutable revisions and archive hashes.

The active application has no shared demo session or JSON API fallback. Historical demo implementation is archived under `../reference/retired-demo`; isolated synthetic tests remain separate from application records. Do not reuse the rejected generic starter.
