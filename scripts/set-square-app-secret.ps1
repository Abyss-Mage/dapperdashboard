# Run this yourself in a terminal. Stores the Square application ID and secret that let each franchise connect its own
# Square account (OAuth). The secret is typed hidden and saved DPAPI-encrypted for your Windows user only (ignored .cache).
# It is never written to source, the vault, logs or chat. Restart the application afterwards.
# In the Square Developer Dashboard, set the OAuth redirect URL to: https://127.0.0.1:8443/api/v1/integrations/square/oauth/callback
param([ValidateSet('sandbox','production')][string]$Environment='sandbox')
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$dir=Join-Path $root '.cache/secrets'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$path=Join-Path $dir 'square-app.clixml'
$saved=if(Test-Path -LiteralPath $path){Import-Clixml -LiteralPath $path}else{[pscustomobject]@{sandbox=$null;production=$null}}
$id=(Read-Host "Square $Environment application ID (public)").Trim()
if(!$id){throw 'Application ID is required.'}
$secret=Read-Host "Square $Environment application secret (hidden)" -AsSecureString
$saved.$Environment=[pscustomobject]@{applicationId=$id;secret=$secret}
$saved|Export-Clixml -LiteralPath $path
Write-Output "Saved the $Environment Square application. Restart the application to use it."
