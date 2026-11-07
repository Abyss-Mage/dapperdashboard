# DAPPER DETAILING PROS
# MAIN OPERATIONS PLATFORM — PRODUCT & ENGINEERING MASTER SPECIFICATION

**Version:** 1.0 — Initial development brief  
**Target:** Seven-day staging demonstration  
**Backend:** Go  
**Frontend:** Next.js / TypeScript  
**Database:** PostgreSQL  
**Infrastructure:** Isolated Ubuntu 26.04 VM  
**Planning:** Astra  
**Implementation:** Sol 6.1

---

## 1. YOUR ROLE AND INSTRUCTIONS

Act as the Principal Software Architect, Product Engineer, and Technical Lead for Dapper Detailing Pros.

The attached 16-page Scope of Work is the primary requirements document.

Read the entire SOW before making implementation decisions.

Your responsibilities:

- Translate every SOW requirement into an implementation backlog.
- Examine the existing repository before writing code.
- Identify reusable open-source modules and their license obligations.
- Design an extensible, secure, multi-franchise architecture.
- Prepare precise implementation tasks for Sol 6.1.
- Maintain complete documentation and traceability.
- Ensure every implemented feature has appropriate tests.
- Prepare a client-demonstrable staging environment within seven days.

**Do not treat assumptions as approved business requirements.**

Classify decisions as:

- `CONFIRMED` — explicitly established by the client or SOW.
- `PROPOSED` — recommended technical implementation.
- `PENDING_CLIENT_APPROVAL` — unresolved business rule.
- `BLOCKED` — requires credentials, source data, or another dependency.

Do not silently remove SOW requirements to fit the deadline. Track all of them, even when deferred beyond the first staging demonstration.

## 2. PLATFORM OBJECTIVE

Build a centralized franchise-oriented operations management platform for Dapper Detailing Pros, a Canadian automotive detailing and vehicle protection business.

The platform must support:

1. Central company administration.
2. Franchise administration.
3. Multiple operating locations.
4. Customer and vehicle management.
5. Service and package configuration.
6. Online and internal booking management.
7. Employee management and onboarding.
8. Job assignments and service completion.
9. Employee time and attendance.
10. Authorized GPS/location tracking.
11. Dealership and B2B operations.
12. Square payment integration.
13. Manual cash payment recording.
14. Customized invoices and PDF generation.
15. Calendar synchronization.
16. Email notifications.
17. Mailchimp integration and popup management.
18. Operational and financial reporting.
19. Existing booking data migration.
20. Secure role-based and location-based permissions.

The system must support expansion to additional franchises without requiring a separate application for each franchise.

## 3. DESIGN SYSTEM

Use the agreed Modern Premium UI direction.

### Branding

Official client brand color:

`#FB1A2B`

Proposed interface tokens:

- Primary brand: `#FB1A2B`
- Accessible primary button: `#D91627`
- Button hover: `#C60E1F`
- Background: `#F8F9F7`
- Surface: `#FFFFFF`
- Main text: `#152229`
- Muted text: `#66727A`
- Borders: `#E7E9E7`
- Subtle brand surface: `#FFF1F2`

Typography: Manrope.

Use a professional, minimal dashboard style with compact information hierarchy, comfortable spacing, clear tables, accessible forms, and responsive layouts.

Use shadcn/ui and Tailwind CSS.

The public marketing landing page may use GSAP, motion graphics and Three.js.

**Do not use cinematic scrolling, WebGL, heavy parallax, or Three.js in internal operations dashboards.**

Only implement lightweight transitions where they improve clarity.

## 4. USER ROLES

Implement a configurable role-based access system.

### Super Administrator

Authorized global control over:

- All franchises and locations.
- Global platform configuration.
- Employee and administrator management.
- Services and applicable pricing.
- All bookings and operational records.
- Authorized financial reports.
- Payment and invoice administration.
- Global website content.
- Integration settings.
- Audit history.

### Franchise Administrator

Access limited to authorized franchise operations:

- Assigned locations.
- Employees and schedules.
- Booking management.
- Customer records.
- Authorized pricing and services.
- Financial summaries.
- Invoices and payments.
- Dealership accounts.
- Franchise reports.

### Location Manager

Access limited to assigned locations:

