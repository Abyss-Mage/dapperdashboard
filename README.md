# Dapper Detailing Pros

**Install and run:** follow the full guide in [INSTALL.md](INSTALL.md). Short version: install Git, Go, Node and PostgreSQL, clone this repository, then run `powershell -File scripts/start.ps1` (Windows) or `sh scripts/start.sh` (macOS/Linux). Until a completed `.env` exists, the setup page opens first (database, site address and HTTPS, email, first owner), then Dapper starts.

# Dapper Detailing Pros

Open `Dapper Detailing Knowledge` as an Obsidian vault and start at [00 Home](<Dapper Detailing Knowledge/00 Home.md>).

The vault is canonical for requirements, decisions and delivery status. The active application uses Go/PostgreSQL in `dapper-platform/apps/api` and Next.js in `dapper-platform/apps/web`. [ADR-005](<Dapper Detailing Knowledge/02 Decisions/ADR-005 Production Application Direction.md>) records the current production application direction and website API integration requirement. ERPNext, HRMS, BookingPress and Employee Portal imports are reference material, not the runtime.

The application is under implementation; full production acceptance is unfinished. Historical demo code and test fixtures must not be exposed as real operations or connected to the application database.
