# ADR-001 — Existing operations platform
Date: 2026-10-21
Status: SELECTED FOR ADAPTATION; production readiness unverified.

## Decision
Use frappe/erpnext as the primary application base. Use frappe/hrms for employee operations on the same Frappe framework. Preserve upstream applications and implement Dapper-specific changes in a separate Frappe extension, rather than rewriting their modules.

The user explicitly approved stack flexibility to maximize functional reuse. Python/Frappe, its existing web UI and MariaDB replace the earlier mandatory Go/Next.js/PostgreSQL proposal. No Go rewrite or parallel authoritative financial database is planned. A custom frontend is a later decision only if needed; do not rebuild every screen before proving the existing workflows.

## Pins selected
- ERPNext v16.51.1: 5af6561f5aa48104559692fb4fd82f2580e8d338
- HRMS v16.51.0: 2478afc7a8f4959d500f6d17522853f0d244e38f
- Frappe v16.52.0: 4515014c87ece6cb20ee91d2cfb121c5cbd1ff4a

ERPNext declares Frappe >=16.50.0,<17.0.0. HRMS declares both Frappe and ERPNext >=16.50.0,<17.0.0. These pins satisfy declared ranges; an actual installation and integration smoke test are still required. Newer releases are not adopted automatically. Frappe at this pin requires Python >=3.14,<3.15.

## Why
ERPNext supplies customers, contacts, quotations, service items, price lists, taxes, sales invoices, payment entries, timesheets, reports and appointment primitives. HRMS supplies employees, onboarding, training records, shifts, check-ins and attendance. The finance and HR domains are reused instead of created from zero.

## Material gaps
The stock appointment model is not Dapper's complete booking engine. Company/branch concepts are not proof of approved franchise isolation. Stock fleet records are not customer-owned boats, aircraft and machinery. Square, Apple-compatible calendars, Mailchimp/popups, legacy migration and SOP assessment/policy-signature parity still require work. All business approvals remain pending.

## Rejected direction
moasq/production-saas-starter is not the application base: it has infrastructure and auth but insufficient operational workflows. Its unmodified preliminary import was moved to .cache/rejected-starter-678523e and must not be used as active product code. District Scheduler is a scheduling-only candidate and will not be introduced alongside a second independently writable scheduler.

## License
ERPNext and HRMS carry GPLv3 license files; Frappe carries an MIT license. Retain all three licenses and upstream notices. Distribution and modifications must follow each actual license. Full transitive dependency/media license review remains open. No claim of unrestricted proprietary relicensing is made.

