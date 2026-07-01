[CmdletBinding()]
param(
    [string]$Version = "0.1.0-readiness",
    [string]$Root = "",
    [switch]$RunLiveDogfood,
    [switch]$RunGitLabGate,
    [string]$GitLabProject = "",
    [string]$GitLabMR = "",
    [string]$GitLabBaseURL = ""
)

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$readinessRoot = $Root
if ([string]::IsNullOrWhiteSpace($readinessRoot)) {
    $readinessRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('devflow-release-readiness-' + [guid]::NewGuid().ToString('N'))
}
$readinessRoot = [System.IO.Path]::GetFullPath($readinessRoot)
$tempPath = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
if (-not $readinessRoot.StartsWith($tempPath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Release readiness root must be inside the system temp directory unless the script is edited intentionally: $readinessRoot"
}
New-Item -ItemType Directory -Force -Path $readinessRoot | Out-Null

$report = Join-Path $readinessRoot 'release-readiness.md'
Set-Content -LiteralPath $report -Value "# Devflow Release Readiness`n" -Encoding UTF8
Add-Content -LiteralPath $report -Value "Version: ``$Version```n"
Add-Content -LiteralPath $report -Value "Repo: ``$repoRoot```n"

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Command
    )
    Write-Host "==> $Name"
    Add-Content -LiteralPath $report -Value "## $Name`n"
    & $Command 2>&1 | Tee-Object -Variable output
    $exit = $LASTEXITCODE
    if ($null -eq $exit) { $exit = 0 }
    Add-Content -LiteralPath $report -Value '```text'
    Add-Content -LiteralPath $report -Value ($output -join [Environment]::NewLine)
    Add-Content -LiteralPath $report -Value '```'
    Add-Content -LiteralPath $report -Value ""
    if ($exit -ne 0) {
        throw "$Name failed with exit code $exit"
    }
}

Push-Location $repoRoot
try {
    Invoke-Step "go test" { go test ./... -count=1 -timeout 5m }
    Invoke-Step "go vet" { go vet ./... }
    Invoke-Step "go build" { go build ./cmd/devflow }
    Invoke-Step "windows build" { powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repoRoot 'scripts\build-windows.ps1') -Version $Version -Output (Join-Path $repoRoot 'dist\devflow-windows-amd64.exe') }
    Invoke-Step "intake smoke" {
        $intakeRoot = Join-Path $readinessRoot 'intake-smoke'
        New-Item -ItemType Directory -Force $intakeRoot | Out-Null
        $prdPath = Join-Path $intakeRoot 'coupon-eligibility.md'
        @"
# Coupon eligibility

## Goals
- Active members can claim coupons.

## Business Rules
- User status must be active.

## Acceptance Criteria
- Inactive users are blocked.
"@ | Set-Content -Encoding UTF8 $prdPath

        .\dist\devflow-windows-amd64.exe intake --root $intakeRoot --file $prdPath | Tee-Object -FilePath (Join-Path $intakeRoot 'intake-output.txt') | Out-Host
        .\dist\devflow-windows-amd64.exe recall --root $intakeRoot --demand coupon-eligibility | Tee-Object -FilePath (Join-Path $intakeRoot 'recall-output.txt') | Out-Host
        $contextPath = Join-Path $intakeRoot '.devflow\demands\coupon-eligibility\context.md'
        if (-not (Test-Path $contextPath)) {
            throw "context.md was not created by intake recall"
        }
        $requirementsPath = Join-Path $intakeRoot '.devflow\demands\coupon-eligibility\requirements.md'
        @"
# Requirements: Coupon eligibility

## 业务规则

- User status must be active.

## 验收标准

- Active users can claim coupons.

## 风险与歧义

- Confirm inactive-user handling before approval.
"@ | Set-Content -Encoding UTF8 $requirementsPath

        $evaluateOutput = .\dist\devflow-windows-amd64.exe evaluate --root $intakeRoot --demand coupon-eligibility --stage requirements 2>&1
        $evaluateOutput | Tee-Object -FilePath (Join-Path $intakeRoot 'evaluate-output.txt') | Out-Host
        $evaluateText = $evaluateOutput -join [Environment]::NewLine
        if ($evaluateText -notmatch 'requirements\.intake_coverage') {
            throw "requirements.intake_coverage missing from evaluate output"
        }

        $consoleOutput = .\dist\devflow-windows-amd64.exe console --root $intakeRoot --demand coupon-eligibility 2>&1
        $consoleOutput | Tee-Object -FilePath (Join-Path $intakeRoot 'console-output.txt') | Out-Host
        $consoleText = $consoleOutput -join [Environment]::NewLine
        if ($consoleText -notmatch 'Quality:') {
            throw "Quality section missing from console output"
        }

        $snapshotOutput = .\dist\devflow-windows-amd64.exe workbench --root $intakeRoot --snapshot --demand coupon-eligibility 2>&1
        $snapshotOutput | Tee-Object -FilePath (Join-Path $intakeRoot 'workbench-output.txt') | Out-Host
        $snapshotText = $snapshotOutput -join [Environment]::NewLine
        if ($snapshotText -notmatch 'requirements\.intake_coverage') {
            throw "requirements.intake_coverage missing from workbench snapshot"
        }
    }
    Invoke-Step "deterministic dogfood" { powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repoRoot 'scripts\dogfood-local.ps1') -Version $Version }
    Invoke-Step "operator dogfood" { .\dist\devflow-windows-amd64.exe dogfood --operator-loop --root (Join-Path $readinessRoot 'operator-dogfood') --quality-root $repoRoot --quality-command "go test ./... -count=1 -timeout 5m" }
    Invoke-Step "git diff check" { git diff --check }

    if ($RunLiveDogfood) {
        if ($env:DEVFLOW_LIVE_DOGFOOD -ne "1") {
            throw "RunLiveDogfood requires DEVFLOW_LIVE_DOGFOOD=1"
        }
        Invoke-Step "live dogfood" { .\dist\devflow-windows-amd64.exe live-dogfood --root (Join-Path $readinessRoot 'live-dogfood') }
    } else {
        Add-Content -LiteralPath $report -Value "## live dogfood`nSkipped. Pass -RunLiveDogfood and set DEVFLOW_LIVE_DOGFOOD=1 to run.`n"
    }

    if ($RunGitLabGate) {
        if ([string]::IsNullOrWhiteSpace($GitLabProject) -or [string]::IsNullOrWhiteSpace($GitLabMR)) {
            throw "-RunGitLabGate requires -GitLabProject and -GitLabMR"
        }
        $gateArgs = @('review-gate', '--gitlab-project', $GitLabProject, '--gitlab-mr', $GitLabMR)
        if (-not [string]::IsNullOrWhiteSpace($GitLabBaseURL)) {
            $gateArgs += @('--gitlab-base-url', $GitLabBaseURL)
        }
        Invoke-Step "gitlab review gate" { .\dist\devflow-windows-amd64.exe @gateArgs }
    } else {
        Add-Content -LiteralPath $report -Value "## gitlab review gate`nSkipped. Pass -RunGitLabGate with -GitLabProject and -GitLabMR to run.`n"
    }
}
finally {
    Pop-Location
}

Write-Host "release readiness report: $report"
