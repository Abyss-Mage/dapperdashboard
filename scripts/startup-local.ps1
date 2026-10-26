# Brings the local Dapper stack up after a sign-in or reboot: PostgreSQL first, then the API, gateway and website.
# Does nothing if the website is already answering on 8443. Run by the "Dapper local startup" scheduled task (see install-startup-task.ps1).
param([ValidatePattern('^([1-9]|1[0-9]|2[0-4])h$')][string]$SessionLifetime = '8h')
$ErrorActionPreference = 'Stop'
$log = Join-Path (Split-Path $PSScriptRoot -Parent) '.cache/startup-local.log'
New-Item -ItemType Directory -Force -Path (Split-Path $log) | Out-Null
function Note($m) { Add-Content -LiteralPath $log -Value ('{0:u} {1}' -f (Get-Date), $m) }
try {
  if (Get-NetTCPConnection -State Listen -LocalPort 8443 -ErrorAction SilentlyContinue) { Note 'Already running; nothing to do.'; return }
  Note 'Starting PostgreSQL.'
  & (Join-Path $PSScriptRoot 'start-postgres.ps1') | Out-Null
  Note 'Starting the application.'
  & (Join-Path $PSScriptRoot 'start-application.ps1') -SessionLifetime $SessionLifetime | Out-Null
  Note 'Started.'
} catch {
  Note ('Failed: ' + $_.Exception.Message)
  throw
}
