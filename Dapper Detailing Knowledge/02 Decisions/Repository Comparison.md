# Repository comparison
Assessment date: 2026-10-21. Static source and documentation review; no candidate runtime was tested during selection.
Goal: reuse working business domains, not maximize README feature counts. No completion percentage is asserted.

| Candidate | Existing reusable domains | Material gaps / reason |
| --- | --- | --- |
| **frappe/erpnext + frappe/hrms — selected** | Customers/B2B, item/pricing/quotation/invoice/payment systems, reports, appointment primitives; HRMS onboarding, shifts/check-ins/attendance | Requires Dapper vehicle and franchise semantics, service booking extension, integrations, policy/assessment parity and branding. GPLv3; Frappe/MariaDB stack. |
| niko4244/openfieldpro | Next.js/Fastify/Postgres: work orders, dispatch, quotes, invoices, offline payments, documents, technician surfaces | AGPLv3-only. Less HR/CMS coverage. Reviewed appointment route checks overlap before insertion without a transaction in that path; reviewed operational guard has a development-owner fallback. Release-candidate status. |
| EdwardSoaresJr/ark-core | Automotive customers, vehicles, repair orders, inspections, scheduling, time clock, ledger, PDF, portal | AGPLv3-only. One database per shop. README explicitly excludes self-hosted card capture, customer email and website editor. Square-related filenames do not prove included processing. |
| Torqvoice/torqvoice | Next.js workshop operations, calendar, customers/vehicles, quotes/invoices, time tracking | Elastic License 2.0 limits hosted/managed-service use and license-key changes. Not a default franchise deployment base without resolving these terms. |
| one-repo-to-rule-them-all/poc_fieldpro | MIT FastAPI/Next.js: tenants, work orders, crews, GPS check-in, invoices, audit | Curated proof-of-concept snapshot. No complete automotive catalogue, booking checkout, employee onboarding or integrations. |
| carstio/carstio | MIT Symfony: clients/cars, offers, work orders, partial payments, PDF, SMTP, calendar | README explicitly excludes multi-location/multi-tenant and separate invoices. |
| mohsin86/auto-garage-services | Next.js/NestJS/Postgres vehicle/job/mechanic/invoice workflows | PDF export, gateway integration and automated critical tests listed as future work. LICENSE text is incomplete despite README's MIT statement. Not cleared for import as product base. |
| extradetailers/Extradetailers | Next.js/Django detailing booking journeys | No top-level license found in complete tree; reuse permission unresolved. |
| d0linger/Parkrr | MIT Go/Postgres vehicles, storage reservations, invoices/payments | Storage-rental domain, German UI and Austrian financial assumptions; detailing and employee workflows would largely be new work. |
| clawnify/OpenFieldService | MIT job scheduling, customers, equipment, technician dispatch, invoices | Lightweight Preact/Hono/SQLite application; limited match for franchise finance/HR/platform requirements. |
| odoo/odoo | Broad existing business suite | Community/Enterprise module boundaries need a separate edition audit; not credited with Enterprise appointment/field-service features by default. ERPNext/HRMS is the clearer selected open-source operations package. |
| distronode-corporation/district-scheduler | Go/Postgres availability, host scheduling, RLS and concurrency code | A component, not the main operations platform. |
| moasq/production-saas-starter | Go/Next/Postgres auth and organizations | Generic starter; explicitly rejected as the Dapper base. |

## Inspected evidence
- ERPNext appointment source: https://github.com/frappe/erpnext/blob/f01723b912f8f5572a7afa7e9caafb164aced7b3/erpnext/crm/doctype/appointment/appointment.py (develop discovery snapshot; confirm behavior at imported release).
- HRMS check-in source: https://github.com/frappe/hrms/blob/7fc314bdbdb69d0161bff1ba4dd46f4a7dffd8a2/hrms/hr/doctype/employee_checkin/employee_checkin.py (develop discovery snapshot).
- OpenFieldPro source pin: 25ba62297ba4b6890656017dd425117de166b94a; inspect apps/api/src/routes/appointments.ts, routes/invoices.ts, routes/public.ts and operational-authorization.ts.
- ARK README/source pin: dce3054619f36787e9e1d6784c9e4c4b907d2dce; README explicitly defines Core/Hosted and one-shop boundaries.
- Torqvoice LICENSE pin: f0a08003e532644c855a3790d70c34c817a20199.
- Carstio README/LICENSE pin: 1a059a4d60fb909aa26378616a30baa6e8790e5d.
- Auto Garage README/LICENSE pin: f5b7b60e1111b671955ee4717b1e49ee66e0803f.
- Extradetailers tree pin: 170451134898dec312e98ab5b38000851355182a.
- Parkrr README/LICENSE pin: 84a6ab68996af95d6ff592627f80815c8cf60805.
- FieldPro LICENSE/tree pin: de18256e0ae66b36a45297b5b9802f5dd4b9a29c.
- OpenFieldService tree pin: e23c1d416ca8716e67f51f5c6a25945b830c73a1.

See [[01 Requirements/REQUIREMENTS_MATRIX]] for the Dapper gap disposition and [[05 Evidence/BASE_IMPORT]] for actual import verification.

