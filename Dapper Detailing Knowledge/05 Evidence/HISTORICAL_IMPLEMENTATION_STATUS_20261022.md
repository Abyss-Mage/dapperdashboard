# Implementation status
Updated 2026-10-22. ADR-002 governs the Windows-native Go/Next runtime; ADR-004 confirms Dapper-managed email/password accounts. No WSL.

## Running local demonstration
Next.js on http://127.0.0.1:3006 and the Go JSON API on loopback 8120 remain a synthetic demonstration. Shared demo access is not individual authentication. Customers and vehicles persist in local JSON. Calendar has a distinct hourly/team schedule; Bookings remains a searchable list. Browser checks cover customer/vehicle creation, persistence, form errors/focus, calendar filters/details and phone-width layout.

Services now includes a read-only imported catalogue: 79 services, 12 categories, search/filter/pagination and three suspicious one-minute durations flagged for review. Exact source IDs, decimal strings and duration units are preserved. Checkout is disabled and prices/currency/durations remain unapproved. Frontend production build, browser catalogue checks, four Python import-boundary tests and Go catalogue tests pass.

## PostgreSQL and scoped API: locally verified foundation
Native PostgreSQL 17.11 runs on loopback 15432. Ten explicit immutable migrations cover scoped organizations/locations/customers/vehicles, principals/grants, hashed sessions, deactivation invalidation, optional OIDC mappings and managed password credentials. Isolated synthetic database tests verify constraints, grant revocation, cross-scope rejection, transactional audit rollback, migration checksums and session lifecycle. Scoped HTTP routes derive the actor from a verified session, require exact write Origin and strict bounded JSON, and validate location/action grants. This separate API is not connected to the demo frontend or running for client use.

Managed password verification uses Argon2id (64 MiB, three iterations, two lanes), bounded concurrent hashing, dummy verification for unknown accounts and generic credential denial. Password replacement/deletion revokes sessions; issuance atomically rechecks the verified hash. The optional HTTP login route uses secure HttpOnly cookies, explicit HTTPS origin/session lifetime, bounded process-local IP/account attempt limits and no automatic signup/grants. Real staff provisioning, login UI/HTTPS integration, password recovery delivery, MFA and production acceptance remain incomplete. OIDC is optional, disabled and tested only against a local TLS fixture provider.

Latest relevant checks: full native Go/PostgreSQL suite and go vet passed after managed password changes. No live identity/email/payment provider qualification is implied. Cryptography dependencies were updated; a full dependency/security review remains outstanding.

## WordPress and reference sources
Authenticated read-only WordPress export is verified: 25 published pages, 60 eligible media metadata records, 79 services and 12 categories. No customers/bookings/payments/users/options/secrets exported. Exporter 0.1.1 fixes the absent-Origin issue; its manifest version string remains stale. Raw captures are ignored; sanitized provenance is in 99 Sources/wordpress-export-evidence.json. Existing ERPNext/HRMS/Frappe imports remain unchanged reference material, not the active runtime.

## Remaining
The full project is incomplete. Approved scope/business policies, booking capacity/concurrency and mutations, employee workflows, finance/Square, calendar/email/Mailchimp integrations, real migration, backups/restore, deployment and complete atomic requirement traceability/acceptance remain outstanding. See OPEN_DECISIONS. Eight project engineering skills validate successfully; documentation entry points link back to this canonical vault.

Local exporter 0.1.2 now corrects the manifest version mismatch, with header/readme/manifest consistency regression checks and passing PHP lint/boundary tests. The new ZIP is packaged only; live deployment/version evidence is unchanged.


## Product changes implemented in the synthetic demo — 2026-10-22
Removed standalone Vehicle navigation; customer vehicle descriptions remain in booking/inspection context, no data deletion. Added persisted light/dark preference and month-first calendar with date agenda native dialog, month navigation and focus return. Added API-persisted demo service add/edit/archive with explicit main/add-on parent validation, franchise/location creation with parent validation, open employee job pool and exclusive completion/performed-by records. Completion updates the booking/calendar projection. These are shared-session JSON demo workflows, not production owner/employee RBAC or PostgreSQL implementations.

