package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/tui"
	"github.com/jesseedcp/devflow-agent/internal/templates"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

const helpText = `devflow - backend demand delivery agent

Usage:
  devflow help
  devflow version
  devflow start --title <title> --description <text>
  devflow init --provider <openai-compat|openai|anthropic>
  devflow confirm --demand <id> --stage <requirements|plan|verification|closeout> --by <name> --summary <text>
  devflow verify --demand <id> --command <program and args>
  devflow closeout --demand <id> --result <text> --knowledge <text>
  devflow status --demand <id>
  devflow next --demand <id>
  devflow doctor [--require-gitlab]
  devflow smoke --title <title> --description <text>
  devflow run --demand <id> --stage <requirements|plan|implementation|mr-review|verification|closeout>
  devflow chat
  devflow tui

Commands:
  help      Show this help text
  version   Show build version and platform metadata
  start     Create a new demand workspace
  init      Create a no-secret .devflow/config.yaml
  confirm   Record a human confirmation and advance the workflow gate
  verify    Record local verification evidence without advancing workflow
  closeout  Record closeout and memory-candidate reports without advancing workflow
  status    Show demand state, artifacts, and next actions
  next      Print the next recommended command for a demand
  doctor   Diagnose config, environment, git, and GitLab readiness
  smoke    Run an explicit local requirements-stage smoke test
  run       Run one backend-demand agent stage
  chat      Launch the interactive runtime (alias: tui)
  tui       Alias for chat
`

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return runChat(stdout, stderr)
	}
	if args[0] == "help" {
		_, err := fmt.Fprint(stdout, helpText)
		return err
	}

	switch args[0] {
	case "":
		return runChat(stdout, stderr)
	case "version":
		return runVersion(stdout)
	case "chat", "tui":
		return runChat(stdout, stderr)
	case "start":
		return runStart(args[1:], stdout)
	case "init":
		return runInit(args[1:], stdout)
	case "confirm":
		return runConfirm(args[1:], stdout)
	case "verify":
		return runVerify(args[1:], stdout)
	case "closeout":
		return runCloseout(args[1:], stdout)
	case "status":
		return runStatus(args[1:], stdout)
	case "next":
		return runNext(args[1:], stdout)
	case "doctor":
		return runDoctor(args[1:], stdout)
	case "smoke":
		return runSmoke(args[1:], stdout, stderr)
	case "run":
		return runDemandStage(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText)
	}
}

// runChat loads .devflow configuration and starts the interactive runtime
// surface. With no args, devflow runs this path; devflow chat and devflow tui
// do the same.
func runChat(stdout io.Writer, stderr io.Writer) error {
	cfg, err := config.LoadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w\n\nCreate .devflow/config.yaml with at least one provider, then run `devflow help`.", err)
	}
	model := tui.New(cfg.Providers, cfg.MCPServers, cfg.Hooks)
	return runTeaProgram(model)
}

// runTeaProgram runs the Bubble Tea program. It is a package-level variable so
// tests can stub it and avoid taking over the terminal.
var runTeaProgram = func(model tea.Model) error {
	_, err := tea.NewProgram(model).Run()
	return err
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func runStart(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root string
	var title string
	var description string

	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&title, "title", "", "demand title")
	fs.StringVar(&description, "description", "", "demand description")

	if err := fs.Parse(args); err != nil {
		return err
	}

	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return fmt.Errorf("--title is required")
	}

	demand := artifacts.Demand{
		ID:          slugify(title),
		Title:       title,
		Description: description,
		Source:      "manual",
		State:       string(workflow.Created),
	}

	store := artifacts.NewStore(root)
	if err := store.CreateDemand(demand); err != nil {
		return err
	}

	displayRoot := root
	if root == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		displayRoot = cwd
	}

	_, err := fmt.Fprintf(stdout, "Created demand %s under %s\n", demand.ID, displayRoot)
	return err
}

