package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	entries := Load(dir)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestAppendWritesToDevflow(t *testing.T) {
	dir := t.TempDir()

	Append(dir, "hello")
	Append(dir, "world")

	entries := Load(dir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0] != "hello" || entries[1] != "world" {
		t.Fatalf("unexpected entries: %v", entries)
	}

	path := filepath.Join(dir, ".devflow", "prompt_history.jsonl")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("devflow history file was not created")
	}

	legacy := filepath.Join(dir, ".mewcode", "prompt_history.jsonl")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatal("legacy mewcode history file should not be created")
	}
}

func TestLoadPrefersDevflowOverMewcode(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".mewcode"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, ".mewcode", "prompt_history.jsonl")
	if err := os.WriteFile(legacy, []byte(`{"text":"legacy","ts":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, ".devflow"), 0o755); err != nil {
		t.Fatal(err)
	}
	devflow := filepath.Join(dir, ".devflow", "prompt_history.jsonl")
	if err := os.WriteFile(devflow, []byte(`{"text":"devflow","ts":2}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := Load(dir)
	if len(entries) != 1 || entries[0] != "devflow" {
		t.Fatalf("expected devflow entry, got %v", entries)
	}
}

func TestLoadFallsBackToMewcode(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".mewcode"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, ".mewcode", "prompt_history.jsonl")
	if err := os.WriteFile(legacy, []byte(`{"text":"legacy","ts":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := Load(dir)
	if len(entries) != 1 || entries[0] != "legacy" {
		t.Fatalf("expected legacy entry, got %v", entries)
	}
}

func TestDedup(t *testing.T) {
	dir := t.TempDir()

	Append(dir, "same")
	Append(dir, "same")

	entries := Load(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after dedup, got %d", len(entries))
	}
}

func TestTrim(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 210; i++ {
		Append(dir, "entry"+string(rune('A'+i%26))+string(rune('0'+i/26)))
	}

	entries := Load(dir)
	if len(entries) > maxEntries {
		t.Fatalf("expected <= %d entries, got %d", maxEntries, len(entries))
	}
}
