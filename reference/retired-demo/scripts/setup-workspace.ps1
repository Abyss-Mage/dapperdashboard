param([Parameter(Mandatory=$true)][string]$SessionLifetime)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/workspace-runtime'
$settingsPath=Join-Path $runtimeRoot 'settings.json'
if(Test-Path -LiteralPath $settingsPath){throw 'Workspace settings already exist. Setup never overwrites accounts or schemas.'}
if($SessionLifetime -notmatch '^([1-9]|1[0-9]|2[0-4])h$'){throw 'Choose an explicit synthetic session lifetime from 1h to 24h.'}
$databaseCredential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
New-Item -ItemType Directory -Force -Path $runtimeRoot | Out-Null
Push-Location (Join-Path $projectRoot 'dapper-platform/apps/api')
try{foreach($target in @('fixture','localcert','workspace','gateway','account')){& go.exe build -o (Join-Path $runtimeRoot "$target.exe") "./cmd/$target";if($LASTEXITCODE -ne 0){throw 'Workspace tool build failed'}}}finally{Pop-Location}
$schemaName='dapper_workspace_demo_'+[DateTime]::UtcNow.Ticks
$secretBytes=New-Object byte[] 32
$generator=[System.Security.Cryptography.RandomNumberGenerator]::Create()
$generator.GetBytes($secretBytes)
$fixturePassword=[Convert]::ToBase64String($secretBytes)
[Array]::Clear($secretBytes,0,$secretBytes.Length)
$generator.Dispose()
$previousDatabase=$env:DAPPER_DATABASE_URL
$previousFixture=$env:DAPPER_SYNTHETIC_FIXTURE
try{
 $databasePassword=[Uri]::EscapeDataString($databaseCredential.GetNetworkCredential().Password)
 $env:DAPPER_DATABASE_URL="postgres://dapper_local:${databasePassword}@127.0.0.1:15432/postgres?sslmode=disable&search_path=$schemaName"
 $env:DAPPER_SYNTHETIC_FIXTURE='1'
 $fixtureOutput=$fixturePassword | & (Join-Path $runtimeRoot 'fixture.exe')
 if($LASTEXITCODE -ne 0){throw 'Synthetic bootstrap failed; existing data was not reused.'}
 $fixture=$fixtureOutput | ConvertFrom-Json
 $securePassword=ConvertTo-SecureString -String $fixturePassword -AsPlainText -Force
 $credential=[System.Management.Automation.PSCredential]::new($fixture.email,$securePassword)
 $credential | Export-Clixml -LiteralPath (Join-Path $runtimeRoot 'synthetic-credential.clixml')
 & (Join-Path $runtimeRoot 'localcert.exe') -directory (Join-Path $runtimeRoot 'tls')
 if($LASTEXITCODE -ne 0){throw 'Local certificate generation failed; no trust settings were changed.'}
 @{schema=$fixture.schema;email=$fixture.email;organizationId=$fixture.organizationId;locationId=$fixture.locationId;principalId=$fixture.principalId;sessionLifetime=$SessionLifetime;mode='synthetic-development-only'} | ConvertTo-Json | Set-Content -LiteralPath $settingsPath
 Write-Output 'Synthetic workspace prepared in a new dedicated schema. Credential is Windows-encrypted under ignored .cache. No real accounts or certificate trust settings changed.'
}finally{$env:DAPPER_DATABASE_URL=$previousDatabase;$env:DAPPER_SYNTHETIC_FIXTURE=$previousFixture;$fixturePassword=$null;$databasePassword=$null;$securePassword=$null;$credential=$null}
