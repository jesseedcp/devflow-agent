[CmdletBinding()]
param(
    [string]$Version = "0.1.0-dev",
    [string]$Output = "dist/devflow-windows-amd64.exe"
)

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$outputPath = $Output
if (-not [System.IO.Path]::IsPathRooted($outputPath)) {
    $outputPath = Join-Path $repoRoot $outputPath
}
$outputPath = [System.IO.Path]::GetFullPath($outputPath)
$outputDir = Split-Path -Parent $outputPath

New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

$commit = (git -C $repoRoot rev-parse --short=12 HEAD).Trim()
$date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = @(
    "-X", "github.com/jesseedcp/devflow-agent/internal/version.Version=$Version",
    "-X", "github.com/jesseedcp/devflow-agent/internal/version.Commit=$commit",
    "-X", "github.com/jesseedcp/devflow-agent/internal/version.Date=$date"
) -join " "

$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags $ldflags -o $outputPath ./cmd/devflow
} finally {
    $env:GOOS = $oldGOOS
    $env:GOARCH = $oldGOARCH
}

Write-Host "wrote $outputPath"
& $outputPath version