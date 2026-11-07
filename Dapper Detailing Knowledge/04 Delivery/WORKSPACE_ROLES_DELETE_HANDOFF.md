# Workspace next changes and Antigravity prompts

Date: 2026-10-23. This is a source/native-test review and implementation handoff, not browser acceptance or a completed implementation.

## Confirmed latest user direction

- Add Delete for services, and Edit/Delete for locations and franchises; extend lifecycle controls to appropriate other entities.
- Master manages named roles and their capabilities. Authorized Owner/Sudo Administrator can create, edit and delete roles and change their capabilities.
- Workspace Member access assigns an existing role to a member for a specific location or franchise. Remove individual capability editing from Workspace once the replacement is functional.
- Existing visual baseline stays intact. No blanket redesign or operational data deletion.

This confirms the role-management feature, not a preapproved permission set for every proposed role, global access for a particular email, or any current user's actual grants. Role names do not establish authority. Sudo is application authorization, not shell/root access.

## Verified current state

| Area | Current source | Gap |
| --- | --- | --- |
| Location/central services | ServiceEditor.tsx and CentralServiceEditor.tsx call POST archive; service edits preserve revisions | No Delete operation; current revision foreign keys prevent simply deleting parent rows |
| Franchises/locations | StructureEditor.tsx; workspace/structure.go and postgres/franchises.go implement list/create | No edit/delete API/UI; versioned change/delete checks needed |
| Member access | AccessManager.tsx edits one organization/location capability; access_management.go persists exact action grants | No persisted role definitions, role capabilities, scoped role assignments or franchise role resolution |
| Booking filters | BookingManager.tsx already filters via status query | Extend rather than duplicate; awaiting_payment missing from visible filter options, date-range query/UI not present in inspected manager |
| Shift counts | SecureWorkspace.tsx uses aggregate overview API | Refresh effect depends on jobs.length; completing a job without changing count need not refresh counts |
| Async location changes | Jobs/overview/booking loaders write results without request-generation or abort guard | Old responses can overwrite newer location/filter state; reset alone does not prevent this |
| Attendance | Active/history and break fields present | Non-OK reads silently retain prior state; distinguish unavailable/access revoked from valid empty data before adding a timer |

Source anchors: apps/web/app/workspace/{SecureWorkspace,ServiceEditor,CentralServiceEditor,StructureEditor,AccessManager,BookingManager,AttendancePanel}.tsx; apps/api/internal/workspace/{structure,central_services,access_management}.go; apps/api/internal/postgres/{services,central_services,franchises,access_management,access}.go and related SQL. Paths are relative to dapper-platform.

Verification this review: npm run typecheck PASS exit0; scripts/test-postgres.ps1 session83182 PASS exit0, all9 tested packages including real disposable PostgreSQL and scoped HTTP tests. Existing tests verify current archive/grant/create behavior, not the requested new features. No frontend build/restart, authenticated browser session, production provider test or application data change performed.

## Ordered work

1. Fix stale-scope responses, error state and shift refresh without changing layout.
2. Design shared role/scoped-assignment contract with Codex, then implement backend role authority and tests before connecting either UI.
3. Implement structure edits/deletion and service deletion with explicit dependency outcomes and history retention.
4. Wire Workspace role assignment and Master role management under separate owners.
5. Extend existing booking filters with supported date range/status pagination, then finish daily home drilldowns and remaining acceptance from DASHBOARD_SCOPE_MAP.

## Prompt 1 — Workspace correctness, no redesign

Read AGENTS/vault/current register and register exact files before editing. Preserve the restored Workspace layout. In SecureWorkspace jobs and overview loaders, and BookingManager loads, prevent old location/filter requests from overwriting the latest state using cancellation plus request identity or equivalent. Clear stale data on scope change/revocation and show loading/error/empty distinctly. Review AttendancePanel for the same scope/failure behavior. A rejected request must not leave previous location data displayed.

Refresh overview counts after successful relevant mutations even if list length is unchanged; preserve filter choice. Use a mutation revision/callback or explicit refresh, not jobs.length alone. Recheck permissions after session/access changes. Add deterministic delayed-response tests for rapid A-to-B switch and logout, same-length job completion, failed fetch, and revoked access. Run appropriate checks and browser desktop/mobile verification; coordinate shared build/restart. Do not alter Master/website/payment files. Record actual evidence and remaining gaps.

## Prompt 2 — scoped roles backend contract first

