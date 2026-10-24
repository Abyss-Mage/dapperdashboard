# Workspace and Master Dashboard scope map
Date:2026-10-23. User-requested implementation map. Canonical requirements/ADR005 apply. Proposed organization of existing scope, not new business-policy approval or production acceptance.

## Dashboard boundary
- /workspace: daily work for one explicitly selected authorized location. Used by staff and location managers according to grants.
- /owner: Master Dashboard for Owner/central administration; organization and authorized franchise/location oversight. Reuse Antigravity's Owner dashboard rather than creating a competing third admin application.
- /account and /booking: customer account/booking journey. Employee self-service stays separately scoped. Do not put internal finance/customer exports into customer or employee surfaces.
Both dashboards share the same authenticated Go/PostgreSQL authority. Dashboard labels never grant access. Cross-location totals include only explicit authorized scopes; central catalogue authority does not imply access to franchise customer/finance data. Final business-role templates and legal issuer/franchise mappings remain pending where unapproved.

## Workspace modules — daily location operations
| Area | Required screens/actions | Main evidence boundary |
|---|---|---|
| Home | Configured-location date/period, upcoming appointments, pending requests/quotes, open work orders, balances requiring attention, actual staff attendance, exception queue; direct links to actionable records | Existing scoped overview counts are a foundation; operational home/action drilldowns and time-local period reporting require acceptance |
| Bookings and calendar | Day/week/list, real availability/capacity, phone/walk-in/internal/website channels, customer+equipment intake, quote/payment gates, status history, conflict-safe moves/cancellation | Staff reserve/cancel/reschedule and request/quote primitives locally tested; complete customer confirmation/provider/policy gates remain |
| Customers and equipment | Search/create/view, customer-owned cars/marine/aircraft/heavy equipment, service/booking/job/invoice history, explicit account-link review, permitted notes | Scoped customer/equipment/account-link primitives present; complete directory/history/rebooking UX and privacy gates require verification |
| Work orders | Booking conversion, open work, multiple contributors, start/complete, service/addon snapshot, inspection/photos/tags/customer notes, handover/history | No fixed employee assignment or invented supervisor completion gate; uploads/inspection/mobile parity not implied complete |
| Quotations | Request inbox, scoped draft/review/issue, immutable terms/price revisions, customer acceptance/decline, expiry and fulfillment trace | Source/local tests do not approve legal text, prices, cancellation/hold policy or final merchant settlement |
| Payments and invoices | Approved deposits/full/partial settlement, cash, Square online/POS where configured, balances, invoice issue/PDF/delivery, refunds/reconciliation exceptions | Exact ledger/refunds locally tested; durable intent candidate unregistered, no live Square/hardware/email acceptance |
| Team and time | Present/on-break/off-duty from actual sessions, clock events/breaks, timesheets, correction/review/export and permitted job contributions | Overtime, supervisor permissions/exports and GPS policy cannot be inferred |
| Dealership/B2B | Customer/dealer contact, fleet intake/VIN/stock/PO/tags, equipment/bulk jobs, progress/history, approved billing/account balance | Existing module39source/local tests; billing terms/bulk finalization/client UAT remain |
| Schedule and resources | Approved service durations/buffers, available resources, working hours/holidays/closures, capacity configuration; explain conflicts | Configure only with explicit manage grants and reviewed rules; no assumed duration repair from legacy |
| Documents and communication | SOP/training/policy status, permitted uploads, actual notification delivery/failure/retry, calendar subscription status | SOP source presence is not legally complete signature/assessment or live external delivery |

## Master modules — central oversight and administration
| Area | Required screens/actions | Guardrails |
|---|---|---|
| Executive overview | Authorized organization/location selector, clear reporting period, bookings/completions, net recorded payments, outstanding balances, staffing/capacity and exceptions, drilldowns | Define metrics; no fake charts/zeroes for unavailable data or unsupported revenue claims |
| Locations and franchises | Organization/franchise/location records, activation/status, contact/configuration, hours/resources, scoped access and comparison | Company/legal issuer/franchise boundaries require explicit mapping; no automatic global scope |
| Central catalogue | Services/packages/addons/equipment categories, versioned published offers, reviewed price/tax/deposit/duration and location availability | Central catalogue confirmed; override/promotion authority and actual policy values pending |
| People and access | Invite/enroll/deactivate/revoke, explicit grant review, approved role templates, employee profile/onboarding/training/assessment status | No email-based owner bypass or auto-linking legacy users; initial administrator is not every-location finance authority |
| Finance and merchants | Scoped invoice/payment/refund/balance reports, issuer/numbering/tax configuration, separately bound franchise Square merchants/devices, unallocated capture/reconciliation queue | No credential display, provider-reference-as-payment proof, payroll or royalty accounting expansion |
| Reporting | Period/location/service/channel/job/customer/B2B/staff reports, exact net cash/card and receivables, export permission/audit, historical provenance | Report definitions and timezone/range semantics explicit; totals must reconcile to accessible ledger/records |
| Policies and booking controls | Approved customer booking/cancellation/reschedule/no-show/deposit/quote/hold/expiry settings, SOP durations/resource rules, consent/retention | Versioned accountable review; pending settings stay unconfigured rather than inherited from WordPress |
| Marketing and content | Mailchimp consent/subscription status, popup/forms/rules/submission outcome; CTO/Owner content editing and future public website integration | Website delivery is separately coordinated; no new marketing consent inferred from booking/account creation |
| Integrations | Square environment/merchant health, sender/outbox status, Twilio SMS, Mailchimp, Apple-compatible one-way calendars, webhook/recovery exceptions | Status derives from actual provider evidence; sandbox/local/mock distinguished from live |
| Audit and privacy | Who changed what/when/scope, account/grant history, financial/booking revisions, approved data retention/export/deletion handling | Immutable history integrity must coexist with approved privacy/redaction rules; sensitive data minimized |
| Migration control | Source/batch hashes/IDs, sanitized dry run, missing references/status mapping, duplicate policy, reconciliation, approved import/cutover/rollback | New full export is453customers/584appointments;105historical-service references preserved. No operational import authorization inferred |
| System administration | Domain/hosting/TLS, backup+restore evidence, monitoring/error alerts, job queues, deployment/configuration and recovery runbooks | Infrastructure diagnostics never expose secrets; local healthy ingress is not production readiness |

