# Base import evidence
Import completed on 2026-10-21. The root upstream-lock.json records archive SHA256 hashes, immutable commits, license files and verified file counts. Imported 5,454 ERPNext files, 1,664 HRMS files and 4,141 Frappe files: 11,259 total. Every file matched the Git blob SHA from GitHub's complete tree for its pinned commit; unsafe paths and symlinks were rejected. The source is unmodified.

Selected repositories and immutable commits:
- frappe/erpnext: 5af6561f5aa48104559692fb4fd82f2580e8d338 (v16.51.1)
- frappe/hrms: 2478afc7a8f4959d500f6d17522853f0d244e38f (v16.51.0)
- frappe/frappe: 4515014c87ece6cb20ee91d2cfb121c5cbd1ff4a (v16.52.0)

Imported source is for adaptation, not evidence that a site is installed. Preserve license files and source pins. Runtime/tests and transitive license review remain pending. The earlier SaaS starter is not part of the selected application.

The pinned ERPNext Appointment controller was inspected after import. It uses a configured appointment duration/agent count and an overlapping-row locking read. Do not infer Dapper's multi-resource concurrency guarantee from its comment: real concurrent creation/rescheduling/verification tests and resource adaptation remain required. Earlier develop-branch discovery is not silently credited to this release.

Knowledge checks: all wiki links in 15 authored notes resolve. Exact SOW/master copies have SHA256 hashes in 99 Sources/source-hashes.json. WSL prerequisite inspection found Ubuntu-24.04 and Python 3.12.3; selected Frappe requires Python 3.14. No running Dapper site is claimed.

