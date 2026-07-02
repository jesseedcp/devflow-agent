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


function New-LocalServerReadyFile {
    param(
        [string]$Name
    )
    $safeName = ($Name -replace '[^a-zA-Z0-9_-]', '-')
    return Join-Path $readinessRoot ("$safeName.ready")
}

function Wait-LocalServerReadyFile {
    param(
        [System.Management.Automation.Job]$Job,
        [string]$ReadyFile,
        [int]$Milliseconds = 10000
    )
    $deadline = (Get-Date).AddMilliseconds($Milliseconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $ReadyFile) {
            return
        }
        if ($Job.State -eq 'Failed') {
            Receive-Job $Job
            throw "local server job failed before readiness file was written"
        }
        if ($Job.State -eq 'Completed' -and -not (Test-Path -LiteralPath $ReadyFile)) {
            Receive-Job $Job
            throw "local server job completed before readiness file was written"
        }
        Start-Sleep -Milliseconds 100
    }
    if ($Job.State -eq 'Failed') {
        Receive-Job $Job
    }
    throw "local server did not become ready: $ReadyFile state=$($Job.State)"
}

function Invoke-LocalServerCommand {
    param(
        [scriptblock]$Command,
        [int]$Attempts = 5
    )
    $output = @()
    $exit = 1
    for ($attempt = 1; $attempt -le $Attempts; $attempt++) {
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            $output = & $Command 2>&1
            $exit = $LASTEXITCODE
            if ($null -eq $exit) { $exit = 0 }
        } finally {
            $ErrorActionPreference = $previousErrorActionPreference
        }
        if ($exit -eq 0) {
            return $output
        }
        $text = $output -join [Environment]::NewLine
        if ($text -notmatch 'connection refused|actively refused|connectex|No connection could be made') {
            return $output
        }
        Start-Sleep -Milliseconds (200 * $attempt)
    }
    return $output
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
    Invoke-Step "url intake smoke" {
        $urlRoot = Join-Path $readinessRoot 'url-intake-smoke'
        New-Item -ItemType Directory -Force $urlRoot | Out-Null
        $port = Get-Random -Minimum 20000 -Maximum 50000
        $prefix = "http://127.0.0.1:$port/"
        $html = @"
<!doctype html>
<html>
<head><title>Coupon URL PRD</title></head>
<body>
  <h1>Coupon URL PRD</h1>
  <h2>Goals</h2>
  <p>Active members can claim URL coupons.</p>
  <h2>Business Rules</h2>
  <p>User status must be active.</p>
  <h2>Acceptance Criteria</h2>
  <p>Inactive users are blocked.</p>
</body>
</html>
"@
        $readyFile = New-LocalServerReadyFile 'url-intake'
        $serverJob = Start-Job -ScriptBlock {
            param($Prefix, $Body, $ReadyFile)
            $listener = [System.Net.HttpListener]::new()
            $listener.Prefixes.Add($Prefix)
            $listener.Start()
            [System.IO.File]::WriteAllText($ReadyFile, 'ready')
            try {
                for ($i = 0; $i -lt 5; $i++) {
                    $context = $listener.GetContext()
                    $bytes = [System.Text.Encoding]::UTF8.GetBytes($Body)
                    $context.Response.StatusCode = 200
                    $context.Response.ContentType = 'text/html; charset=utf-8'
                    $context.Response.ContentLength64 = $bytes.Length
                    $context.Response.OutputStream.Write($bytes, 0, $bytes.Length)
                    $context.Response.OutputStream.Close()
                }
            }
            finally {
                $listener.Stop()
                $listener.Close()
            }
        } -ArgumentList $prefix, $html, $readyFile
        try {
            Wait-LocalServerReadyFile -Job $serverJob -ReadyFile $readyFile
            Start-Sleep -Milliseconds 500
            $url = $prefix + 'prd'
            $urlOutput = Invoke-LocalServerCommand { .\dist\devflow-windows-amd64.exe intake --root $urlRoot --url $url }
            $urlOutput | Tee-Object -FilePath (Join-Path $urlRoot 'url-intake-output.txt') | Out-Host
            Wait-Job $serverJob -Timeout 10 | Out-Null
            if ($serverJob.State -eq 'Failed') {
                Receive-Job $serverJob
                throw "local URL intake server failed"
            }
            $urlText = $urlOutput -join [Environment]::NewLine
            if ($urlText -notmatch 'url: http://127\.0\.0\.1:') {
                throw "URL source missing from intake output"
            }
            $requirementsPath = Join-Path $urlRoot '.devflow\demands\coupon-url-prd\requirements.md'
            if (-not (Test-Path $requirementsPath)) {
                throw "URL intake requirements.md was not created"
            }
            $requirementsText = Get-Content -Raw -LiteralPath $requirementsPath
            if ($requirementsText -notmatch 'Active members can claim URL coupons') {
                throw "URL intake requirements missing fetched PRD content"
            }
        }
        finally {
            if ($serverJob.State -eq 'Running') {
                Stop-Job $serverJob
            }
            Remove-Job $serverJob -Force
        }
    }
    Invoke-Step "github issue fake intake smoke" {
        $githubRoot = Join-Path $readinessRoot 'github-issue-intake-smoke'
        New-Item -ItemType Directory -Force $githubRoot | Out-Null
        $port = Get-Random -Minimum 20000 -Maximum 50000
        $prefix = "http://127.0.0.1:$port/"
        $readyFile = New-LocalServerReadyFile 'github-issue-intake'
        $serverJob = Start-Job -ScriptBlock {
            param($Prefix, $ReadyFile)
            $listener = [System.Net.HttpListener]::new()
            $listener.Prefixes.Add($Prefix)
            $listener.Start()
            [System.IO.File]::WriteAllText($ReadyFile, 'ready')
            try {
                for ($i = 0; $i -lt 2; $i++) {
                    $context = $listener.GetContext()
                    $path = $context.Request.Url.AbsolutePath
                    if ($path -eq "/repos/owner/repo/issues/123") {
                        $body = '{"number":123,"title":"Coupon issue","body":"Users need coupon eligibility.","html_url":"https://github.com/owner/repo/issues/123","state":"open","user":{"login":"alice"},"labels":[{"name":"backend"}]}'
                    } else {
                        $body = '[{"id":10,"body":"Remember inactive users.","html_url":"https://github.com/owner/repo/issues/123#issuecomment-10","created_at":"2026-07-02T02:03:04Z","user":{"login":"bob"}}]'
                    }
                    $bytes = [System.Text.Encoding]::UTF8.GetBytes($body)
                    $context.Response.StatusCode = 200
                    $context.Response.ContentType = 'application/json; charset=utf-8'
                    $context.Response.OutputStream.Write($bytes, 0, $bytes.Length)
                    $context.Response.OutputStream.Close()
                }
            } finally {
                $listener.Stop()
                $listener.Close()
            }
        } -ArgumentList $prefix, $readyFile
        try {
            Wait-LocalServerReadyFile -Job $serverJob -ReadyFile $readyFile
            $githubOutput = Invoke-LocalServerCommand { .\dist\devflow-windows-amd64.exe intake --root $githubRoot --github-issue owner/repo#123 --github-base-url $prefix.TrimEnd('/') --github-token fake }
            $githubOutput | Out-Host
            $intakePath = Join-Path $githubRoot '.devflow\demands\coupon-issue\intake.md'
            if (-not (Test-Path $intakePath)) { throw "GitHub issue intake did not create intake.md" }
        } finally {
            if ($serverJob.State -eq 'Running') { Stop-Job $serverJob }
            Remove-Job $serverJob -Force
        }
    }
    Invoke-Step "feishu doc fake intake smoke" {
        $feishuDocRoot = Join-Path $readinessRoot 'feishu-doc-intake-smoke'
        New-Item -ItemType Directory -Force $feishuDocRoot | Out-Null
        $port = Get-Random -Minimum 20000 -Maximum 50000
        $prefix = "http://127.0.0.1:$port/"
        $readyFile = New-LocalServerReadyFile 'feishu-doc-intake'
        $serverJob = Start-Job -ScriptBlock {
            param($Prefix, $ReadyFile)
            $listener = [System.Net.HttpListener]::new()
            $listener.Prefixes.Add($Prefix)
            $listener.Start()
            [System.IO.File]::WriteAllText($ReadyFile, 'ready')
            try {
                for ($i = 0; $i -lt 3; $i++) {
                    $context = $listener.GetContext()
                    $path = $context.Request.Url.AbsolutePath
                    if ($path -eq "/open-apis/auth/v3/tenant_access_token/internal") {
                        $body = '{"code":0,"tenant_access_token":"tenant-token","expire":7200}'
                    } elseif ($path -eq "/open-apis/docx/v1/documents/doc_token") {
                        $body = '{"code":0,"data":{"document":{"title":"Coupon PRD"}}}'
                    } else {
                        $body = '{"code":0,"data":{"items":[{"block_id":"b1","block_type":3,"heading1":{"elements":[{"text_run":{"content":"Goals"}}]}},{"block_id":"b2","block_type":2,"text":{"elements":[{"text_run":{"content":"Active users can claim coupons."}}]}}]}}'
                    }
                    $bytes = [System.Text.Encoding]::UTF8.GetBytes($body)
                    $context.Response.StatusCode = 200
                    $context.Response.ContentType = 'application/json; charset=utf-8'
                    $context.Response.OutputStream.Write($bytes, 0, $bytes.Length)
                    $context.Response.OutputStream.Close()
                }
            } finally {
                $listener.Stop()
                $listener.Close()
            }
        } -ArgumentList $prefix, $readyFile
        try {
            Wait-LocalServerReadyFile -Job $serverJob -ReadyFile $readyFile
            $feishuDocOutput = Invoke-LocalServerCommand { .\dist\devflow-windows-amd64.exe intake --root $feishuDocRoot --feishu-doc doc_token --feishu-base-url $prefix.TrimEnd('/') --feishu-app-id cli_test --feishu-app-secret fake }
            $feishuDocOutput | Out-Host
            $intakePath = Join-Path $feishuDocRoot '.devflow\demands\coupon-prd\intake.md'
            if (-not (Test-Path $intakePath)) { throw "Feishu doc intake did not create intake.md" }
        } finally {
            if ($serverJob.State -eq 'Running') { Stop-Job $serverJob }
            Remove-Job $serverJob -Force
        }
    }
    Invoke-Step "feishu bitable fake pool smoke" {
        $feishuPoolRoot = Join-Path $readinessRoot 'feishu-bitable-pool-smoke'
        New-Item -ItemType Directory -Force $feishuPoolRoot | Out-Null
        $port = Get-Random -Minimum 20000 -Maximum 50000
        $prefix = "http://127.0.0.1:$port/"
        $readyFile = New-LocalServerReadyFile 'feishu-bitable-pool'
        $serverJob = Start-Job -ScriptBlock {
            param($Prefix, $ReadyFile)
            $listener = [System.Net.HttpListener]::new()
            $listener.Prefixes.Add($Prefix)
            $listener.Start()
            [System.IO.File]::WriteAllText($ReadyFile, 'ready')
            try {
                for ($i = 0; $i -lt 10; $i++) {
                    $context = $listener.GetContext()
                    $path = $context.Request.Url.AbsolutePath
                    if ($path -eq "/open-apis/auth/v3/tenant_access_token/internal") {
                        $body = '{"code":0,"tenant_access_token":"tenant-token","expire":7200}'
                    } else {
                        $body = '{"code":0,"data":{"items":[{"record_id":"rec1","fields":{"\u9700\u6c42\u6807\u9898":"Coupon","\u9700\u6c42\u63cf\u8ff0":"Coupon eligibility","\u72b6\u6001":"\u5f85\u6f84\u6e05"}}]}}'
                    }
                    $bytes = [System.Text.Encoding]::UTF8.GetBytes($body)
                    $context.Response.StatusCode = 200
                    $context.Response.ContentType = 'application/json; charset=utf-8'
                    $context.Response.OutputStream.Write($bytes, 0, $bytes.Length)
                    $context.Response.OutputStream.Close()
                }
            } finally {
                $listener.Stop()
                $listener.Close()
            }
        } -ArgumentList $prefix, $readyFile
        try {
            Wait-LocalServerReadyFile -Job $serverJob -ReadyFile $readyFile
            $poolOutput = @()
            $poolExit = 1
            for ($attempt = 1; $attempt -le 5; $attempt++) {
                $previousErrorActionPreference = $ErrorActionPreference
                $ErrorActionPreference = 'Continue'
                try {
                    $poolOutput = .\dist\devflow-windows-amd64.exe pool list --feishu-bitable app_token --table tbl --feishu-base-url $prefix.TrimEnd('/') --feishu-app-id cli_test --feishu-app-secret fake 2>&1
                    $poolExit = $LASTEXITCODE
                } finally {
                    $ErrorActionPreference = $previousErrorActionPreference
                }
                if ($poolExit -eq 0) { break }
                Start-Sleep -Milliseconds 500
            }
            $poolOutput | Out-Host
            if ($poolExit -ne 0) { throw "Feishu bitable pool command failed" }
            if (($poolOutput -join [Environment]::NewLine) -notmatch 'rec1') { throw "Feishu bitable pool output missing rec1" }
        } finally {
            if ($serverJob.State -eq 'Running') { Stop-Job $serverJob }
            Remove-Job $serverJob -Force
        }
    }
    Invoke-Step "api evidence fetch smoke" {
        $evidenceRoot = Join-Path $readinessRoot 'api-evidence-smoke'
        New-Item -ItemType Directory -Force $evidenceRoot | Out-Null
        $demandId = 'api-evidence-coupon'
        $demandDir = Join-Path $evidenceRoot ".devflow\demands\$demandId"
        New-Item -ItemType Directory -Force $demandDir | Out-Null
        $now = (Get-Date).ToUniversalTime().ToString('o')
        $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
        [System.IO.File]::WriteAllText((Join-Path $demandDir 'demand.json'), (@{ id = $demandId; title = 'API evidence coupon'; description = 'Inactive users are blocked'; source = 'release-readiness'; state = 'verification'; created_at = $now; updated_at = $now } | ConvertTo-Json), $utf8NoBom)
        [System.IO.File]::WriteAllText((Join-Path $demandDir 'verification.md'), "# Verification: API evidence coupon`n", $utf8NoBom)
        [System.IO.File]::WriteAllText((Join-Path $demandDir 'events.jsonl'), ('{"time":"2026-07-01T00:00:00Z","type":"verification.recorded","message":"verification pass","data":{"status":"PASS","command":"go test ./internal/version","evidence_file":"verification.md"}}' + "`n"), $utf8NoBom)

        $port = Get-Random -Minimum 20000 -Maximum 50000
        $prefix = "http://127.0.0.1:$port/"
        $readyFile = New-LocalServerReadyFile 'api-evidence-fetch'
        $serverJob = Start-Job -ScriptBlock {
            param($prefix, $ReadyFile)
            $listener = [System.Net.HttpListener]::new()
            $listener.Prefixes.Add($prefix)
            $listener.Start()
            [System.IO.File]::WriteAllText($ReadyFile, 'ready')
            try {
                $context = $listener.GetContext()
                $body = '{"code":"COUPON_USER_INACTIVE"}'
                $bytes = [System.Text.Encoding]::UTF8.GetBytes($body)
                $context.Response.StatusCode = 403
                $context.Response.ContentType = 'application/json; charset=utf-8'
                $context.Response.OutputStream.Write($bytes, 0, $bytes.Length)
                $context.Response.OutputStream.Close()
            } finally {
                $listener.Stop()
                $listener.Close()
            }
        } -ArgumentList $prefix, $readyFile
        try {
            Wait-LocalServerReadyFile -Job $serverJob -ReadyFile $readyFile
            $evidenceOutput = Invoke-LocalServerCommand { .\dist\devflow-windows-amd64.exe evidence fetch --root $evidenceRoot --demand $demandId --type api --criterion "Inactive users are blocked" --method POST --url ($prefix + 'coupon?token=secret-token') --header "Authorization: Bearer secret-token" --body '{"password":"pw"}' --expect-status 403 --expect-contains COUPON_USER_INACTIVE }
            $evidenceOutput | Out-Host
        } finally {
            if ($serverJob.State -eq 'Running') { Stop-Job $serverJob }
            Remove-Job $serverJob -Force
        }

        $verificationText = [System.IO.File]::ReadAllText((Join-Path $demandDir 'verification.md'))
        $eventsText = [System.IO.File]::ReadAllText((Join-Path $demandDir 'events.jsonl'))
        if ($verificationText -notmatch '\[PASS\] api - Inactive users are blocked') { throw "API evidence fetch missing PASS evidence" }
        if (($verificationText + $eventsText) -match 'secret-token|token=secret-token|"password":"pw"') { throw "API evidence fetch leaked a secret" }

        $evidenceList = .\dist\devflow-windows-amd64.exe evidence list --root $evidenceRoot --demand $demandId 2>&1
        $evidenceList | Tee-Object -FilePath (Join-Path $evidenceRoot 'evidence-list-output.txt') | Out-Host
        if (($evidenceList -join [Environment]::NewLine) -notmatch 'PASS api Inactive users are blocked') {
            throw "acceptance evidence missing from evidence list"
        }

        $statusOutput = .\dist\devflow-windows-amd64.exe status --root $evidenceRoot --demand $demandId 2>&1
        $statusOutput | Tee-Object -FilePath (Join-Path $evidenceRoot 'status-output.txt') | Out-Host
        if (($statusOutput -join [Environment]::NewLine) -notmatch 'pass=1 fail=0 blocked=0') {
            throw "acceptance evidence counts missing from status"
        }
    }
    Invoke-Step "codemap smoke" {
        $codemapRoot = Join-Path $readinessRoot 'codemap-smoke'
        New-Item -ItemType Directory -Force (Join-Path $codemapRoot 'internal\coupon') | Out-Null
        @"
package coupon

type Service struct{}

func (s Service) CheckEligibility(userID string) bool {
    route := "/coupon/claim"
    _ = route
    return userID != "inactive"
}
"@ | Set-Content -Encoding UTF8 (Join-Path $codemapRoot 'internal\coupon\service.go')
        @"
package coupon

import "testing"

func TestCheckEligibilityInactiveUser(t *testing.T) {}
"@ | Set-Content -Encoding UTF8 (Join-Path $codemapRoot 'internal\coupon\service_test.go')

        .\dist\devflow-windows-amd64.exe start --root $codemapRoot --title "Coupon Eligibility" --description "Inactive users are blocked"
        .\dist\devflow-windows-amd64.exe codemap index --root $codemapRoot
        $searchOutput = .\dist\devflow-windows-amd64.exe codemap search --root $codemapRoot coupon eligibility inactive 2>&1
        $searchOutput | Out-Host
        $searchText = $searchOutput -join [Environment]::NewLine
        if ($searchText -notmatch 'internal/coupon/service.go') { throw "codemap search missing service.go" }
        if ($searchText -notmatch 'internal/coupon/service_test.go') { throw "codemap search missing service_test.go" }

        .\dist\devflow-windows-amd64.exe codemap refresh --root $codemapRoot --demand coupon-eligibility --query "coupon eligibility inactive"
        $codemapPath = Join-Path $codemapRoot '.devflow\demands\coupon-eligibility\codemap.md'
        if (-not (Test-Path $codemapPath)) { throw "codemap.md missing" }
        $codemapText = Get-Content -Raw -LiteralPath $codemapPath
        if ($codemapText -notmatch 'internal/coupon/service.go') { throw "codemap.md missing service.go" }

        $planPath = Join-Path $codemapRoot '.devflow\demands\coupon-eligibility\plan.md'
        @"
# Plan

## Implementation Steps

- Update `internal/coupon/service.go`.

## Test Strategy

- Add `internal/coupon/service_test.go` coverage.

## Risks

- Keep inactive users blocked.
"@ | Set-Content -Encoding UTF8 $planPath
        $evaluationOutput = .\dist\devflow-windows-amd64.exe evaluate --root $codemapRoot --demand coupon-eligibility --stage plan 2>&1
        $evaluationOutput | Out-Host
        if (($evaluationOutput -join [Environment]::NewLine) -notmatch 'plan.codemap_reference') { throw "plan.codemap_reference missing" }
    }
    Invoke-Step "plan grounding smoke" {
        $groundingRoot = Join-Path $readinessRoot 'plan-grounding-smoke'
        New-Item -ItemType Directory -Force (Join-Path $groundingRoot 'internal\coupon') | Out-Null
        @"
package coupon

func CheckEligibility(userID string) bool {
    return userID != ""
}
"@ | Set-Content -Encoding UTF8 (Join-Path $groundingRoot 'internal\coupon\service.go')
        @"
package coupon

import "testing"

func TestCheckEligibilityInactiveUser(t *testing.T) {}
"@ | Set-Content -Encoding UTF8 (Join-Path $groundingRoot 'internal\coupon\service_test.go')

        .\dist\devflow-windows-amd64.exe start --root $groundingRoot --title "Coupon Eligibility" --description "Inactive users are blocked"
        .\dist\devflow-windows-amd64.exe codemap index --root $groundingRoot
        .\dist\devflow-windows-amd64.exe codemap refresh --root $groundingRoot --demand coupon-eligibility --query "coupon eligibility inactive"
        .\dist\devflow-windows-amd64.exe plan-context refresh --root $groundingRoot --demand coupon-eligibility
        .\dist\devflow-windows-amd64.exe scope declare --root $groundingRoot --demand coupon-eligibility --source internal/coupon/service.go --test internal/coupon/service_test.go

        $planPath = Join-Path $groundingRoot '.devflow\demands\coupon-eligibility\plan.md'
        @"
# Plan

## Implementation Steps

- Update `internal/coupon/service.go`.

## Test Strategy

- Add `internal/coupon/service_test.go`.

## Risks

- Keep inactive users blocked.
"@ | Set-Content -Encoding UTF8 $planPath

        $evaluationOutput = .\dist\devflow-windows-amd64.exe evaluate --root $groundingRoot --demand coupon-eligibility --stage plan 2>&1
        $evaluationOutput | Out-Host
        $evaluationText = $evaluationOutput -join [Environment]::NewLine
        if ($evaluationText -notmatch 'plan.context_grounding') { throw "plan.context_grounding missing" }
        if ($evaluationText -notmatch 'plan.change_scope') { throw "plan.change_scope missing" }
    }
    Invoke-Step "deterministic dogfood" { powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repoRoot 'scripts\dogfood-local.ps1') -Version $Version }
    Invoke-Step "operator dogfood" { .\dist\devflow-windows-amd64.exe dogfood --operator-loop --root (Join-Path $readinessRoot 'operator-dogfood') --quality-root $repoRoot --quality-command "go test ./internal/version -count=1" }
    Invoke-Step "git diff check" {
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            git diff --check
        } finally {
            $ErrorActionPreference = $previousErrorActionPreference
        }
    }

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

    Add-Content -LiteralPath $report -Value "## github ci gate`nDefault readiness does not call live GitHub. Run devflow ci-gate --github-repo <owner/repo> --github-pr <number> with GITHUB_TOKEN for a live private check.`n"
}
finally {
    Pop-Location
}

Write-Host "release readiness report: $report"
