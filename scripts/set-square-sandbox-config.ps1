# Run this yourself in a terminal. The access token is typed as a hidden
# SecureString and stored DPAPI-encrypted for your Windows user only (ignored .cache).
# It is never written to source, the vault, logs or chat.
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$dir=Join-Path $root '.cache/square-sandbox'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$token=Read-Host 'Square SANDBOX access token (hidden)' -AsSecureString
$merchant=(Read-Host 'Sandbox merchant ID (non-secret)').Trim()
$location=(Read-Host 'Sandbox location ID (non-secret; must be a CAD location)').Trim()
$application=(Read-Host 'Sandbox application ID (public, starts with sandbox-sq0idb-; needed for the browser card form)').Trim()
if(!$merchant -or !$location){throw 'Merchant and location IDs are required.'}
[pscustomobject]@{token=$token;merchantId=$merchant;locationId=$location;applicationId=$application}|Export-Clixml -LiteralPath (Join-Path $dir 'config.clixml')
Write-Output 'Saved encrypted sandbox configuration. Now run scripts/verify-square-sandbox.ps1'
