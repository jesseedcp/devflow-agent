package cli

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestStartCreatesDemandWorkspace(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer

	err := Run([]string{
		"start",
		"--root", root,
		"--title", "Add coupon check",
		"--description", "Only active members can claim coupons",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "add-coupon-check") {
		t.Fatalf("stdout = %q, want created demand slug", output)
	}

	if _, err := os.Stat(filepath.Join(root, ".devflow", "demands", "add-coupon-check", "requirements.md")); err != nil {
		t.Fatalf("requirements workspace missing: %v", err)
	}

	demand, err := artifacts.NewStore(root).LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("load demand: %v", err)
	}
	if demand.State != string(workflow.Created) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.Created)
	}
}

func TestConfirmStagesAdvanceWorkflow(t *testing.T) {
	testCases := []struct {
		name         string
		current      workflow.State
		stage        string
		artifactName string
		label        string
		next         workflow.State
	}{
		{
			name:         "requirements review advances to plan drafting",
			current:      workflow.RequirementsReview,
			stage:        "requirements",
			artifactName: artifacts.RequirementsFile,
			label:        "requirements",
			next:         workflow.PlanDrafting,
		},
		{
			name:         "plan review advances to implementation",
			current:      workflow.PlanReview,
			stage:        "plan",
			artifactName: artifacts.PlanFile,
			label:        "plan",
			next:         workflow.Implementation,
		},
		{
			name:         "verification advances to closeout",
			current:      workflow.Verification,
			stage:        "verification",
			artifactName: artifacts.VerificationFile,
			label:        "verification",
			next:         workflow.Closeout,
		},
		{
			name:         "closeout advances to completed",
			current:      workflow.Closeout,
			stage:        "closeout",
			artifactName: artifacts.CloseoutFile,
			label:        "closeout",
			next:         workflow.Completed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			var stdout bytes.Buffer

			store := setupDemandAtState(t, root, tc.current)

			err := Run([]string{
				"confirm",
				"--root", root,
				"--demand", "add-coupon-check",
				"--stage", tc.stage,
				"--by", "alice",
				"--summary", tc.label + " is accurate",
			}, &stdout, &bytes.Buffer{})
			if err != nil {
				t.Fatalf("confirm: %v", err)
			}

			if !strings.Contains(stdout.String(), tc.label+" confirmed") {
				t.Fatalf("stdout = %q, want %s confirmed", stdout.String(), tc.label)
			}

			artifactPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", tc.artifactName)
			artifactBody, err := os.ReadFile(artifactPath)
			if err != nil {
				t.Fatalf("ReadFile(%s) returned error: %v", artifactPath, err)
			}
			if !strings.Contains(string(artifactBody), "alice") {
				t.Fatalf("%s = %q, want confirmation author", tc.artifactName, string(artifactBody))
			}

			demand, err := store.LoadDemand("add-coupon-check")
			if err != nil {
				t.Fatalf("LoadDemand after confirm returned error: %v", err)
			}
			if demand.State != string(tc.next) {
				t.Fatalf("demand state = %q, want %q", demand.State, tc.next)
			}
		})
	}
}

func TestConfirmRequirementsCannotBeRepeated(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer

	store := setupDemandAtState(t, root, workflow.RequirementsReview)
	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand before first confirm returned error: %v", err)
	}
	confirmationID := expectedConfirmationID("add-coupon-check", "requirements", demand.UpdatedAt.UTC().Format(time.RFC3339Nano), "alice", "requirements are accurate")
	marker := expectedConfirmationStartMarker(confirmationID)

	firstArgs := []string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--by", "alice",
		"--summary", "requirements are accurate",
	}
	if err := Run(firstArgs, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("first confirm: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.RequirementsFile)
	eventsPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile)

	beforeRequirements, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	beforeEvents, err := readCLIEventsFile(eventsPath)
	if err != nil {
		t.Fatalf("readCLIEventsFile returned error: %v", err)
	}

	stdout.Reset()
	err = Run(firstArgs, &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("second confirm returned nil error")
	}
	if !strings.Contains(err.Error(), `confirmation stage "requirements" requires current state requirements_review, got plan_drafting`) {
		t.Fatalf("second confirm error = %v", err)
	}

	afterRequirements, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	afterEvents, err := readCLIEventsFile(eventsPath)
	if err != nil {
		t.Fatalf("readCLIEventsFile after second confirm returned error: %v", err)
	}

	if beforeCount := strings.Count(string(beforeRequirements), marker); beforeCount != 1 {
		t.Fatalf("marker count after first confirm = %d, want 1", beforeCount)
	}
	if afterCount := strings.Count(string(afterRequirements), marker); afterCount != 1 {
		t.Fatalf("marker count after second confirm = %d, want 1", afterCount)
	}
	if beforeCount := countCLIConfirmationEvents(beforeEvents, confirmationID); beforeCount != 1 {
		t.Fatalf("confirmation event count after first confirm = %d, want 1", beforeCount)
	}
	if afterCount := countCLIConfirmationEvents(afterEvents, confirmationID); afterCount != 1 {
		t.Fatalf("confirmation event count after second confirm = %d, want 1", afterCount)
	}

	demand, err = store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after repeat returned error: %v", err)
	}
	if demand.State != string(workflow.PlanDrafting) {
		t.Fatalf("demand state after repeat = %q, want %q", demand.State, workflow.PlanDrafting)
	}
}

func TestConfirmPlanRequiresPlanReviewState(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.RequirementsReview)

	var stdout bytes.Buffer
	err := Run([]string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "plan",
		"--by", "alice",
		"--summary", "plan is accurate",
	}, &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("confirm plan returned nil error")
	}
	if !strings.Contains(err.Error(), `confirmation stage "plan" requires current state plan_review, got requirements_review`) {
		t.Fatalf("confirm plan error = %v", err)
	}

	planPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.PlanFile)
	planBody, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", planPath, err)
	}
	if strings.Contains(string(planBody), "alice") {
		t.Fatalf("plan.md = %q, want no confirmation record", string(planBody))
	}

	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after failed plan confirm returned error: %v", err)
	}
	if demand.State != string(workflow.RequirementsReview) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.RequirementsReview)
	}
}

