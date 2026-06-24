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

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
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
