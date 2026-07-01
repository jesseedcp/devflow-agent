# Wave 20 Local PRD Intake Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a deterministic `devflow intake --file <markdown>` entry point that turns a local PRD/需求文档 into a demand workspace with an `intake.md` source snapshot and review-ready `requirements.md`.

**Architecture:** Keep intake deterministic and local-only for Wave 20. A new `internal/intake` package parses Markdown/text into a structured requirements draft; the CLI command creates the demand through the existing artifact store, writes `intake.md` and `requirements.md`, moves the demand to `requirements_review`, and records intake events. Existing `confirm`, `plan`, `console`, `drive`, `evaluate`, memory, and quality gates continue unchanged.

**Tech Stack:** Go 1.25, existing `internal/artifacts`, `internal/workflow`, `internal/demandflow`, standard-library Markdown-ish parsing via line scanning, existing PowerShell release-readiness script.

---

## Scope

Wave 20 implements only local file intake:

```powershell
devflow intake --file docs/examples/demands/coupon-eligibility.md
```

Out of scope:

- URL fetching.
- WeChat/HTML parsing.
- Live LLM summarization.
- Automatic requirements confirmation.
- Direct writing to long-term memory.
- GitLab/Aone/钉钉 adapter work.

The output should be immediately usable by the existing loop:

```powershell
devflow intake --file docs/examples/demands/coupon-eligibility.md
devflow evaluate --demand coupon-eligibility --stage requirements --strict
devflow console --demand coupon-eligibility
devflow confirm --demand coupon-eligibility --stage requirements --by dd --summary "requirements accepted"
devflow run --demand coupon-eligibility --stage plan
```

---

## File Structure

- Create: `internal/intake/intake.go`
  - Owns local PRD parsing and requirements rendering.
  - No CLI, filesystem, or artifact-store dependency.

- Create: `internal/intake/intake_test.go`
  - Unit tests for title extraction, section extraction, fallback questions, acceptance criteria, and deterministic rendering.

- Modify: `internal/artifacts/model.go`
  - Add `IntakeFile = "intake.md"`.

- Modify: `internal/artifacts/store.go`
  - Create `intake.md` during demand workspace creation.
  - Allow `WriteArtifact` / `AppendToArtifact` to write `intake.md`.

- Modify: `internal/artifacts/store_test.go`
  - Assert new workspaces include `intake.md`.
  - Assert supported artifact validation accepts `IntakeFile`.

- Modify: `internal/templates/templates.go`
  - Add `Intake(title, source string) string`.

- Create: `internal/cli/intake.go`
  - Implements `devflow intake`.
  - Reads local file.
  - Derives title and demand id.
  - Creates workspace.
  - Writes source snapshot and requirements draft.
  - Advances state to `requirements_review`.
  - Prints paths and next command.

- Create: `internal/cli/intake_test.go`
  - CLI coverage for successful intake, title override, demand id override, missing file, duplicate demand, and help text wiring.

- Modify: `internal/cli/cli.go`
  - Add help text and command dispatch for `intake`.

- Modify: `internal/demandflow/workspace.go`
  - Include `intake.md` in workspace artifact summaries.

- Modify: `internal/demandflow/status_test.go`
  - Update status expectations to include `intake.md`.

- Modify: `internal/demandflow/workspace_test.go`
  - Assert console/workspace summary sees `intake.md`.

- Modify: `scripts/release-readiness.ps1`
  - Add a deterministic intake smoke after build.

- Modify: `docs/user-guide/backend-demand-loop.md`
  - Document intake as the preferred demand creation path for PRD files.

- Modify: `docs/release/v0.1.md`
  - Add Wave 20 release note.

---

## Task 1: Add Deterministic Intake Parser

**Files:**
- Create: `internal/intake/intake.go`
- Create: `internal/intake/intake_test.go`

- [ ] **Step 1: Write parser tests**

Create `internal/intake/intake_test.go`:

```go
package intake

import (
	"strings"
	"testing"
)

func TestParseMarkdownPRDExtractsRequirementsDraft(t *testing.T) {
	input := `# Coupon eligibility

## 背景
Marketing wants to block inactive users from claiming coupons.

## 目标
- Active members can claim eligible coupons.
- Inactive members are blocked with a clear reason.

## 非目标
- Do not redesign coupon creation.

## 业务规则
- User status must be active.
- Coupon must be inside the claim window.

## 验收标准
- Given an active member, claim succeeds.
- Given an inactive member, claim fails.

