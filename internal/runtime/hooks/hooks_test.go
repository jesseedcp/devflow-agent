package hooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvaluateConditionLeafOps(t *testing.T) {
	ctx := HookContext{
		EventName: EventPreToolUse,
		ToolName:  "Bash",
		FilePath:  "src/foo.go",
		ToolArgs:  map[string]any{"command": "rm -rf /"},
	}
	cases := map[string]bool{
		`tool == "Bash"`:                            true,
		`tool == "Read"`:                            false,
		`tool != "Read"`:                            true,
		`event =~ /^pre_/`:                          true,
		`args.command =~ /rm -rf/`:                  true,
		`file_path =* "src/*.go"`:                   true,
		`file_path =* "src/*.py"`:                   false,
		`tool == "Bash" && file_path =* "src/*.go"`: true,
		`tool == "Bash" && file_path =* "src/*.py"`: false,
		`tool == "Read" || tool == "Bash"`:          true,
		`tool == "Read" || tool == "Write"`:         false,
		`!(tool == "Read")`:                         true,
		`!tool == "Read"`:                           true,
	}
	for cond, want := range cases {
		if got := evaluateCondition(cond, ctx); got != want {
			t.Errorf("evaluateCondition(%q) = %v, want %v", cond, got, want)
		}
	}
}

func TestEvaluateConditionGlobNormalizesSlashStyles(t *testing.T) {
	for _, filePath := range []string{"src/foo.go", `src\foo.go`} {
		ctx := HookContext{FilePath: filePath}
		if !evaluateCondition(`file_path =* "src/*.go"`, ctx) {
			t.Fatalf("expected pattern to match %q after slash normalization", filePath)
		}
	}
}

func TestRunPreToolHooksReject(t *testing.T) {
	eng := NewEngine()
	eng.LoadHooks([]Hook{{
		ID:        "block-rm-rf",
		Event:     EventPreToolUse,
		Condition: `tool == "Bash" && args.command =~ /rm -rf/`,
		Action:    Action{Type: ActionPrompt, Message: "destructive command blocked"},
		Reject:    true,
	}})

	ctx := HookContext{
		EventName: EventPreToolUse,
		ToolName:  "Bash",
		ToolArgs:  map[string]any{"command": "rm -rf /tmp/x"},
	}
	rejected, msg := eng.RunPreToolHooks(ctx)
	if !rejected {
		t.Fatal("expected rejection")
	}
	if !strings.Contains(msg, "destructive command blocked") {
		t.Errorf("unexpected reject message: %q", msg)
	}
}

func TestRunPreToolHooksAllowsWhenConditionFails(t *testing.T) {
	eng := NewEngine()
	eng.LoadHooks([]Hook{{
		ID:        "block-go",
		Event:     EventPreToolUse,
		Condition: `file_path =* "**/*.go"`,
		Action:    Action{Type: ActionPrompt, Message: "blocked"},
		Reject:    true,
	}})
	rejected, _ := eng.RunPreToolHooks(HookContext{
		EventName: EventPreToolUse,
		ToolName:  "WriteFile",
		FilePath:  "src/foo.py",
	})
	if rejected {
		t.Fatal("expected allow for non-matching path")
	}
}

func TestHookOnceOnlyFiresOnce(t *testing.T) {
	eng := NewEngine()
	eng.LoadHooks([]Hook{{
		ID:     "greet",
		Event:  EventSessionStart,
		Action: Action{Type: ActionPrompt, Message: "hello"},
		Once:   true,
	}})

	res1 := eng.RunHooks(HookContext{EventName: EventSessionStart})
	res2 := eng.RunHooks(HookContext{EventName: EventSessionStart})
	if len(res1) != 1 {
		t.Errorf("first run should produce 1 result, got %d", len(res1))
	}
	if len(res2) != 0 {
		t.Errorf("second run should produce 0 results (once), got %d", len(res2))
	}
}

func TestHookHTTPAction(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Method != "POST" {
			t.Errorf("want POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing JSON content-type")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	eng := NewEngine()
	eng.LoadHooks([]Hook{{
		ID:    "notify",
		Event: EventPostToolUse,
		Action: Action{
			Type: ActionHTTP,
			URL:  server.URL,
		},
	}})
	results := eng.RunHooks(HookContext{EventName: EventPostToolUse, ToolName: "Bash"})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("expected one successful HTTP result, got %#v", results)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("server expected 1 hit, got %d", hits)
	}
}

