package planfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveLoadPlanRoundTrip(t *testing.T) {
	ResetPlanPath()
	workDir := t.TempDir()

	if err := SavePlan(workDir, "first plan"); err != nil {
		t.Fatal(err)
	}
	if !PlanExists(workDir) {
		t.Fatal("expected saved plan to exist")
	}
	loaded, err := LoadPlan(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != "first plan" {
		t.Fatalf("expected saved content, got %q", loaded)
	}
}

func TestResetRediscoverExistingPlan(t *testing.T) {
	ResetPlanPath()
	workDir := t.TempDir()
	if err := SavePlan(workDir, "persisted"); err != nil {
		t.Fatal(err)
	}
	pathBefore := GetPlanFilePath(workDir)

	ResetPlanPath()
	if !PlanExists(workDir) {
		t.Fatal("expected existing plan to be rediscovered after reset")
	}
	pathAfter := GetPlanFilePath(workDir)
	if pathAfter != pathBefore {
		t.Fatalf("expected rediscovered path %q, got %q", pathBefore, pathAfter)
	}
	loaded, err := LoadPlan(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != "persisted" {
		t.Fatalf("expected rediscovered content, got %q", loaded)
	}
}

func TestSeparateWorkDirsUseSeparatePlanFiles(t *testing.T) {
	ResetPlanPath()
	workDirA := t.TempDir()
	workDirB := t.TempDir()

	if err := SavePlan(workDirA, "plan a"); err != nil {
		t.Fatal(err)
	}
	if err := SavePlan(workDirB, "plan b"); err != nil {
		t.Fatal(err)
	}

	pathA := GetPlanFilePath(workDirA)
	pathB := GetPlanFilePath(workDirB)
	if pathA == pathB {
		t.Fatalf("expected separate plan paths, both were %q", pathA)
	}
	if !strings.HasPrefix(pathA, filepath.Join(workDirA, PlansDir)) {
		t.Fatalf("path A outside workdir A: %q", pathA)
	}
	if !strings.HasPrefix(pathB, filepath.Join(workDirB, PlansDir)) {
		t.Fatalf("path B outside workdir B: %q", pathB)
	}

	loadedA, err := LoadPlan(workDirA)
	if err != nil {
		t.Fatal(err)
	}
	loadedB, err := LoadPlan(workDirB)
	if err != nil {
		t.Fatal(err)
	}
	if loadedA != "plan a" || loadedB != "plan b" {
		t.Fatalf("unexpected contents: A=%q B=%q", loadedA, loadedB)
	}
}

func TestGetPlanFilePathDiscoversNewestExistingPlan(t *testing.T) {
	ResetPlanPath()
	workDir := t.TempDir()
	dir := filepath.Join(workDir, PlansDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "old.md")
	newPath := filepath.Join(dir, "new.md")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	if got := GetPlanFilePath(workDir); got != newPath {
		t.Fatalf("expected newest plan %q, got %q", newPath, got)
	}
	loaded, err := LoadPlan(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != "new" {
		t.Fatalf("expected newest content, got %q", loaded)
	}
}
