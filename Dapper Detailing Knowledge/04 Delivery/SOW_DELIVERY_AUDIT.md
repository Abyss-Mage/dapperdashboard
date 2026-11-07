# SOW delivery audit
Updated 2026-10-22. Source: [[99 Sources/SOW Extract]], section18 p13 and section22 p15. This register preserves all27 named deliverables and15 launch acceptance gates. It is not a replacement for the unfinished atomic clause-to-test decomposition of sections3–17. Source requirements are requirements evidence, not authorization to alter live systems. Direct user changes in [[02 Decisions/2026-10-22 Product Changes]] supersede fixed employee assignment and standalone vehicle navigation.

No deliverable below has full approved-scope acceptance. Local synthetic checks do not prove provider, client UAT or production acceptance. Status describes the complete deliverable, not just its strongest implemented fragment.

## Named deliverables — SOW18
| ID | Required deliverable | Current evidence / gap | Proof required to close |
| --- | --- | --- | --- |
| DEL-01 | Approved requirements specification | Source documents and module matrix present; atomic decomposition and client decisions incomplete | Versioned requirements, each clause mapped to acceptance and recorded client approval |
| DEL-02 | Approved sitemap and UI/UX designs | Local dashboard views present; public sitemap/design approval missing | Approved public/customer/employee/admin flows across supported devices |
| DEL-03 | Custom responsive public website | Existing WordPress source captured; active Next routes are operations previews | Approved public pages, CMS content editing, SEO/legal/location pages, accessibility/browser acceptance |
| DEL-04 | Custom booking engine | Implemented in active scoped runtime (Migration 22, public /booking customer portal, workspace BookingManager, state lifecycle transitions, work order linkages, 18 live HTTPS checks) | Production client approval on cancellation refund policy, automated reminder triggers, and production payment gating |
| DEL-05 | Customer accounts and history | Staff-style managed identity and customer records exist; customer-owned portal missing | Own-only profile/history/invoices, multiple equipment records, eligible changes and rebooking |
| DEL-06 | Central administrative dashboard | Implemented in active runtime: production Owner Dashboard (`/owner`) with sticky navigation, live KPI pulse (`GET .../overview`), 8 domain sections, honest empty/access-denied states, responsive layout, Next.js Turbopack build PASS, 6/6 TLS ingress PASS. Dedicated Operations Workspace (`/workspace`) provides direct unified location shop-floor management | Client UAT on owner KPI summaries and multi-location management workflows |
| DEL-07 | Multi-franchise/location management | Scoped create/list and explicit grants verified | Full management/account lifecycle, hours/services and approved franchise boundaries |
| DEL-08 | Role-based access control | Exact location/action and organization structure grants, negative API tests | Approved configurable roles, protected provisioning/revocation, report/media/provider scope and full negative matrix |
| DEL-09 | Revamped employee portal | Implemented in active runtime: Operations Workspace (`/workspace`) delivering unified shop-floor controls (location intake, appointments, work order dispatch & contributor tracking, customer equipment, quotations, billing/payments, time & attendance clock, dealership/B2B fleet, service management, and safety SOPs) | Agreed employee lifecycle, schedules, attendance, inspections/tags/reports and mobile parity |
| DEL-10 | Employee onboarding and assessments | Implemented in active runtime (Migration 24, 5 canonical SOP PDFs registered with SHA-256 hashes and page counts, 4 training tracks, SOPCompliancePanel, non-repudiation log) | Client approval on legal signature standards and periodic retraining recertification intervals |
| DEL-11 | Time and attendance | Implemented in active scoped runtime (Migration 21, Go API, events ledger, Next.js panel, 18 HTTPS acceptance cases) | Approved policy on overtime rates/exports, automated supervisor approvals, and production payroll handoff |
| DEL-12 | GPS/location tracking subject to approval | Disabled; policy/device/retention unresolved | Approved notice/consent/session/visibility/retention, scoped data tests and supported-device evidence |
| DEL-13 | Dealership and B2B jobs | Migration 39 applied (`dealership_b2b.sql`), Go API endpoints, `DealershipManager.tsx` workspace UI wired into Next.js, HTTP unit/integration tests passing (100%) | Client UAT on dealership billing cycles and bulk invoice finalization |
| DEL-14 | Square online payments | No connected payment provider | Sandbox deposit/full/failure/refund, signature/idempotency/reconciliation, approved merchant routing and production qualification |
| DEL-15 | Supported Square POS | User specifies NFC reader; exact hardware/host unknown | Approved supported integration with actual reader/device/merchant and payment-reference reconciliation |
| DEL-16 | Cash payment recording | Implemented in active runtime (Migration 23, payments ledger with exact integer minor units, cash method, idempotency, operator audit trail) | End-of-day register float reconciliation and cashier shift sign-off policy approval |
| DEL-17 | Custom invoices | Implemented in active runtime (Migration 23, itemized line items, tax presets, exact integer cents, balance due invariants, InvoiceManager, printable receipts) | Approved corporate legal tax registration number (HST#), custom header logo asset, and automated customer email dispatch |
| DEL-18 | Apple Calendar synchronization | Implemented in active runtime (RFC 5545 calendar.ics feed for macOS/iOS/Google/Outlook, VCALENDAR/VEVENT components, UI sync button) | Approved one-way subscription authorization/refresh/cancellation and real external-client acceptance; bidirectional editing is not required |
| DEL-19 | Automated email workflows | Provider/sender configuration missing | Approved templates/routing/triggers/reminders, real delivery/failure/retry evidence and status monitoring |
| DEL-20 | Mailchimp integration | Migration 40 applied (`marketing_popups.sql`), Mailchimp API v3 Go client with MD5 hashing/tags/merge-fields, CASL/CAN-SPAM consent ledger, isolated PG tests PASS | Real Mailchimp API key and audience list ID configured in production |
| DEL-21 | Custom popup form builder | Migration 40, public endpoints (`/api/v1/public/marketing/popups`), `MarketingPopupManager.tsx` staff UI and `MarketingPopupModal.tsx` public modal with frequency/page targeting | Client approval on live promotional creative copy and display delay rules |
| DEL-22 | Booking migration | Live export assessed (SHA256: `E7CA26...`, 453 customers, 584 bookings, CAD $172,385.15 paid). Production cutover engine (`migrate_bookingpress_to_postgres.py`) with 5/5 unit tests and native PG transaction rollback verification PASS | Final client approval for production database `--apply` execution |
| DEL-23 | Integration and security testing | Scoped database/HTTP/TLS/browser tests pass for current slices | Complete requirement/permission/provider/performance/security matrix, dependency review and resolved critical defects |
| DEL-24 | Production deployment | Native loopback synthetic HTTPS/PostgreSQL only | Approved infrastructure/domain/secrets/least privilege, TLS, off-site backup/restore, monitoring/recovery and UAT release gate |
| DEL-25 | Technical documentation | Canonical architecture/API/schema/checkpoints exist; some historical statements superseded by appendices | Current coherent docs for final behavior, configuration, operations, recovery and integration limits |
| DEL-26 | Admin and user documentation | Secure workspace notes only; full manuals missing | Tested role-specific guides, employee/customer tasks, troubleshooting and training fixtures |
| DEL-27 | Handover and administrator training | Not completed | Designated recipients, delivered training/runbooks, recorded acceptance and support ownership |

## Launch acceptance gates — SOW22
All gates remain OPEN for full project scope, even when a component has local passing checks.

| Gate | Closure evidence |
| --- | --- |
| ACC-01 Approved design across devices | DEL-02/03, approved responsive/browser matrix |
| ACC-02 Agreed customer booking journey | DEL-04/05, actual end-to-end accounts/availability/payment |
| ACC-03 Authorized internal/walk-in booking | DEL-04/08, internal intake plus audit and payment handling |
| ACC-04 Shared admin/employee booking state | DEL-04/09, concurrent mutation/reload/cancellation evidence |
| ACC-05 Approved franchise/location isolation | DEL-07/08, complete cross-role/API/report/attachment negative matrix |
| ACC-06 Existing employee functionality | DEL-09/10/11/12/13, discovered parity scenarios and client approval |
| ACC-07 Square test and supported production | DEL-14/15, real provider/hardware evidence |
| ACC-08 Audited cash recording | DEL-16, approved requirement resolution and ledger tests |
| ACC-09 Approved invoice generation/delivery | DEL-17/19, exact approved PDFs and actual delivery |
| ACC-10 Calendar create/update rules | DEL-18, approved scopes and external synchronization checks |
| ACC-11 Email delivery | DEL-19, normal and failure/retry delivery evidence |
| ACC-12 Mailchimp forms/popups | DEL-20/21, Migration 40, Go v3 client, consent ledger, Next.js management + public modal |
| ACC-13 Reconciled booking migration | DEL-22, live export SHA256 verified, deterministic UUIDv5 mapping, 584 bookings / 453 customers, PG transactional verification PASS |
| ACC-14 Critical acceptance defects resolved | DEL-23, full defect/UAT report with no unresolved critical issues |
| ACC-15 Documentation/handover delivered | DEL-25/26/27, recipient review and completed training |

## Verified local evidence pointers
- `output/acceptance/scoped-browser-verification.json`: synthetic customer → main/add-on work order → completion → reload. Does not cover real reservations or finance.
- `output/acceptance/scoped-runtime-verification.json`: nine synthetic HTTPS groups, certificate/hostname validation, grant/actor/origin/service/add-on/session checks.
- `output/acceptance/scoped-restart-verification.json`: prior same-session/state restart check; rerun when runtime behavior changes materially.
- `output/acceptance/synthetic-restore-verification.json`: dedicated synthetic database restore, selected table counts; no production/off-site qualification.
- Native `scripts/test-postgres.ps1`, Go vet and web production build passed latest implementation increments; these checks are scoped to actual current tests, not whole-SOW proof.
- [[05 Evidence/BOOKINGPRESS_INVESTIGATION]]: partial read-only/static source evidence; exporter0.1.3 extras capture still requires user-controlled installation/export.

Next decomposition must enumerate every functional clause, conditional requirement, exclusion and decision in SOW3–17/20–25, and the additional master-brief constraints, with stable requirement IDs and concrete test/provider/UAT evidence. Do not mark this register as that completed atomic matrix.

## ADR-005 correction
Production application direction supersedes demo delivery. Old named-deliverable rows describing a complete production booking engine, working Square payments or legally complete invoices overstate the available evidence. Current implementation status is authoritative: real application uses an empty separate PostgreSQL database; no provider payment integration or complete production acceptance. Manual Square-reference settlement is now rejected. Retired demo screenshots and fixture-runtime checks are historical evidence only. All27 deliverables and15 launch gates remain open for full scope.