- Location bookings.
- Walk-in bookings.
- Appointment scheduling.
- Employee assignments.
- Job completion.
- Cash payment recording where authorized.
- Customer and vehicle information necessary for operations.

### Employee / Detailer

Access only to permitted employee functionality:

- Assigned jobs.
- Schedule.
- Required customer and vehicle details.
- Job status updates.
- Clock-in and clock-out.
- Attendance history.
- Training and assessments.
- Authorized GPS tracking sessions.

### Finance / Accounting

Access to authorized:

- Invoices.
- Payments.
- Refunds.
- Reconciliation.
- Financial reports.
- Outstanding balances.
- Dealership invoices.

### Customer

Access only to:

- Own profile.
- Own vehicle records.
- Own bookings.
- Rescheduling and cancellation where permitted.
- Own invoices and payment records.

### Security requirement

Every API request must validate:

- User identity.
- Required permission.
- Authorized franchise.
- Authorized location.
- Resource ownership or approved scope.

Do not trust franchise or location identifiers supplied by the browser without server-side membership validation.

Add tests proving a user from Franchise A cannot retrieve or modify records belonging exclusively to Franchise B.

## 5. DASHBOARD NAVIGATION

### Central Admin Dashboard

Main sidebar:

- Overview
- Calendar
- Bookings
- Customers
- Vehicles
- Services & Packages
- Quotes
- Franchises
- Locations
- Employees
- Job Management
- Dealerships & Fleet
- Invoices
- Payments
- Reports
- Website Content
- Marketing & Popups
- Notifications
- Integrations
- Audit Logs
- Settings

Navigation items and data must be permission-filtered.

### Franchise Dashboard

- Overview
- Bookings
- Calendar
- Locations
- Employees
- Customers
- Jobs
- Services
- Quotes
- Invoices
- Payments
- Reports
- Settings

### Employee Portal

Mobile-first navigation:

- My Dashboard
- Today's Jobs
- My Schedule
- Clock In / Out
- Attendance
- Training
- Assessments
- Profile

### Customer Portal

- Overview
- My Bookings
- My Vehicles
- Book Again
- Invoices
- Profile

Do not create separate applications for every role unless a clear technical requirement emerges. Prefer shared components and role-aware layouts.

## 6. MODULE A — ADMIN OVERVIEW

The main dashboard should provide:

- Today's bookings.
- Upcoming bookings.
- Pending booking approvals.
- In-progress jobs.
- Completed jobs.
- Pending quotations.
- Outstanding invoices.
- Payment summaries.
- Franchise/location performance.
- Employee assignments.
- Dealership activity.

Required UI:

- KPI cards.
- Date-range selector.
- Franchise selector.
- Location selector.
- Revenue chart.
- Booking-status distribution.
- Appointment calendar preview.
- Recent activity.
- Operational alerts.
- Quick actions.

Quick actions:

- Create Booking
- Add Customer
- Create Quote
- Create Invoice
- Assign Employee

KPIs must be calculated from real accessible records, not frontend placeholder values.

## 7. MODULE B — FRANCHISE MANAGEMENT

Model the business hierarchy:

Dapper Central Administration → Franchise → Location → Employees and resources.

Franchise data:

- Franchise ID.
- Name.
- Status.
- Authorized administrator accounts.
- Business/contact information.
- Assigned locations.
- Permitted services.
- Pricing permissions.
- Financial configuration.
- Square merchant configuration reference.
- Notification routing preferences.

Location data:

- Location ID.
- Franchise ID.
- Name.
- Address.
- Canadian province and timezone.
- Contact details.
- Opening hours.
- Holidays.
- Blocked dates.
- Booking capacity.
- Services available.
- Assigned employees.
- Operational status.

Features:

- Create/edit/deactivate franchise.
- Create/edit/deactivate location.
- Assign users and employees.
- Configure services.
- Configure availability.
- View scoped reports.
- Manage permitted booking settings.

Whether a franchise can independently edit pricing, promotions, customer data and financial settings is pending client approval.

## 8. MODULE C — CUSTOMERS AND VEHICLES

Customer records should support:

- Customer ID.
- Name.
- Email.
- Phone.
- Contact preferences.
- Relevant address details.
- Booking history.
- Invoice history.
- Associated vehicles.
- Marketing consent as a separate field.
- Created/updated timestamps.

