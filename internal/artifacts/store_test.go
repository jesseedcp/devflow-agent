package artifacts

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateDemandWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)

	demand := testDemand("add-coupon-check")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	demandDir := filepath.Join(root, ".devflow", "demands", demand.ID)
	files := []string{
		DemandFile,
		IntakeFile,
		ContextFile,
		RequirementsFile,
		PlanFile,
		ProgressFile,
		VerificationFile,
		CloseoutFile,
		MemoryCandidatesFile,
		EventsFile,
	}

	for _, name := range files {
		path := filepath.Join(demandDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func TestCreateDemandRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	invalidIDs := []string{
		"../escape",
		"..\\escape",
		"a/b",
		"a\\b",
		"-bad",
		"bad-",
		"bad--id",
		"Upper",
		".",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			store := NewStore(root)

			err := store.CreateDemand(testDemand(id))
			if err == nil {
				t.Fatalf("CreateDemand(%q) returned nil error", id)
			}
			if !strings.Contains(err.Error(), "invalid demand id") {
				t.Fatalf("CreateDemand(%q) error = %q, want invalid demand id", id, err)
			}
		})
	}
}

func TestCreateDemandRequiresStoreRoot(t *testing.T) {
	t.Parallel()

	store := NewStore("")

	err := store.CreateDemand(testDemand("add-coupon-check"))
	if err == nil {
		t.Fatal("CreateDemand returned nil error")
	}
	if !strings.Contains(err.Error(), "store root is required") {
		t.Fatalf("CreateDemand error = %q, want store root is required", err)
	}
}

func TestPublicIORejectsInvalidDemandID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)

	if _, err := store.LoadDemand("../escape"); err == nil || !strings.Contains(err.Error(), "invalid demand id") {
		t.Fatalf("LoadDemand error = %v, want invalid demand id", err)
	}

	err := store.SaveDemand(testDemand("../escape"))
	if err == nil || !strings.Contains(err.Error(), "invalid demand id") {
		t.Fatalf("SaveDemand error = %v, want invalid demand id", err)
	}

	err = store.AppendEvent("../escape", Event{Type: "marker", Message: "marker"})
	if err == nil || !strings.Contains(err.Error(), "invalid demand id") {
		t.Fatalf("AppendEvent error = %v, want invalid demand id", err)
	}

	err = store.AppendToArtifact("../escape", RequirementsFile, "marker")
	if err == nil || !strings.Contains(err.Error(), "invalid demand id") {
		t.Fatalf("AppendToArtifact invalid id error = %v, want invalid demand id", err)
	}
}

func TestAppendToArtifactRejectsUnsupportedArtifactName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := store.AppendToArtifact(demand.ID, "../outside", "marker")
	if err == nil {
		t.Fatal("AppendToArtifact returned nil error for unsupported artifact")
	}
	if !strings.Contains(err.Error(), `unsupported artifact "../outside"`) {
		t.Fatalf("AppendToArtifact error = %v, want unsupported artifact error", err)
	}
}

func TestAppendToArtifactRejectsEventsFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := store.AppendToArtifact(demand.ID, EventsFile, "not-json")
	if err == nil {
		t.Fatal("AppendToArtifact returned nil error for events.jsonl")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf(`unsupported artifact %q`, EventsFile)) {
		t.Fatalf("AppendToArtifact error = %v, want unsupported artifact events.jsonl", err)
	}
}

func TestAppendToArtifactAppendsContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	appendText := "\nConfirmed by alice\n"
	if err := store.AppendToArtifact(demand.ID, RequirementsFile, appendText); err != nil {
		t.Fatalf("AppendToArtifact returned error: %v", err)
	}

	path := filepath.Join(root, ".devflow", "demands", demand.ID, RequirementsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", path, err)
	}
	if !strings.HasSuffix(string(data), appendText) {
		t.Fatalf("requirements.md suffix mismatch: got %q, want suffix %q", string(data), appendText)
	}
}

func TestWriteArtifactOverwritesContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	updated := "# Verification: Add risk flag\n\nOverwritten content\n"
	if err := store.WriteArtifact(demand.ID, VerificationFile, updated); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	path := filepath.Join(root, ".devflow", "demands", demand.ID, VerificationFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", path, err)
	}
	if string(data) != updated {
		t.Fatalf("verification.md = %q, want %q", string(data), updated)
	}
}

func TestWriteArtifactRejectsInvalidDemandID(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())

	err := store.WriteArtifact("../escape", VerificationFile, "marker")
	if err == nil {
		t.Fatal("WriteArtifact returned nil error")
	}
	if !strings.Contains(err.Error(), "invalid demand id") {
		t.Fatalf("WriteArtifact error = %v, want invalid demand id", err)
	}
}

func TestWriteArtifactRejectsUnsupportedArtifactNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	invalidNames := []string{EventsFile, "../x"}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := store.WriteArtifact(demand.ID, name, "marker")
			if err == nil {
				t.Fatalf("WriteArtifact(%q) returned nil error", name)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf(`unsupported artifact %q`, name)) {
				t.Fatalf("WriteArtifact(%q) error = %v, want unsupported artifact", name, err)
			}
		})
	}
}

func TestWriteArtifactRequiresExistingWorkspace(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())

	err := store.WriteArtifact("risk-flag", VerificationFile, "marker")
	if err == nil {
		t.Fatal("WriteArtifact returned nil error")
	}
	if !strings.Contains(err.Error(), "demand risk-flag does not exist") {
		t.Fatalf("WriteArtifact error = %v, want missing workspace detail", err)
	}
}

func TestWriteArtifactSupportsIntakeFile(t *testing.T) {
	t.Parallel()

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
func TestWriteArtifactSupportsContextFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("context-artifact")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	if err := store.WriteArtifact(demand.ID, ContextFile, "# Context\n\nmemory recall"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, ContextFile))
	if err != nil {
		t.Fatalf("ReadFile context returned error: %v", err)
	}
	if string(body) != "# Context\n\nmemory recall" {
		t.Fatalf("context.md = %q", string(body))
	}
}
func TestEnsureConfirmationEvidenceIsIdempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	confirmationID := "abc123def4567890"
	record := "- requirements confirmed by alice at 2026-06-24T12:00:00Z: requirements are accurate\n"
	event := Event{
		Time:    time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
		Type:    "stage.confirmed",
		Message: "requirements confirmed",
		Data: map[string]string{
			"by":      "alice",
			"record":  record,
			"stage":   "requirements",
			"summary": "requirements are accurate",
		},
	}

	if err := store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, record, event); err != nil {
		t.Fatalf("EnsureConfirmationEvidence first call returned error: %v", err)
	}
	if err := store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, record, event); err != nil {
		t.Fatalf("EnsureConfirmationEvidence second call returned error: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", demand.ID, RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	startMarker := expectedConfirmationStartMarker(confirmationID)
	if count := strings.Count(string(body), startMarker); count != 1 {
		t.Fatalf("start marker count = %d, want 1", count)
	}

	events, err := readEventsFile(filepath.Join(root, ".devflow", "demands", demand.ID, EventsFile))
	if err != nil {
		t.Fatalf("readEventsFile returned error: %v", err)
	}
	if count := countConfirmationID(events, confirmationID); count != 1 {
		t.Fatalf("confirmation event count = %d, want 1", count)
	}
}

func TestWithDemandLockWaitsForLiveOwnerEvenIfLockFileLooksStale(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	lockPath := filepath.Join(root, ".devflow", "demands", demand.ID, demandLockFile)
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- store.WithDemandLock(demand.ID, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()

	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first lock holder did not enter critical section")
	}

	stale := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(lockPath, stale, stale); err != nil {
		t.Fatalf("Chtimes(%s) returned error: %v", lockPath, err)
	}

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- store.WithDemandLock(demand.ID, func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second lock holder entered before live owner released the lock")
	case <-time.After(200 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first WithDemandLock returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first lock holder to finish")
	}

	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second WithDemandLock returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second lock holder to finish")
	}

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("Stat(%s) returned error: %v, want persistent lock file", lockPath, err)
	}
}