func TestConfirmRequirementsRetriesAfterEvidenceOnlyWrite(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer

	store := setupDemandAtState(t, root, workflow.RequirementsReview)
	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand before retry returned error: %v", err)
	}
	normalizedBy := normalizeConfirmationText("alice")
	normalizedSummary := normalizeConfirmationText("requirements are accurate")
	confirmationID := expectedConfirmationID("add-coupon-check", "requirements", demand.UpdatedAt.UTC().Format(time.RFC3339Nano), normalizedBy, normalizedSummary)
	confirmedAt := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	record := fmt.Sprintf("- requirements confirmed by %s at %s: %s\n", normalizedBy, confirmedAt.Format(time.RFC3339), normalizedSummary)

	if err := store.EnsureConfirmationEvidence("add-coupon-check", artifacts.RequirementsFile, confirmationID, record, artifacts.Event{
		Time:    confirmedAt,
		Type:    "stage.confirmed",
		Message: "requirements confirmed",
		Data: map[string]string{
			"by":      normalizedBy,
			"stage":   "requirements",
			"record":  record,
			"summary": normalizedSummary,
		},
	}); err != nil {
		t.Fatalf("EnsureConfirmationEvidence returned error: %v", err)
	}

	err = Run([]string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--by", "alice",
		"--summary", "requirements are accurate",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("confirm retry: %v", err)
	}

	if !strings.Contains(stdout.String(), "requirements confirmed") {
		t.Fatalf("stdout = %q, want requirements confirmed", stdout.String())
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	if count := strings.Count(string(body), expectedConfirmationStartMarker(confirmationID)); count != 1 {
		t.Fatalf("start marker count = %d, want 1", count)
	}

	events, err := readCLIEventsFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile))
	if err != nil {
		t.Fatalf("readCLIEventsFile returned error: %v", err)
	}
	if count := countCLIConfirmationEvents(events, confirmationID); count != 1 {
		t.Fatalf("confirmation event count = %d, want 1", count)
	}

	demand, err = store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after retry returned error: %v", err)
	}
	if demand.State != string(workflow.PlanDrafting) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.PlanDrafting)
	}
}

func TestConfirmRequirementsRepairsMarkerOnlyWithoutDeletingAdjacentContentBeforeAdvancing(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.RequirementsReview)
	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand before repair returned error: %v", err)
	}
	confirmationID := expectedConfirmationID("add-coupon-check", "requirements", demand.UpdatedAt.UTC().Format(time.RFC3339Nano), "alice", "requirements are accurate")
	startMarker := expectedConfirmationStartMarker(confirmationID)
	if err := store.AppendToArtifact("add-coupon-check", artifacts.RequirementsFile, "\n"+startMarker+"\n- user bullet that should stay\n"); err != nil {
		t.Fatalf("AppendToArtifact marker returned error: %v", err)
	}

	var stdout bytes.Buffer
	err = Run([]string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--by", "alice",
		"--summary", "requirements are accurate",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	if count := strings.Count(string(body), startMarker); count != 1 {
		t.Fatalf("start marker count = %d, want 1", count)
	}
	if !strings.Contains(string(body), expectedConfirmationBlockPrefix(confirmationID, "- requirements confirmed by alice at ")) {
		t.Fatalf("requirements.md = %q, want complete confirmation block", string(body))
	}
	if !strings.Contains(string(body), "- user bullet that should stay") {
		t.Fatalf("requirements.md = %q, want preserved adjacent user bullet", string(body))
	}

	demand, err = store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after confirm returned error: %v", err)
	}
	if demand.State != string(workflow.PlanDrafting) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.PlanDrafting)
	}
}

func TestConfirmRequirementsRepairsMalformedTrailingEventBeforeAdvancing(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.RequirementsReview)
	eventsPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile)
	if err := appendCLIRawFile(eventsPath, "not-json\n"); err != nil {
		t.Fatalf("appendCLIRawFile malformed tail returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--by", "alice",
		"--summary", "requirements are accurate",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}

	events, err := readCLIEventsFile(eventsPath)
	if err != nil {
		t.Fatalf("readCLIEventsFile returned error: %v", err)
	}
	if !hasCLIEventType(events, "events.repaired") {
		t.Fatal("events log does not contain events.repaired")
	}
	if !hasCLIEventType(events, "stage.confirmed") {
		t.Fatal("events log does not contain stage.confirmed")
	}

	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after confirm returned error: %v", err)
	}
	if demand.State != string(workflow.PlanDrafting) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.PlanDrafting)
	}
}

func TestConfirmRequirementsNormalizesConfirmationText(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.RequirementsReview)

	var stdout bytes.Buffer
	err := Run([]string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--by", "alice\nadmin",
		"--summary", "accurate\n- forged",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	text := string(body)
	if !strings.Contains(text, "alice admin") {
		t.Fatalf("requirements.md = %q, want normalized confirmer", text)
	}
	if !strings.Contains(text, "accurate - forged") {
		t.Fatalf("requirements.md = %q, want normalized summary", text)
	}
	if strings.Contains(text, "alice\nadmin") {
		t.Fatalf("requirements.md = %q, want no multiline confirmer", text)
	}
	if strings.Contains(text, "accurate\n- forged") {
		t.Fatalf("requirements.md = %q, want no multiline summary", text)
	}
}

func TestSlugifyGeneratesStableSafeIDForNonASCIIOnlyTitles(t *testing.T) {
	t.Parallel()

	first := slugify("新增优惠券校验")
	second := slugify("新增优惠券校验")

	if first == "demand" {
		t.Fatalf("slugify returned fallback %q, want hashed demand id", first)
	}
	if !regexp.MustCompile(`^demand-[0-9a-f]{12}$`).MatchString(first) {
		t.Fatalf("slugify = %q, want demand-<12hex>", first)
	}
	if first != second {
		t.Fatalf("slugify not stable: first %q second %q", first, second)
	}
}

