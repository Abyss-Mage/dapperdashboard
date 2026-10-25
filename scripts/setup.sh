#!/usr/bin/env sh
# First-run setup for a fresh checkout: builds the web app if needed, then opens the browser wizard (database, site, email, owner).
set -eu
root="$(cd "$(dirname "$0")/.." && pwd)"
for tool in go node npm; do command -v "$tool" >/dev/null 2>&1 || { echo "$tool is required and was not found on PATH." >&2; exit 1; }; done
if [ ! -f "$root/dapper-platform/apps/web/.next/BUILD_ID" ]; then
  (cd "$root/dapper-platform/apps/web" && npm ci && npm run build)
fi
cd "$root/dapper-platform/apps/api" && exec go run ./cmd/dapper setup --home "$root"
