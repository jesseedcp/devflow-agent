# Wave 23 URL PRD Intake Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic URL intake so `devflow intake --url <url>` can create the same review-ready demand workspace as local file intake.

**Architecture:** Keep intake as a local deterministic adapter, not a new workflow state. `internal/intake` owns URL fetching and HTML-to-text normalization; `internal/cli/intake.go` chooses exactly one source (`--file` or `--url`) and reuses the existing artifact, memory recall, and requirements review flow.

**Tech Stack:** Go standard library only (`net/http`, `html`, `regexp`, `strings`, `time`), existing artifacts/demandflow/intake packages, PowerShell release readiness.

---

## File Structure

- Modify `internal/intake/intake.go`: add source URL field support and keep snapshot rendering source-neutral.
- Create `internal/intake/url.go`: fetch URL content with timeout, content-type handling, body limit, and deterministic HTML-to-text conversion.
- Modify `internal/intake/intake_test.go`: cover HTML extraction, title fallback, and fetch failure behavior.
- Modify `internal/cli/intake.go`: add `--url`, require exactly one of `--file` or `--url`, store `intake:url:<url>` source, and print URL-aware output.
- Modify `internal/cli/intake_test.go`: cover URL intake with `httptest`, mutual exclusion, and missing source.
- Modify `internal/cli/cli.go`: update help text for `devflow intake`.
- Modify `scripts/release-readiness.ps1`: add deterministic local URL intake smoke using a temporary PowerShell HTTP listener or an already built fixture if simpler.
- Modify `docs/user-guide/backend-demand-loop.md` and `docs/release/v0.1.md`: document URL intake scope and limitations.

## Task 1: URL Material Extraction

**Files:**
- Create: `internal/intake/url.go`
- Modify: `internal/intake/intake.go`
- Modify: `internal/intake/intake_test.go`

- [ ] Step 1: Add failing tests for HTML title/body extraction and URL source rendering.
- [ ] Step 2: Run `go test ./internal/intake -run "TestURL|TestParseHTML" -count=1` and verify it fails because URL helpers do not exist.
- [ ] Step 3: Implement minimal `FetchURL`, `HTMLToText`, and `ParseHTML` helpers using standard library only.
- [ ] Step 4: Run `gofmt -w internal/intake/*.go` and `go test ./internal/intake -count=1`.
- [ ] Step 5: Commit with Lore trailers.

## Task 2: CLI `devflow intake --url`

**Files:**
- Modify: `internal/cli/intake.go`
- Modify: `internal/cli/intake_test.go`
- Modify: `internal/cli/cli.go`

- [ ] Step 1: Add failing CLI tests for `intake --url`, `--file`/`--url` mutual exclusion, and missing source error.
- [ ] Step 2: Run focused CLI tests and verify they fail for missing `--url` behavior.
- [ ] Step 3: Refactor `runIntake` around a small source loader that returns parsed intake result plus source label.
- [ ] Step 4: Run `gofmt -w internal/cli/*.go` and `go test ./internal/cli -run "TestIntake|TestHelpIncludesIntake" -count=1`.
- [ ] Step 5: Commit with Lore trailers.

## Task 3: Release Gate And Docs

**Files:**
- Modify: `scripts/release-readiness.ps1`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] Step 1: Add release-readiness smoke for URL intake against a deterministic local HTML fixture served by the script.
- [ ] Step 2: Document that URL intake fetches accessible HTTP(S) pages and does not bypass login, enterprise permissions, JavaScript-only rendering, or WeChat anti-scraping.
- [ ] Step 3: Run docs grep and release readiness.
- [ ] Step 4: Commit with Lore trailers.

## Task 4: Final Verification And PR

**Files:**
- No code edits unless verification exposes a defect.

- [ ] Step 1: Run `go test ./internal/intake ./internal/cli -count=1`.
- [ ] Step 2: Run `go vet ./...`.
- [ ] Step 3: Run `go build ./cmd/devflow`.
- [ ] Step 4: Run `git diff --check`.
- [ ] Step 5: Run `go test ./... -count=1 -timeout 5m`.
- [ ] Step 6: Run `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave23`.
- [ ] Step 7: Manually smoke `devflow intake --url` against a local HTML server and verify `intake.md`, `requirements.md`, `context.md`, `evaluate`, `console`, and `workbench --snapshot`.
- [ ] Step 8: Push branch and open PR after local gates pass.

## Self-Review

- Scope coverage: This implements URL PRD intake only; it intentionally does not add demand-platform APIs, authenticated browser scraping, or semantic extraction.
- Placeholder scan: No implementation step relies on TBD behavior.
- Risk boundary: URL content is untrusted input; it is stored as Markdown text only and never executed.
