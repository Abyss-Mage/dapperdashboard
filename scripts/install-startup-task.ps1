# Registers (or removes with -Remove) a per-user scheduled task that starts the local Dapper stack when you sign in to Windows.
# No administrator rights needed. Logins to the dashboards last the session lifetime given here (default 8h).
param([ValidatePattern('^([1-9]|1[0-9]|2[0-4])h$')][string]$SessionLifetime = '8h', [switch]$Remove)
$ErrorActionPreference = 'Stop'
$name = 'Dapper local startup'
if ($Remove) { Unregister-ScheduledTask -TaskName $name -Confirm:$false -ErrorAction SilentlyContinue; Write-Output "Removed '$name'."; return }
$script = Join-Path $PSScriptRoot 'startup-local.ps1'
$pwsh = (Get-Command pwsh.exe -ErrorAction SilentlyContinue).Source
if (-not $pwsh) { $pwsh = (Get-Command powershell.exe).Source }
$action = New-ScheduledTaskAction -Execute $pwsh -Argument ('-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "{0}" -SessionLifetime {1}' -f $script, $SessionLifetime)
$trigger = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 10)
Register-ScheduledTask -TaskName $name -Action $action -Trigger $trigger -Settings $settings -Description 'Starts local PostgreSQL, API, gateway and website for Dapper Detailing' -Force | Out-Null
Write-Output "Registered '$name': runs at sign-in, logins last $SessionLifetime. Log: .cache/startup-local.log"