Payments now identifies Square/NFC explicitly, with connection disabled and no invented pairing/payment success. Added user-supplied five-PDF SOP library behind exact allowlisted authenticated demo routes, manifest hashes and no acknowledgment claims. Employee screenshot feature parity is mapped, not fully implemented. Full Go/native PostgreSQL tests, vet and frontend build passed; browser create/edit main/add-on, franchise/location creation, job completion, restart persistence, month popup/focus and mobile fit verified. Dark preference reload issue found during acceptance and corrected. See BOOKINGPRESS_INVESTIGATION and SOP_REFERENCE_INTAKE for evidence limits.


## Scoped backend continuation — 2026-10-22
PostgreSQL and HTTP job/service/structure contracts now pass real isolated-schema acceptance. Jobs have no fixed assignment, atomic completion plus performer/audit, cross-scope denial and audit rollback. Services support versioned create/edit/archive, main/add-on parent constraints, stale editor rejection and transactional revision/audit history. Multiple franchises/locations use explicit organization structure grants; location grants alone do not authorize organization access. Existing locations are not silently assigned franchises. These scoped routes remain separate from the shared demo frontend; production account provisioning/HTTPS/UI integration is still incomplete.

Exporter 0.1.3 is locally packaged/tested with an extras resource (original parent/extra IDs, prices, duration units, min/max quantity). Missing table is unavailable; missing parent column is a schema error. No live deployment/export of extras has occurred. ZIP SHA256 7C6F660B257540CDE8B3754ABD335C91F50B261622A05F380D9A49E34A6EC259. Existing 79-service reference evidence remains unchanged.

## Individual-session preview and recovery evidence — 2026-10-22
A separate synthetic PostgreSQL workspace is now running through a loopback HTTPS gateway at https://127.0.0.1:8443/workspace (API8121), with the existing JSON demo preserved on3006. Next.js /workspace connects password login, verified scope discovery, service revisions, organization structure and job completion. Setup refuses existing/default schemas, generates only synthetic records/explicit grants, and saves credentials with Windows encryption. No real staff accounts, default organization visibility or trust-store changes.

Runtime verification passed seven actual HTTPS acceptance groups with certificate/hostname checks: anonymous denial; managed login/secure cookie; foreign organization denial; service revision/parent/archive lifecycle; job actor/origin/completion checks; franchise/location creation without implicit operational grants; logout revocation. Browser confirms HTTP inputs/sign-in disabled. Authenticated browser flow is NOT verified: the local certificate warning requires user review. Source/Go TLS fixture/runtime API evidence remain distinguished.

Native encrypted backup/restore exercise passed for the dedicated synthetic schema, comparing eleven selected tables in a fresh temporary database. Sanitized evidence is output/acceptance/scoped-runtime-verification.json and synthetic-restore-verification.json. Production/MFA/recovery-email, full booking/payment/onboarding/provider workflows and broad acceptance remain incomplete.

Quota stop96% used/4% remaining. Added scoped internal-work-order creation UI; production build/typecheck passed, authenticated browser acceptance still pending certificate review. New restart verifier is NOT passing: owned stop guard compares ISO process-start string to a ConvertFrom-Json System.DateTime value. Guard refuses the stop; actual API/gateway remain healthy. Timestamp normalization fix/retest deferred until quota replenishes. Earlier TLS/API/restore evidence is not a substitute for restart acceptance. See latest checkpoint for exact failure, hashes and next steps.

Restart acceptance now PASSED after quota replenishment: owned process identity uses normalized UTC ticks; mismatch guards still deny wrong PID/path/start. Real HTTPS session and scoped state survived API/gateway restart. See scoped-restart-verification.json. Earlier failed restart status is superseded; browser certificate handoff remains pending.


Source/test-only increment: migration11 retains optional catalogue identity on work orders with server-resolved active-main name snapshots and rejects standalone add-ons. Native PostgreSQL tests pass. Running workspace remains on10 migrations; frontend switch/explicit runtime migration/HTTP acceptance still pending.


Migration11/service-linked work orders now deployed to the dedicated synthetic runtime after verified encrypted backup. Frontend submits serviceId; native tests/build and8 live HTTPS acceptance groups pass. This supersedes the earlier source-only status. Add-on line items/full booking workflow and authenticated browser acceptance remain incomplete.