func TestWithFileLockTimesOutWhileHeld(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), demandLockFile)

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- withFileLock(lockPath, time.Second, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()

	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first withFileLock call did not acquire the lock")
	}

	entered := false
	err := withFileLock(lockPath, 100*time.Millisecond, func() error {
		entered = true
		return nil
	})
	if err == nil {
		t.Fatal("withFileLock returned nil error, want timeout")
	}
	if err.Error() != "timed out waiting for demand lock" {
		t.Fatalf("withFileLock timeout error = %q, want %q", err, "timed out waiting for demand lock")
	}
	if entered {
		t.Fatal("withFileLock entered critical section despite timeout")
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first withFileLock returned error: %v", err)
	}
}

func TestWithFileLockReleasesWhenOwnerProcessExits(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), demandLockFile)

	cmd := exec.Command(os.Args[0], "-test.run", "^TestWithFileLockHelperProcess$")
	cmd.Env = append(os.Environ(),
		"DEVFLOW_LOCK_HELPER=1",
		"DEVFLOW_LOCK_PATH="+lockPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process returned error: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("Stat(%s) after helper exit returned error: %v, want persistent lock file", lockPath, err)
	}

	acquired := false
	if err := withFileLock(lockPath, 500*time.Millisecond, func() error {
		acquired = true
		return nil
	}); err != nil {
		t.Fatalf("withFileLock after owner exit returned error: %v", err)
	}
	if !acquired {
		t.Fatal("withFileLock did not enter critical section after owner process exit")
	}

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("Stat(%s) after parent release returned error: %v, want persistent lock file", lockPath, err)
	}
}

func TestWithFileLockHelperProcess(t *testing.T) {
	if os.Getenv("DEVFLOW_LOCK_HELPER") != "1" {
		return
	}

	lockPath := os.Getenv("DEVFLOW_LOCK_PATH")
	if lockPath == "" {
		t.Fatal("DEVFLOW_LOCK_PATH is required")
	}

	if err := withFileLock(lockPath, time.Second, func() error {
		os.Exit(0)
		return nil
	}); err != nil {
		t.Fatalf("withFileLock helper returned error: %v", err)
	}
}

