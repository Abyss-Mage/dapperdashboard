# Deployment and runtime
Current runtime is a Windows-native local demo. No WSL, container setup, system service or public exposure is required or authorized by the local demonstration.

Runtime instructions: dapper-platform/DEMO.md. Build Go from apps/api and Next.js from apps/web, then scripts/start-demo.ps1. It starts hidden loopback processes on ports 8120/3006, verifies readiness and records process identity. scripts/stop-demo.ps1 stops only matching PID/start-time/executable records. Port 3000 belongs to an unrelated project and is preserved.

Native PostgreSQL is isolated under ignored .cache/postgresql, listening on 127.0.0.1:15432 with SCRAM authentication. scripts/start-postgres.ps1 and stop-postgres.ps1 operate only that data directory. Credentials are Windows-user encrypted, never copied into source/Obsidian. scripts/test-postgres.ps1 configures its process environment privately and removes isolated test schemas.

cmd/database explicitly applies reviewed checksummed migrations with -migrate and DAPPER_DATABASE_URL. cmd/workspace is an opt-in separate loopback API on 8121, requiring DAPPER_WORKSPACE=1 and an existing configured database. It does not create accounts, issue sessions, seed grants or auto-migrate. It is not connected to the frontend; identity provider configuration remains absent.

Production deployment is blocked on approved infrastructure, TLS/domain, real identity/MFA/session flow, role/scope policy, least-privilege database roles, backups and restoration, security/acceptance checks, migration approval and provider configuration. No production launch or remote-access claim. The eventual hosting environment is a client decision; do not use WSL to simulate it.

WordPress exporter was installed/updated by the user and its read-only permitted exports were verified. Do not automatically update other live plugins, alter site settings or retire the old system. Version metadata cleanup is separate from successful export behavior.

## Current native individual-session preview — 2026-10-22
The older paragraph describing absent login/frontend integration is superseded for the synthetic development preview. cmd/workspace now validates ten existing migration checksums at startup and exposes configured password login plus scoped job/catalogue/structure routes. It still never auto-migrates or auto-provisions real accounts.

Explicit setup-workspace.ps1 creates a fresh dedicated synthetic schema, Windows-encrypted test credential, local certificate and native tools. start-workspace.ps1 starts scoped API8121 and TLS gateway8443; stop-workspace.ps1 checks PID/start/executable identity. Next.js /workspace connects these verified-session routes. The gateway binds loopback only, preserves secure cookies, routes APIs to the scoped backend and never installs certificate trust. HTTP credential inputs are disabled.

Actual HTTPS acceptance passed using certificate/hostname validation and in-memory credentials/cookies. Browser hit the expected untrusted local certificate warning, left untouched for user review; authenticated browser acceptance remains pending. No trust-store change, security-warning bypass or real staff provisioning occurred. Full instructions: dapper-platform/SECURE_WORKSPACE.md.

A native DPAPI-encrypted synthetic schema backup was restored into a fresh disposable database and selected table counts matched. Test database/plaintext files were removed; encrypted backup remains ignored. This does not qualify production/off-site recovery, shared-key restoration, retention, monitoring or remote deployment.
