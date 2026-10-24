---
name: dapper-dashboard-ui
description: Build or fix Dapper Next.js dashboard workflows and verify responsive behavior against the running Go API.
---

# Dapper dashboard UI
Work in dapper-platform/apps/web. Read canonical DASHBOARD_UIUX.md and current API_SPECIFICATION.md. Preserve existing Dapper branding and public cinematic work; do not replace the product with a generic starter or force a new component stack.

The current UI uses custom CSS, local Manrope and Lucide. Calendar is an hourly/team schedule; Bookings is a search table. Source catalogue review preserves original amounts/durations and disables transactions. Keep unfinished modules and synthetic records honestly labeled.

Preserve entered values on validation/network errors. Focus invalid fields after disabled-submit state ends, keep modal Tab/Escape/return-focus behavior and associated errors, and verify navigation on phones.

Run the production build/typecheck, then inspect the actual browser flow with the environment's documented UI automation tool. Test the API-backed action and resulting persisted state, not just appearance. Distinguish app failures from unrelated extension noise.

Port 3006 is Dapper; port 3000 belongs to another project. Start/stop only recorded Dapper processes, leave the verified demo running for review, and save screenshot evidence without credentials or private records. No WSL.
