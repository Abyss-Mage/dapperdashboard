# Creates resources, weekly hours and one scheduling rule per active service from the durations and capacities in the owner-supplied BookingPress export.
param(
 [Parameter(Mandatory=$true)][string]$Email,
 [Parameter(Mandatory=$true)][string]$Franchise,
 [Parameter(Mandatory=$true)][string]$ExportPath,
 [switch]$WorkingTime,
 [switch]$Apply
)
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
if(!(Test-Path -LiteralPath $ExportPath)){throw 'Export file not found.'}
$credential=Import-Clixml -LiteralPath (Join-Path $root '.cache/postgresql/local-credential.clixml')
$password=[Uri]::EscapeDataString($credential.GetNetworkCredential().Password)
$previous=$env:DAPPER_DATABASE_URL
try{
 $env:DAPPER_DATABASE_URL="postgres://dapper_local:${password}@127.0.0.1:15432/dapper_application?sslmode=disable"
 Push-Location (Join-Path $root 'dapper-platform/apps/api')
 $arguments=@('run','./cmd/seed-scheduling','-email',$Email,'-franchise',$Franchise,'-export',$ExportPath)
 if($WorkingTime){$arguments+='-working-time'}
 if($Apply){$arguments+='-apply'}
 & go @arguments
 if($LASTEXITCODE -ne 0){throw 'Catalogue seeding did not complete.'}
}finally{Pop-Location;$env:DAPPER_DATABASE_URL=$previous;$password=$null;$credential=$null}