## 待确认
- Should expired coupons return a business code or generic error?
`

	result := ParseMarkdown(Source{
		Path: "docs/examples/demands/coupon-eligibility.md",
		Text: input,
	})

	if result.Title != "Coupon eligibility" {
		t.Fatalf("Title = %q, want Coupon eligibility", result.Title)
	}
	for _, want := range []string{
		"Active members can claim eligible coupons.",
		"User status must be active.",
		"Given an inactive member, claim fails.",
		"Should expired coupons return a business code or generic error?",
	} {
		if !strings.Contains(result.RequirementsMarkdown, want) {
			t.Fatalf("requirements missing %q:\n%s", want, result.RequirementsMarkdown)
		}
	}
	if result.Readiness != ReadinessNeedsReview {
		t.Fatalf("Readiness = %q, want %q", result.Readiness, ReadinessNeedsReview)
	}
}

func TestParseMarkdownPRDUsesFileNameWhenHeadingMissing(t *testing.T) {
	result := ParseMarkdown(Source{
		Path: "docs/examples/demands/refund-policy.md",
		Text: "Refunds should be rejected after the configured window.",
	})

	if result.Title != "refund policy" {
		t.Fatalf("Title = %q, want refund policy", result.Title)
	}
	if !strings.Contains(result.RequirementsMarkdown, "Refunds should be rejected after the configured window.") {
		t.Fatalf("requirements missing raw text:\n%s", result.RequirementsMarkdown)
	}
	if !strings.Contains(result.RequirementsMarkdown, "请确认完整业务规则") {
		t.Fatalf("requirements missing fallback confirmation question:\n%s", result.RequirementsMarkdown)
	}
}

func TestRenderIntakeSnapshotRecordsSourceAndRawText(t *testing.T) {
	result := ParseMarkdown(Source{
		Path: "prd.md",
		Text: "# Title\n\nBody",
	})

	snapshot := RenderSnapshot(result)
	for _, want := range []string{
		"# Intake: Title",
		"Source: `prd.md`",
		"## 原始需求材料",
		"Body",
	} {
		if !strings.Contains(snapshot, want) {
			t.Fatalf("snapshot missing %q:\n%s", want, snapshot)
		}
	}
}
```

- [ ] **Step 2: Run tests and verify red**

Run:

```powershell
go test ./internal/intake -count=1
```

Expected: FAIL because package `internal/intake` does not exist.

- [ ] **Step 3: Implement parser types and renderer**

Create `internal/intake/intake.go`:

```go
package intake

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Readiness string

const (
	ReadinessNeedsReview Readiness = "needs_review"
)

type Source struct {
	Path string
	Text string
}

type Result struct {
	Title                string
	SourcePath           string
	RawText              string
	Goals                []string
	NonGoals             []string
	Rules                []string
	AcceptanceCriteria   []string
	Risks                []string
	Questions            []string
	RequirementsMarkdown string
	Readiness            Readiness
}

type section struct {
	heading string
	lines   []string
}

var markdownHeading = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.+?)\s*$`)

func ParseMarkdown(src Source) Result {
	raw := normalizeNewlines(src.Text)
	title := extractTitle(raw)
	if title == "" {
		title = titleFromPath(src.Path)
	}
	sections := parseSections(raw)
	out := Result{
		Title:      title,
		SourcePath: strings.TrimSpace(src.Path),
		RawText:    strings.TrimSpace(raw),
		Readiness:  ReadinessNeedsReview,
	}
	out.Goals = collectSections(sections, "目标", "需求", "背景", "goal", "objective", "behavior")
	out.NonGoals = collectSections(sections, "非目标", "不做", "out of scope", "non-goal")
	out.Rules = collectSections(sections, "业务规则", "规则", "business rule", "rule")
	out.AcceptanceCriteria = collectSections(sections, "验收", "acceptance", "criteria")
	out.Risks = collectSections(sections, "风险", "歧义", "risk", "ambiguity")
	out.Questions = collectSections(sections, "待确认", "问题", "question", "open")
	if len(out.Goals) == 0 {
		out.Goals = fallbackBody(raw)
	}
	if len(out.Questions) == 0 {
		out.Questions = []string{"请确认完整业务规则、边界条件、异常返回和验收口径是否准确。"}
	}
	out.RequirementsMarkdown = RenderRequirements(out)
	return out
}

func RenderRequirements(result Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Requirements: %s\n\n", result.Title)
	writeBullets(&b, "## 目标行为", result.Goals)
	writeBullets(&b, "## 非目标范围", result.NonGoals)
	writeBullets(&b, "## 业务规则", result.Rules)
	writeBullets(&b, "## 用户/调用方影响", []string{"根据 intake 材料确认调用方、用户提示、错误码和兼容性影响。"})
	writeBullets(&b, "## 验收标准", result.AcceptanceCriteria)
	writeBullets(&b, "## 风险与歧义", result.Risks)
	writeBullets(&b, "## 待确认问题", result.Questions)
	b.WriteString("## 人工确认记录\n")
	return b.String()
}