func TestSlugifyDistinguishesDifferentNonASCIITitles(t *testing.T) {
	t.Parallel()

	left := slugify("新增优惠券校验")
	right := slugify("新增风险标记")
	if left == right {
		t.Fatalf("slugify collision: left %q right %q", left, right)
	}
}

func TestSlugifyAppendsHashForMixedLanguageTitles(t *testing.T) {
	t.Parallel()

	slug := slugify("新增 coupon 校验")
	if !regexp.MustCompile(`^coupon-[0-9a-f]{12}$`).MatchString(slug) {
		t.Fatalf("slugify = %q, want coupon-<12hex>", slug)
	}
}

func TestStartCreatesDistinctWorkspacesForDifferentChineseTitles(t *testing.T) {
	root := t.TempDir()
	var firstStdout bytes.Buffer
	var secondStdout bytes.Buffer

	firstTitle := "新增优惠券校验"
	secondTitle := "新增风险标记"

	if err := Run([]string{"start", "--root", root, "--title", firstTitle}, &firstStdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := Run([]string{"start", "--root", root, "--title", secondTitle}, &secondStdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("second start: %v", err)
	}

	firstID := strings.TrimSpace(strings.TrimPrefix(firstStdout.String(), "Created demand "))
	firstID = strings.SplitN(firstID, " under ", 2)[0]
	secondID := strings.TrimSpace(strings.TrimPrefix(secondStdout.String(), "Created demand "))
	secondID = strings.SplitN(secondID, " under ", 2)[0]

	if firstID == secondID {
		t.Fatalf("expected different ids, got %q and %q", firstID, secondID)
	}

	if _, err := os.Stat(filepath.Join(root, ".devflow", "demands", firstID, "requirements.md")); err != nil {
		t.Fatalf("first requirements workspace missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".devflow", "demands", secondID, "requirements.md")); err != nil {
		t.Fatalf("second requirements workspace missing: %v", err)
	}
}

func TestConfirmRequirementsPreservesHistoryAcrossReReviewCycles(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.RequirementsReview)

	firstDemand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand before first confirm returned error: %v", err)
	}
	firstID := expectedConfirmationID("add-coupon-check", "requirements", firstDemand.UpdatedAt.UTC().Format(time.RFC3339Nano), "alice", "requirements are accurate")

	if err := Run([]string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--by", "alice",
		"--summary", "requirements are accurate",
	}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("first confirm: %v", err)
	}

	advanceDemandThrough(t, store, "add-coupon-check",
		workflow.PlanReview,
		workflow.Implementation,
		workflow.MRReview,
		workflow.ReturnedToRequirements,
		workflow.RequirementsDrafting,
		workflow.RequirementsReview,
	)

	secondDemand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand before second confirm returned error: %v", err)
	}
	secondID := expectedConfirmationID("add-coupon-check", "requirements", secondDemand.UpdatedAt.UTC().Format(time.RFC3339Nano), "alice", "requirements are accurate")
	if secondID == firstID {
		t.Fatalf("re-review confirmation id = %q, want new id after persisted review cycle change", secondID)
	}

	if err := Run([]string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--by", "alice",
		"--summary", "requirements are accurate",
	}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("second confirm: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	if count := strings.Count(string(body), "<!-- devflow-confirmation:"); count != 4 {
		t.Fatalf("requirements confirmation marker line count = %d, want 4 for two bounded blocks", count)
	}
	if !strings.Contains(string(body), expectedConfirmationStartMarker(firstID)) {
		t.Fatalf("requirements.md = %q, want first confirmation block", string(body))
	}
	if !strings.Contains(string(body), expectedConfirmationStartMarker(secondID)) {
		t.Fatalf("requirements.md = %q, want second confirmation block", string(body))
	}

	events, err := readCLIEventsFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile))
	if err != nil {
		t.Fatalf("readCLIEventsFile returned error: %v", err)
	}
	if count := countCLIConfirmationEvents(events, firstID); count != 1 {
		t.Fatalf("first confirmation event count = %d, want 1", count)
	}
	if count := countCLIConfirmationEvents(events, secondID); count != 1 {
		t.Fatalf("second confirmation event count = %d, want 1", count)
	}

	finalDemand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after second confirm returned error: %v", err)
	}
	if finalDemand.State != string(workflow.PlanDrafting) {
		t.Fatalf("demand state = %q, want %q", finalDemand.State, workflow.PlanDrafting)
	}
}

func TestConfirmConcurrentRunsSerializePerDemand(t *testing.T) {
	for attempt := 0; attempt < 25; attempt++ {
		root := t.TempDir()
		store := setupDemandAtState(t, root, workflow.RequirementsReview)

		args := []string{
			"confirm",
			"--root", root,
			"--demand", "add-coupon-check",
			"--stage", "requirements",
			"--by", "alice",
			"--summary", "requirements are accurate",
		}

		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				results <- Run(args, &bytes.Buffer{}, &bytes.Buffer{})
			}()
		}
		close(start)
		wg.Wait()
		close(results)

		successes := 0
		wrongStateFailures := 0
		for err := range results {
			if err == nil {
				successes++
				continue
			}
			if strings.Contains(err.Error(), `confirmation stage "requirements" requires current state requirements_review, got plan_drafting`) {
				wrongStateFailures++
				continue
			}
			t.Fatalf("attempt %d unexpected confirm error: %v", attempt, err)
		}
		if successes != 1 || wrongStateFailures != 1 {
			t.Fatalf("attempt %d results: successes=%d wrongStateFailures=%d, want exactly one success and one wrong-state failure", attempt, successes, wrongStateFailures)
		}

		requirementsPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.RequirementsFile)
		body, err := os.ReadFile(requirementsPath)
		if err != nil {
			t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
		}
		if count := strings.Count(string(body), ":start -->"); count != 1 {
			t.Fatalf("attempt %d confirmation block count = %d, want 1", attempt, count)
		}
		if count := strings.Count(string(body), expectedConfirmationStartMarkerPrefix()); count != 2 {
			t.Fatalf("attempt %d confirmation marker line count = %d, want 2 for one bounded block", attempt, count)
		}

		events, err := readCLIEventsFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile))
		if err != nil {
			t.Fatalf("readCLIEventsFile returned error: %v", err)
		}
		stageConfirmed := 0
		for _, event := range events {
			if event.Type == "stage.confirmed" {
				stageConfirmed++
			}
		}
		if stageConfirmed != 1 {
			t.Fatalf("attempt %d stage.confirmed count = %d, want 1", attempt, stageConfirmed)
		}

		demand, err := store.LoadDemand("add-coupon-check")
		if err != nil {
			t.Fatalf("LoadDemand after concurrent confirm returned error: %v", err)
		}
		if demand.State != string(workflow.PlanDrafting) {
			t.Fatalf("attempt %d demand state = %q, want %q", attempt, demand.State, workflow.PlanDrafting)
		}
	}
}

