# Local demo script
Use only fictional details. Start the app with dapper-platform/DEMO.md and open http://127.0.0.1:3006/.

1. Overview: confirm synthetic banner, four illustrative bookings and API-connected status. Do not interpret counts as live business activity.
2. Customers: Add customer with a unique fictional name/email at example.test. Save, reopen and try the same email; verify field-specific error and preserved values. Escape closes and returns focus. Reload/restart and confirm persistence.
3. Vehicles: choose that customer, enter a fictional vehicle or boat/equipment description and optional fictional identifier. Save and reload. A repeated owner identifier is rejected. Do not enter real VIN/registration/contact information.
4. Calendar: review hourly team columns, filter a detailer, select a card and close its details. Start times are illustrative; durations/availability and booking changes remain gated.
5. Bookings: verify the separate searchable table. Search by fictional customer/service/record ID. Calendar and list share source records, not duplicated scheduling engines.
6. Services & packages: view 79 service and 12 category references, select a category, search, change pages and open descriptions. Needs review only shows three one-minute source durations. Clear an empty filter result. Website references remains a separate source view.
7. Locations: confirm sourced business address/contact details. Review pending modules without presenting them as completed workflows.
8. Repeat key interactions at phone width. Do not submit payments, GPS, live notifications, legacy migration or booking mutations.

For backend evidence, run scripts/test-postgres.ps1: isolated real database tests cover scoped records, explicit grants, session expiry/revocation/deactivation and HTTP authorization boundaries. Tests use ephemeral schemas; there is no production login connected to this demo. Exporter live evidence and current limitations belong in IMPLEMENTATION_STATUS.
