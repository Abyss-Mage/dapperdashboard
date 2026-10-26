param(
 [Parameter(Mandatory=$true)][string]$Email,
 [Parameter(Mandatory=$true)][string]$Organization
)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/application-runtime'
$settings=Get-Content -LiteralPath (Join-Path $runtimeRoot 'settings.json') -Raw|ConvertFrom-Json
if($settings.mode -ne 'application' -or $settings.database -ne 'dapper_application'){throw 'Only the separate application database can be initialized.'}
$executable=Join-Path $runtimeRoot 'initialize-admin.exe'
Push-Location (Join-Path $projectRoot 'dapper-platform/apps/api')
try{& go.exe build -o $executable ./cmd/initialize-admin;if($LASTEXITCODE -ne 0){throw 'Administrator tool build failed'}}finally{Pop-Location}
Write-Host 'Choose a unique administrator password (15–1024 characters). It is not displayed or saved by this script.'
$securePassword=Read-Host 'Password' -AsSecureString
$secureConfirmation=Read-Host 'Confirm password' -AsSecureString
$passwordPointer=[IntPtr]::Zero;$confirmationPointer=[IntPtr]::Zero
$previousDatabase=$env:DAPPER_DATABASE_URL;$previousOptIn=$env:DAPPER_INITIAL_ADMIN
try{
 $passwordPointer=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
 $confirmationPointer=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureConfirmation)
 $password=[Runtime.InteropServices.Marshal]::PtrToStringBSTR($passwordPointer)
 $confirmation=[Runtime.InteropServices.Marshal]::PtrToStringBSTR($confirmationPointer)
 if($password -cne $confirmation -or $password.Length -lt 15 -or $password.Length -gt 1024){throw 'Passwords must match and contain 15–1024 characters.'}
 $credential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
 $databasePassword=[Uri]::EscapeDataString($credential.GetNetworkCredential().Password)
 $env:DAPPER_DATABASE_URL="postgres://dapper_local:${databasePassword}@127.0.0.1:15432/dapper_application?sslmode=disable"
 $env:DAPPER_INITIAL_ADMIN='1'
 $result=$password|& $executable -organization $Organization -email $Email
 if($LASTEXITCODE -ne 0){throw 'Initial administrator was not provisioned. No password or database details are logged.'}
 $result|ConvertFrom-Json|Select-Object organizationId,principalId,email
 Write-Host 'Administrator created. Sign in at the configured HTTPS application address. No location or financial permissions were inferred.'
}finally{
 $env:DAPPER_DATABASE_URL=$previousDatabase;$env:DAPPER_INITIAL_ADMIN=$previousOptIn
 $password=$null;$confirmation=$null;$credential=$null;$databasePassword=$null
 if($passwordPointer -ne [IntPtr]::Zero){[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($passwordPointer)}
 if($confirmationPointer -ne [IntPtr]::Zero){[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($confirmationPointer)}
 $securePassword.Dispose();$secureConfirmation.Dispose()
}