func TestConfirmMalformedMiddleEventsFailsWithoutMutatingStateOrEvidence(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.RequirementsReview)

	demandDir := filepath.Join(root, ".devflow", "demands", "add-coupon-check")
	eventsPath := filepath.Join(demandDir, artifacts.EventsFile)
	validEvent, err := json.Marshal(artifacts.Event{
		Time:    time.Date(2026, 6, 24, 11, 0, 0, 0, time.UTC),
		Type:    "test.valid",
		Message: "valid event after corruption",
	})
	if err != nil {
		t.Fatalf("Marshal valid event returned error: %v", err)
	}
	if err := appendCLIRawFile(eventsPath, "not-json\n"+string(validEvent)+"\n"); err != nil {
		t.Fatalf("appendCLIRawFile malformed middle returned error: %v", err)
	}
	eventsBefore, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", eventsPath, err)
	}

	err = Run([]string{
		"confirm",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--by", "alice",
		"--summary", "requirements are accurate",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("confirm returned nil error for malformed middle events")
	}

	requirementsPath := filepath.Join(demandDir, artifacts.RequirementsFile)
	body, readErr := os.ReadFile(requirementsPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, readErr)
	}
	if strings.Contains(string(body), expectedConfirmationStartMarkerPrefix()) {
		t.Fatalf("requirements.md = %q, want no confirmation marker", string(body))
	}

	eventsAfter, readErr := os.ReadFile(eventsPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) after failure returned error: %v", eventsPath, readErr)
	}
	if !bytes.Equal(eventsAfter, eventsBefore) {
		t.Fatalf("events.jsonl changed on middle corruption:\nbefore %q\nafter  %q", eventsBefore, eventsAfter)
	}

	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after failed confirm returned error: %v", err)
	}
	if demand.State != string(workflow.RequirementsReview) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.RequirementsReview)
	}
}