func TestHookAsyncIsNonBlocking(t *testing.T) {
	eng := NewEngine()
	eng.LoadHooks([]Hook{{
		ID:    "slow",
		Event: EventTurnEnd,
		Async: true,
		Action: Action{
			Type:    ActionCommand,
			Command: testSleepCommand(200 * time.Millisecond),
		},
	}})
	start := time.Now()
	res := eng.RunHooks(HookContext{EventName: EventTurnEnd})
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("async hook blocked the caller for %v", elapsed)
	}
	if len(res) != 1 || res[0].Output != "(async)" {
		t.Errorf("expected async stub result, got %#v", res)
	}
}

func TestHookAsyncClonesToolArgsContext(t *testing.T) {
	eng := NewEngine()
	release := make(chan struct{})
	observed := make(chan map[string]any, 1)
	eng.AgentRunner = func(prompt string, ctx HookContext) (string, error) {
		<-release
		payload, err := json.Marshal(ctx.ToolArgs)
		if err != nil {
			return "", err
		}
		var snapshot map[string]any
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return "", err
		}
		observed <- snapshot
		return string(payload), nil
	}
	eng.LoadHooks([]Hook{{
		ID:    "async-agent",
		Event: EventTurnEnd,
		Async: true,
		Action: Action{
			Type:    ActionAgent,
			Message: "inspect",
		},
	}})

	args := map[string]any{
		"command": "build",
		"meta": map[string]any{
			"tags":  []any{"before", "keep"},
			"steps": []any{map[string]any{"name": "lint"}},
		},
	}
	res := eng.RunHooks(HookContext{
		EventName: EventTurnEnd,
		ToolArgs:  args,
	})
	if len(res) != 1 || res[0].Output != "(async)" {
		t.Fatalf("expected async stub result, got %#v", res)
	}

	args["command"] = "deploy"
	meta := args["meta"].(map[string]any)
	meta["tags"].([]any)[0] = "after"
	meta["steps"].([]any)[0].(map[string]any)["name"] = "ship"
	meta["extra"] = "added"

	close(release)

	select {
	case got := <-observed:
		want := map[string]any{
			"command": "build",
			"meta": map[string]any{
				"tags":  []any{"before", "keep"},
				"steps": []any{map[string]any{"name": "lint"}},
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected async hook to observe cloned args %#v, got %#v", want, got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async agent snapshot")
	}
}

func TestHookOnErrorReject(t *testing.T) {
	eng := NewEngine()
	eng.LoadHooks([]Hook{{
		ID:      "fail",
		Event:   EventPreToolUse,
		OnError: "reject",
		Action: Action{
			Type:    ActionCommand,
			Command: testExitCommand(7),
		},
	}})
	rejected, _ := eng.RunPreToolHooks(HookContext{EventName: EventPreToolUse, ToolName: "Bash"})
	if !rejected {
		t.Fatal("expected reject on command failure with on_error=reject")
	}
}

func TestValidateCatchesMissingFields(t *testing.T) {
	cases := []struct {
		name string
		hook Hook
		want string
	}{
		{
			name: "command missing command field",
			hook: Hook{ID: "no-cmd", Event: EventPreToolUse, Action: Action{Type: ActionCommand}},
			want: "action.command must be non-empty",
		},
		{
			name: "prompt missing message",
			hook: Hook{ID: "no-msg", Event: EventSessionStart, Action: Action{Type: ActionPrompt}},
			want: "action.message must be non-empty",
		},
		{
			name: "http missing url",
			hook: Hook{ID: "no-url", Event: EventPostToolUse, Action: Action{Type: ActionHTTP}},
			want: "action.url must be non-empty",
		},
		{
			name: "http with malformed url",
			hook: Hook{ID: "bad-url", Event: EventPostToolUse, Action: Action{Type: ActionHTTP, URL: "not-a-url"}},
			want: "action.url must be a valid http(s) URL",
		},
		{
			name: "unknown event",
			hook: Hook{ID: "unknown-evt", Event: "made_up_event", Action: Action{Type: ActionPrompt, Message: "hi"}},
			want: `unknown event "made_up_event"`,
		},
		{
			name: "unknown action type",
			hook: Hook{ID: "unknown-act", Event: EventPreToolUse, Action: Action{Type: "weird"}},
			want: `unknown action.type "weird"`,
		},
		{
			name: "missing action type",
			hook: Hook{ID: "no-type", Event: EventPreToolUse, Action: Action{Command: "echo"}},
			want: "action.type is required",
		},
		{
			name: "negative timeout",
			hook: Hook{ID: "neg-to", Event: EventPostToolUse, Action: Action{Type: ActionCommand, Command: testEchoCommand("ok"), Timeout: -time.Second}},
			want: "action.timeout must be >= 0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate([]Hook{c.hook})
			if err == nil {
				t.Fatalf("expected error for %s, got nil", c.name)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected error to contain %q, got %q", c.want, err.Error())
			}
		})
	}
}