func runConfirm(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("confirm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root string
	var demandID string
	var stage string
	var by string
	var summary string

	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&stage, "stage", "", "stage name")
	fs.StringVar(&by, "by", "", "confirming person")
	fs.StringVar(&summary, "summary", "", "confirmation summary")

	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := demandflow.Confirm(demandflow.ConfirmOptions{
		Root:     root,
		DemandID: demandID,
		Stage:    stage,
		By:       by,
		Summary:  summary,
		Now:      time.Now,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s confirmed for %s\n", result.Label, result.DemandID)
	return err
}

func runVerify(args []string, stdout io.Writer) error {
	return runVerifyWithTimeout(args, stdout, 2*time.Minute)
}

func runVerifyWithTimeout(args []string, stdout io.Writer, timeout time.Duration) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root string
	var demandID string
	var commandText string

	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&commandText, "command", "", "verification command")

	if err := fs.Parse(args); err != nil {
		return err
	}

	root = strings.TrimSpace(root)
	demandID = strings.TrimSpace(demandID)
	commandText = strings.TrimSpace(commandText)
	if root == "" {
		root = "."
	}
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}

	parts, err := parseCommandLine(commandText)
	if err != nil {
		return err
	}
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return fmt.Errorf("--command must contain a program")
	}

	store := artifacts.NewStore(root)
	return store.WithDemandLock(demandID, func() error {
		demand, err := store.LoadDemand(demandID)
		if err != nil {
			return err
		}
		current := workflow.State(demand.State)
		if current != workflow.Verification {
			return fmt.Errorf("verify requires current state %s, got %s", workflow.Verification, current)
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		gateResult := quality.Gate{}.Run(ctx, root, quality.Command{
			Name: parts[0],
			Args: parts[1:],
		})
		result := gateResult.Results[0]
		status := "FAIL"
		if gateResult.Passed {
			status = "PASS"
		}
		failureKind := verificationFailureKind(ctx, gateResult.Passed, result)

		if err := store.WriteArtifact(demandID, artifacts.VerificationFile, verificationReport(demand.Title, commandText, status, result)); err != nil {
			return err
		}
		if err := store.AppendEvent(demandID, artifacts.Event{
			Time:    time.Now().UTC(),
			Type:    "verification.recorded",
			Message: "verification evidence recorded",
			Data: map[string]string{
				"command":       commandText,
				"status":        status,
				"exit_code":     strconv.Itoa(result.ExitCode),
				"failure_kind":  failureKind,
				"evidence_file": artifacts.VerificationFile,
				"excerpt":       verificationExcerpt(result),
			},
		}); err != nil {
			return err
		}

		_, outputErr := fmt.Fprintf(stdout, "verification recorded for %s: %s\n", demandID, status)
		if !gateResult.Passed {
			return fmt.Errorf("verification command failed: %s", verificationFailureMessage(result))
		}

		return outputErr
	})
}

func runCloseout(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("closeout", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root string
	var demandID string
	var result string
	var knowledge string

	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&result, "result", "", "delivery result summary")
	fs.StringVar(&knowledge, "knowledge", "", "stable knowledge candidate")

	if err := fs.Parse(args); err != nil {
		return err
	}

	root = strings.TrimSpace(root)
	demandID = strings.TrimSpace(demandID)
	result = strings.Join(strings.Fields(result), " ")
	knowledge = strings.Join(strings.Fields(knowledge), " ")
	if root == "" {
		root = "."
	}
	if demandID == "" || result == "" || knowledge == "" {
		return fmt.Errorf("--demand, --result, and --knowledge are required")
	}

	store := artifacts.NewStore(root)
	return store.WithDemandLock(demandID, func() error {
		demand, err := store.LoadDemand(demandID)
		if err != nil {
			return err
		}
		current := workflow.State(demand.State)
		if current != workflow.Closeout {
			return fmt.Errorf("closeout requires current state %s, got %s", workflow.Closeout, current)
		}

		memoryContent := memoryCandidatesReport(demand.Title, knowledge)
		closeoutContent := closeoutReport(demand.Title, result, knowledge)

		if err := store.WriteArtifact(demandID, artifacts.MemoryCandidatesFile, memoryContent); err != nil {
			return err
		}
		if err := store.WriteArtifact(demandID, artifacts.CloseoutFile, closeoutContent); err != nil {
			return err
		}
		if err := store.AppendEvent(demandID, artifacts.Event{
			Time:    time.Now().UTC(),
			Type:    "closeout.created",
			Message: "closeout reports recorded",
			Data: map[string]string{
				"result":    result,
				"knowledge": knowledge,
			},
		}); err != nil {
			return err
		}

		_, err = fmt.Fprintf(stdout, "closeout recorded for %s\n", demandID)
		return err
	})
}