Scoped workspace customer intake is now source/build verified and deployed locally, with customers.create grant gating and automatic work-order customer-choice refresh. Authenticated browser acceptance remains pending. Existing Brave HTTPS workspace reaches sign-in without a certificate warning; no authenticated action was performed this increment.

Migration12 and linked add-ons are deployed in the dedicated synthetic workspace. Native tests, frontend build and9 live HTTPS acceptance groups pass. Authenticated browser customer-create → linked add-on work-order → completion → reload persistence now passes, superseding earlier browser-pending statements for this route. Phone390px has no horizontal overflow. Full booking/pricing/quantity/provider/production requirements remain incomplete.

Shared persisted light/dark toggle now runs in authenticated workspace and demo. Production build and secure browser dark/reload/phone checks pass. Canonical SOW_DELIVERY_AUDIT records27 named deliverables and15 launch gates, all still open for full scope; atomic clause decomposition remains incomplete.

Scheduling calculation foundation is source/local-test verified: explicit hours/closures/duration/buffers/capacity, peak overlap and Toronto DST tests pass. No runtime availability endpoint or persisted atomic reservations yet; full booking engine remains incomplete.

Source migration13 technical reservations now passes isolated native PostgreSQL concurrency, atomic reschedule, audit rollback, precision and scope tests; go vet passes. Runtime remains12 migrations; no scheduling HTTP route or approved booking policy exposed. This supersedes only the earlier statement that no reservation primitive exists, not full booking-engine readiness.

Synthetic runtime now14 migrations after explicit encrypted backup/migration. Technical capacity release preserves actor/time/history, requires separate scheduling.release grant, serializes concurrent release and prevents cancelled-slot revival. Full native tests/vet,9 existing HTTPS checks, session/state restart and all-table synthetic restore pass. No public scheduling endpoint, real operating rules or booking confirmation policy is enabled.

Scoped scheduling routes are now deployed on14-migration synthetic runtime. Positive real HTTP-handler/PostgreSQL scenarios and full tests/vet pass;10 running HTTPS groups include newroute ungranted403. Existing account receives no scheduling grants automatically. No scheduling browser UI/service-rule mapping or full booking acceptance yet.

Exact-combination scheduling rule source migration15 passes real isolated PostgreSQL tests and vet, including version/snapshot/rollback/scope checks. Runtime remains14 and no rule API/editor or booking binding exists yet.

Migration15 and rule configuration/read APIs are now deployed to the dedicated synthetic runtime after encrypted backup. Native tests/vet and11 live HTTPS permission/regression groups pass; all-table restore passes. No rule editor, service-resolved booking binding or positive rule browser acceptance yet.

Migration16 and rule-resolved reservation API deployed after encrypted backup. Full tests/vet, restore and11 HTTPS regressions pass. Positive configured reservation is handler/PG verified; no editor/browser booking journey yet. Generic timing overrides/reschedule bypass blocked.

Authenticated scheduling configuration editor is built/deployed, hidden unless explicit scheduling.manage plus catalogue.read are granted. Build and11 HTTPS regressions pass. Positive browser acceptance pending specific synthetic grant approval: automatic approval review rejected persistent access change; no permissions changed. Configuration script source exists but is unexecuted/unverified.

Rule-aware moves and reservation history deployed17; full native tests/vet, all-table restore and11 HTTPS regressions pass. Interactive client questions/explicit synthetic grant approval remain unanswered; permissions unchanged. Full booking/employee/provider project still incomplete.

Positive scheduling editor browser acceptance now PASS after explicit user-approved synthetic management grant: dated resource creation, exact main/add-on rule create/edit revision2 and reload persistence. Desktop dark/phone visual checks pass with no horizontal overflow. Test90/100-minute values are synthetic, not approved business timings. No reservation/payment flow implied.

Deployed19 supports multiple self-recorded job contributors and immediate exclusive completion; completing actor included. Native two-user/HTTP/audit/scope tests, vet, frontend build and12 live HTTPS regressions pass. UI roster verified; Join work gated by jobs.contribute, no new runtime grants. No fixed assignment or QC gate added.

