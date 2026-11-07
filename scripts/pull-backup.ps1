# Copies the newest nightly database backup (and the .env it needs to be restored) from the production server to this computer,
# so losing the server does not lose the data. Uses the SSH alias "dapper-prod" (key only).
#   pwsh -NoProfile -File scripts/pull-backup.ps1                 (folder: %USERPROFILE%\DapperBackups)
#   pwsh -NoProfile -File scripts/pull-backup.ps1 -Folder D:\Safe  (another folder; keep it OUTSIDE the project, it holds secrets)
# Keeps the newest 30 copies. To run it every morning at 10:00 while this computer is on:
#   schtasks /Create /SC DAILY /ST 10:00 /TN "Dapper backup pull" /TR "pwsh -NoProfile -File \"<full path to this script>\""
param([string]$Folder = (Join-Path $env:USERPROFILE 'DapperBackups'), [string]$HostAlias = 'dapper-prod')
$ErrorActionPreference = 'Stop'
New-Item -ItemType Directory -Force -Path $Folder | Out-Null
$dump = (ssh -o BatchMode=yes $HostAlias 'ls -1t /opt/dapper/backups/dapper-*.dump | head -1').Trim()
if (-not $dump) { throw 'No backup found on the server.' }
$day = [regex]::Match($dump, '\d{4}-\d{2}-\d{2}').Value
scp -o BatchMode=yes "${HostAlias}:$dump" (Join-Path $Folder (Split-Path $dump -Leaf))
scp -o BatchMode=yes "${HostAlias}:/opt/dapper/backups/env-$day" (Join-Path $Folder "env-$day")
# A dump that did not copy completely is of no use: compare sizes with the server's copy.
$remote = [int64](ssh -o BatchMode=yes $HostAlias "stat -c %s $dump")
$local = (Get-Item (Join-Path $Folder (Split-Path $dump -Leaf))).Length
if ($remote -ne $local) { throw "The copy is incomplete ($local of $remote bytes)." }
Get-ChildItem $Folder -Filter 'dapper-*.dump' | Sort-Object LastWriteTime -Descending | Select-Object -Skip 30 | Remove-Item -Force
Get-ChildItem $Folder -Filter 'env-*' | Sort-Object LastWriteTime -Descending | Select-Object -Skip 30 | Remove-Item -Force
Write-Output "Saved $(Split-Path $dump -Leaf) ($local bytes) to $Folder"