Vehicle records should support:

- Vehicle ID.
- Customer ID.
- Vehicle category.
- Make.
- Model.
- Year.
- Optional registration identifier.
- Optional color and notes.
- Other category-specific information.

Vehicle categories must be configurable.

Initial supported categories from the SOW:

- Motorcycle.
- Hatchback.
- Sedan.
- SUV.
- Pickup truck.
- Van.
- Other passenger vehicle.
- Commercial truck.
- Heavy-duty vehicle.
- Heavy machinery.
- Aircraft.
- Boat/marine vessel.

Do not force aircraft, boats, and heavy equipment into automobile-only registration fields.

## 9. MODULE D — SERVICES AND PRICING

Implement an administrator-configurable service catalogue.

Core service families:

- Interior detailing.
- Exterior detailing.
- Full detailing.
- Ceramic coating.
- Paint Protection Film.
- Paint correction.
- Polishing and restoration.
- Fleet and commercial detailing.
- Dealership detailing.
- Aircraft detailing.
- Marine detailing.
- Heavy machinery detailing.

Required capabilities:

- Service categories.
- Services.
- Packages.
- Add-ons.
- Vehicle-specific pricing.
- Location-specific availability.
- Authorized franchise-specific pricing.
- Service durations.
- Buffer times.
- Fixed prices.
- Custom quotations.
- Deposit requirements.
- Promotional discounts.
- Applicable Canadian taxes.

Pricing must be computed by the Go backend.

Do not embed authoritative prices in the Next.js UI.

Use integer minor currency units or an exact decimal money representation. Do not calculate payment totals using unbounded floating-point arithmetic.

All existing website pricing is reference data requiring client approval before production use.

## 10. MODULE E — BOOKING ENGINE

The booking engine is business-critical.

Customer booking flow:

1. Choose service location.
2. Select vehicle category or existing vehicle.
3. Select detailing service/package.
4. Select eligible add-ons.
5. View total or request quotation.
6. Select available appointment.
7. Enter customer details.
8. Complete required deposit/full payment.
9. Receive confirmation.
10. View, modify, or cancel when permitted.

Internal booking flow:

- Search or create customer.
- Associate vehicle.
- Select service.
- Set location.
- Choose appointment.
- Assign employee.
- Record notes.
- Set approved booking status.
- Record applicable payment information.

Must support walk-in, telephone, web, dealership, and commercial bookings.

Required SOW booking statuses:

- Pending.
- Awaiting Payment.
- Confirmed.
- Scheduled.
- In Progress.
- Completed.
- Cancelled.
- Rescheduled.
- No-Show.
- Awaiting Quote or Approval.

Create an explicit state-transition table for client approval.

Scheduling must account for:

- Location operating hours.
- Employee working hours.
- Service duration.
- Booking capacity.
- Blocked dates.
- Buffer times.
- Holidays.
- Approved rescheduling windows.
- Concurrent booking attempts.

**Never allow two overlapping appointments to consume the same exclusive employee/resource capacity.**

### Booking engine implementation decision

Evaluate the public GitHub project:

https://github.com/distronode-corporation/district-scheduler

This is a candidate, not yet an approved dependency.

Before adopting it:

1. Verify its license and dependency compatibility.
2. Pin a reviewed commit.
3. Inspect scheduling APIs.
4. Run tests.
5. Validate booking concurrency.
6. Evaluate multi-tenant security.
7. Validate franchise-to-location mapping.
8. Check calendar integration requirements.
9. Confirm it works with the chosen PostgreSQL configuration.
10. Produce an integration feasibility report.

If adopted, the scheduler must be authoritative for bookings and availability.

The Dapper business API may keep a synchronized booking projection for reporting and business references, but must not create a second independently writable booking system.

If the candidate cannot meet the requirements without excessive adaptation, propose a focused Go scheduling implementation instead. Document the trade-offs before implementation.

## 11. MODULE F — EMPLOYEE MANAGEMENT

Employee records:

- Employee ID.
- User account.
- Name/contact details.
- Franchise memberships.
- Location assignments.
- Authorized roles.
- Employment status.
- Training status.
- Onboarding completion.

Onboarding:

- Account activation.
- Required information.
- Documentation.
- Training materials.
- Assessments and tests.
- Completion tracking.

