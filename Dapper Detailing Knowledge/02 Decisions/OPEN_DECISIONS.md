# Open decisions
No approved demo business policies have been supplied. UI defaults and legacy values do not constitute approval.

| ID | Decision | Status | Blocks |
| --- | --- | --- | --- |
| DEC-01 | Franchise pricing/service/promotion authority | PENDING_CLIENT_APPROVAL | Catalogue grants and overrides |
| DEC-02 | Franchise/location customer and finance visibility | PENDING_CLIENT_APPROVAL | Data model scoping and access matrix |
| DEC-03 | Legal issuer and company-versus-franchise mapping | PENDING_CLIENT_APPROVAL | ERPNext Company mapping, invoice numbering |
| DEC-04 | Canadian tax rules and rounding | PENDING_CLIENT_APPROVAL | Final prices and invoices |
| DEC-05 | Service/category/package prices, durations and resources | PENDING_CLIENT_APPROVAL | Availability and checkout |
| DEC-06 | Guest booking, deposits, cancellation/reschedule/no-show policies | PENDING_CLIENT_APPROVAL | Customer booking lifecycle |
| DEC-07 | Booking/job transitions and approval authority | PENDING_CLIENT_APPROVAL | Operational actions |
| DEC-08 | Square merchant allocation, application and hardware | PENDING_CLIENT_APPROVAL | Online/POS integration |
| DEC-09 | Calendar authority, sharing and refresh expectations | PENDING_CLIENT_APPROVAL | Calendar adapter |
| DEC-10 | GPS devices, sessions, notice, consent and retention | PENDING_CLIENT_APPROVAL | Location collection |
| DEC-11 | CMS workflow and assigned public website work | PENDING_CLIENT_APPROVAL | Marketing website changes |
| DEC-12 | Transactional email/Mailchimp accounts and recipients | BLOCKED | Live provider checks |
| DEC-13 | Booking exports, timezone and historical mapping | BLOCKED | Real migration/reconciliation |
| DEC-14 | Missing portal backend, SOP/policy/assessment rules | BLOCKED | Proven functional parity |
| DEC-15 | Compatible local Frappe runtime | SUPERSEDED_BY_ADR_002 | Native Windows Go/Next runtime; no WSL; PostgreSQL production foundation remains incomplete |
| DEC-16 | Production infrastructure and remote client access | PENDING_CLIENT_APPROVAL | Deployment beyond local demo |

## Confirmed user changes
- New project-local Obsidian vault is canonical.
- Existing domain application required; generic starter rejected.
- Stack may change to maximize reuse.
- One implementer and Windows local demo.
- Plan with blocked tickets rather than invent business policy.


## Confirmed identity decision — 2026-10-22
DEC-17: CONFIRMED — user selected Dapper-managed email/password accounts. See ADR-004. OIDC remains optional and disabled. Actual staff provisioning, recovery email delivery and MFA remain separate implementation/configuration work.


## Direct answers received2026-10-22
See [[02 Decisions/2026-10-22 Client Answers]] for confirmed central catalogue/franchise isolation, separate Square merchants and2nd-gen Reader, cash, multi-contributor immediate completion, approved SOP-derived durations, account-only checkout, one-way calendars, CTO/Owner content editing, separate public website work, Mailchimp+Twilio SMS, exporter0.1.3 reported installed and exact synthetic editor grant approval. Partial GPS/booking/migration answers do not approve missing details; invoice/training/hosting decisions remain open. Earlier no-approved-direction wording is superseded only for these confirmed portions.

## Confirmed public website scope — 2026-10-24
Direct user: cinematic3Dsite is the intended production homepage; preserve current Services (+vehicle/protection pages),Portfolio,About Us,Contact Us,Account and Appointment coverage. Service add/modify authority is shared across website,Workspace and Master, with permission-controlled portfolio image management. This supersedes a website-only/demo-content interpretation.
This does not approve discrepant legacy prices/durations, deposit/cancellation terms, real grants, provider delivery or live WordPress/DNS/hosting cutover. Recovery/legal/content/publication review and actual shared runtime activation remain separate gates. Source implementation and local evidence are in04Delivery/PUBLIC_WEBSITE_PARITY.
