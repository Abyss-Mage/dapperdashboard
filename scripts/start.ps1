# Starts an installed Dapper (API, web app and HTTPS gateway) from .dapper/config.json. Run scripts/setup.ps1 first.
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
Push-Location (Join-Path $root 'dapper-platform/apps/api'); try{ go run ./cmd/dapper start --home $root } finally { Pop-Location }