func RenderSnapshot(result Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Intake: %s\n\n", result.Title)
	if result.SourcePath != "" {
		fmt.Fprintf(&b, "Source: `%s`\n\n", result.SourcePath)
	}
	fmt.Fprintf(&b, "Readiness: `%s`\n\n", result.Readiness)
	b.WriteString("## 原始需求材料\n\n")
	if strings.TrimSpace(result.RawText) == "" {
		b.WriteString("_empty intake text_\n")
	} else {
		b.WriteString(strings.TrimSpace(result.RawText))
		b.WriteString("\n")
	}
	return b.String()
}

func normalizeNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

func extractTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		match := markdownHeading.FindStringSubmatch(line)
		if len(match) == 3 && match[1] == "#" {
			return strings.TrimSpace(strings.Trim(match[2], "#"))
		}
	}
	return ""
}

func titleFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.Join(strings.Fields(base), " ")
	if base == "" || base == "." {
		return "untitled demand"
	}
	return base
}

func parseSections(text string) []section {
	var sections []section
	current := section{heading: "body"}
	for _, line := range strings.Split(text, "\n") {
		if match := markdownHeading.FindStringSubmatch(line); len(match) == 3 {
			if len(current.lines) > 0 || current.heading != "body" {
				sections = append(sections, current)
			}
			current = section{heading: strings.ToLower(strings.TrimSpace(match[2]))}
			continue
		}
		current.lines = append(current.lines, line)
	}
	if len(current.lines) > 0 || current.heading != "body" {
		sections = append(sections, current)
	}
	return sections
}

func collectSections(sections []section, needles ...string) []string {
	var out []string
	for _, sec := range sections {
		heading := strings.ToLower(sec.heading)
		if !headingMatches(heading, needles) {
			continue
		}
		out = append(out, normalizeBullets(sec.lines)...)
	}
	return compactLines(out)
}