Attendance:

- Clock in.
- Clock out.
- Breaks where required.
- Time entry history.
- Timesheet review.
- Timesheet approval.
- Overtime calculation where applicable.
- Exportable reports.

GPS/location tracking:

- Authorized session start/end.
- Appropriate employee notice and consent handling.
- Related job/activity reference.
- Retention policies.
- Restricted visibility.
- Auditable access.

Do not assume continuous background GPS tracking is technically possible or legally approved.

Implement only the tracking model validated for the agreed deployment devices, browser permissions, and applicable Canadian privacy/employment requirements.

## 12. MODULE G — JOB MANAGEMENT

Separate the appointment/booking record from the actual detailing work performed.

A job should reference:

- Booking.
- Customer.
- Vehicle.
- Location.
- Services.
- Assigned employee(s).
- Instructions.
- Status.
- Start/end timestamps.
- Notes.
- Completion records.

Core workflow:

Booking Confirmed → Job Assigned → Job Started → Job Completed → Charges Verified → Invoice Finalized.

The actual job status names and allowed transitions should be finalized during discovery.

Employee screens:

- Today's jobs.
- Upcoming jobs.
- Job detail view.
- Start work.
- Update status.
- Add notes.
- Mark completion.

Administrative screens:

- Job queue.
- Employee workload.
- Assignment management.
- Completion verification.
- Outstanding jobs.
- Job history.

## 13. MODULE H — DEALERSHIP AND B2B OPERATIONS

Support dealership and commercial fleet customers.

Required features:

- Dealership/company records.
- Account identifiers/tags.
- Business contact details.
- Fleet/vehicle records.
- Bulk job creation.
- Employee assignments.
- Job progress.
- Service history.
- Dealership invoices.
- Outstanding balances.
- Reporting by account and service period.

Ensure B2B customers and ordinary consumer customers can share common customer/contact infrastructure without forcing identical workflows.

## 14. MODULE I — SQUARE PAYMENTS

Use Square's supported integration methods.

Required workflows:

- Online deposit payment.
- Full prepayment.
- Supported in-person POS workflow.
- Payment-status reconciliation.
- Authorized refunds.
- Payment reference storage.
- Failed payment handling.

Cash payments:

- Recorded by authorized staff.
- Associated with booking/invoice.
- Amount and currency.
- Timestamp.
- Recording employee.
- Notes/reference.
- Audit history.

Security requirements:

- Never store raw card information.
- Validate Square webhook authenticity.
- Process events idempotently.
- Protect against duplicate payment processing.
- Do not mark payment as successful before verified confirmation.
- Make refunds permission-restricted and auditable.

Square account structure, franchise merchant allocation, terminal hardware, and POS integration capabilities must be confirmed with the client.

Use the Square sandbox for the initial demonstration.

If access is unavailable, provide clearly labeled integration-unavailable or simulated demonstration states. Never depict a simulated payment as a successfully processed real transaction.

## 15. MODULE J — CUSTOM INVOICING

Invoice fields:

- Invoice ID.
- Unique invoice number.
- Franchise/legal business identity.
- Location.
- Customer.
- Associated booking or B2B account.
- Relevant vehicle.
- Itemized services.
- Add-ons.
- Discounts.
- Applicable taxes.
- Total.
- Paid amount.
- Outstanding amount.
- Due date.
- Invoice status.
- Notes.
- Audit history.

Functionality:

- Draft.
- Review.
- Finalize.
- Generate PDF.
- Email delivery.
- Record partial/full payment.
- Outstanding balance.
- Approved correction/refund/credit workflow.
- Search/filter.
- Download.
- Invoice history.
- Dealership billing.

Make a clear distinction among invoices, paid invoices, and payment receipts.

Invoice numbering, Canadian tax treatment and legal issuer identity must follow the approved franchise billing model.

## 16. MODULE K — CALENDAR AND NOTIFICATIONS

Calendar integration must support:

- Company calendar.
- Franchise calendar.
- Location calendar.
- Employee calendar.
- Booking creation.
- Rescheduling.
- Cancellation.
- Relevant event updates.
- Duplicate-event prevention.

Investigate the supported Apple Calendar-compatible approach.

A standards-based feed or CalDAV strategy may be considered, but direct bidirectional calendar editing must not be promised without technical validation and client approval.

