param(
  [string]$BaseUrl = "http://127.0.0.1:18088",
  [string]$WorkspaceID = "ui-lifecycle-smoke",
  [string]$ArtifactRoot = ""
)

$ErrorActionPreference = "Stop"

function Invoke-Json($Method, $Path, $Body = $null) {
  $uri = "$BaseUrl$Path"
  if ($null -eq $Body) {
    return Invoke-RestMethod -Method $Method -Uri $uri
  }
  return Invoke-RestMethod -Method $Method -Uri $uri -ContentType "application/json" -Body ($Body | ConvertTo-Json -Depth 8)
}

if ($ArtifactRoot -eq "") {
  $ArtifactRoot = Join-Path $env:TEMP "devflow-ui-lifecycle-smoke"
}
New-Item -ItemType Directory -Force -Path $ArtifactRoot | Out-Null

$health = Invoke-Json GET "/api/health"
if ($health.status -ne "ok") { throw "health status was $($health.status)" }

try {
  Invoke-Json POST "/api/workspaces" @{
    id = $WorkspaceID
    name = "UI lifecycle smoke"
    artifact_root = $ArtifactRoot
  } | Out-Null
} catch {
  if ($_.Exception.Message -notmatch "already") { throw }
}

$demandKey = "coupon-ui-" + (Get-Date -Format "yyyyMMddHHmmss")
$created = Invoke-Json POST "/api/workspaces/$WorkspaceID/demands" @{
  key = $demandKey
  title = "Coupon UI lifecycle"
  description = "Inactive users must be blocked from claiming coupons."
  source = "smoke"
}
if ($created.demand.state -ne "requirements_review") { throw "created state was $($created.demand.state)" }

$confirmed = Invoke-Json POST "/api/workspaces/$WorkspaceID/demands/$demandKey/confirm" @{
  stage = "requirements"
  summary = "Requirements reviewed by smoke."
}
if ($confirmed.demand.state -ne "plan_drafting") { throw "confirm state was $($confirmed.demand.state)" }

$detail = Invoke-Json GET "/api/workspaces/$WorkspaceID/demands/$demandKey"
if (-not $detail.next_actions) { throw "expected next actions in detail" }

Write-Host "PASS platform page lifecycle API smoke: $WorkspaceID/$demandKey"