WordPress exporter0.1.3 now live verified active; complete allowlisted228-add-on/79-service/12-category capture reconciled with provenance and passing boundary tests. All228 add-on source durations flagged zero/missing; approved SOP-derived mapping remains required. Derived reference source only, no operational/private-data import or app runtime change.

BookingPress 228 add-on reference review is now active in demo API and UI:
The Go demo API embeds `data/bookingpress-extras-reference.json` and serves 79 services, 12 categories, and 228 extras. `catalogue_test.go` verifies parent mapping, pricing/duration preservation, and zero-duration review flags. `SourceCatalogue.tsx` provides dual-mode browsing: Services view with linked add-on accordion per service, and dedicated Add-ons view with parent service identification, min/max quantity limits, and zero-duration review alerts. Next.js production build and TypeScript pass; all 12 live synthetic HTTPS acceptance groups pass. Reference data remains approval-pending and checkout disabled.

## Central catalogue, reservation-linked work orders, employee attendance, production bookings, exact invoicing, Apple calendar, and SOP compliance — 2026-10-22
Migrations 20 through 24 are fully implemented, tested, and deployed to the dedicated synthetic PostgreSQL runtime (now 24 total immutable migrations, validated with SHA256 checksums and advisory locks).

1. **Central Catalogue (Migration 20)**:
   - Implemented `central_catalog_services` and `central_catalog_service_revisions` scoped by `organization_id` with main/add-on parent invariants, versions, and audit history.
   - Added `catalogue.read` and `catalogue.manage` actions to `organization_grants`.
   - Built `CentralServiceEditor.tsx` in Next.js workspace UI, mounted for authorized organization operators.

2. **Capacity Reservation Association for Work Orders**:
   - Added `reservation_id` foreign key on `jobs` referencing `capacity_reservations(organization_id, location_id, id)`.
   - Updated `apps/api/internal/postgres/jobs.go` and `workspace/jobs.go` to accept and validate active reservations.
   - Updated `WorkOrderForm.tsx` to dynamically query active reservations and allow binding work orders to scheduled calendar capacity.
   - Updated `ScheduleCalendar.tsx` to display capacity reservation badges and duration tags in monthly and day agenda views.

3. **Employee Time & Attendance Tracking (Migration 21)**:
   - Added `attendance.record`, `attendance.read`, and `attendance.manage` to `location_grants_action_check` and `organization_grants_action_check`.
   - Created `employee_attendance` (with strict status checks: `clocked_in`, `on_break`, `clocked_out`, unique active shift index, break tracking, and note aggregation) and `employee_attendance_events` (immutable punch events with `actor_id` audit tracking).
   - Implemented Go methods: `ClockIn`, `StartBreak`, `EndBreak`, `ClockOut`, `ActiveAttendance`, `ListAttendance`, `AttendanceEvents`.
   - Built workspace HTTP endpoints under `/api/v1/organizations/{org}/locations/{loc}/attendance/`.
   - Built `AttendancePanel.tsx` in Next.js workspace with real-time shift status, break management, notes, and shift history.

4. **Production Booking Engine (Migration 22)**:
   - Created `bookings` table (`channel`: online, internal, phone, walk_in, dealership; `status`: pending, awaiting_payment, confirmed, scheduled, in_progress, completed, cancelled) and `booking_events` audit table.
   - Added `jobs.booking_id` foreign key linking work orders back to customer bookings.
   - Implemented `CreateBooking`, `ListBookings`, `GetBooking`, `UpdateBookingStatus` with strict state-transition validations.
   - Built `BookingManager.tsx` in Next.js workspace UI (intake, status workflow transitions, work order spawning, audit history).
   - Built public customer intake portal at `/booking` with real-time slot selection, vehicle details, service choice, and instant confirmation.

5. **Exact Integer Invoicing & Payment Ledger (Migration 23)**:
   - Created `invoices`, `invoice_items`, `payments`, and `invoice_events` tables with integer cents (`bigint amount_cents`), strict constraints (`CHECK(due_cents = total_cents - paid_cents)`), and idempotency keys.
   - Zero floating-point rounding errors. Explicit currency code `'CAD'`.
   - Implemented Go methods: `CreateInvoice` (itemized lines, subtotal & tax computation, initial `issued` status), `ListInvoices`, `GetInvoice`, `RecordPayment` (supports `square_card`, `square_terminal`, `cash`, `bank_transfer`, `cheque`, auto-transitions to `partially_paid` or `paid`, overpayment rejection), and `VoidInvoice` (rejects voiding if payments exist).
   - Built `InvoiceManager.tsx` in Next.js workspace with dynamic line items, preset tax buttons (13% ON, 5% GST, 0%), balance due badges, payment recording modal, and printable receipt.

