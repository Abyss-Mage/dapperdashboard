=== Dapper Data Exporter ===
Contributors: dapper-platform
Requires at least: 6.2
Requires PHP: 8.0
Stable tag: 0.1.3
License: GPLv3 only (see LICENSE.txt)

Administrator-only, read-only exports for the Dapper migration discovery process.

== Changelog ==
0.1.3: Add an allowlisted extras catalogue resource with original add-on/parent IDs, decimal price strings, raw duration units and quantity bounds. Missing extras tables are reported unavailable; no customer or transaction records are exported.
0.1.2: Report the current exporter version in the manifest. Export fields and permissions are unchanged.
0.1.1: Fix authenticated GET requests without an Origin header being incorrectly rejected. Administrator/capability, REST nonce/application-password authentication and cross-origin rejection remain enforced.

== Installation ==
Upload dapper-data-exporter.zip in WordPress Plugins > Add New > Upload Plugin, then activate it. Installing on a live site is a separate client-controlled action; this package has not been deployed by Codex.

== Authentication ==
Use HTTPS and WordPress Application Password authentication for an existing administrator. Cookie authentication requires WordPress's normal REST nonce. The plugin creates no users, credentials or custom authentication bypasses. Do not paste credentials into source, chat logs or Obsidian. Revoke the temporary Application Password after export.

GET /wp-json/dapper-export/v1/manifest
GET /wp-json/dapper-export/v1/content?page=1&per_page=50
GET /wp-json/dapper-export/v1/media?page=1&per_page=50
GET /wp-json/dapper-export/v1/catalogue?page=1&per_page=50

Pagination is bounded to 100 items per resource. Media pagination counts scanned attachments; a page may have no eligible items even when later pages exist. Follow total_pages, not item count. Catalogue services and categories have their own total_pages values.

== Data boundaries ==
Published, non-password-protected posts/pages are exported as plain text. Images are limited to attachments with published non-password-protected post/page parents. Raw custom fields, Elementor configuration, plugin settings, users, bookings, customers, transactions, secrets and unassigned uploads are excluded. BookingPress services/categories use a fixed column allowlist and report unsupported schemas without returning database errors. Prices and durations are legacy reference values requiring client approval; currency is not inferred.

The plugin has no write routes, background crawler, remote destination, telemetry, activation migration or uninstall deletion. It does not send data to Codex or another server automatically. Protected data exports and booking migration need a separately approved scope.