func headingMatches(heading string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(heading, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func normalizeBullets(lines []string) []string {
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func fallbackBody(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lines = append(lines, trimmed)
	}
	return compactLines(lines)
}

func compactLines(lines []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

func writeBullets(b *strings.Builder, heading string, values []string) {
	b.WriteString(heading)
	b.WriteString("\n\n")
	if len(values) == 0 {
		b.WriteString("- 待人工补充。\n\n")
		return
	}
	for _, value := range values {
		fmt.Fprintf(b, "- %s\n", value)
	}
	b.WriteString("\n")
}
```

- [ ] **Step 4: Run parser tests and verify green**

Run:

```powershell
go test ./internal/intake -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit parser**

```powershell
git add internal/intake/intake.go internal/intake/intake_test.go
git commit -m "Parse local PRD intake into requirements drafts" -m "Local PRD intake needs deterministic behavior before live URL or LLM summarization. This package turns markdown-like demand text into a review-ready requirements draft and a source snapshot without touching CLI or artifacts." -m "Constraint: Wave 20 is local-file only; no network fetching or model calls. `nConfidence: high`nScope-risk: narrow`nDirective: Keep intake parsing deterministic until URL/live-provider support has a separate plan.`nTested: go test ./internal/intake -count=1`nNot-tested: CLI workspace creation"
```

---

## Task 2: Add `intake.md` Artifact Support

**Files:**
- Modify: `internal/artifacts/model.go`
- Modify: `internal/artifacts/store.go`
- Modify: `internal/artifacts/store_test.go`
- Modify: `internal/templates/templates.go`

- [ ] **Step 1: Add artifact tests**

In `internal/artifacts/store_test.go`, update the existing workspace creation test that checks default files. The expected list must include `IntakeFile`.

If the test currently has a literal list like this:

```go
expected := []string{
	DemandFile,
	RequirementsFile,
	PlanFile,
	ProgressFile,
	VerificationFile,
	CloseoutFile,
	MemoryCandidatesFile,
	EventsFile,
}
```

Change it to:

```go
expected := []string{
	DemandFile,
	IntakeFile,
	RequirementsFile,
	PlanFile,
	ProgressFile,
	VerificationFile,
	CloseoutFile,
	MemoryCandidatesFile,
	EventsFile,
}
```

Add this test near the other `WriteArtifact` / append validation tests:

```go
func TestWriteArtifactSupportsIntakeFile(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("intake-artifact")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	if err := store.WriteArtifact(demand.ID, IntakeFile, "# Intake\n\nraw demand"); err != nil {
		t.Fatalf("WriteArtifact intake returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, IntakeFile))
	if err != nil {
		t.Fatalf("ReadFile intake returned error: %v", err)
	}
	if string(body) != "# Intake\n\nraw demand" {
		t.Fatalf("intake.md = %q", string(body))
	}
}
```

- [ ] **Step 2: Run artifact tests and verify red**

Run:

```powershell
go test ./internal/artifacts -run "TestCreateDemandWorkspace|TestWriteArtifactSupportsIntakeFile" -count=1
```

Expected: FAIL because `IntakeFile` is undefined or unsupported.

- [ ] **Step 3: Add artifact constant**

Modify `internal/artifacts/model.go`:

```go
const (
	DemandFile           = "demand.json"
	IntakeFile           = "intake.md"
	RequirementsFile     = "requirements.md"
	PlanFile             = "plan.md"
	ProgressFile         = "progress.md"
	VerificationFile     = "verification.md"
	CloseoutFile         = "closeout.md"
	MemoryCandidatesFile = "memory-candidates.md"
	EventsFile           = "events.jsonl"
)
```

- [ ] **Step 4: Add intake template**

Modify `internal/templates/templates.go`:

```go
func Intake(title, source string) string {
	return fmt.Sprintf("# Intake: %s\n\nSource: `%s`\n\n## 原始需求材料\n", title, source)
}
```

- [ ] **Step 5: Write intake template during workspace creation**

Modify `internal/artifacts/store.go` inside `CreateDemand`, after `demand.json` is written and before `requirements.md`:

```go
if err := writeTextFile(filepath.Join(tempDir, IntakeFile), templates.Intake(demand.Title, demand.Source)); err != nil {
	return fmt.Errorf("write intake template: %w", err)
}
```

Modify `validateAppendableArtifactName`:

```go
func validateAppendableArtifactName(name string) error {
	switch name {
	case IntakeFile,
		RequirementsFile,
		PlanFile,
		ProgressFile,
		VerificationFile,
		CloseoutFile,
		MemoryCandidatesFile:
		return nil
	default:
		return fmt.Errorf("unsupported artifact %q", name)
	}
}
```

- [ ] **Step 6: Run artifact tests and verify green**

Run:

```powershell
go test ./internal/artifacts -run "TestCreateDemandWorkspace|TestWriteArtifactSupportsIntakeFile" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit artifact support**

```powershell
git add internal/artifacts/model.go internal/artifacts/store.go internal/artifacts/store_test.go internal/templates/templates.go
git commit -m "Store intake snapshots with demand artifacts" -m "Intake needs a durable source snapshot separate from generated requirements. Demand workspaces now include intake.md and the artifact store allows deterministic writes to it." -m "Constraint: Existing demand artifact safety checks must still reject arbitrary paths.`nConfidence: high`nScope-risk: narrow`nDirective: Do not mix raw intake material into requirements.md; keep source snapshots in intake.md.`nTested: go test ./internal/artifacts -run \"TestCreateDemandWorkspace|TestWriteArtifactSupportsIntakeFile\" -count=1`nNot-tested: CLI intake command"
```

---

## Task 3: Add `devflow intake --file`

**Files:**
- Create: `internal/cli/intake.go`
- Create: `internal/cli/intake_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write CLI intake tests**

Create `internal/cli/intake_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestIntakeFileCreatesReviewReadyDemand(t *testing.T) {
	root := t.TempDir()
	prdPath := filepath.Join(root, "coupon-eligibility.md")
	if err := os.WriteFile(prdPath, []byte(`# Coupon eligibility

## 目标
- Active members can claim coupons.

## 业务规则
- User status must be active.

## 验收标准
- Inactive users are blocked.
`), 0o644); err != nil {
		t.Fatalf("write prd: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"intake", "--root", root, "--file", prdPath}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("intake returned error: %v\n%s", err, stdout.String())
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand("coupon-eligibility")
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if demand.State != string(workflow.RequirementsReview) {
		t.Fatalf("state = %q, want requirements_review", demand.State)
	}
	if demand.Source != "intake:file:"+prdPath {
		t.Fatalf("source = %q", demand.Source)
	}

	requirements, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.RequirementsFile))
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	for _, want := range []string{"# Requirements: Coupon eligibility", "Active members can claim coupons.", "Inactive users are blocked."} {
		if !strings.Contains(string(requirements), want) {
			t.Fatalf("requirements missing %q:\n%s", want, string(requirements))
		}
	}

	intakeBody, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.IntakeFile))
	if err != nil {
		t.Fatalf("read intake: %v", err)
	}
	if !strings.Contains(string(intakeBody), "Source: `"+prdPath+"`") {
		t.Fatalf("intake snapshot missing source:\n%s", string(intakeBody))
	}
	if !strings.Contains(stdout.String(), "next: devflow evaluate --demand coupon-eligibility --stage requirements --strict") {
		t.Fatalf("stdout missing next command:\n%s", stdout.String())
	}
}

func TestIntakeFileAllowsTitleAndDemandOverride(t *testing.T) {
	root := t.TempDir()
	prdPath := filepath.Join(root, "rough.md")
	if err := os.WriteFile(prdPath, []byte("Raw requirement body"), 0o644); err != nil {
		t.Fatalf("write prd: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"intake",
		"--root", root,
		"--file", prdPath,
		"--title", "Manual title",
		"--demand", "manual-id",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("intake returned error: %v", err)
	}

	demand, err := artifacts.NewStore(root).LoadDemand("manual-id")
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if demand.Title != "Manual title" {
		t.Fatalf("title = %q, want Manual title", demand.Title)
	}
}

func TestIntakeFileRejectsMissingFile(t *testing.T) {
	root := t.TempDir()
	err := Run([]string{"intake", "--root", root, "--file", filepath.Join(root, "missing.md")}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "read intake file") {
		t.Fatalf("err = %v, want read intake file error", err)
	}
}

func TestIntakeFileRejectsDuplicateDemand(t *testing.T) {
	root := t.TempDir()
	prdPath := filepath.Join(root, "duplicate.md")
	if err := os.WriteFile(prdPath, []byte("# Duplicate"), 0o644); err != nil {
		t.Fatalf("write prd: %v", err)
	}
	if err := Run([]string{"intake", "--root", root, "--file", prdPath}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("first intake returned error: %v", err)
	}
	err := Run([]string{"intake", "--root", root, "--file", prdPath}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already exists", err)
	}
}

func TestHelpIncludesIntake(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	for _, want := range []string{"devflow intake --file <path>", "intake   Create a demand workspace from a local PRD file"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] **Step 2: Run CLI intake tests and verify red**

Run:

```powershell
go test ./internal/cli -run "TestIntake|TestHelpIncludesIntake" -count=1
```

Expected: FAIL because command `intake` is unknown.

- [ ] **Step 3: Implement intake command**

Create `internal/cli/intake.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/intake"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func runIntake(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("intake", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root, filePath, title, demandID string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&filePath, "file", "", "local PRD or requirements markdown file")
	fs.StringVar(&title, "title", "", "override demand title")
	fs.StringVar(&demandID, "demand", "", "override demand id")

	if err := fs.Parse(args); err != nil {
		return err
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return fmt.Errorf("--file is required")
	}

	body, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read intake file: %w", err)
	}
	result := intake.ParseMarkdown(intake.Source{Path: filePath, Text: string(body)})
	if strings.TrimSpace(title) != "" {
		result.Title = strings.TrimSpace(title)
		result.RequirementsMarkdown = intake.RenderRequirements(result)
	}
	if strings.TrimSpace(demandID) == "" {
		demandID = slugify(result.Title)
	} else {
		demandID = strings.TrimSpace(demandID)
	}

	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:          demandID,
		Title:       result.Title,
		Description: intakeDescription(result),
		Source:      "intake:file:" + filePath,
		State:       string(workflow.Created),
	}
	if err := store.CreateDemand(demand); err != nil {
		return err
	}
	if err := store.WriteArtifact(demand.ID, artifacts.IntakeFile, intake.RenderSnapshot(result)); err != nil {
		return err
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, result.RequirementsMarkdown); err != nil {
		return err
	}
	demand.State = string(workflow.RequirementsReview)
	if err := store.SaveDemand(demand); err != nil {
		return err
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{
		Type:    "intake.created",
		Message: "local PRD intake created requirements draft",
		Data: map[string]string{
			"file":      filePath,
			"readiness": string(result.Readiness),
		},
	}); err != nil {
		return err
	}

	demandDir := store.DemandDir(demand.ID)
	fmt.Fprintf(stdout, "Created intake demand %s\n", demand.ID)
	fmt.Fprintf(stdout, "root: %s\n", displayPath(root))
	fmt.Fprintf(stdout, "intake: %s\n", filepath.Join(demandDir, artifacts.IntakeFile))
	fmt.Fprintf(stdout, "requirements: %s\n", filepath.Join(demandDir, artifacts.RequirementsFile))
	fmt.Fprintf(stdout, "state: %s\n", workflow.RequirementsReview)
	fmt.Fprintf(stdout, "next: devflow evaluate --demand %s --stage requirements --strict\n", demand.ID)
	fmt.Fprintf(stdout, "then: devflow confirm --demand %s --stage requirements --by dd --summary \"requirements accepted\"\n", demand.ID)
	return nil
}

func intakeDescription(result intake.Result) string {
	if len(result.Goals) == 0 {
		return result.Title
	}
	return result.Goals[0]
}

func displayPath(root string) string {
	if strings.TrimSpace(root) == "." {
		cwd, err := os.Getwd()
		if err == nil {
			return cwd
		}
	}
	return root
}
```

- [ ] **Step 4: Wire help and command dispatch**

Modify `internal/cli/cli.go`.

In usage block, add after `devflow start`:

```text
  devflow intake --file <path>
```

In command list, add after `start`:

```text
  intake   Create a demand workspace from a local PRD file
```

In `Run`, add after `case "start":`:

```go
	case "intake":
		return runIntake(args[1:], stdout)
```

- [ ] **Step 5: Run CLI intake tests and verify green**

Run:

```powershell
go test ./internal/cli -run "TestIntake|TestHelpIncludesIntake" -count=1
```

Expected: PASS.

- [ ] **Step 6: Run focused integration smoke**

Run:

```powershell
$tmp = Join-Path $env:TEMP "devflow-wave20-intake-smoke"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $tmp | Out-Null
$prd = Join-Path $tmp "coupon-eligibility.md"
@"
# Coupon eligibility

## 目标
- Active members can claim coupons.

## 业务规则
- User status must be active.

## 验收标准
- Inactive users are blocked.
"@ | Set-Content -Encoding UTF8 $prd
go run ./cmd/devflow intake --root $tmp --file $prd
go run ./cmd/devflow evaluate --root $tmp --demand coupon-eligibility --stage requirements --strict
go run ./cmd/devflow console --root $tmp --demand coupon-eligibility
```

Expected:

- `intake` prints `Created intake demand coupon-eligibility`.
- `evaluate` exits 0 and reports requirements pass.
- `console` shows `State: requirements_review` and recommended confirmation.

- [ ] **Step 7: Commit CLI command**

```powershell
git add internal/cli/intake.go internal/cli/intake_test.go internal/cli/cli.go
git commit -m "Create demand workspaces from local PRD intake" -m "Operators need a product-shaped entry point before agent drafting. The intake command reads a local PRD, stores the source snapshot, renders requirements, and parks the demand at the existing requirements review gate." -m "Constraint: Intake must not auto-confirm requirements or call live providers.`nConfidence: high`nScope-risk: moderate`nDirective: Keep intake output review-ready, not approved; requirements_review is still a human gate.`nTested: go test ./internal/cli -run \"TestIntake|TestHelpIncludesIntake\" -count=1; manual go run intake/evaluate/console smoke`nNot-tested: URL intake"
```

---

## Task 4: Surface Intake In Status And Workbench Evidence

**Files:**
- Modify: `internal/demandflow/workspace.go`
- Modify: `internal/demandflow/status_test.go`
- Modify: `internal/demandflow/workspace_test.go`

- [ ] **Step 1: Add status/workspace expectations**

In `internal/demandflow/status_test.go`, update the status artifact expectation to include `intake.md`.

Where a test currently checks artifact names like:

```go
if artifact.Name == artifacts.RequirementsFile {
	foundRequirements = true
}
```

Add:

```go
if artifact.Name == artifacts.IntakeFile {
	foundIntake = true
}
```

And assert:

```go
if !foundIntake {
	t.Fatalf("intake artifact missing from %#v", report.Artifacts)
}
```

In `internal/demandflow/workspace_test.go`, add a small assertion to an existing workspace summary test:

```go
assertArtifactStatus(t, summary, artifacts.IntakeFile, "template")
```

If the test uses a generated intake command fixture, the expected status can be `"present"`. For a plain `CreateDemand`, use `"template"`.

- [ ] **Step 2: Run status/workspace tests and verify red**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectStatus|TestWorkspace" -count=1
```

Expected: FAIL because `intake.md` is not included in workspace artifact summaries.

- [ ] **Step 3: Include intake artifact in workspace summaries**

Modify `internal/demandflow/workspace.go`.

Find the artifact list near existing files:

```go
names := []string{"requirements", "plan", "implementation", "mr-review", "verification", "closeout"}
```

Do not add a new stage. Intake is an artifact, not a workflow stage.

Find the artifact file list:

```go
files := []string{
	artifacts.RequirementsFile,
	artifacts.PlanFile,
	artifacts.ProgressFile,
	artifacts.VerificationFile,
	artifacts.CloseoutFile,
	artifacts.MemoryCandidatesFile,
	artifacts.EventsFile,
}
```

Change it to:

```go
files := []string{
	artifacts.IntakeFile,
	artifacts.RequirementsFile,
	artifacts.PlanFile,
	artifacts.ProgressFile,
	artifacts.VerificationFile,
	artifacts.CloseoutFile,
	artifacts.MemoryCandidatesFile,
	artifacts.EventsFile,
}
```

Find artifact status classification:

```go
case artifacts.RequirementsFile:
	if stageStatus(summary, "requirements") == "confirmed" {
		return "confirmed"
	}
```

Add a preceding intake case:

```go
case artifacts.IntakeFile:
	if strings.Contains(strings.ToLower(text), "## 原始需求材料") && hasNonTemplateArtifactContent(text) {
		return "present"
	}
	return "template"
```

If `hasNonTemplateArtifactContent` does not exist, add this helper near related artifact helpers:

```go
func hasNonTemplateArtifactContent(text string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "Source:") || strings.HasPrefix(trimmed, "Readiness:") {
			continue
		}
		return true
	}
	return false
}
```

- [ ] **Step 4: Run status/workspace tests and verify green**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectStatus|TestWorkspace" -count=1
```

Expected: PASS.

- [ ] **Step 5: Run CLI status smoke**

Run:

```powershell
$tmp = Join-Path $env:TEMP "devflow-wave20-status-smoke"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $tmp | Out-Null
$prd = Join-Path $tmp "status-intake.md"
"# Status intake`n`n## 验收标准`n- pass" | Set-Content -Encoding UTF8 $prd
go run ./cmd/devflow intake --root $tmp --file $prd
go run ./cmd/devflow status --root $tmp --demand status-intake
```

Expected: status output includes `intake.md`.

- [ ] **Step 6: Commit status integration**

```powershell
git add internal/demandflow/workspace.go internal/demandflow/status_test.go internal/demandflow/workspace_test.go
git commit -m "Surface intake snapshots in demand status" -m "Operators need to see the original PRD snapshot beside generated requirements. Status and workspace summaries now report intake.md as a demand artifact without adding a new workflow stage." -m "Constraint: Intake is evidence, not a confirmation gate.`nConfidence: high`nScope-risk: narrow`nDirective: Do not add an intake workflow state unless a future plan defines an explicit human gate before requirements review.`nTested: go test ./internal/demandflow -run \"TestInspectStatus|TestWorkspace\" -count=1; manual intake/status smoke`nNot-tested: workbench visual rendering"
```

---

## Task 5: Add Release-Readiness Intake Smoke

**Files:**
- Modify: `scripts/release-readiness.ps1`

- [ ] **Step 1: Add script smoke expectation**

Open `scripts/release-readiness.ps1`. After the build/version steps and before deterministic dogfood, add an `Invoke-Step "intake smoke"` block.

Use this exact block:

```powershell
Invoke-Step "intake smoke" {
    $intakeRoot = Join-Path $readinessRoot 'intake-smoke'
    New-Item -ItemType Directory -Force $intakeRoot | Out-Null
    $prdPath = Join-Path $intakeRoot 'coupon-eligibility.md'
    @"
# Coupon eligibility

## 目标
- Active members can claim coupons.

## 业务规则
- User status must be active.

## 验收标准
- Inactive users are blocked.
"@ | Set-Content -Encoding UTF8 $prdPath

    .\dist\devflow-windows-amd64.exe intake --root $intakeRoot --file $prdPath | Tee-Object -FilePath (Join-Path $intakeRoot 'intake-output.txt') | Out-Host
    .\dist\devflow-windows-amd64.exe evaluate --root $intakeRoot --demand coupon-eligibility --stage requirements --strict
    .\dist\devflow-windows-amd64.exe console --root $intakeRoot --demand coupon-eligibility | Tee-Object -FilePath (Join-Path $intakeRoot 'console-output.txt') | Out-Host
}
```

- [ ] **Step 2: Run release-readiness and verify green**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave20
```

Expected:

- Existing Go tests pass.
- Existing vet/build/windows build pass.
- New `==> intake smoke` step passes.
- Deterministic dogfood and operator dogfood still pass.
- `git diff check` passes.

- [ ] **Step 3: Commit release-readiness gate**

```powershell
git add scripts/release-readiness.ps1
git commit -m "Gate releases with local intake smoke" -m "The new PRD intake entry point should stay part of release confidence. Release readiness now creates a local PRD, runs intake, evaluates requirements strictly, and inspects the operator console." -m "Constraint: Release readiness remains deterministic and credential-free.`nConfidence: high`nScope-risk: narrow`nTested: powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\release-readiness.ps1 -Version 0.1.0-wave20`nNot-tested: live provider intake"
```

---

## Task 6: Document Wave 20 Intake Workflow

**Files:**
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`
- Optionally modify: `docs/examples/demands/coupon-eligibility.md`

- [ ] **Step 1: Update user guide**

In `docs/user-guide/backend-demand-loop.md`, replace section `## 2. Create A Demand` with:

```markdown
## 2. Create A Demand From A Local PRD

For a PRD or rough需求文档, prefer intake:

```powershell
devflow intake --file docs/examples/demands/coupon-eligibility.md
```

`intake` creates a demand workspace, stores the original material in `intake.md`, renders review-ready `requirements.md`, and stops at `requirements_review`. It does not confirm the requirements.

After intake, inspect deterministic quality signals:

```powershell
devflow evaluate --demand coupon-eligibility --stage requirements --strict
devflow console --demand coupon-eligibility
```

If you do not have a PRD file yet, create a manual demand:

```powershell
devflow start --title "Add coupon eligibility check" --description "Only active members can claim coupons"
devflow run --demand add-coupon-eligibility-check --stage requirements
```
```

- [ ] **Step 2: Update release notes**

In `docs/release/v0.1.md`, add under the current feature bullets:

```markdown
- Adds `devflow intake --file <path>` for local PRD intake. It stores `intake.md`, renders `requirements.md`, and stops at the existing requirements review gate.
```

Under remaining risks or limitations, add:

```markdown
- Intake is local-file only in Wave 20. URL, WeChat, Aone, and DingTalk document intake remain future adapter work.
```

- [ ] **Step 3: Ensure example PRD is intake-friendly**

Open `docs/examples/demands/coupon-eligibility.md`. If it does not include `## 业务规则` and `## 验收标准`, update it to:

```markdown
# Coupon eligibility

The coupon service should prevent ineligible users from claiming a coupon. The first v0.1 dogfood demand is intentionally small enough for the backend-demand loop to process in one sitting.

## 目标

- Active members can claim coupons when the coupon is available.
- Inactive members are blocked with a clear business reason.

## 业务规则

- User status must be active before a coupon claim succeeds.
- The coupon must be inside the valid claim window.

## 验收标准

- Given an active member and an available coupon, the claim succeeds.
- Given an inactive member, the claim fails and records the eligibility reason.

## 待确认

- Confirm the exact business error code for inactive members.
```

- [ ] **Step 4: Run docs smoke**

Run:

```powershell
go run ./cmd/devflow intake --root $env:TEMP\devflow-wave20-docs-smoke --file docs\examples\demands\coupon-eligibility.md
go run ./cmd/devflow evaluate --root $env:TEMP\devflow-wave20-docs-smoke --demand coupon-eligibility --stage requirements --strict
rg -n "devflow intake|intake.md|Wave 20|local-file only" docs\user-guide\backend-demand-loop.md docs\release\v0.1.md docs\examples\demands\coupon-eligibility.md
```

Expected:

- Intake command succeeds.
- Evaluation command exits 0.
- `rg` finds all documented terms.

- [ ] **Step 5: Commit docs**

```powershell
git add docs/user-guide/backend-demand-loop.md docs/release/v0.1.md docs/examples/demands/coupon-eligibility.md
git commit -m "Document local PRD intake workflow" -m "Wave 20 makes intake the preferred demand entry point when a PRD file exists. The guide and release notes now show the intake-to-requirements-review path and keep manual start as a fallback." -m "Constraint: URL and platform document intake are explicitly out of scope for this wave.`nConfidence: high`nScope-risk: narrow`nTested: go run ./cmd/devflow intake/evaluate docs smoke; rg docs`nNot-tested: rendered documentation site"
```

---

## Final Verification

Run from the Wave 20 worktree:

```powershell
go test ./internal/intake ./internal/artifacts ./internal/cli ./internal/demandflow -count=1
go vet ./...
go build ./cmd/devflow
git diff --check
go test ./... -count=1 -timeout 5m
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave20
```

Expected:

- All commands exit 0.
- Release readiness includes the new `intake smoke` step.
- `devflow intake --file docs/examples/demands/coupon-eligibility.md` produces:
  - `.devflow/demands/coupon-eligibility/intake.md`
  - `.devflow/demands/coupon-eligibility/requirements.md`
  - demand state `requirements_review`
  - event type `intake.created`

Check worktree:

```powershell
git status --short --branch
```

Expected:

- Clean feature branch after all commits.

Open PR:

```powershell
git push -u origin wave20-intake-local-prd
gh pr create --base main --head wave20-intake-local-prd --title "Wave 20 local PRD intake" --body "Adds local-file PRD intake that stores intake.md, renders requirements.md, and stops at requirements_review. Verification: go test ./...; go vet ./...; go build ./cmd/devflow; git diff --check; release-readiness 0.1.0-wave20."
```

Wait for CI:

```powershell
gh pr view --json number,state,mergeable,statusCheckRollup,url
```

Expected:

- PR is mergeable.
- Ubuntu and Windows Go verification pass.

---

## Self-Review

- Spec coverage: This plan implements the chosen Wave 20 route, local PRD/text intake. It intentionally excludes URL/WeChat/platform intake and documents that boundary.
- Stage safety: Intake moves demands only to `requirements_review`, preserving the human confirmation gate before planning.
- Data separation: Raw source material goes to `intake.md`; generated review material goes to `requirements.md`.
- Adapter safety: No new external adapters, credentials, network calls, or dependencies.
- Test coverage: Parser unit tests, artifact tests, CLI tests, status/workspace tests, manual smoke, full suite, and release-readiness smoke are all specified.
- Placeholder scan: The plan uses concrete file names, command names, demand IDs, and test bodies. Angle-bracket notation appears only in user-facing help text and PR command examples where the CLI already uses that convention.