Implement the user's role model as real authenticated Go/PostgreSQL authorization, not frontend checkbox grouping. First coordinate shared backend/migration ownership and exact contract with Codex in PARALLEL_AGENT_COORDINATION.md; Master UI remains Codex-owned. Allocate a new migration version explicitly; do not edit old checksums or register the unrelated payment candidate.

Provide organization-owned named role definitions with versions and supported capability sets; separate role-definition management from member-role assignment permissions. Only explicitly authorized Owner/Sudo administrators manage role definitions in Master. Workspace exposes member + existing role + scope type (location/franchise) + target scope, current assignments, and revoke action. No arbitrary capability picker/role editor in Workspace. Do not seed invented business role policies or derive Sudo authority from email/name.

Enforce exact organization membership and location/franchise ownership on every read/write; franchise assignment never confers organization-wide or sibling-franchise access. Treat any organization/platform role scope as a distinct privileged grant, not an ordinary franchise dropdown option. Document and review whether franchise assignment includes future child locations; do not silently enable broader inheritance. Prevent location reparenting from silently changing effective access.

The actor must be allowed to assign the particular role at the target scope; ordinary assignment authority cannot grant privileged role-management/delegation capabilities. Prevent self-escalation and last-active-owner/Sudo lockout, including concurrent edits/revokes/deactivation. Preserve multiple independent grant sources: revoking one assignment must not remove permissions supplied by another. Capability reductions/revocations must affect existing sessions on the next request. Audit actor, scope, before/after and role version transactionally.

Use a server-known capability catalogue: role editing selects existing supported capabilities; it cannot invent executable permissions. Test and reconcile mismatches between current action allowlists and endpoints (including quotes, marketing and B2B) without auto-granting them. Existing direct grants require an explicit reviewed migration/provenance plan; no loss or silent widening. Old direct-grant APIs must not remain an alternate privilege-escalation route after switching the UI.

Tests: two organizations/two franchises/multiple locations, out-of-scope assignment/read, unknown capability, inactive member, role edit affecting existing assignments, assignment revoke, overlapping roles, stale versions, transactional audit failure and concurrent last-admin protection. Return the tested API contract for Codex's Master UI; do not edit owner files.

## Prompt 3 — edit/delete lifecycle controls

Add Edit/Delete for franchises and locations and Delete for location/central services. Preserve the restored UI; shared editors also appear in Master, so coordinate their props and backend ownership with Codex first. This is not authorization to delete existing records during development.

Implement authenticated scoped APIs and persisted changes, audit and optimistic concurrency. Show dependency-aware confirmation explaining exactly what Delete does. Do not merely relabel Archive. For a used entity, Delete may remove it from active selection while retaining its historical identity; state clearly that history remains. Offer permanent removal only when the designed schema permits it without destroying revisions, audit or dependent records. Keep a retained-history view where needed; no misleading success.

Services: distinguish location and central catalogues; handle add-on children, bookings/jobs, offers/scheduling/pricing, quotes/invoices and revision references. Deleted services cannot be selected for new transactions; historical descriptions/amounts remain intact. Never cascade-delete commercial records. Current revision foreign keys must be addressed in a reviewed migration/design rather than bypassed.

Structure: edit names first. Keep location identifiers stable. Treat moving a location to another franchise as a separate impact-reviewed operation because it can affect role scope, merchant routing and history. Refuse franchise deletion with dependent locations; return a useful dependency summary rather than deleting children. Used locations need explicit inactive/deleted lifecycle and stopped new operations while preserving history/access needed for review. Unused entities may support permanent deletion after reference checks, including grants/config/history. Backend constraints and locks must prevent races with new bookings/configuration while deleting; a frontend precheck alone is insufficient.

For other entities, create an explicit supported-operation matrix before adding buttons. Issued invoices use void/correction, payments use verified refunds, bookings use cancellation, staff access uses revoke/deactivate. Do not add generic destructive Delete to ledgers, audit, attendance or customer history. Draft deletion or privacy erasure needs its own dependency/retention design.

Verify authorized create/edit/delete and DB state, stale version409, foreign-scope denial, used-entity dependency behavior, no accidental cascade, historical reads and concurrent reference creation. Run native tests/typecheck/build plus actual desktop/mobile browser confirmation/error/focus checks in a coordinated runtime window.

## Handoff boundaries

Antigravity owns Workspace; Codex owns Master and payment candidate; website chat owns main website. This note proposes backend work for Antigravity only after explicit shared-file/migration allocation in the register. No ownership of shared handler/store.go is silently assumed. Full project acceptance remains open.
