package todo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestTaskList(t *testing.T) *TaskList {
	t.Helper()
	return NewTaskList(NewStore(t.TempDir(), "test"))
}

func TestCreateReturnsLoadErrorWithoutOverwritingStore(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, "corrupt")
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	list := NewTaskList(store)
	if _, err := list.Create("new", "task", "", nil); err == nil {
		t.Fatal("expected corrupt store error")
	}
	data, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{bad json" {
		t.Fatalf("corrupt store was overwritten: %q", data)
	}
}

func TestTaskListUpdateDeleteAndInternalFiltering(t *testing.T) {
	list := newTestTaskList(t)
	visible, err := list.Create("visible", "visible task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	internal, err := list.Create("internal", "internal task", "", map[string]any{"_internal": true})
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := list.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != visible.ID {
		t.Fatalf("expected only visible task, got %+v", tasks)
	}

	updated, changed, err := list.Update(visible.ID, map[string]any{
		"status":       "in_progress",
		"owner":        "agent",
		"addBlocks":    []any{internal.ID},
		"addBlockedBy": []any{"external"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusInProgress || updated.Owner != "agent" {
		t.Fatalf("task was not updated: %+v", updated)
	}
	if len(changed) != 4 {
		t.Fatalf("expected 4 changed fields, got %v", changed)
	}

	deleted, changed, err := list.Update(visible.ID, map[string]any{"status": "deleted"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted == nil || len(changed) != 1 || changed[0] != "deleted" {
		t.Fatalf("expected delete change, got task=%+v changed=%v", deleted, changed)
	}
	if got, err := list.Get(visible.ID); err != nil || got != nil {
		t.Fatalf("expected deleted task to be absent, got task=%+v err=%v", got, err)
	}
}

func TestTaskToolsReturnErrorWhenListMissing(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		run  func() string
	}{
		{"create", func() string {
			return (&TaskCreateTool{}).Execute(ctx, map[string]any{"subject": "s", "description": "d"}).Output
		}},
		{"get", func() string {
			return (&TaskGetTool{}).Execute(ctx, map[string]any{"taskId": "t1"}).Output
		}},
		{"list", func() string {
			return (&TaskListTool{}).Execute(ctx, nil).Output
		}},
		{"update", func() string {
			return (&TaskUpdateTool{}).Execute(ctx, map[string]any{"taskId": "t1"}).Output
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if output := tc.run(); !strings.Contains(output, "not configured") {
				t.Fatalf("expected not configured error, got %q", output)
			}
		})
	}
}

func TestTaskCreateToolCreatesTask(t *testing.T) {
	list := newTestTaskList(t)
	result := (&TaskCreateTool{List: list}).Execute(context.Background(), map[string]any{
		"subject":     "ship",
		"description": "ship the task",
	})
	if result.IsError {
		t.Fatalf("create tool returned error: %s", result.Output)
	}

	tasks, err := list.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Subject != "ship" {
		t.Fatalf("expected created task, got %+v", tasks)
	}
}
