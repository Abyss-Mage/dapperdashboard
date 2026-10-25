#!/usr/bin/env sh
# Starts an installed Dapper (API, web app and HTTPS gateway) from .dapper/config.json. Run scripts/setup.sh first.
set -eu
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root/dapper-platform/apps/api" && exec go run ./cmd/dapper start --home "$root"