func TestValidateAggregatesAllErrors(t *testing.T) {
	hooks := []Hook{
		{ID: "bad1", Event: "nope", Action: Action{Type: ActionCommand}},
		{ID: "bad2", Event: EventPostToolUse, Action: Action{Type: "weird"}},
	}
	err := Validate(hooks)
	if err == nil {
		t.Fatal("expected aggregated errors")
	}
	msg := err.Error()
	for _, want := range []string{`unknown event "nope"`, "action.command must be non-empty", `unknown action.type "weird"`} {
		if !strings.Contains(msg, want) {
			t.Errorf("aggregated error missing %q, got: %s", want, msg)
		}
	}
}

func TestValidateAcceptsGoodConfig(t *testing.T) {
	hooks := []Hook{
		{ID: "fmt", Event: EventPostToolUse, Action: Action{Type: ActionCommand, Command: testEchoCommand("ok")}},
		{ID: "ctx", Event: EventSessionStart, Action: Action{Type: ActionPrompt, Message: "hello"}},
		{ID: "slack", Event: EventPostToolUse, Action: Action{Type: ActionHTTP, URL: "https://hooks.slack.com/services/xxx"}},
		{ID: "review", Event: EventPostToolUse, Action: Action{Type: ActionAgent, Message: "review the change"}},
	}
	if err := Validate(hooks); err != nil {
		t.Fatalf("expected no error, got: %s", err)
	}
}

func TestRunCommandTimeout(t *testing.T) {
	h := Hook{
		ID: "slow",
		Action: Action{
			Type:    ActionCommand,
			Command: testSleepCommand(2 * time.Second),
			Timeout: 100 * time.Millisecond,
		},
	}
	start := time.Now()
	result := runCommand(h, HookContext{EventName: EventPostToolUse, ToolName: "Bash"})
	elapsed := time.Since(start)

	if result.Success {
		t.Fatalf("expected timed-out command to report Success=false, output=%q", result.Output)
	}
	if !strings.Contains(result.Output, "timed out") {
		t.Fatalf("expected output to mention timeout, got: %q", result.Output)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("expected command to be killed near 100ms, but took %s", elapsed)
	}
}

func TestRunCommandDefaultTimeoutAllowsFastCommand(t *testing.T) {
	h := Hook{
		ID: "fast",
		Action: Action{
			Type:    ActionCommand,
			Command: testEchoCommand("ok"),
		},
	}
	result := runCommand(h, HookContext{EventName: EventPostToolUse, ToolName: "Bash"})
	if !result.Success {
		t.Fatalf("expected fast command to succeed under default timeout, got output=%q", result.Output)
	}
	if !strings.Contains(result.Output, "ok") {
		t.Fatalf("expected output to contain stdout 'ok', got %q", result.Output)
	}
}

func TestRunCommandExportsDevflowAndMewcodeAliases(t *testing.T) {
	h := Hook{
		ID: "env",
		Action: Action{
			Type:    ActionCommand,
			Command: testEnvEchoCommand(),
		},
	}
	result := runCommand(h, HookContext{
		EventName: EventPostToolUse,
		ToolName:  "Bash",
		FilePath:  "src/foo.go",
	})
	if !result.Success {
		t.Fatalf("expected env echo command to succeed, got output=%q", result.Output)
	}
	want := "post_tool_use|post_tool_use|Bash|Bash|src/foo.go|src/foo.go"
	if strings.TrimSpace(result.Output) != want {
		t.Fatalf("expected env output %q, got %q", want, result.Output)
	}
}
