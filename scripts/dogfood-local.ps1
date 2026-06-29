[CmdletBinding()]
param(
    [string]$Root = "",
    [string]$Version = "0.1.0-dogfood",
    [switch]$UseExistingBinary
)

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Join-Path ([System.IO.Path]::GetTempPath()) 'devflow-dogfood-local'
}
$rootPath = [System.IO.Path]::GetFullPath($Root)
$tempPath = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
if (-not $rootPath.StartsWith($tempPath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Dogfood root must be inside the system temp directory unless the script is edited intentionally: $rootPath"
}
if (Test-Path -LiteralPath $rootPath) {
    Remove-Item -LiteralPath $rootPath -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $rootPath | Out-Null

$binary = Join-Path $repoRoot 'dist\devflow-windows-amd64.exe'
if ((-not $UseExistingBinary) -or (-not (Test-Path -LiteralPath $binary))) {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repoRoot 'scripts\build-windows.ps1') -Version $Version -Output $binary
}

& $binary version
if ($LASTEXITCODE -ne 0) { throw "devflow version failed" }

& $binary init --root $rootPath --provider openai-compat
if ($LASTEXITCODE -ne 0) { throw "devflow init failed" }

& $binary start --root $rootPath --title "Dogfood coupon eligibility" --description "Only active members can claim coupons once"
if ($LASTEXITCODE -ne 0) { throw "devflow start failed" }

$demandID = "dogfood-coupon-eligibility"
& $binary status --root $rootPath --demand $demandID
if ($LASTEXITCODE -ne 0) { throw "devflow status failed" }

$next = & $binary next --root $rootPath --demand $demandID
if ($LASTEXITCODE -ne 0) { throw "devflow next failed" }
if ($next -notmatch 'devflow run --demand dogfood-coupon-eligibility --stage requirements') {
    throw "unexpected next command: $next"
}

Write-Host "dogfood root: $rootPath"
Write-Host "next command: $next"