func parseCommandLine(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	started := false
	characters := []rune(input)

	for index := 0; index < len(characters); index++ {
		character := characters[index]

		if quote == '\'' {
			if character == '\'' {
				quote = 0
				continue
			}
			current.WriteRune(character)
			started = true
			continue
		}

		if quote == '"' {
			if character == '"' {
				quote = 0
				continue
			}
			if character == '\\' && index+1 < len(characters) && characters[index+1] == '"' {
				afterQuote := index + 2
				if afterQuote >= len(characters) || unicode.IsSpace(characters[afterQuote]) {
					current.WriteRune('\\')
					index++
					quote = 0
					started = true
					continue
				}
				current.WriteRune('"')
				index++
				started = true
				continue
			}
			current.WriteRune(character)
			started = true
			continue
		}

		switch {
		case character == '\'' || character == '"':
			quote = character
			started = true
		case unicode.IsSpace(character):
			if started {
				args = append(args, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(character)
			started = true
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("command line has an unclosed %q quote", quote)
	}
	if started {
		args = append(args, current.String())
	}
	return args, nil
}

func slugify(title string) string {
	normalized := strings.ToLower(strings.TrimSpace(title))
	slug := slugPattern.ReplaceAllString(normalized, "-")
	slug = strings.Trim(slug, "-")

	hash := sha256.Sum256([]byte(normalized))
	suffix := hex.EncodeToString(hash[:6])

	if slug == "" {
		return "demand-" + suffix
	}
	if containsNonASCII(normalized) {
		return slug + "-" + suffix
	}
	return slug
}

func containsNonASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return true
		}
	}
	return false
}

func verificationReport(title, commandText, status string, result quality.Result) string {
	var builder strings.Builder
	builder.WriteString(templates.Verification(title))
	builder.WriteString("\n## Recorded Verification Evidence\n\n")
	builder.WriteString("Command: ")
	builder.WriteString(commandText)
	builder.WriteString("\n")
	builder.WriteString("Status: ")
	builder.WriteString(status)
	builder.WriteString("\n")
	builder.WriteString("ExitCode: ")
	builder.WriteString(strconv.Itoa(result.ExitCode))
	builder.WriteString("\n\n")
	builder.WriteString("Stdout:\n")
	builder.WriteString(indentEvidence(result.Stdout))
	builder.WriteString("\n")
	builder.WriteString("Stderr:\n")
	builder.WriteString(indentEvidence(result.Stderr))
	return builder.String()
}

func indentEvidence(value string) string {
	if value == "" {
		return "    (empty)\n"
	}

	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	var builder strings.Builder
	for _, line := range lines {
		if line == "" {
			builder.WriteString("    \n")
			continue
		}
		builder.WriteString("    ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return builder.String()
}

func verificationFailureMessage(result quality.Result) string {
	message := strings.TrimSpace(result.Stderr)
	if message != "" {
		return message
	}
	message = strings.TrimSpace(result.Stdout)
	if message != "" {
		return message
	}
	return "verification command failed"
}

func verificationFailureKind(ctx context.Context, passed bool, result quality.Result) string {
	if passed {
		return "none"
	}
	switch ctx.Err() {
	case context.DeadlineExceeded:
		return "timeout"
	case context.Canceled:
		return "canceled"
	}

	stderr := strings.ToLower(result.Stderr)
	execErrorMarkers := []string{
		"not found",
		"executable file not found",
		"cannot find the file specified",
		"the system cannot find",
		"系统找不到指定的文件",
	}
	for _, marker := range execErrorMarkers {
		if strings.Contains(stderr, marker) {
			return "exec_error"
		}
	}
	return "exit_nonzero"
}

func verificationExcerpt(result quality.Result) string {
	parts := make([]string, 0, 2)
	if stdout := strings.TrimSpace(result.Stdout); stdout != "" {
		parts = append(parts, stdout)
	}
	if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
		parts = append(parts, stderr)
	}
	if len(parts) == 0 {
		return "(empty)"
	}
	return truncateUTF8(strings.Join(parts, "\n"), 512)
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	truncated := []byte(value)[:maxBytes]
	for !utf8.Valid(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return string(truncated)
}

func closeoutReport(title, result, knowledge string) string {
	report := templates.Closeout(title)
	report = strings.Replace(report, "## 需求结果\n\n", "## 需求结果\n\n"+result+"\n\n", 1)
	return strings.Replace(report, "## 稳定知识候选\n\n", "## 稳定知识候选\n\n- "+knowledge+"\n\n", 1)
}

func memoryCandidatesReport(title, knowledge string) string {
	report := templates.MemoryCandidates(title)
	return strings.Replace(report, "## 稳定知识候选\n\n", "## 稳定知识候选\n\n- "+knowledge+"\n\n", 1)
}