func TestEnsureConfirmationEvidenceRepairsMarkerOnlyArtifactWithoutDeletingAdjacentContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	confirmationID, record, event := testConfirmationEvidence()
	startMarker := expectedConfirmationStartMarker(confirmationID)
	if err := store.AppendToArtifact(demand.ID, RequirementsFile, "\n"+startMarker+"\n- user bullet that should stay\n"); err != nil {
		t.Fatalf("AppendToArtifact marker returned error: %v", err)
	}

	if err := store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, record, event); err != nil {
		t.Fatalf("EnsureConfirmationEvidence returned error: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", demand.ID, RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	block := expectedConfirmationBlock(confirmationID, record)
	if !strings.Contains(string(body), block) {
		t.Fatalf("requirements.md = %q, want complete block %q", string(body), block)
	}
	if !strings.Contains(string(body), "- user bullet that should stay") {
		t.Fatalf("requirements.md = %q, want preserved adjacent user content", string(body))
	}
	if count := strings.Count(string(body), startMarker); count != 1 {
		t.Fatalf("start marker count = %d, want 1", count)
	}
}

func TestEnsureConfirmationEvidenceReplacesBoundedMismatchedArtifactBlock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	confirmationID, record, event := testConfirmationEvidence()
	startMarker := expectedConfirmationStartMarker(confirmationID)
	endMarker := expectedConfirmationEndMarker(confirmationID)
	wrongRecord := "- requirements confirmed by mallory at 2026-06-24T12:00:00Z: forged approval"
	if err := store.AppendToArtifact(demand.ID, RequirementsFile, "\n"+startMarker+"\n"+wrongRecord+"\n"+endMarker+"\n"); err != nil {
		t.Fatalf("AppendToArtifact wrong block returned error: %v", err)
	}

	if err := store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, record, event); err != nil {
		t.Fatalf("EnsureConfirmationEvidence returned error: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", demand.ID, RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	block := expectedConfirmationBlock(confirmationID, record)
	if !strings.Contains(string(body), block) {
		t.Fatalf("requirements.md = %q, want complete block %q", string(body), block)
	}
	if strings.Contains(string(body), wrongRecord) {
		t.Fatalf("requirements.md = %q, want mismatched record removed", string(body))
	}
	if count := strings.Count(string(body), startMarker); count != 1 {
		t.Fatalf("start marker count = %d, want 1", count)
	}
}

func TestEnsureConfirmationEvidenceDeduplicatesBoundedArtifactBlocks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	confirmationID, record, event := testConfirmationEvidence()
	startMarker := expectedConfirmationStartMarker(confirmationID)
	endMarker := expectedConfirmationEndMarker(confirmationID)
	duplicateBlocks := "\n" + startMarker + "\n" + strings.TrimSpace(record) + "\n" + endMarker + "\n" +
		startMarker + "\n- requirements confirmed by mallory at 2026-06-24T12:00:00Z: forged approval\n" + endMarker + "\n"
	if err := store.AppendToArtifact(demand.ID, RequirementsFile, duplicateBlocks); err != nil {
		t.Fatalf("AppendToArtifact duplicate blocks returned error: %v", err)
	}

	if err := store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, record, event); err != nil {
		t.Fatalf("EnsureConfirmationEvidence returned error: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", demand.ID, RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	if count := strings.Count(string(body), startMarker); count != 1 {
		t.Fatalf("start marker count = %d, want 1", count)
	}
	if count := strings.Count(string(body), strings.TrimSpace(record)); count != 1 {
		t.Fatalf("record count = %d, want 1", count)
	}
	if strings.Contains(string(body), "forged approval") {
		t.Fatalf("requirements.md = %q, want duplicate mismatched record removed", string(body))
	}
}

func TestEnsureConfirmationEvidenceReusesRecordedRecordOnRetry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	confirmationID, record, event := testConfirmationEvidence()
	if err := store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, record, event); err != nil {
		t.Fatalf("EnsureConfirmationEvidence first call returned error: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	retryRecord := "- requirements confirmed by alice at 2026-06-24T12:00:05Z: requirements are accurate\n"
	retryEvent := Event{
		Time:    time.Date(2026, 6, 24, 12, 0, 5, 0, time.UTC),
		Type:    "stage.confirmed",
		Message: "requirements confirmed",
		Data: map[string]string{
			"by":      "alice",
			"record":  retryRecord,
			"stage":   "requirements",
			"summary": "requirements are accurate",
		},
	}
	if err := store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, retryRecord, retryEvent); err != nil {
		t.Fatalf("EnsureConfirmationEvidence retry returned error: %v", err)
	}

	requirementsPath := filepath.Join(root, ".devflow", "demands", demand.ID, RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	if !strings.Contains(string(body), expectedConfirmationBlock(confirmationID, record)) {
		t.Fatalf("requirements.md = %q, want original persisted record", string(body))
	}
	if strings.Contains(string(body), strings.TrimSpace(retryRecord)) {
		t.Fatalf("requirements.md = %q, want retry record not to overwrite persisted record", string(body))
	}

	events, err := readEventsFile(filepath.Join(root, ".devflow", "demands", demand.ID, EventsFile))
	if err != nil {
		t.Fatalf("readEventsFile returned error: %v", err)
	}
	if count := countConfirmationID(events, confirmationID); count != 1 {
		t.Fatalf("confirmation event count = %d, want 1", count)
	}
}

func TestEnsureConfirmationEvidenceRepairsMalformedTrailingEvent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	demandDir := filepath.Join(root, ".devflow", "demands", demand.ID)
	eventsPath := filepath.Join(demandDir, EventsFile)
	if err := appendRawFile(eventsPath, "not-json\n"); err != nil {
		t.Fatalf("appendRawFile malformed tail returned error: %v", err)
	}

	confirmationID, record, event := testConfirmationEvidence()
	if err := store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, record, event); err != nil {
		t.Fatalf("EnsureConfirmationEvidence returned error: %v", err)
	}

	events, err := readEventsFile(eventsPath)
	if err != nil {
		t.Fatalf("readEventsFile after repair returned error: %v", err)
	}
	if !hasEventType(events, "events.repaired") {
		t.Fatal("events log does not contain events.repaired")
	}
	if !hasEventType(events, "stage.confirmed") {
		t.Fatal("events log does not contain stage.confirmed")
	}
	if len(events) != 3 || events[0].Type != "demand.created" {
		t.Fatalf("events = %#v, want preserved demand.created plus repair and confirmation", events)
	}
	repairEvent, ok := findEventType(events, "events.repaired")
	if !ok {
		t.Fatal("events log does not contain events.repaired")
	}
	badLineHash := sha256.Sum256([]byte("not-json"))
	if repairEvent.Data["sha256"] != fmt.Sprintf("%x", badLineHash) {
		t.Fatalf("repair sha256 = %q, want %x", repairEvent.Data["sha256"], badLineHash)
	}
	if repairEvent.Data["line"] != "2" {
		t.Fatalf("repair line = %q, want 2", repairEvent.Data["line"])
	}

	requirementsPath := filepath.Join(demandDir, RequirementsFile)
	body, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	block := expectedConfirmationBlock(confirmationID, record)
	if !strings.Contains(string(body), block) {
		t.Fatalf("requirements.md = %q, want complete block %q", string(body), block)
	}
}

