# Workspace review — 2026-10-24

Read-only independent review of Antigravity's three completed prompts. No code changes, application DB writes, migrations, shared build/restart or provider action.

## Checks performed

- Fresh native scripts/test-postgres.ps1 session72264 exit0: all9 tested packages PASS (postgres71.640s, workspace33.929s).
- Fresh npm run typecheck session53047 exit0 PASS.
- scripts/test-workspace-correctness.mjs:5/5 PASS. These are independent simulations of request counters/state, not tests importing/mounting the actual React components; do not call them component/browser acceptance.
- Current browser https://127.0.0.1:8443/workspace: unstyled sign-in, disabled inputs with secure-sign-in unavailable message. Body computed font Times New Roman, transparent background.
- Pinned certificate/hostname HTTPS GET: Workspace200; both referenced CSS assets `/_next/static/chunks/3k8uovkzekn1i.css` and `/_next/static/chunks/2kti2wa1nt_rs.css` return500 text/plain; /api/v1/health200.
- Source root layout still imports Manrope and globals.css; globals source still defines intended fonts/layout. web.stderr.log reports missing client reference manifest for root route. Inconsistent/stale shared build/runtime is a supported diagnosis; exact underlying sequence not proven. No stylesheet redesign is indicated.

## What is implemented in source

Prompt1: generation/abort handling, explicit jobRevision pulse refresh, awaiting_payment filter and error states present. Actual component delayed-request/logout/access-revoke acceptance still needed.

Prompt2: migration44 role definitions/capabilities/scoped assignments/audit; role APIs and Workspace existing-role location/franchise assignment UI present. New role reads are integrated into scope and transactional checks. Master role management screen still absent in inspected owner source. AccessManager depends on older /access and /structure APIs, so roles.assign-only usability must be tested end-to-end rather than inferred from role endpoint tests.

Prompt3: service deletion and structure update/delete source present. Used-service deletion can retain archive/history and returns an explicit message. Website owner added publication/central-link dependency edges. Successful source tests do not establish deployed/browser behavior.

## Release blockers found in role source

These require fixes and targeted regression proof; no live exploitation was attempted.

1. jobs.go authorizeTransaction now uses SELECT EXISTS/UNION without principal/grant/assignment/capability row locks, despite comments promising locks. Role/grant revocation and principal deactivation can race mutations. Central catalogue/org-role checks need the same review.
2. roles.go Create/Update/DeleteRole permit access.manage OR roles.manage. This does not enforce the requested separation of assignment administration from Owner/Sudo role-definition authority. No role-name/email bypass should replace explicit privileged authorization.
3. AssignMemberRole checks special admin capabilities and organization ownership but does not establish the assigner's authority over every requested role/capability and location/franchise target. roles.assign alone must not imply every operational scope; test anti-escalation beyond just access.manage/roles.manage.
4. UpdateRole counts administrators before removing admin capability. If the same role supplies two administrators, count2 can permit removing both. Role changes and assignment revocation also use different advisory lock keys. Last-admin protection must evaluate the post-change effective set and serialize all relevant paths, including direct grants and deactivation.
5. Assignment DELETE ignores expected version; Workspace sends no version. Completion report claimed optimistic revoke checking, but the inspected handler/store signature does not implement it.

## Today's ordered work

P0: coordinated local runtime repair first: preserve settings/data, recorded-process guarded stop, encrypted backup/isolated restore, review actual DB migration state against immutable source (latest website note41/43;44roles/45central-links pending), apply only reviewed registered migrations, one clean frontend build then guarded start and pinned HTTPS/static asset checks. Never register unrelated42payment candidate to fill a version gap. Prior website attempt was auto-review rejected before execution; requires explicit user authorization for local downtime and DB migration. No WordPress/DNS/live launch action.

P0: repair role authorization/last-admin/concurrency/version gaps and targeted tests before treating role administration as production-safe. Agree exact backend ownership with Antigravity; no concurrent edits/builds.

P1: Codex Master role CRUD/capability editor; Workspace member-role scope assignment only. Audit privileged configuration/provisioning explicitly; do not seed business roles/grants silently.

P1: authenticated browser acceptance of new controls: create/edit/delete unused entities, used-entity history retention, dependent franchise refusal, stale version, canceled confirmations, scoped role assign/revoke and changed capability behavior. Verify persisted state, 1440desktop/390mobile, fonts/CSS and no missing chunks. Never test destructive actions on real operational records.

P1: core operational acceptance: customer/equipment -> conflict-safe booking/reschedule/cancel -> multi-contributor job -> completion -> invoice -> cash/reconciliation, plus quote and attendance/break flows. Fix concrete failures; don't redesign.

P2/full-launch gates: complete remaining calendar/date-range/history/drilldown, inspection/photos/handover, B2B billing, training/notification/provider and production recovery requirements from DASHBOARD_SCOPE_MAP. Durable Square candidate integration and live provider evidence remain separate; no claim of all-scope completion today without those gates. Client policies/issuer/tax/provider/hosting approvals still constrain launch.

## Decision boundary

The requested prompts have meaningful source implementation but cannot be accepted as fully complete. Fresh tests pass; active CSS is demonstrably broken and new role semantics need hardening. Shared runtime activation and actual authenticated workflow acceptance come before additional UI features. No verified production-readiness percentage is available.
