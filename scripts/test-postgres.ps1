param([string]$TestFilter)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$credentialPath = Join-Path $projectRoot '.cache/postgresql/local-credential.clixml'
if (!(Test-Path $credentialPath)) { throw 'Run scripts/start-postgres.ps1 first.' }
$credential = Import-Clixml -LiteralPath $credentialPath
$previousDatabase = $env:DAPPER_TEST_DATABASE_URL
try {
    $password = [Uri]::EscapeDataString($credential.GetNetworkCredential().Password)
    $env:DAPPER_TEST_DATABASE_URL = "postgres://dapper_local:${password}@127.0.0.1:15432/postgres?sslmode=disable"
    Push-Location (Join-Path $projectRoot 'dapper-platform/apps/api')
    try {
        if ($TestFilter) {
            & go.exe test -count=1 ./... -run $TestFilter
        } else {
            & go.exe test -count=1 ./...
        }
        if ($LASTEXITCODE -ne 0) { throw 'Go/PostgreSQL tests failed.' }
    } finally { Pop-Location }
} finally {
    $env:DAPPER_TEST_DATABASE_URL = $previousDatabase
    $password = $null
}
