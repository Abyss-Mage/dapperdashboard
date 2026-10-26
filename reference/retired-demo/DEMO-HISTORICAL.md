# Dapper local demo
Native Windows. Synthetic records only. ERPNext/HRMS imports are reference material, not running services.

From the project root:
1. Run go test ./... and go vet ./... inside dapper-platform/apps/api.
2. Build the API there: go build -o ../../../.cache/dapper-api.exe .
3. Run npm ci and npm run build inside dapper-platform/apps/web.
4. Run ./scripts/start-demo.ps1 from the project root.
5. Open http://127.0.0.1:3006/ . The API listens on 127.0.0.1:8120.
6. Stop with ./scripts/stop-demo.ps1. Only recorded PID/start-time/executable matches are stopped.

The launcher checks ports and readiness, uses hidden native processes, stores logs/process metadata under .cache/demo-runtime and enables DAPPER_DEMO=1 only for its child API. Do not expose this demo to a network. The shared demo cookie is not production authentication.

Customers and owned vehicles/equipment persist in apps/api/demo-store.json (ignored by Git). Do not enter real contacts, plates, VINs or other private records. The demo API supports customer and vehicle creation, strict input/ownership validation and duplicate detection. Booking mutations, payments and invoices remain gated.

Public website references are embedded in the API from the canonical vault capture. Run scripts/sync-reference-catalogue.ps1 after reviewing an updated capture, then rebuild the API. Published price labels are not category/package mappings or approved transaction values.

WordPress ZIP: output/wordpress/dapper-data-exporter-0.1.0.zip. See integrations/wordpress/dapper-data-exporter/readme.txt. It is read-only and requires an existing administrator. No live WordPress deployment has been verified.

Optional PostgreSQL foundation: scripts/start-postgres.ps1 starts a project-local native instance on loopback port 15432 after unpacking the official Windows archive. scripts/test-postgres.ps1 runs the real scoped database tests, using an isolated temporary schema. scripts/stop-postgres.ps1 stops that project instance. Credentials stay Windows-user encrypted under ignored .cache. The HTTP demo still uses JSON; PostgreSQL does not establish production authentication or approved franchise policy.
