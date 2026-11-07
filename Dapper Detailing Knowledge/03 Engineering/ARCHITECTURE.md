# Architecture
Status: ADR-002 is the confirmed runtime direction. Native Windows only; no WSL.

Active application: Next.js/React in dapper-platform/apps/web and a unified Go API in apps/api. ERPNext, HRMS and Frappe are pinned reference sources, not running services or Go modules. Preserve provenance and licenses when adapting source or designs.

Browser -> Next.js on 127.0.0.1:3006 -> Go API on 127.0.0.1:8120 -> synthetic local JSON.

Current slice: synthetic workspace, customer creation, field validation, duplicate checks and persistence. DAPPER_DEMO=1 and a random HttpOnly SameSite cookie gate the local demo. This cookie is not production identity or authorization. Writes require the exact frontend Origin. Port 3006 avoids an unrelated project using 3000.

Proposed production foundation: PostgreSQL, authenticated identities, scoped organizations, audit events and a durable provider outbox. These remain to implement and verify. Company/franchise/legal issuer mappings remain pending client approval. Neither a location selector nor hidden navigation grants authorization.

WordPress integration exports published content/media and allowlisted BookingPress catalogue fields. It does not migrate customers, bookings or payments. Public website data is reference material, not approved transaction policy. No live installation has occurred.

Scripts under scripts launch/stop the native synthetic runtime and record process identity. Logs and demo storage are Git-ignored. No system services, WSL or legacy modifications are required.
