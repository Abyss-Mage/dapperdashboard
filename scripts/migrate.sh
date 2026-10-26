#!/usr/bin/env sh
# Applies database changes after an update (git pull). Back up the database first. Uses the database named in .env.
set -eu
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root/dapper-platform/apps/api" && exec go run ./cmd/dapper migrate --home "$root"