Transactional email events:

- Booking confirmation.
- Rescheduling.
- Cancellation.
- Appointment reminder.
- Payment confirmation.
- Invoice delivery.
- Job assignment.
- Operational exceptions.
- Quote request.
- Service completion.

Implement:

- Configurable templates.
- Recipient routing.
- Scheduled reminders.
- Retry handling.
- Delivery state recording.
- Idempotent event processing.

Future SMS should remain an extensibility point and is not required for initial release.

## 17. MODULE L — MARKETING AND CONTENT

Mailchimp integration:

- Configurable audience.
- Field mapping.
- Duplicate subscription handling.
- Marketing consent.
- Subscription status.
- Error handling.

Popup builder:

- Form title.
- Description.
- Fields.
- Required/optional settings.
- Audience mapping.
- Enable/disable.
- Display delay.
- Selected pages.
- Repeat-display limits.
- Submission feedback.
- Submission tracking.

Booking and contact requests must not automatically grant marketing consent.

Website CMS:

- Home page content.
- Service descriptions.
- Vehicle content.
- Images/media.
- FAQs.
- Franchise/location pages.
- SEO metadata.
- Promotions.
- Forms.

The specific CMS approach is pending approval. Do not introduce WordPress again without an explicit decision.

## 18. REPORTING

Implement permission-aware reporting for:

- Bookings by day/week/month.
- Booking statuses.
- Revenue by service.
- Revenue by location.
- Revenue by franchise.
- Payment methods.
- Outstanding invoices.
- Employee working hours.
- Completed jobs.
- Dealership activity.
- Fleet activity.
- Customer booking history.

All reports must enforce the same data-access rules as the underlying records.

Detailed payroll, advanced business intelligence and external accounting integrations are excluded unless separately approved.

## 19. PROPOSED DATA MODEL

Produce an ER diagram and PostgreSQL schema covering at least:

**Identity and organization**

- users
- roles
- permissions
- role_permissions
- franchises
- locations
- user_franchise_memberships
- user_location_memberships

**Customers and catalogue**

- customers
- customer_addresses
- vehicles
- vehicle_categories
- service_categories
- services
- service_packages
- package_items
- service_addons
- pricing_rules
- location_services
- discount_rules

**Booking and operations**

- bookings or booking_projections
- booking_status_events
- booking_assignments
- location_business_hours
- employee_availability
- blocked_periods
- jobs
- job_status_events
- job_notes

**Employees**

- employees
- employee_assignments
- onboarding_records
- training_materials
- assessments
- assessment_attempts
- time_entries
- timesheets
- attendance_adjustments
- gps_sessions
- gps_observations

**B2B**

- business_accounts
- dealership_vehicle_records
- business_jobs

**Finance**

- payments
- payment_events
- cash_payment_records
- refunds
- invoices
- invoice_lines
- invoice_adjustments
- tax_rules
- invoice_sequences

**Platform**

- notification_templates
- notification_events
- calendar_connections
- calendar_event_links
- mailchimp_connections
- popup_forms
- popup_submissions
- audit_logs
- migration_batches
- migration_reconciliation
- integration_events
- outbox_events
- integration_failures

These are proposed conceptual entities. Normalize and refine them before generating migrations.

If District Scheduler is adopted, its canonical tables should stay under its ownership. Avoid copying the entire scheduler schema into the Dapper business database.

## 20. REST API DESIGN

Use a versioned API:

`/api/v1`

Create OpenAPI specifications before developing frontend integrations.

Representative endpoint groups:

**Authentication**

- `POST /auth/login`
- `POST /auth/logout`
- `POST /auth/refresh`
- `GET /auth/me`

**Franchises and locations**

- `GET /franchises`
- `POST /franchises`
- `GET /franchises/{id}`
- `PATCH /franchises/{id}`
- `GET /locations`
- `POST /locations`
- `PATCH /locations/{id}`

**Customers and vehicles**

- `GET /customers`
- `POST /customers`
- `GET /customers/{id}`
- `PATCH /customers/{id}`
- `GET /customers/{id}/vehicles`
- `POST /customers/{id}/vehicles`

**Services**

- `GET /services`
- `POST /services`
- `PATCH /services/{id}`
- `GET /pricing/quote`

**Booking**