func TestEnsureConfirmationEvidenceRejectsMalformedMiddleEventBeforeArtifactWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	demandDir := filepath.Join(root, ".devflow", "demands", demand.ID)
	eventsPath := filepath.Join(demandDir, EventsFile)
	validEvent, err := json.Marshal(Event{
		Time:    time.Date(2026, 6, 24, 11, 0, 0, 0, time.UTC),
		Type:    "test.valid",
		Message: "valid event after corruption",
	})
	if err != nil {
		t.Fatalf("Marshal valid event returned error: %v", err)
	}
	if err := appendRawFile(eventsPath, "not-json\n"+string(validEvent)+"\n"); err != nil {
		t.Fatalf("appendRawFile malformed middle returned error: %v", err)
	}
	eventsBefore, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", eventsPath, err)
	}

	confirmationID, record, event := testConfirmationEvidence()
	err = store.EnsureConfirmationEvidence(demand.ID, RequirementsFile, confirmationID, record, event)
	if err == nil {
		t.Fatal("EnsureConfirmationEvidence returned nil error for malformed middle event")
	}

	requirementsPath := filepath.Join(demandDir, RequirementsFile)
	body, readErr := os.ReadFile(requirementsPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, readErr)
	}
	startMarker := expectedConfirmationStartMarker(confirmationID)
	if strings.Contains(string(body), startMarker) {
		t.Fatalf("requirements.md = %q, want no confirmation marker", string(body))
	}

	eventsAfter, readErr := os.ReadFile(eventsPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s) after failure returned error: %v", eventsPath, readErr)
	}
	if !bytes.Equal(eventsAfter, eventsBefore) {
		t.Fatalf("events.jsonl changed on middle corruption:\nbefore %q\nafter  %q", eventsBefore, eventsAfter)
	}
}

