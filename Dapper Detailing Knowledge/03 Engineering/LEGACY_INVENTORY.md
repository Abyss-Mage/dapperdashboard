# Legacy inventory
## BookingPress
Supplied source: core, Pro, Apple Calendar, Square, invoice, tax, tip, conversion tracking. This establishes plugin structures and possible source fields. It does not supply actual records, deployed settings, merchant mappings or current licenses. Do not copy plugin code into ERPNext without a separate license decision.

No SQL dump, CSV/XLSX booking export or SQLite database was found outside bundled environments during the October 21 inventory. Preserve original source identifiers and reconcile real exports when supplied.

## Employee portal
Asset manifest selects static/js/main.4ec6aaf4.js. Its source map contains original-named React components and API call sites. Other bundle versions are retained but not assumed deployed.

Static frontend observations:
- Login, magic/invite password setup, forgot/reset password.
- Onboarding profile and emergency contacts, ID upload, policy signature, SOP completion, assessment.
- Clock-in/out and break actions with foreground geolocation.
- Dealership photo tagging and customer inspection photos, personal upload history.
- Schedule requests/cancellation and administrator approval/rejection.
- Staff invitation/status control, dealership management.
- Weekly report views/CSV/email, activity logs, live employee map and storage overview.

Supplied backend: Employee, Dealership, TimeLog and Onboarding SQLAlchemy models; main.py exposes only GET /health. Actual implementations of frontend endpoint contracts are unavailable.

Do not label frontend-recovered rules as operationally verified. Document/photo privacy, signed-policy versioning, assessment pass rules, schedule approvals and GPS policies require confirmation.

REA static bundle evidence: ev_18a08427506be4c8c37158fe8048d61ca457c618484e8d4ca2c5bd9d3cffe6f0. Analysis was inert and partial; no application execution or complete feature verification is claimed.

