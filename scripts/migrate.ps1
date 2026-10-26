# Applies database changes after an update (git pull). Back up the database first. Uses the database named in .env.
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
Push-Location (Join-Path $root 'dapper-platform/apps/api'); try{ go run ./cmd/dapper migrate --home $root } finally { Pop-Location }
