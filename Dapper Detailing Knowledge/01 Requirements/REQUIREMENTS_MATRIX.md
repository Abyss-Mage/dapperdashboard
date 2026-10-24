# Requirements reuse matrix
Historical source-reuse assessment: the Existing base column describes imported Python reference primitives. Under ADR-002 these do not execute in the active Go application. Current implemented evidence belongs in IMPLEMENTATION_STATUS; every active workflow requires Go/Next implementation and verification.
This is a module-level selection assessment. Full atomic requirement-to-ticket/test traceability remains to be expanded before feature implementation. All source requirements remain available in [[99 Sources/SOW Extract]] and [[99 Sources/Master Specification]].

Legend: **Reuse** = existing domain primitive, not accepted Dapper behavior. **Adapt** = configuration or extension required. **New adapter** = provider-specific work. **Blocked** = missing approval/data. Source presence does not establish passing tests.

| ID | SOW / brief | Required capability | Existing base | Dapper work and gate |
| --- | --- | --- | --- | --- |
| DDP-01 | SOW 3.1,6.3; brief 17 | Responsive public pages, SEO, editable content/media, location pages, legal pages | Frappe website/content infrastructure | Adapt approved CMS/public experience; cinematic landing work remains separately assigned |
| DDP-02 | SOW 3.1.2,4.1 | Customer-owned configurable vehicle/equipment categories | Customer/contact and stock vehicle primitives | Adapt generalized customer vehicle record for aircraft/marine/heavy equipment; do not assume fleet vehicle is a customer vehicle |
| DDP-03 | SOW 3.1.3,4.2 | Services/packages/add-ons, location/vehicle pricing, quotes, deposits, discounts/taxes | Items, price lists, pricing rules, quotations | Adapt detailing catalogue and category/duration rules; approved prices/taxes/authority blocked |
| DDP-04 | SOW 4.1,4.5 | Online/internal/walk-in/phone/dealership booking | Appointment/customer/assignment primitives | Extend public booking journey and intake channels, payment/quote gates |
| DDP-05 | SOW 4.3,4.4 | Capacity/hours/holidays/buffers, no conflicts, required statuses | Appointment/calendar primitives | Extend location/resources/service durations; client transitions/cancellation rules pending; concurrency proof required |
| DDP-06 | SOW 4.6 | Customer account, multiple vehicles, history, rebooking, own invoices | Frappe website users/customer portal | Adapt ownership and vehicle/rebook journey; guest policy blocked |
| DDP-07 | SOW 5,6 | Central/franchise/location administration, permissions and organization hierarchy | Company/branch, users, roles, permissions, reports | Approve legal/business mapping; enforce location/resource/attachment/report scope; no automatic Company=Franchise assumption |
| DDP-08 | SOW 5.1,16 | Permission-aware real dashboard and operational/finance reports | ERPNext reports/workspaces/dashboards | Configure role-aware Dapper KPIs and status/vehicle/job reports; verify accessible records |
| DDP-09 | SOW 7.1 | Employees, assignments, onboarding documents/training/assessments | HRMS employee/onboarding/training | Adapt signed policy/SOP/assessment parity from recovered portal; source rules missing |
| DDP-10 | SOW 7.2 | Clock-in/out, attendance, breaks, timesheets/review/overtime/export | HRMS check-in/attendance/shifts; ERPNext timesheets | Configure and adapt approved time/break/overtime rules; payroll excluded |
| DDP-11 | SOW 7.3 | Authorized GPS sessions/activity reference/access/retention | HRMS geolocation check-in primitive | Session model/visibility/consent/device validation blocked; no continuous-background claim |
| DDP-12 | SOW 7.4,7.6 | Assigned mobile employee jobs, start/complete/notes | Assignments/tasks and HRMS employee surfaces | Adapt detailing job lifecycle, completion verification and restricted customer details |
| DDP-13 | SOW 7.5 | Dealership tags/fleet intake/bulk jobs/history/billing | B2B customers/contacts, quotation/invoice primitives | Adapt vehicle intake, tagging/photos, bulk jobs and scoped dealership summaries |
| DDP-14 | SOW 8 | Square deposits/full payments/POS/refunds/reconciliation | ERPNext payment accounting primitives | New Square adapter; merchant/hardware approvals and credentials blocked |
| DDP-15 | SOW 8.3 | Audited cash payment recording | Payment Entry / cash modes | Configure scoped employee permissions, booking/invoice allocations and audit; no Square cash sync required |
| DDP-16 | SOW 9 | Draft/final invoice, PDF/email, partial balance, corrections/history | Sales Invoice, Payment Entry, print formats | Configure approved issuer/numbering/tax; connect job verification and configurable delivery triggers |
| DDP-17 | SOW 10 | Company/franchise/location/employee Apple-compatible calendar updates/cancellations | Internal Event/calendar primitives | Validate ICS/CalDAV approach; hierarchy/sharing/refresh expectations pending |
| DDP-18 | SOW 11 | Configurable transactional email/routing/reminders/retry/status | Frappe Email Queue / Notification | Configure Dapper events/providers and failure tests; actual sender access blocked |
| DDP-19 | SOW 12 | Mailchimp, consent, popup builder/display rules/feedback/tracking | Web forms/content and background jobs | New adapter and popup settings; independent consent and subscription handling |
| DDP-20 | SOW 13 | BookingPress migration, original IDs, dry run/reconciliation/cutover | Frappe import tooling | Custom source mapping and repeatable importer; source records absent; cutover approval required |
| DDP-21 | SOW 14,17 | Auth, data isolation, privacy/audit, backups, monitoring/tests | Frappe permissions/session/version/backup/test framework | Dapper-specific isolation and negative tests; full license/dependency/security/runtime review still open |
| DDP-22 | SOW 18,22; brief 22–29 | Deployment, docs, training/handover, demo data and acceptance | Existing upstream deployment/testing/docs | Windows container setup; synthetic data; documented local demo; later Ubuntu/production deployment and training |
| DDP-23 | Legacy portal discovery | ID proof, emergency contacts, policy receipt/SOP tests, image tagging, inspection uploads, schedule requests/approvals, breaks/weekly reports | HRMS onboarding/shift/attendance/attachment/report primitives | Explicit parity tickets and client validation; backend implementation missing |
| DDP-24 | SOW 21 | Exclusions and separate approvals | Existing upstream modules may exceed Dapper scope | Hide unused payroll/inventory/royalty/SMS/native-app capabilities; do not expand delivery obligations |

## Acceptance and status
Each row must be split into independently testable tickets in the backlog, linked to approved configuration and actual test evidence. Initial reuse assessment is source/document based. No row is currently marked fully implemented for Dapper.


Full named-deliverable and launch-gate audit: [[04 Delivery/SOW_DELIVERY_AUDIT]]. All27 SOW18 deliverables and15 SOW22 gates are retained with closure evidence; full atomic functional-clause decomposition is still unfinished.
