$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$databaseRoot = Join-Path $projectRoot '.cache/postgresql'
& (Join-Path $databaseRoot 'runtime/pgsql/bin/pg_ctl.exe') stop -w -m fast -D (Join-Path $databaseRoot 'data')
if ($LASTEXITCODE -ne 0) { throw 'Project database stop failed.' }