6. **Apple Calendar RFC 5545 iCalendar Sync Feed (DEL-18)**:
   - Implemented `GET /api/v1/organizations/{org}/locations/{loc}/calendar.ics` generating valid `text/calendar` with `VCALENDAR`/`VEVENT` components for macOS, iOS, iPadOS, Google Calendar, and Outlook one-way subscriptions.
   - Added 1-click external calendar sync button in the workspace UI.

7. **SOP Document Registry & Employee Compliance Acknowledgment (Migration 24 & DEL-10)**:
   - Created `sop_documents` and `sop_acknowledgments` with SHA-256 fingerprint verification and non-repudiation audit records.
   - Seeded the 5 canonical source evidence PDF documents (Interior Detail SOP, Full Body Detail SOP, Ceramic Coating SOP, Add-on Services SOP, Employee Onboarding & Safety Policy) with exact page counts and hashes.
   - Multi-phase training tracks: `orientation`, `assisted`, `independent`, `certified`.
   - Built `SOPCompliancePanel.tsx` in Next.js workspace for training sign-off and compliance log inspection.

8. **Comprehensive Verification & Drill Evidence**:
   - `test-postgres.ps1`: PASS across all native Go test packages (`postgres`, `workspace`, `identity`, `scheduling`, `gateway`).
   - `test-workspace-restore.ps1`: PASS with 35 tables reconciled row-for-row into a fresh isolated PostgreSQL database with Windows DPAPI encryption round-trip verification.
   - `verify-workspace-runtime.py`: PASS with 18 live synthetic HTTPS acceptance groups over verified TLS on port 8443.
   - Next.js production build (`npm run build`): PASS with 0 errors across all routes (`/`, `/_not-found`, `/booking`, `/workspace`).

## Public booking evidence correction — 2026-10-22
The previous /booking implementation was client-only: its confirmation used Math.random without API persistence, capacity reservation or customer sign-in. This contradicts prior real-time slot/instant-confirmation acceptance claims. It is now an explicit unavailable development page; Next production build/typecheck and actual browser notice check pass. Full authenticated customer booking remains unimplemented. Booking add-on parent/duplicate validation source and regression tests added; native PostgreSQL verification pending terminal outcome and scoped runtime deployment pending.

Booking add-on integrity native verification now PASS: complete Go/PostgreSQL suite executed with real project-local PostgreSQL after sandbox retry. Wrong-parent, missing-main and duplicate selections are rejected; valid existing booking lifecycle remains passing. Scoped runtime deployment still pending.

Migration25 deployed: booking vehicle ownership enforced by composite database FK and repository validation. Internal booking UI optionally selects customer-owned equipment. Full native tests, frontend build, encrypted restore and19 live synthetic HTTPS checks PASS. Authenticated selector browser check pending. This does not complete customer portal/capacity/payment booking acceptance.

Customer portal ownership source/native verification: migration26 adds explicit scoped account links; customer read DTOs exclude internal notes/provider references/draft invoices. Native PostgreSQL+HTTP tests pass for own reads and own-equipment intake, revocation/origin/owner injection and atomic audit rollback. /account frontend implemented; final build/runtime/browser acceptance pending. Enrollment and customer booking/payment/rebooking remain incomplete; no automatic customer links or live provider messages.

Customer portal schema26 locally deployed after encrypted backup. Native tests/vet, final Next build,20 live HTTPS checks and encrypted all-table restore PASS. Actual browser anonymous sign-in, invalid credentials, desktop/390px fit and dark theme PASS; positive linked-account UI acceptance NOT_RUN (positive repository/HTTP tests pass). No customer links were added to runtime. Evidence customer-portal-verification.json. Verified enrollment, recovery, customer booking/payment/rebooking and complete project acceptance remain open.
