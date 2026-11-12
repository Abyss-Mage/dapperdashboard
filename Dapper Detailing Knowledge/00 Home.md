# Dapper Detailing Pros
Updated: 2026-10-22
This Obsidian vault is the single source of project knowledge. Start here before implementation. Application repositories and upstream manuals describe code; they do not approve business policy.

## Current direction
- **CONFIRMED:** adapt an existing application with maximum reusable workflows. Do not start from a generic starter.
- **CONFIRMED:** functionality takes priority; the original Go/Next.js/PostgreSQL stack may change.
- **CONFIRMED:** Windows development, local demonstration, one implementation agent.
- **CONFIRMED:** business decisions stay blocked until client approval; no assumed demo pricing or operating policies.
- **CURRENT DIRECTION:** production-grade application and real website-integrated workflows/APIs, native Windows Go/PostgreSQL/Next.js; no WSL. See [[02 Decisions/ADR-005 Production Application Direction]]. Demo-product direction is superseded. ERPNext/HRMS remains reference knowledge.
- **NOT COMPLETE:** runtime setup, independent functional acceptance, full dependency audit, migration and provider integration.

## Navigate
- [[01 Requirements/Source Register]]
- [[01 Requirements/REQUIREMENTS_MATRIX]]
- [[02 Decisions/Repository Comparison]]
- [[02 Decisions/ADR-001 Base Platform]]
- [[02 Decisions/OPEN_DECISIONS]]
- [[03 Engineering/ARCHITECTURE]]
- [[03 Engineering/LEGACY_INVENTORY]]
- [[03 Engineering/SECURITY_MODEL]]
- [[03 Engineering/INTEGRATION_PLAN]]
- [[03 Engineering/B2B_DEALERSHIP_INVOICING_PLAN]] (error404 #17, built and deployed 2026-11-11)
- [[03 Engineering/SHOP_IPAD_CLOCK]] (error404 #15, #20: shop login, worker list, 6-digit PIN opens the worker's dashboard; deployed 2026-11-11)
- [[04 Delivery/MASTER_PLAN]]
- [[04 Delivery/SPRINT_BACKLOG]]
- [[04 Delivery/TEST_PLAN]]
- [[04 Delivery/IMPLEMENTATION_STATUS]]
- [[04 Delivery/LATEST_CHECKPOINT]]
- [[05 Evidence/BASE_IMPORT]]
- [[99 Sources/Master Specification]]
- [[99 Sources/SOW Extract]]

## Knowledge rules
Use CONFIRMED, PROPOSED, PENDING_CLIENT_APPROVAL and BLOCKED for decisions. Track implementation separately as unstarted, source-present, configured, locally-verified, externally-verified, blocked or deferred. Preserve source page/section references. Never upgrade an upstream claim into current Dapper verification. Record changes here first and link implementation evidence. No credentials, personal data or production exports belong in this vault.


- [[04 Delivery/SOW_DELIVERY_AUDIT]] — all27 named SOW deliverables and15 launch gates; full scope remains incomplete.

- **Start here for the latest session:** [[04 Delivery/HANDOFF_2026-10-24]] (what changed, live state, open work) and [[03 Engineering/DOMAIN_GLOSSARY]] (dealership vs franchise vs location).
