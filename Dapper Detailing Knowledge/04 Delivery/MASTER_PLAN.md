# Master plan
Current authority: ADR-002 and direct user direction. Native Windows Go API plus Next.js demo; no WSL. Imported ERPNext/HRMS/Frappe sources remain pinned reference knowledge with licenses preserved.

1. Preserve domain-source provenance and extract reusable workflow requirements; do not reuse the rejected generic starter.
2. Build a labeled synthetic local demo connected to the unified Go backend, with real local persistence and field-specific errors.
3. Implement production PostgreSQL, identity, authorization and audit foundations separately from demo-only storage/session mechanisms.
4. Expand each SOW requirement into an independently testable ticket and evidence link. Module-level source assessments do not establish functional reuse in Go.
5. Adapt approved customer/vehicle/catalogue, booking/capacity/job and employee workflows. Keep unapproved pricing, transitions, visibility and financial rules gated.
6. Prove negative access boundaries and concurrent booking behavior before real use.
7. Integrate Square/email/calendar/marketing with sandbox evidence, idempotency and failure recovery once accounts and policies are supplied.
8. Import legacy records only through approved dry runs, explicit mappings and reconciliation. Do not modify or retire legacy systems.
9. Complete responsive, security, recovery, deployment and handoff acceptance. Preserve every blocked SOW requirement.

Seven-day demonstration scope is conditional on approvals and provider access. A local synthetic demo is not operational or production acceptance.