func TestParseCommandLine(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "double quoted argument",
			input: `helper --value "hello world"`,
			want:  []string{"helper", "--value", "hello world"},
		},
		{
			name:  "single quoted argument",
			input: `helper --value 'hello world'`,
			want:  []string{"helper", "--value", "hello world"},
		},
		{
			name:  "empty quoted argument",
			input: `helper ""`,
			want:  []string{"helper", ""},
		},
		{
			name:  "adjacent quoted and unquoted fragments",
			input: `helper pre"middle"'post'`,
			want:  []string{"helper", "premiddlepost"},
		},
		{
			name:  "unquoted backslash is literal",
			input: `helper hello\ world \"quoted\"`,
			want:  []string{"helper", `hello\`, "world", `\quoted\`},
		},
		{
			name:  "backslash before non-quote is literal inside quotes",
			input: `helper "hello\ world" 'single\ quote'`,
			want:  []string{"helper", `hello\ world`, `single\ quote`},
		},
		{
			name:  "double quoted Windows executable path",
			input: `"C:\Program Files\helper.exe" --flag value`,
			want:  []string{`C:\Program Files\helper.exe`, "--flag", "value"},
		},
		{
			name:  "unquoted Windows paths",
			input: `C:\tools\helper.exe C:\temp\file.txt`,
			want:  []string{`C:\tools\helper.exe`, `C:\temp\file.txt`},
		},
		{
			name:  "unquoted UNC path",
			input: `\\server\share\tool.exe --flag`,
			want:  []string{`\\server\share\tool.exe`, "--flag"},
		},
		{
			name:  "quoted UNC path",
			input: `"\\server\share\tool.exe" --flag`,
			want:  []string{`\\server\share\tool.exe`, "--flag"},
		},
		{
			name:  "quoted trailing backslash closes argument",
			input: `"C:\temp\"`,
			want:  []string{`C:\temp\`},
		},
		{
			name:  "double quoted Windows path with escaped quotes",
			input: `"C:\temp\"quoted\"\file"`,
			want:  []string{`C:\temp"quoted"\file`},
		},
		{
			name:  "single quoted Windows path",
			input: `'C:\Program Files\helper.exe'`,
			want:  []string{`C:\Program Files\helper.exe`},
		},
		{
			name:  "trailing backslash is literal",
			input: `C:\temp\`,
			want:  []string{`C:\temp\`},
		},
		{
			name:  "repeated backslashes are preserved",
			input: `C:\\temp\\file`,
			want:  []string{`C:\\temp\\file`},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCommandLine(tc.input)
			if err != nil {
				t.Fatalf("parseCommandLine(%q) returned error: %v", tc.input, err)
			}
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Fatalf("parseCommandLine(%q) = %#v, want %#v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseCommandLineRejectsIncompleteInput(t *testing.T) {
	t.Parallel()

	for _, input := range []string{`helper "unclosed`, `helper 'unclosed`} {
		t.Run(input, func(t *testing.T) {
			_, err := parseCommandLine(input)
			if err == nil {
				t.Fatalf("parseCommandLine(%q) returned nil error", input)
			}
			if !strings.Contains(err.Error(), "command line") {
				t.Fatalf("parseCommandLine(%q) error = %v, want clear command line error", input, err)
			}
		})
	}
}

func TestVerifyWaitsForDemandLock(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.Verification)
	t.Setenv("DEVFLOW_CLI_HELPER", "args")
	executable := filepath.ToSlash(testCLIExecutable(t))
	commandText := fmt.Sprintf(`"%s" -test.run=^TestCLICommandHelper$ -- lock-check`, executable)

	lockEntered := make(chan struct{})
	releaseLock := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- store.WithDemandLock("add-coupon-check", func() error {
			close(lockEntered)
			<-releaseLock
			return nil
		})
	}()
	select {
	case <-lockEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("lock holder did not enter")
	}

	verifyDone := make(chan error, 1)
	go func() {
		verifyDone <- Run([]string{
			"verify",
			"--root", root,
			"--demand", "add-coupon-check",
			"--command", commandText,
		}, &bytes.Buffer{}, &bytes.Buffer{})
	}()

	select {
	case err := <-verifyDone:
		t.Fatalf("verify completed before lock release: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseLock)
	if err := <-lockDone; err != nil {
		t.Fatalf("lock holder returned error: %v", err)
	}
	select {
	case err := <-verifyDone:
		if err != nil {
			t.Fatalf("verify after lock release returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("verify did not finish after lock release")
	}

	verificationPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.VerificationFile)
	body, err := os.ReadFile(verificationPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", verificationPath, err)
	}
	if !strings.Contains(string(body), "Status: PASS") || !strings.Contains(string(body), `["lock-check"]`) {
		t.Fatalf("verification.md = %q, want successful helper evidence", body)
	}
}

func TestCLICommandHelper(t *testing.T) {
	mode := os.Getenv("DEVFLOW_CLI_HELPER")
	if mode == "" {
		return
	}

	if mode == "sleep" {
		time.Sleep(5 * time.Second)
		return
	}

	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		t.Fatal("helper argument separator missing")
	}
	encoded, err := json.Marshal(os.Args[separator+1:])
	if err != nil {
		t.Fatalf("Marshal helper args returned error: %v", err)
	}
	fmt.Println(string(encoded))
}

func TestVerifyRecordsPassingEvidence(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.Verification)
	t.Setenv("DEVFLOW_CLI_HELPER", "args")
	executable := filepath.ToSlash(testCLIExecutable(t))
	commandText := fmt.Sprintf(`"%s" -test.run=^TestCLICommandHelper$ -- --value "hello world" ""`, executable)

	var stdout bytes.Buffer
	err := Run([]string{
		"verify",
		"--root", root,
		"--demand", "add-coupon-check",
		"--command", commandText,
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if !strings.Contains(stdout.String(), "verification recorded for add-coupon-check: PASS") {
		t.Fatalf("stdout = %q, want PASS recording", stdout.String())
	}

	verificationPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.VerificationFile)
	body, err := os.ReadFile(verificationPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", verificationPath, err)
	}
	text := string(body)
	if !strings.Contains(text, commandText) {
		t.Fatalf("verification.md = %q, want command text", text)
	}
	if !strings.Contains(text, "Status: PASS") {
		t.Fatalf("verification.md = %q, want PASS status", text)
	}
	if !strings.Contains(text, "ExitCode: 0") {
		t.Fatalf("verification.md = %q, want exit code 0", text)
	}
	if !strings.Contains(text, `["--value","hello world",""]`) {
		t.Fatalf("verification.md = %q, want quoted argv preserved", text)
	}

	events, err := readCLIEventsFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile))
	if err != nil {
		t.Fatalf("readCLIEventsFile returned error: %v", err)
	}
	event, ok := findCLIEventType(events, "verification.recorded")
	if !ok {
		t.Fatal("events log does not contain verification.recorded")
	}
	if event.Data["command"] != commandText {
		t.Fatalf("verification command = %q, want %q", event.Data["command"], commandText)
	}
	if event.Data["status"] != "PASS" {
		t.Fatalf("verification status = %q, want PASS", event.Data["status"])
	}
	if event.Data["exit_code"] != "0" {
		t.Fatalf("verification exit_code = %q, want 0", event.Data["exit_code"])
	}
	if event.Data["failure_kind"] != "none" {
		t.Fatalf("verification failure_kind = %q, want none", event.Data["failure_kind"])
	}
	if event.Data["evidence_file"] != artifacts.VerificationFile {
		t.Fatalf("verification evidence_file = %q, want %q", event.Data["evidence_file"], artifacts.VerificationFile)
	}
	if !strings.Contains(event.Data["excerpt"], `["--value","hello world",""]`) {
		t.Fatalf("verification excerpt = %q, want helper argv", event.Data["excerpt"])
	}

	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after verify returned error: %v", err)
	}
	if demand.State != string(workflow.Verification) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.Verification)
	}
}

func TestVerifyRecordsFailingEvidenceBeforeReturningError(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.Verification)

	var stdout bytes.Buffer
	err := Run([]string{
		"verify",
		"--root", root,
		"--demand", "add-coupon-check",
		"--command", "definitely-not-a-real-command",
	}, &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("verify returned nil error")
	}
	if !strings.Contains(err.Error(), "verification command failed:") {
		t.Fatalf("verify error = %v, want verification command failed", err)
	}

	verificationPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.VerificationFile)
	body, readErr := os.ReadFile(verificationPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", verificationPath, readErr)
	}
	text := string(body)
	if !strings.Contains(text, "definitely-not-a-real-command") {
		t.Fatalf("verification.md = %q, want command text", text)
	}
	if !strings.Contains(text, "Status: FAIL") {
		t.Fatalf("verification.md = %q, want FAIL status", text)
	}
	if strings.Contains(text, "ExitCode: 0") {
		t.Fatalf("verification.md = %q, want non-zero exit code", text)
	}
	if !strings.Contains(text, "Stderr:\n    ") {
		t.Fatalf("verification.md = %q, want stderr evidence block", text)
	}

	events, readErr := readCLIEventsFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile))
	if readErr != nil {
		t.Fatalf("readCLIEventsFile returned error: %v", readErr)
	}
	event, ok := findCLIEventType(events, "verification.recorded")
	if !ok {
		t.Fatal("events log does not contain verification.recorded")
	}
	if event.Data["status"] != "FAIL" {
		t.Fatalf("verification status = %q, want FAIL", event.Data["status"])
	}
	if event.Data["exit_code"] == "0" || event.Data["exit_code"] == "" {
		t.Fatalf("verification exit_code = %q, want non-zero", event.Data["exit_code"])
	}
	if event.Data["failure_kind"] != "exec_error" {
		t.Fatalf("verification failure_kind = %q, want exec_error", event.Data["failure_kind"])
	}
	if event.Data["evidence_file"] != artifacts.VerificationFile {
		t.Fatalf("verification evidence_file = %q, want %q", event.Data["evidence_file"], artifacts.VerificationFile)
	}
	if event.Data["excerpt"] == "" {
		t.Fatal("verification excerpt = empty, want bounded failure evidence")
	}
}