- `GET /availability`
- `POST /bookings`
- `GET /bookings`
- `GET /bookings/{id}`
- `POST /bookings/{id}/reschedule`
- `POST /bookings/{id}/cancel`
- `POST /bookings/{id}/assign`

**Operations**

- `GET /employees`
- `POST /employees`
- `GET /jobs`
- `POST /jobs/{id}/start`
- `POST /jobs/{id}/complete`
- `POST /attendance/clock-in`
- `POST /attendance/clock-out`
- `GET /timesheets`

**Finance**

- `GET /invoices`
- `POST /invoices`
- `POST /invoices/{id}/finalize`
- `GET /invoices/{id}/pdf`
- `POST /payments/checkout`
- `POST /payments/cash`
- `POST /payments/{id}/refund`

**Reporting**

- `GET /reports/overview`
- `GET /reports/bookings`
- `GET /reports/revenue`
- `GET /reports/employees`
- `GET /reports/dealerships`

These routes are proposals, not final contracts. Verify their methods, validation rules, authorization requirements and ownership boundaries during architecture design.

For each endpoint document:

- Request schema.
- Response schema.
- Error responses.
- Permission requirements.
- Tenant/location scope.
- Idempotency rules when applicable.
- Pagination/filtering.
- Audit requirements.

## 21. SECURITY REQUIREMENTS

Security is a release-blocking requirement.

Implement:

- Secure authentication and session handling.
- HTTP-only secure cookies where appropriate.
- CSRF protection for cookie-authenticated mutations.
- Backend-enforced RBAC.
- Franchise/location isolation.
- Object-level access checks.
- PostgreSQL row-level security where appropriate.
- Parameterized queries.
- Input validation.
- Output encoding.
- Rate limiting.
- Secure file-upload validation.
- Encryption in transit.
- Encrypted sensitive data where justified.
- Secret management.
- MFA capability for administrators.
- Secure backups.
- Audit logs.
- Integration webhook signature verification.
- Restricted infrastructure access.

Never store API secrets in frontend bundles, committed `.env` files, or public GitHub repositories.

Avoid placing long-lived authentication credentials in browser localStorage.

Ensure audit records capture actor, action, affected record, timestamp and relevant authorized scope.

## 22. DEPLOYMENT ARCHITECTURE

Environment:

Dedicated Ubuntu 26.04 virtual machine.

Use Docker Compose for initial development.

Proposed containers:

- Next.js frontend.
- Go business API.
- Go scheduling service if adopted.
- PostgreSQL.
- Background worker.
- Reverse proxy.
- Optional queue if justified.

Use separate development and staging configurations.

Do not publicly expose PostgreSQL.

Bind development services to private interfaces or loopback and use an approved secure access method for client demonstrations.

Create:

- `.env.example`
- `compose.yaml`
- `compose.staging.yaml`
- Database migrations.
- Health checks.
- Logging configuration.
- Backup scripts.
- Restore verification procedure.
- Deployment documentation.

Pin dependency versions and review external code before adoption.

## 23. CODEBASE STRUCTURE

Preferred monorepo:

dapper-platform/

- `apps/web/` — Next.js frontend
- `apps/api/` — Go business API
- `services/scheduler/` — scheduling integration or service
- `services/worker/` — asynchronous processing
- `packages/contracts/` — OpenAPI and generated types
- `infra/docker/` — container configuration
- `infra/database/` — migration and seed infrastructure
- `tests/e2e/` — end-to-end tests
- `tests/integration/` — integration tests
- `docs/` — requirements, architecture and decisions
- `.agents/skills/` — Codex development skills
- `AGENTS.md` — project-wide engineering rules

Avoid mixing external open-source repositories directly into application directories without deciding whether they will be used as separate services, forks, or vendored code.

## 24. DEMO DATA

Generate a clearly labeled synthetic staging dataset.

Include:

- 1 central administrator.
- 2 sample franchises.
- 2–3 sample locations.
- 2 franchise administrators.
- 2 location managers.
- 5–8 detailers.
- 1 finance user.
- 15–25 sample customers.
- Multiple customer vehicle categories.
- 10–15 services/packages.
- 30–50 sample bookings.
- Several in-progress jobs.
- Completed and cancelled bookings.
- Sample invoices and payments.
- A sample dealership account.
- Example employee attendance records.

