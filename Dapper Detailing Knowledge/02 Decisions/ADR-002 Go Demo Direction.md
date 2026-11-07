# ADR-002 — Windows-native demo frontend and unified Go API
Date: 2026-10-21
Status: CONFIRMED direct user instruction; supersedes ADR-001 runtime choice.

The user instructed: "Dont use WSL" and "make the demo frontend start building the main unified go backend as discussed". Build Next.js/TypeScript in dapper-platform/apps/web and Go in apps/api. Keep the imported ERPNext/HRMS source unchanged as reference knowledge. Those modules cannot execute as Go code; porting/adapting Dapper behavior requires explicit implementation and tests. Preserve original source provenance and licenses.

First slice: a clearly labeled local-only synthetic demo with a live Go data API, dashboard, bookings/jobs, customer/vehicle views, scope filters and customer creation. No approved prices, taxes or payment success are invented. Policy-dependent mutations remain disabled. Demo storage is local JSON, explicitly not the proposed production PostgreSQL persistence. Authentication/production deployment are not complete.

The automatic continuation must never invoke WSL. Stop at or below 5% remaining in the five-hour quota, checkpoint here, then resume after replenishment. Automation id: continue-dapper-after-quota-reset; checks every 30 minutes and stays quiet on unchanged state.

Environment incident: the prior WSL package-install command was interrupted on user correction. It had downloaded packages and begun unpacking libatomic1/libjemalloc2. Completion/configuration state is unverified. Do not resume or repair WSL without explicit user authorization.
