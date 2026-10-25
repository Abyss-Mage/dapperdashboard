# Installer and dashboard routes (2026-10-26)

Owner direction: the website stays unconnected to the dashboard for now; the dashboards become two areas, `/dashboard` (main, owners/admins) and `/workspace` (employees); and the app installs like WordPress: pull the repository, run setup, answer database / SMTP / first user.

Built (source + static + runtime on this machine, not on another computer or OS):
- `dapper-platform/apps/api/internal/install` (config file, PostgreSQL probe/create, SMTP test, self-signed certificate, embedded browser wizard with one-time setup code, lock) and `cmd/dapper` (`setup`, `start`, `status`). Scripts: `scripts/setup.ps1|sh`, `scripts/start.ps1|sh`. Guide: `INSTALL.md`.
- Gateway: `DAPPER_LISTEN`, `DAPPER_PUBLIC_HOST`, `DAPPER_BEHIND_PROXY`; rewrites Host for the API hop so a non-loopback site address works.
- Web: `/dashboard` is the main dashboard; `/owner` and `/owners` redirect (308). `/workspace` is unchanged (employee). The Square OAuth return now lands on `/dashboard`.
- Evidence: unit tests for the install package and gateway; a real run against the local PostgreSQL (create database, migrate, owner, certificate, `dapper start`, `/dashboard` 200, `/workspace` 200, redirects, owner password sign-in 204, wrong Host 403) on a temporary database that was dropped afterwards.

Not done / open: a second operating system, an HTTPS certificate from a real CA, a reverse-proxy deployment, SMTP against a real mail server (the Send test is implemented but untried), upgrade/migration flow for later releases, the wizard on a phone. `DAPPER_TIMEZONE` and `DAPPER_BUSINESS_NAME` are written to the environment but nothing reads them yet. Existing dev scripts (`scripts/start-application.ps1`) are unchanged.
