# Production server (2026-10-30)

User direction (2026-10-30): host the full system on the user's server and apply every later change to both this machine and the server. The user chose to run on the bare IP for now (no domain yet).

## Setup (evidence: runtime on the server)
- Host: Ubuntu 24.04.5 LTS VPS, 4 vCPU, 15 GB RAM, 193 GB disk, 201.18.205.129. Access by SSH key (alias `dapper-prod`); the user added the key and set every password themselves. No credentials are recorded here.
- Installed: PostgreSQL 16.15, Go 1.26.4, Node 24.16.0, Caddy 2.11.7; ufw enabled (SSH, 80, 443 only).
- App: /opt/dapper/app, run as system user `dapper` by `dapper.service` (the repo's own launcher; gateway on 127.0.0.1:8443 in reverse-proxy mode). Caddy serves https://201.18.205.129 with its own certificate for the IP (browsers warn until a domain points here).
- Setup: the user completed the installer wizard through an SSH tunnel. Afterwards the site address was corrected from the wizard default `https://localhost:8443` to `https://201.18.205.129` and the timezone from the browser's `Asia/Calcutta` to `America/Toronto` (previous `.env` kept as `.dapper/env.before-site-fix`).
- Deploys: `scripts/deploy.sh` (working tree minus this vault, reference material, ERPNext/Frappe/HRMS copies and the unlicensed Mercedes model; rebuild, migrate, restart).
- Backups: nightly pg_dump + `.env` copy to /opt/dapper/backups, 14 days, same disk only.

## Checked
- All public pages, /studio, /dashboard, /workspace, /account, /booking answer 200 over HTTPS from outside; HTTP redirects to HTTPS.
- Not checked: signing in, booking, uploads, email (SMTP skipped), Square, performance under load, restore from backup, reboot survival.

## Open
- Domain and real certificate (dapperdetailing.ca stays on the legacy host until the user decides).
- Off-server backup copy; SMTP; turn off SSH password login and change the root password (it was shared in chat).

## Later on 2026-10-30
- At the user's request the owner account neonabyss01@gmail.com was given the Super Admin role (all 45 capabilities, organization scope) using the SQL of scripts/seed-owner-roles.ps1; recorded in role history.
- The dashboard looked blank because the fresh database had no franchise/location. Created franchise + location "Dapper Detailing Pros Peterborough" (code PTBO, 660 The Kingsway, Peterborough, ON K9J 7C8, Canada); no contact person (unknown).
- "Copy the local data for testing": server backed up first (/opt/dapper/backups/dapper-before-local-import-2026-10-30-1647.dump), then copied from the local development database with organization/location/franchise/actor IDs remapped to the server's: 312 central services (+ revisions), 310 location services (73 active main, 228 add-ons, 9 archived; + revisions), 2 scheduling resources, 73 scheduling rules (+ revisions), HST 13% tax rate (+ events). Every parent/central/resource link verified before commit. Not copied: local test customers, booking, invoice, dealership, SOP documents (files are local), accounts.
- The public website's service list, portfolio and instant quote still read separately published content and the online-booking setting; none of that existed locally either, so those public sections stay empty until published from the dashboard.
