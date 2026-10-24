$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/workspace-runtime'
$settings=Get-Content -LiteralPath (Join-Path $runtimeRoot 'settings.json') -Raw|ConvertFrom-Json
if($settings.mode -ne 'synthetic-development-only' -or $settings.schema -notmatch '^dapper_workspace_demo_[a-z0-9_]{1,40}$'){throw 'Only the dedicated synthetic workspace can be updated.'}
Push-Location (Join-Path $projectRoot 'dapper-platform/apps/api')
try{
 & go.exe build -o (Join-Path $runtimeRoot 'workspace-next.exe') ./cmd/workspace
 if($LASTEXITCODE -ne 0){throw 'Staged workspace build failed.'}
 & go.exe build -o (Join-Path $runtimeRoot 'database-migrate.exe') ./cmd/database
 if($LASTEXITCODE -ne 0){throw 'Migration tool build failed.'}
}finally{Pop-Location}
& (Join-Path $PSScriptRoot 'test-workspace-restore.ps1')
& (Join-Path $PSScriptRoot 'stop-workspace.ps1')
$previousDatabase=$env:DAPPER_DATABASE_URL
$credential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
try{
 $password=[Uri]::EscapeDataString($credential.GetNetworkCredential().Password)
 $env:DAPPER_DATABASE_URL="postgres://dapper_local:${password}@127.0.0.1:15432/postgres?sslmode=disable&search_path=$($settings.schema)"
 & (Join-Path $runtimeRoot 'database-migrate.exe') -migrate
 if($LASTEXITCODE -ne 0){throw 'Explicit migration failed; inspect compatibility before restarting.'}
}finally{$env:DAPPER_DATABASE_URL=$previousDatabase;$password=$null;$credential=$null}
Copy-Item -LiteralPath (Join-Path $runtimeRoot 'workspace.exe') -Destination (Join-Path $runtimeRoot 'workspace-previous.exe') -Force
Move-Item -LiteralPath (Join-Path $runtimeRoot 'workspace-next.exe') -Destination (Join-Path $runtimeRoot 'workspace.exe') -Force
& (Join-Path $PSScriptRoot 'start-workspace.ps1')
Write-Output 'Synthetic workspace updated after a verified encrypted backup. Prior executable retained; no schema downgrade was attempted.'