## Minimum overview metric definitions
- Upcoming bookings: actual bookings in displayed range, cancelled/completed excluded when label says upcoming. Submitted requests are separate and do not consume reserved capacity.
- Work completed: completed job/booking records with an explicit period basis; do not count quote acceptance as completion.
- Net recorded payments: completed captured/cash ledger amount less cumulative refunds. This is not automatically recognized revenue, profit or bank settlement.
- Outstanding: due balances on issued/partially-paid invoices; exclude draft/void invoices and unaccepted quotes.
- Staffing/capacity: actual attendance sessions and configured resource/shift/rule evidence. Do not infer efficiency from bookings alone.
Every card supports loading/error/empty/access-denied/unconfigured states, scope+period label, refresh timestamp and matching drilldown. Missing permission is not0. Historical imported figures need source labels and reconciliation.

## Navigation and shared UX
Workspace: Home | Bookings & calendar | Customers | Work orders | Quotes | Payments & invoices | Team & time | B2B | Resources | Documents.
Master: Overview | Locations | Catalogue | People & access | Finance | Reports | Policies | Marketing & content | Integrations | Audit & privacy | Migration | System.
Navigation is grant-filtered; no inert buttons or fabricated records. Reuse existing operational components where safe. Preserve forms/errors, keyboard focus/escape, responsive navigation, usable empty setup states, explicit confirmation for consequential actions and audit trails. Account-only customer checkout remains confirmed; no guest path introduced.

## Build order and responsibility
1. Confirm current Antigravity /owner source/deployment and exact file ownership; implement shared scope selector/navigation and first-run organization/location/access setup with no automatic grants.
2. Connect operational Workspace home→booking/request/quote→work order→completion drilldowns using real scoped APIs.
3. Complete money path: reviewed merchant routing/durable intents/provider confirmation→invoice allocation/refund/reconciliation; keep exception recovery visible.
4. Complete employee/B2B/document/communication workflows and role-specific mobile parity within approved policies.
5. Add Master cross-location reporting/configuration with exact permission isolation, real provider status and approved migration controls.
6. Full API+DB+browser acceptance, access negatives, ledger reconciliation, provider/hardware acceptance, backup restore, production deployment/UAT/handover.
Antigravity owns main Owner/Master frontend and its dashboard integration. Codex coordinates domain/API/security/payment authority and independent verification. Before every task start/finish update PARALLEL_AGENT_COORDINATION.md; do not run shared builds/migrations/restarts concurrently. Do not redefine full delivery around current fragments or claim chart/UI completion proves business acceptance.

## Current verification limits
Existing /workspace and /owner routes are present; Antigravity reports Owner build/TLS progress, but this document is a scope map, not an independent browser/client acceptance result. Deployed schema41 and local refund/authorization corrections are verified. New durable merchant-bound payment intent SQL remains unregistered and active payment routes have not been replaced by it. Credentials/policies/hosting/export mapping/live integrations and full27deliverables/15gates still require closure evidence. Do not promote older stale demo UI notes into production proof.

## Dashboard coverage snapshot2026-10-23 — numbers requested by user
Method: equal count of scope groups in this map, source-level presence only. A group counts as started if at least one relevant wired component exists. No weighting by effort, no browser/full workflow/production acceptance implied. Files inspected: SecureWorkspace.tsx and current Antigravity OwnerDashboardClient.tsx (8navigation tabs).
Workspace10groups:9have partial wired components (90%coverage);1lacks dedicated implementation (10%): location daily Home/action queue. Partial groups: bookings/calendar; customers/equipment; work orders; quotations; payments/invoices; team/time; dealership/B2B; schedule/resources; documents/communication (SOP panel only, communications incomplete). All10still need final scoped workflow acceptance;0fully accepted groups proven by this review.
Master12groups:7have partial wired components (58.3%coverage),5lack dedicated module screens (41.7%): reporting, integration administration, audit/privacy, migration controls, system administration. Started groups: executive overview; locations/franchises; central catalogue; people/access; finance; policies/booking controls (offer/scheduling primitives only); marketing/content (popup/marketing primitives only, complete CMS missing).8existingnavigation tabs do not equal8of12scope groups; multiple domains share admin/bookings panels. All12need full group acceptance;0fully accepted groups proven by this review.
Remaining Workspace work includes real daily home/drilldowns/day-week calendar, complete booking/payment/quote confirmation and policy gates, full customer/equipment/history and job inspection/upload/handover, staff/break/timesheet/mobile controls, B2B billing and live notification/calendar status. Master remaining work includes missing5modules plus cross-location comparisons, complete policy/CMS/role/configuration workflows, merchant routing/provider reconciliation/live status, privacy/backup/production acceptance. Do not present90%/58.3%as overall completion percentages. No reliable weighted finish percentage has been established.