func TestVerifyCommandFailureTakesPrecedenceOverStdoutError(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.Verification)

	err := Run([]string{
		"verify",
		"--root", root,
		"--demand", "add-coupon-check",
		"--command", "definitely-not-a-real-command",
	}, failingCLIWriter{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("verify returned nil error")
	}
	if !strings.Contains(err.Error(), "verification command failed:") {
		t.Fatalf("verify error = %v, want command failure to take precedence", err)
	}
}

func TestVerifyRejectsBlankCommandText(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.Verification)

	err := Run([]string{
		"verify",
		"--root", root,
		"--demand", "add-coupon-check",
		"--command", "   ",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("verify returned nil error")
	}
	if !strings.Contains(err.Error(), "--command must contain a program") {
		t.Fatalf("verify error = %v, want blank command validation", err)
	}
}

func TestVerifyRejectsEmptyQuotedProgramWithoutWritingEvidence(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.Verification)
	demandDir := filepath.Join(root, ".devflow", "demands", "add-coupon-check")
	verificationPath := filepath.Join(demandDir, artifacts.VerificationFile)
	eventsPath := filepath.Join(demandDir, artifacts.EventsFile)
	beforeVerification, err := os.ReadFile(verificationPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", verificationPath, err)
	}
	beforeEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", eventsPath, err)
	}

	err = Run([]string{
		"verify",
		"--root", root,
		"--demand", "add-coupon-check",
		"--command", `""`,
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || err.Error() != "--command must contain a program" {
		t.Fatalf("verify error = %v, want empty program validation", err)
	}

	afterVerification, readErr := os.ReadFile(verificationPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) after failure returned error: %v", verificationPath, readErr)
	}
	afterEvents, readErr := os.ReadFile(eventsPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) after failure returned error: %v", eventsPath, readErr)
	}
	if !bytes.Equal(afterVerification, beforeVerification) {
		t.Fatal("verification.md changed after empty program validation")
	}
	if !bytes.Equal(afterEvents, beforeEvents) {
		t.Fatal("events.jsonl changed after empty program validation")
	}
}

func TestVerifyRejectsUnclosedQuoteWithoutWritingEvidence(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.Verification)
	demandDir := filepath.Join(root, ".devflow", "demands", "add-coupon-check")
	verificationPath := filepath.Join(demandDir, artifacts.VerificationFile)
	eventsPath := filepath.Join(demandDir, artifacts.EventsFile)
	beforeVerification, err := os.ReadFile(verificationPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", verificationPath, err)
	}
	beforeEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", eventsPath, err)
	}

	err = Run([]string{
		"verify",
		"--root", root,
		"--demand", "add-coupon-check",
		"--command", `helper "unclosed`,
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("verify returned nil error")
	}
	if !strings.Contains(err.Error(), "unclosed") {
		t.Fatalf("verify error = %v, want unclosed quote detail", err)
	}

	afterVerification, readErr := os.ReadFile(verificationPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) after failure returned error: %v", verificationPath, readErr)
	}
	afterEvents, readErr := os.ReadFile(eventsPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) after failure returned error: %v", eventsPath, readErr)
	}
	if !bytes.Equal(afterVerification, beforeVerification) {
		t.Fatalf("verification.md changed after parse failure")
	}
	if !bytes.Equal(afterEvents, beforeEvents) {
		t.Fatalf("events.jsonl changed after parse failure")
	}
}

func TestVerifyRequiresVerificationStateWithoutWritingEvidence(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.Created)
	demandDir := filepath.Join(root, ".devflow", "demands", "add-coupon-check")
	verificationPath := filepath.Join(demandDir, artifacts.VerificationFile)
	eventsPath := filepath.Join(demandDir, artifacts.EventsFile)
	beforeVerification, err := os.ReadFile(verificationPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", verificationPath, err)
	}
	beforeEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", eventsPath, err)
	}

	err = Run([]string{
		"verify",
		"--root", root,
		"--demand", "add-coupon-check",
		"--command", "unused-command",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || err.Error() != "verify requires current state verification, got created" {
		t.Fatalf("verify error = %v, want wrong-state error", err)
	}

	afterVerification, _ := os.ReadFile(verificationPath)
	afterEvents, _ := os.ReadFile(eventsPath)
	if !bytes.Equal(afterVerification, beforeVerification) || !bytes.Equal(afterEvents, beforeEvents) {
		t.Fatal("verify wrong-state failure mutated evidence")
	}
}

func TestVerifyRecordsTimeoutFailureKind(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.Verification)
	t.Setenv("DEVFLOW_CLI_HELPER", "sleep")
	executable := filepath.ToSlash(testCLIExecutable(t))
	commandText := fmt.Sprintf(`"%s" -test.run=^TestCLICommandHelper$`, executable)

	err := runVerifyWithTimeout([]string{
		"--root", root,
		"--demand", "add-coupon-check",
		"--command", commandText,
	}, &bytes.Buffer{}, 30*time.Millisecond)
	if err == nil {
		t.Fatal("runVerifyWithTimeout returned nil error")
	}

	verificationPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.VerificationFile)
	body, readErr := os.ReadFile(verificationPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", verificationPath, readErr)
	}
	if !strings.Contains(string(body), "Status: FAIL") {
		t.Fatalf("verification.md = %q, want FAIL status", body)
	}

	events, readErr := readCLIEventsFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile))
	if readErr != nil {
		t.Fatalf("readCLIEventsFile returned error: %v", readErr)
	}
	event, ok := findCLIEventType(events, "verification.recorded")
	if !ok {
		t.Fatal("events log does not contain verification.recorded")
	}
	if event.Data["failure_kind"] != "timeout" {
		t.Fatalf("verification failure_kind = %q, want timeout", event.Data["failure_kind"])
	}
}

func TestVerificationExcerptTruncatesAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	excerpt := verificationExcerpt(quality.Result{
		Stdout: strings.Repeat("界", 200),
		Stderr: "tail",
	})
	if len(excerpt) > 512 {
		t.Fatalf("excerpt byte length = %d, want <= 512", len(excerpt))
	}
	if !utf8.ValidString(excerpt) {
		t.Fatalf("excerpt is not valid UTF-8: %q", excerpt)
	}
}

func TestCloseoutRecordsReportsAndEvent(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.Closeout)

	var stdout bytes.Buffer
	err := Run([]string{
		"closeout",
		"--root", root,
		"--demand", "add-coupon-check",
		"--result", "Delivered\nwith follow-up cleanup",
		"--knowledge", "Keep closeout notes\nshort and stable",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("closeout: %v", err)
	}

	closeoutPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.CloseoutFile)
	closeoutBody, err := os.ReadFile(closeoutPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", closeoutPath, err)
	}
	closeoutText := string(closeoutBody)
	if !strings.Contains(closeoutText, "# Closeout: Add coupon check") {
		t.Fatalf("closeout.md = %q, want title", closeoutText)
	}
	if !strings.Contains(closeoutText, "Delivered with follow-up cleanup") {
		t.Fatalf("closeout.md = %q, want normalized result", closeoutText)
	}
	if strings.Contains(closeoutText, "Delivered\nwith follow-up cleanup") {
		t.Fatalf("closeout.md = %q, want single-line result", closeoutText)
	}
	assertHeadingsInOrder(t, closeoutText, []string{
		"# Closeout: Add coupon check",
		"## 需求结果",
		"## 关键产物链接",
		"## MR 评论与处理摘要",
		"## 验收证据摘要",
		"## 稳定知识候选",
		"## 流程改进候选",
		"## 一次性材料归档",
		"## 人工确认记录",
	})
	if !strings.Contains(closeoutText, "- Keep closeout notes short and stable") {
		t.Fatalf("closeout.md = %q, want knowledge bullet", closeoutText)
	}

	memoryPath := filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.MemoryCandidatesFile)
	memoryBody, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", memoryPath, err)
	}
	memoryText := string(memoryBody)
	if !strings.Contains(memoryText, "# Memory Candidates: Add coupon check") {
		t.Fatalf("memory-candidates.md = %q, want title", memoryText)
	}
	if !strings.Contains(memoryText, "Keep closeout notes short and stable") {
		t.Fatalf("memory-candidates.md = %q, want normalized knowledge", memoryText)
	}
	if strings.Contains(memoryText, "Keep closeout notes\nshort and stable") {
		t.Fatalf("memory-candidates.md = %q, want single-line knowledge", memoryText)
	}
	assertHeadingsInOrder(t, memoryText, []string{
		"# Memory Candidates: Add coupon check",
		"## 稳定知识候选",
		"## 流程改进候选",
		"## 不进入长期知识的材料",
	})
	if !strings.Contains(memoryText, "- Keep closeout notes short and stable") {
		t.Fatalf("memory-candidates.md = %q, want knowledge bullet", memoryText)
	}

	events, err := readCLIEventsFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.EventsFile))
	if err != nil {
		t.Fatalf("readCLIEventsFile returned error: %v", err)
	}
	if _, ok := findCLIEventType(events, "closeout.created"); !ok {
		t.Fatal("events log does not contain closeout.created")
	}

	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("LoadDemand after closeout returned error: %v", err)
	}
	if demand.State != string(workflow.Closeout) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.Closeout)
	}
}

