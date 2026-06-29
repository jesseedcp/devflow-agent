package commands

import (
	"strings"
	"testing"
)

func TestRegistryDuplicateNamePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&Command{Name: "foo", Type: TypeLocal})
	defer func() {
		if recover() == nil {
			t.Fatal("expected duplicate name to panic")
		}
	}()
	r.Register(&Command{Name: "foo", Type: TypeLocal})
}

func TestRegistryAliasCollisionPanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&Command{Name: "foo", Aliases: []string{"f"}, Type: TypeLocal})
	defer func() {
		if recover() == nil {
			t.Fatal("expected alias collision to panic")
		}
	}()
	r.Register(&Command{Name: "bar", Aliases: []string{"f"}, Type: TypeLocal})
}

func TestParse(t *testing.T) {
	name, args := Parse("/review abc")
	if name != "review" || args != "abc" {
		t.Fatalf("expected review/abc, got %q/%q", name, args)
	}

	name, args = Parse("/CLEAR")
	if name != "clear" || args != "" {
		t.Fatalf("expected lowercased clear with no args, got %q/%q", name, args)
	}

	name, args = Parse("not a command")
	if name != "" || args != "" {
		t.Fatalf("expected empty parse for non-slash input, got %q/%q", name, args)
	}
}

func TestRegistryFindAndAlias(t *testing.T) {
	r := NewRegistry()
	r.Register(&Command{Name: "compact", Aliases: []string{"c"}, Type: TypeLocalUI})
	if r.Find("compact") == nil {
		t.Fatal("expected to find compact by name")
	}
	if r.Find("c") == nil {
		t.Fatal("expected to find compact by alias")
	}
	if r.Find("missing") != nil {
		t.Fatal("unexpected find for missing command")
	}
}

func TestHasConflict(t *testing.T) {
	r := NewRegistry()
	r.Register(&Command{Name: "foo", Aliases: []string{"f"}, Type: TypeLocal})
	if !r.HasConflict(&Command{Name: "foo"}) {
		t.Fatal("expected name conflict")
	}
	if !r.HasConflict(&Command{Name: "bar", Aliases: []string{"f"}}) {
		t.Fatal("expected alias conflict")
	}
	if r.HasConflict(&Command{Name: "baz"}) {
		t.Fatal("did not expect conflict for new command")
	}
}

func TestIdentityStringsSayDevflow(t *testing.T) {
	r := CreateDefaultRegistry()

	status := r.Find("status").Handler(&Context{
		PermissionMode: func() string { return "default" },
		TokenCount:     func() (int, int) { return 0, 0 },
		ToolCount:      func() int { return 0 },
		MemoryList:     func() []string { return nil },
		Model:          "test-model",
		WorkDir:        "/tmp/work",
	})
	if !strings.Contains(status, "Devflow Status") {
		t.Fatalf("status should say Devflow, got: %q", status)
	}
	if strings.Contains(strings.ToLower(status), "mewcode") {
		t.Fatalf("status must not mention mewcode, got: %q", status)
	}

	skills := r.Find("skills").Handler(&Context{SkillList: func() []SkillInfo { return nil }})
	if strings.Contains(skills, ".mewcode") {
		t.Fatalf("skills help must reference .devflow, not .mewcode, got: %q", skills)
	}
	if !strings.Contains(skills, ".devflow/skills") {
		t.Fatalf("skills help should mention .devflow/skills, got: %q", skills)
	}
}
