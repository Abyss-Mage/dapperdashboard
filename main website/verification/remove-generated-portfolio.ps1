$ErrorActionPreference = 'Stop'
$websiteRoot = Split-Path $PSScriptRoot -Parent
$projectRoot = [IO.Path]::GetFullPath((Split-Path $websiteRoot -Parent))
$manifestPath = Join-Path $websiteRoot 'portfolio-import.json'
$manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
$targets = @('dapper-platform/apps/web/public/site', '.cache/website-validation/dapper-platform/apps/web/public/site')
foreach ($relative in $targets) {
    $target = [IO.Path]::GetFullPath((Join-Path $projectRoot $relative))
    if (!$target.StartsWith($projectRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Target escaped project workspace' }
    foreach ($item in $manifest.images) {
        if ($item.file -notmatch '^portfolio-\d{2}\.jpg$') { throw 'Unexpected source filename' }
        $source = Join-Path $websiteRoot ('assets/portfolio/' + $item.file)
        if ((Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash -ne $item.localSha256) { throw 'Retained source hash differs' }
        $generated = Join-Path $target ('assets/portfolio/' + $item.file)
        if (Test-Path -LiteralPath $generated) {
            if ((Get-FileHash -LiteralPath $generated -Algorithm SHA256).Hash -ne $item.localSha256) { throw 'Generated copy differs; refusing removal' }
            Remove-Item -LiteralPath $generated
        }
    }
    $generatedManifest = Join-Path $target 'portfolio-import.json'
    if (Test-Path -LiteralPath $generatedManifest) {
        if ((Get-FileHash -LiteralPath $generatedManifest).Hash -ne (Get-FileHash -LiteralPath $manifestPath).Hash) { throw 'Generated manifest differs' }
        Remove-Item -LiteralPath $generatedManifest
    }
}
Write-Output 'Removed only verified generated public archive copies; original and private API sources retained.'
