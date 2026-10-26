# Retired demo implementation

Archived under ADR-005 on 2026-10-22. These files are historical engineering evidence, not active application modules or supported launchers. Script relative paths may describe the old layout. Do not run them against the application database or restore their shared sessions, JSON storage, fabricated dashboards or fixture bootstrap into the product.

The active application uses `scripts/start-application.ps1`, `apps/api/cmd/workspace`, PostgreSQL, and the authenticated Next.js workspace/customer routes. Isolated synthetic integration tests remain under the API test packages; they use disposable database schemas rather than product data.
