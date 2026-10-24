# Runs the real Square SANDBOX evidence tool using the encrypted local config.
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$configPath=Join-Path $root '.cache/square-sandbox/config.clixml'
if(!(Test-Path -LiteralPath $configPath)){throw 'Run scripts/set-square-sandbox-config.ps1 first.'}
$config=Import-Clixml -LiteralPath $configPath
$exe=Join-Path $root '.cache/square-sandbox/squaresandbox.exe'
Push-Location (Join-Path $root 'dapper-platform/apps/api')
try{& go.exe build -o $exe ./cmd/squaresandbox; if($LASTEXITCODE -ne 0){throw 'Build failed.'}}finally{Pop-Location}
$keys='SQUARE_SANDBOX_TOKEN','SQUARE_SANDBOX_MERCHANT_ID','SQUARE_SANDBOX_LOCATION_ID'
$previous=@{};foreach($k in $keys){$previous[$k]=[Environment]::GetEnvironmentVariable($k,'Process')}
try{
 $env:SQUARE_SANDBOX_TOKEN=[Net.NetworkCredential]::new('',$config.token).Password
 $env:SQUARE_SANDBOX_MERCHANT_ID=$config.merchantId;$env:SQUARE_SANDBOX_LOCATION_ID=$config.locationId
 $out=& $exe;$code=$LASTEXITCODE
}finally{foreach($k in $keys){[Environment]::SetEnvironmentVariable($k,$previous[$k],'Process')}}
$evidence=Join-Path $root 'output/acceptance/square-sandbox-verification.json'
$out|Set-Content -LiteralPath $evidence -Encoding utf8
$out
"provider checks exit: $code"

# Second stage: our own prepare -> charge -> capture -> ledger flow against the sandbox, in a disposable schema.
$dbCredential=Import-Clixml -LiteralPath (Join-Path $root '.cache/postgresql/local-credential.clixml')
$keys2=$keys+'DAPPER_TEST_DATABASE_URL'
$previous2=@{};foreach($k in $keys2){$previous2[$k]=[Environment]::GetEnvironmentVariable($k,'Process')}
try{
 $env:SQUARE_SANDBOX_TOKEN=[Net.NetworkCredential]::new('',$config.token).Password
 $env:SQUARE_SANDBOX_MERCHANT_ID=$config.merchantId;$env:SQUARE_SANDBOX_LOCATION_ID=$config.locationId
 $pw=[Uri]::EscapeDataString($dbCredential.GetNetworkCredential().Password)
 $env:DAPPER_TEST_DATABASE_URL="postgres://dapper_local:${pw}@127.0.0.1:15432/postgres?sslmode=disable"
 Push-Location (Join-Path $root 'dapper-platform/apps/api')
 try{$flow=& go.exe test -count=1 -v -run TestSquareSandboxEndToEnd ./internal/postgres 2>&1;$flowCode=$LASTEXITCODE}finally{Pop-Location}
}finally{foreach($k in $keys2){[Environment]::SetEnvironmentVariable($k,$previous2[$k],'Process')}}
$flowEvidence=Join-Path $root 'output/acceptance/square-sandbox-flow-verification.txt'
($flow|Select-Object -Last 15)|Set-Content -LiteralPath $flowEvidence -Encoding utf8
$flow|Select-Object -Last 8
"application flow exit: $flowCode"
if($code -ne 0 -or $flowCode -ne 0){exit 1}
