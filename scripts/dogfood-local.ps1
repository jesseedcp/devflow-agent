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

$dogfoodRoot = Join-Path $rootPath 'artifacts'
New-Item -ItemType Directory -Force -Path $dogfoodRoot | Out-Null

& $binary dogfood --root $dogfoodRoot --quality-root $repoRoot --quality-command "go test ./... -count=1 -timeout 5m"
if ($LASTEXITCODE -ne 0) { throw "devflow dogfood failed" }

$report = Join-Path $dogfoodRoot '.devflow\demands\dogfood-coupon-eligibility\dogfood-report.md'
if (-not (Test-Path -LiteralPath $report)) {
    throw "dogfood report missing: $report"
}

Write-Host "dogfood root: $dogfoodRoot"
Write-Host "dogfood report: $report"