func TestCloseoutRequiresCloseoutStateWithoutWritingEvidence(t *testing.T) {
	root := t.TempDir()
	setupDemandAtState(t, root, workflow.Created)
	demandDir := filepath.Join(root, ".devflow", "demands", "add-coupon-check")
	closeoutPath := filepath.Join(demandDir, artifacts.CloseoutFile)
	memoryPath := filepath.Join(demandDir, artifacts.MemoryCandidatesFile)
	eventsPath := filepath.Join(demandDir, artifacts.EventsFile)
	beforeCloseout, _ := os.ReadFile(closeoutPath)
	beforeMemory, _ := os.ReadFile(memoryPath)
	beforeEvents, _ := os.ReadFile(eventsPath)

	err := Run([]string{
		"closeout",
		"--root", root,
		"--demand", "add-coupon-check",
		"--result", "Delivered",
		"--knowledge", "Stable knowledge",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || err.Error() != "closeout requires current state closeout, got created" {
		t.Fatalf("closeout error = %v, want wrong-state error", err)
	}

	afterCloseout, _ := os.ReadFile(closeoutPath)
	afterMemory, _ := os.ReadFile(memoryPath)
	afterEvents, _ := os.ReadFile(eventsPath)
	if !bytes.Equal(afterCloseout, beforeCloseout) || !bytes.Equal(afterMemory, beforeMemory) || !bytes.Equal(afterEvents, beforeEvents) {
		t.Fatal("closeout wrong-state failure mutated evidence")
	}
}

func TestCloseoutWaitsForDemandLock(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.Closeout)
	lockEntered := make(chan struct{})
	releaseLock := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- store.WithDemandLock("add-coupon-check", func() error {
			close(lockEntered)
			<-releaseLock
			return nil
		})
	}()
	select {
	case <-lockEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("lock holder did not enter")
	}

	closeoutDone := make(chan error, 1)
	go func() {
		closeoutDone <- Run([]string{
			"closeout",
			"--root", root,
			"--demand", "add-coupon-check",
			"--result", "Delivered",
			"--knowledge", "Stable knowledge",
		}, &bytes.Buffer{}, &bytes.Buffer{})
	}()

	select {
	case err := <-closeoutDone:
		t.Fatalf("closeout completed before lock release: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseLock)
	if err := <-lockDone; err != nil {
		t.Fatalf("lock holder returned error: %v", err)
	}
	select {
	case err := <-closeoutDone:
		if err != nil {
			t.Fatalf("closeout after lock release returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closeout did not finish after lock release")
	}
}

func setupDemandAtState(t *testing.T, root string, target workflow.State) artifacts.Store {
	t.Helper()

	var stdout bytes.Buffer
	err := Run([]string{
		"start",
		"--root", root,
		"--title", "Add coupon check",
		"--description", "Only active members can claim coupons",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	store := artifacts.NewStore(root)
	pathByTarget := map[workflow.State][]workflow.State{
		workflow.Created:              {},
		workflow.ContextLoaded:        {workflow.ContextLoaded},
		workflow.RequirementsDrafting: {workflow.ContextLoaded, workflow.RequirementsDrafting},
		workflow.RequirementsReview:   {workflow.ContextLoaded, workflow.RequirementsDrafting, workflow.RequirementsReview},
		workflow.PlanDrafting:         {workflow.ContextLoaded, workflow.RequirementsDrafting, workflow.RequirementsReview, workflow.PlanDrafting},
		workflow.PlanReview:           {workflow.ContextLoaded, workflow.RequirementsDrafting, workflow.RequirementsReview, workflow.PlanDrafting, workflow.PlanReview},
		workflow.Implementation:       {workflow.ContextLoaded, workflow.RequirementsDrafting, workflow.RequirementsReview, workflow.PlanDrafting, workflow.PlanReview, workflow.Implementation},
		workflow.MRReview:             {workflow.ContextLoaded, workflow.RequirementsDrafting, workflow.RequirementsReview, workflow.PlanDrafting, workflow.PlanReview, workflow.Implementation, workflow.MRReview},
		workflow.Verification:         {workflow.ContextLoaded, workflow.RequirementsDrafting, workflow.RequirementsReview, workflow.PlanDrafting, workflow.PlanReview, workflow.Implementation, workflow.MRReview, workflow.Verification},
		workflow.Closeout:             {workflow.ContextLoaded, workflow.RequirementsDrafting, workflow.RequirementsReview, workflow.PlanDrafting, workflow.PlanReview, workflow.Implementation, workflow.MRReview, workflow.Verification, workflow.Closeout},
		workflow.Completed:            {workflow.ContextLoaded, workflow.RequirementsDrafting, workflow.RequirementsReview, workflow.PlanDrafting, workflow.PlanReview, workflow.Implementation, workflow.MRReview, workflow.Verification, workflow.Closeout, workflow.Completed},
	}
	path, ok := pathByTarget[target]
	if !ok {
		t.Fatalf("unsupported setup target state %q", target)
	}
	advanceDemandThrough(t, store, "add-coupon-check", path...)

	return store
}

func advanceDemandThrough(t *testing.T, store artifacts.Store, demandID string, nextStates ...workflow.State) {
	t.Helper()

	demand, err := store.LoadDemand(demandID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	for _, next := range nextStates {
		advanced, err := workflow.Advance(workflow.State(demand.State), next)
		if err != nil {
			t.Fatalf("Advance(%s -> %s) returned error: %v", demand.State, next, err)
		}
		demand.State = string(advanced)
		if err := store.SaveDemand(demand); err != nil {
			t.Fatalf("SaveDemand(%s) returned error: %v", demand.State, err)
		}
		demand, err = store.LoadDemand(demandID)
		if err != nil {
			t.Fatalf("LoadDemand after SaveDemand returned error: %v", err)
		}
	}
}

func expectedConfirmationStartMarker(id string) string {
	return fmt.Sprintf("<!-- devflow-confirmation:%s:start -->", id)
}

func expectedConfirmationBlockPrefix(id, recordContains string) string {
	return expectedConfirmationStartMarker(id) + "\n" + recordContains
}

func expectedConfirmationStartMarkerPrefix() string {
	return "<!-- devflow-confirmation:"
}

func expectedConfirmationID(demandID, stage, cycleToken, by, summary string) string {
	normalizedDemandID := strings.ToLower(strings.TrimSpace(demandID))
	normalizedStage := strings.TrimSpace(stage)
	normalizedCycleToken := strings.TrimSpace(cycleToken)
	normalizedBy := normalizeConfirmationText(by)
	normalizedSummary := normalizeConfirmationText(summary)

	hash := sha256.Sum256([]byte(normalizedDemandID + "\x00" + normalizedStage + "\x00" + normalizedCycleToken + "\x00" + normalizedBy + "\x00" + normalizedSummary))
	return hex.EncodeToString(hash[:8])
}

func readCLIEventsFile(path string) ([]artifacts.Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []artifacts.Event
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		var event artifacts.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode events line %d: %w", line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func countCLIConfirmationEvents(events []artifacts.Event, id string) int {
	count := 0
	for _, event := range events {
		if event.Data["confirmation_id"] == id {
			count++
		}
	}
	return count
}

func appendCLIRawFile(path, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func hasCLIEventType(events []artifacts.Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func findCLIEventType(events []artifacts.Event, eventType string) (artifacts.Event, bool) {
	for _, event := range events {
		if event.Type == eventType {
			return event, true
		}
	}
	return artifacts.Event{}, false
}

func testCLIExecutable(t *testing.T) string {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() returned error: %v", err)
	}
	return executable
}

func assertHeadingsInOrder(t *testing.T, content string, headings []string) {
	t.Helper()

	offset := 0
	for _, heading := range headings {
		index := strings.Index(content[offset:], heading)
		if index < 0 {
			t.Fatalf("content missing heading %q after offset %d:\n%s", heading, offset, content)
		}
		offset += index + len(heading)
	}
}

type failingCLIWriter struct{}

func (failingCLIWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("stdout unavailable")
}