func TestCreateDemandRejectsSymlinkedDemandRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	devflowLink := filepath.Join(root, ".devflow")

	if err := os.Symlink(outside, devflowLink); err != nil {
		t.Skipf("symlink setup unavailable: %v", err)
	}

	store := NewStore(root)
	err := store.CreateDemand(testDemand("risk-flag"))
	if err == nil {
		t.Fatal("CreateDemand returned nil error")
	}

	outsideArtifactsBase := filepath.Join(outside, "demands")
	if _, statErr := os.Stat(outsideArtifactsBase); !os.IsNotExist(statErr) {
		t.Fatalf("expected no escaped artifacts base, stat error = %v", statErr)
	}
}

func TestCreateDemandDoesNotOverwriteExistingWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("first CreateDemand returned error: %v", err)
	}

	demandDir := filepath.Join(root, ".devflow", "demands", demand.ID)
	requirementsPath := filepath.Join(demandDir, RequirementsFile)
	marker := "# marker\n"
	if err := os.WriteFile(requirementsPath, []byte(marker), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", requirementsPath, err)
	}

	if err := store.AppendEvent(demand.ID, Event{Type: "marker", Message: "preserve me"}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	err := store.CreateDemand(demand)
	if err == nil {
		t.Fatal("second CreateDemand returned nil error")
	}
	if !strings.Contains(err.Error(), "demand risk-flag already exists") {
		t.Fatalf("second CreateDemand error = %q, want already exists", err)
	}

	gotRequirements, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", requirementsPath, err)
	}
	if string(gotRequirements) != marker {
		t.Fatalf("requirements.md = %q, want %q", gotRequirements, marker)
	}

	eventsPath := filepath.Join(demandDir, EventsFile)
	eventsData, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", eventsPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(eventsData)), "\n")
	if len(lines) != 2 {
		t.Fatalf("events line count = %d, want 2", len(lines))
	}
}

func TestLoadDemand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)

	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	loaded, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if loaded.Title != demand.Title {
		t.Fatalf("loaded Title = %q, want %q", loaded.Title, demand.Title)
	}
	if loaded.Description != demand.Description {
		t.Fatalf("loaded Description = %q, want %q", loaded.Description, demand.Description)
	}
	if loaded.Source != demand.Source {
		t.Fatalf("loaded Source = %q, want %q", loaded.Source, demand.Source)
	}
	if loaded.State != demand.State {
		t.Fatalf("loaded State = %q, want %q", loaded.State, demand.State)
	}
}

func TestCreateDemandWritesInitialEvent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("add-coupon-check")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	eventsPath := filepath.Join(root, ".devflow", "demands", demand.ID, EventsFile)
	file, err := os.Open(eventsPath)
	if err != nil {
		t.Fatalf("Open(%s) returned error: %v", eventsPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("events.jsonl did not contain an initial event")
	}

	var event Event
	if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
		t.Fatalf("Unmarshal(initial event) returned error: %v", err)
	}
	if event.Type != "demand.created" {
		t.Fatalf("initial event Type = %q, want %q", event.Type, "demand.created")
	}
	if event.Message != "demand workspace created" {
		t.Fatalf("initial event Message = %q, want %q", event.Message, "demand workspace created")
	}
	if event.Time.IsZero() {
		t.Fatal("initial event Time is zero")
	}
	if scanner.Scan() {
		t.Fatal("events.jsonl contained unexpected extra initial events")
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner returned error: %v", err)
	}
}

func TestSaveDemandPersistsUpdates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("risk-flag")

	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	loaded, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	oldUpdatedAt := loaded.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	loaded.Description = "Updated description"
	loaded.State = "context_loaded"
	if err := store.SaveDemand(loaded); err != nil {
		t.Fatalf("SaveDemand returned error: %v", err)
	}

	saved, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand after SaveDemand returned error: %v", err)
	}
	if saved.Description != "Updated description" {
		t.Fatalf("saved Description = %q, want updated description", saved.Description)
	}
	if saved.State != "context_loaded" {
		t.Fatalf("saved State = %q, want %q", saved.State, "context_loaded")
	}
	if saved.UpdatedAt.Before(oldUpdatedAt) {
		t.Fatalf("saved UpdatedAt = %s, want >= %s", saved.UpdatedAt, oldUpdatedAt)
	}
}

