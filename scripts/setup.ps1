# First-run setup for a fresh checkout: builds the web app if needed, then opens the browser wizard (database, site, email, owner).
# Usage (from the repository root):  powershell -File scripts/setup.ps1
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$web=Join-Path $root 'dapper-platform/apps/web'
$api=Join-Path $root 'dapper-platform/apps/api'
foreach($tool in 'go','node','npm'){ if(!(Get-Command $tool -ErrorAction SilentlyContinue)){ throw "$tool is required and was not found on PATH." } }
if(!(Test-Path (Join-Path $web '.next/BUILD_ID'))){
  Push-Location $web; try{ npm ci; if($LASTEXITCODE){throw 'npm ci failed'}; npm run build; if($LASTEXITCODE){throw 'npm run build failed'} } finally { Pop-Location }
}
Push-Location $api; try{ go run ./cmd/dapper setup --home $root } finally { Pop-Location }