All generated data must be marked demo-only.

Do not use actual customer, employee, vehicle-registration, payment, or GPS data in demonstrations without appropriate authorization.

## 25. REQUIRED TEST SCENARIOS

### Authentication and permissions

- Unauthenticated access rejected.
- Unauthorized role access rejected.
- Cross-franchise record access rejected.
- Cross-location access rejected.
- Admin privileges cannot be self-assigned.
- Audit events created for sensitive actions.

### Booking

- Successful booking.
- Unavailable slot rejected.
- Simultaneous requests cannot double-book exclusive capacity.
- Duration and buffers honored.
- Holiday closure honored.
- Rescheduling updates availability.
- Cancellation releases capacity when appropriate.
- Quote-only services cannot bypass approval.

### Payments

- Valid sandbox payment.
- Failed payment.
- Duplicate webhook delivery.
- Payment retry.
- Refund permissions.
- Cash payment audit.
- Booking and invoice reconciliation.

### Employees

- Employee sees only authorized assignments.
- Clock-in/out timestamps recorded.
- Invalid attendance actions rejected.
- Timesheets aggregate correctly.
- GPS tracking requires an authorized session.

### Invoices

- Invoice calculation.
- Tax calculation using approved rules.
- PDF creation.
- Partial payments.
- Outstanding balance.
- Invoice history and permissions.

### Integration reliability

- Email provider unavailable.
- Calendar API unavailable.
- Mailchimp rejects submission.
- Square webhook arrives twice.
- Retry does not create duplicate records.
- Failed jobs remain observable and recoverable.

### Frontend

- Desktop navigation.
- Tablet layout.
- Mobile layout.
- Forms and validation.
- Empty states.
- Loading/error states.
- Keyboard navigation.
- Screen-reader labels.
- Playwright end-to-end scenarios.

Tests must exercise the real authorization and database boundaries wherever possible, not merely mock a successful API response.

## 26. LEGACY DATA MIGRATION

Source systems:

- WordPress website.
- BookingPress Pro.
- Existing employee portal.

Required booking migration:

1. Inspect source database and exports.
2. Inventory available booking fields.
3. Establish field mapping.
4. Preserve original source identifiers.
5. Perform dry-run migration.
6. Reconcile record counts.
7. Verify future bookings individually or using agreed samples.
8. Identify incomplete records.
9. Produce reconciliation report.
10. Obtain cutover approval.

Never invent missing payment values, booking history, or employee records.

Other historical employee, timesheet, dealership, and invoice migration remains subject to separate discovery and approval.

## 27. SEVEN-DAY STAGING TARGET

### Day 1 — Architecture and foundation

Deliver:

- SOW requirement traceability.
- Open-source dependency audit.
- Architecture diagrams.
- Entity-relationship design.
- API contracts.
- Role and permission matrix.
- UI navigation skeleton.
- Working Docker development environment.

### Day 2 — Identity and organization

Deliver:

- Authentication.
- User roles.
- Franchise/location management.
- Server-side authorization.
- Tenant-isolation tests.
- Dashboard shell.

### Day 3 — Customers, services and bookings

Deliver:

- Customer records.
- Vehicles.
- Service catalogue.
- Pricing rules.
- Booking integration.
- Availability.
- Booking list/calendar.

### Day 4 — Employee operations

Deliver:

- Employee profiles.
- Job assignments.
- Job detail screens.
- Attendance basics.
- Franchise/location dashboard views.

### Day 5 — Finance and integrations

Deliver where access and feasibility permit:

- Invoice creation/PDF.
- Cash payment recording.
- Square sandbox proof of integration.
- Transactional email proof of integration.
- Calendar synchronization proof of concept.

### Day 6 — QA and stabilization

Deliver:

- E2E booking tests.
- Permission tests.
- Database and API tests.
- Responsive testing.
- Integration failure tests.
- Resolved critical defects.

### Day 7 — Client staging demonstration

Demonstrate:

- Admin login.
- Franchise and location switching.
- Customer creation.
- Vehicle assignment.
- Service selection.
- Appointment booking.
- Calendar update.
- Employee job assignment.
- Job completion.
- Cash/payment status workflow.
- Invoice generation.
- Reporting.