func testDemand(id string) Demand {
	return Demand{
		ID:          id,
		Title:       "Add risk flag",
		Description: "Flag risky requests before approval",
		Source:      "manual",
		State:       "created",
	}
}

func readEventsFile(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		var event Event
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

func countConfirmationID(events []Event, confirmationID string) int {
	count := 0
	for _, event := range events {
		if event.Data["confirmation_id"] == confirmationID {
			count++
		}
	}
	return count
}

func testConfirmationEvidence() (string, string, Event) {
	confirmationID := "abc123def4567890"
	record := "- requirements confirmed by alice at 2026-06-24T12:00:00Z: requirements are accurate\n"
	event := Event{
		Time:    time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
		Type:    "stage.confirmed",
		Message: "requirements confirmed",
		Data: map[string]string{
			"by":      "alice",
			"record":  record,
			"stage":   "requirements",
			"summary": "requirements are accurate",
		},
	}
	return confirmationID, record, event
}

func expectedConfirmationStartMarker(id string) string {
	return fmt.Sprintf("<!-- devflow-confirmation:%s:start -->", id)
}

func expectedConfirmationEndMarker(id string) string {
	return fmt.Sprintf("<!-- devflow-confirmation:%s:end -->", id)
}

func expectedConfirmationBlock(id, record string) string {
	return expectedConfirmationStartMarker(id) + "\n" + strings.TrimSpace(record) + "\n" + expectedConfirmationEndMarker(id) + "\n"
}

func appendRawFile(path, content string) error {
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

func hasEventType(events []Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func findEventType(events []Event, eventType string) (Event, bool) {
	for _, event := range events {
		if event.Type == eventType {
			return event, true
		}
	}
	return Event{}, false
}

func TestReadEventsRecoversTrailingPartialEvent(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	demand := Demand{
		ID:          "read-events-trailing",
		Title:       "Read events trailing",
		Description: "Exercise event reader",
		Source:      "test",
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	eventsPath := filepath.Join(root, ".devflow", "demands", demand.ID, EventsFile)
	if err := os.WriteFile(eventsPath, []byte(`{"time":"2026-06-30T01:02:03Z","type":"stage.confirmed","message":"requirements confirmed","data":{"stage":"requirements"}}`+"\n"+`{"time":`), 0o644); err != nil {
		t.Fatalf("write events log: %v", err)
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("ReadEvents returned %d events, want 2", len(events))
	}
	if events[0].Type != "stage.confirmed" || events[0].Data["stage"] != "requirements" {
		t.Fatalf("ReadEvents returned unexpected event: %#v", events[0])
	}
	if events[1].Type != "events.repaired" {
		t.Fatalf("ReadEvents repair event = %#v, want events.repaired", events[1])
	}
}

func TestReadEventsFailsOnMalformedMiddleEvent(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	demand := Demand{
		ID:          "read-events-middle",
		Title:       "Read events middle",
		Description: "Exercise event reader",
		Source:      "test",
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	eventsPath := filepath.Join(root, ".devflow", "demands", demand.ID, EventsFile)
	body := strings.Join([]string{
		`{"time":"2026-06-30T01:02:03Z","type":"demand.created","message":"created"}`,
		`{"time":`,
		`{"time":"2026-06-30T01:03:03Z","type":"stage.confirmed","message":"requirements confirmed","data":{"stage":"requirements"}}`,
		"",
	}, "\n")
	if err := os.WriteFile(eventsPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write events log: %v", err)
	}

	_, err := store.ReadEvents(demand.ID)
	if err == nil {
		t.Fatal("ReadEvents returned nil error for malformed middle event")
	}
	if !strings.Contains(err.Error(), "decode events log events.jsonl line 2") {
		t.Fatalf("ReadEvents error = %q, want decode line context", err)
	}
}
