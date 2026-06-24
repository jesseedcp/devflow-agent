package artifacts

import (
	"bufio"
	"encoding/json"
	"os"
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
