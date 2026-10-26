# Run this yourself. Stores your outgoing-mail (SMTP) settings encrypted for your Windows user only (ignored .cache).
# The password is typed as a hidden SecureString and never written to source, the vault, logs or chat.
# After saving, restart the application (scripts/stop-application.ps1 then start-application.ps1).
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$dir=Join-Path $root '.cache/secrets'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$smtpHost=(Read-Host 'SMTP host (e.g. smtp.example.com)').Trim()
$port=[int](Read-Host 'SMTP port (STARTTLS, usually 587)')
$user=(Read-Host 'SMTP username').Trim()
$pass=Read-Host 'SMTP password (hidden)' -AsSecureString
$from=(Read-Host 'From address (e.g. no-reply@dapperdetailing.ca)').Trim()
$fromName=(Read-Host 'From name (e.g. Dapper Detailing Pros)').Trim()
if(!$smtpHost -or $port -lt 1 -or $port -gt 65535 -or !$from){throw 'Host, a valid port and a from address are required.'}
[pscustomobject]@{host=$smtpHost;port=$port;username=$user;password=$pass;fromAddress=$from;fromName=$fromName}|Export-Clixml -LiteralPath (Join-Path $dir 'smtp.clixml')
Write-Output 'Saved encrypted SMTP settings. Restart the application, then use Master > Access > invitations to send an invitation.'