Provide a feature-status matrix stating whether each workflow is implemented, integrated, simulated, blocked, or deferred.

**The seven-day milestone is a staging demonstration, not unconditional completion of the entire SOW.**

## 28. REQUIRED CODEX SKILLS

Create or reuse focused skills for:

### `dapper-go-backend`

Go architecture, service boundaries, API validation, error handling, transactions and database operations.

### `dapper-tenant-security`

Permissions, franchise isolation, location membership, RLS, audit trails and security testing.

### `dapper-booking-engine`

Scheduling, concurrent bookings, service durations, buffers, availability and booking state transitions.

### `dapper-dashboard-ui`

Next.js dashboard architecture, responsive layouts, shadcn/ui, data tables, forms, accessibility and client brand tokens.

### `dapper-finance-integrations`

Square sandbox, payment reconciliation, invoicing, cash records, idempotency and integration boundaries.

### `dapper-employee-operations`

Employee assignments, job management, attendance, onboarding and approved location tracking.

### `dapper-quality-assurance`

Go tests, TypeScript tests, Playwright, CI, performance and security checks.

### `dapper-data-migration`

BookingPress assessment, source mapping, repeatable imports, reconciliation and cutover safeguards.

Skills must not override security requirements or the approved SOW.

## 29. DOCUMENTATION REQUIRED

Generate:

- `docs/MASTER_PLAN.md`
- `docs/REQUIREMENTS_MATRIX.md`
- `docs/ARCHITECTURE.md`
- `docs/DATABASE_SCHEMA.md`
- `docs/API_SPECIFICATION.md`
- `docs/RBAC_MATRIX.md`
- `docs/BOOKING_WORKFLOWS.md`
- `docs/PAYMENT_WORKFLOWS.md`
- `docs/INTEGRATION_PLAN.md`
- `docs/MIGRATION_PLAN.md`
- `docs/DASHBOARD_UIUX.md`
- `docs/TEST_PLAN.md`
- `docs/SECURITY_MODEL.md`
- `docs/OPEN_DECISIONS.md`
- `docs/SPRINT_BACKLOG.md`
- `docs/DEMO_SCRIPT.md`
- `docs/DEPLOYMENT.md`

## 30. OPEN CLIENT DECISIONS

Explicitly track and request approval for:

1. Franchise pricing authority.
2. Franchise access to customer and financial records.
3. Square merchant/account structure.
4. Exact Square POS application and hardware.
5. Invoice issuing entity and numbering model.
6. Applicable tax handling.
7. Customer guest checkout.
8. Booking deposits and cancellation rules.
9. Service price and duration catalogue.
10. Calendar synchronization expectations.
11. Employee GPS tracking and retention policy.
12. Existing employee portal migration scope.
13. Website CMS approach.
14. Production infrastructure and client access.
15. Approved credentials and integration accounts.

Do not invent answers to these questions.

## 31. CODING RULES

- Follow approved architecture documents.
- Use incremental, testable changes.
- Create focused pull requests.
- Keep backend logic out of frontend components.
- Avoid duplicating scheduler logic.
- Do not bypass authorization for speed.
- Never hardcode integration secrets.
- Maintain database migration compatibility.
- Run relevant automated tests after changes.
- Do not report a feature complete merely because its UI renders.
- Document blockers immediately.
- Do not modify the public cinematic landing-page implementation unless assigned.

## 32. FIRST TASK FOR ASTRA

Read the SOW and inspect the repository.

Before starting implementation:

1. Produce the SOW requirement traceability matrix.
2. Confirm architecture and module boundaries.
3. Inspect the candidate booking engine.
4. Build the permission matrix.
5. Create the initial database schema proposal.
6. Create the API specification.
7. Identify client dependencies.
8. Produce a realistic seven-day sprint backlog.
9. Identify features not achievable within the initial staging deadline.
10. Write precise, independently testable implementation tickets for Sol 6.1.

Every ticket must contain:

- Objective.
- SOW reference.
- Files/modules affected.
- Required API/database changes.
- Dependencies.
- Security considerations.
- Acceptance criteria.
- Required tests.
- Definition of Done.

**Do not immediately implement the entire platform in one unreviewed change.**

Complete architecture and implementation planning first, then proceed with the approved sprint tickets.

---

**END OF MASTER SPECIFICATION**
