// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

package compact

import (
	"context"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
	"github.com/jesseedcp/devflow-agent/internal/runtime/llm"
)

// Layer 1 (offload + snip) tests have moved to internal/toolresult/budget_test.go
// where the implementation now lives. compact only owns Layer 2 (autoCompact)
// plus the formatCompactSummary helper, so this file covers those.

func TestFormatCompactSummary(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "both blocks present",
			in:   "<analysis>scratch thoughts</analysis>\n<summary>final text</summary>",
			want: "final text",
		},
		{
			name: "summary block unterminated",
			in:   "<analysis>scratch</analysis>\n<summary>tail with no close tag",
			want: "tail with no close tag",
		},
		{
			name: "only analysis block — drop it",
			in:   "prefix <analysis>scratch</analysis> suffix",
			want: "prefix  suffix",
		},
		{
			name: "neither block — return raw",
			in:   "plain text response",
			want: "plain text response",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatCompactSummary(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// EstimateTokens covers all content sources without crashing on empty.
func TestEstimateTokensZeroAndPopulated(t *testing.T) {
	if got := EstimateTokens(nil); got != 0 {
		t.Errorf("empty input should be 0 tokens, got %d", got)
	}
	conv := conversation.NewManager()
	conv.AddUserMessage(strings.Repeat("x", 700))
	got := EstimateTokens(conv.GetMessages())
	if got < 150 || got > 250 {
		t.Errorf("700-char message should estimate ~200 tokens, got %d", got)
	}
}

type fakeSummaryClient struct {
	called       bool
	summaryInput string
}

func (f *fakeSummaryClient) Stream(ctx context.Context, conv *conversation.Manager, tools []map[string]any) (<-chan llm.StreamEvent, <-chan error) {
	_ = ctx
	_ = tools
	f.called = true
	messages := conv.GetMessages()
	if len(messages) > 0 {
		f.summaryInput = messages[0].Content
	}

	events := make(chan llm.StreamEvent, 1)
	errs := make(chan error, 1)
	events <- llm.TextDelta{Text: "<summary>compressed context</summary>"}
	close(events)
	close(errs)
	return events, errs
}

func TestManageContextCountsEncryptedThinkingTowardThreshold(t *testing.T) {
	conv := conversation.NewManager()
	conv.AddAssistantFull(
		"short visible response",
		[]conversation.ThinkingBlock{{EncryptedContent: strings.Repeat("e", 7000)}},
		nil,
	)
	client := &fakeSummaryClient{}

	message, err := ManageContext(context.Background(), conv, client, t.TempDir(), 1000, nil, nil, nil)
	if err != nil {
		t.Fatalf("ManageContext error: %v", err)
	}
	if !client.called {
		t.Fatal("expected encrypted thinking payload to trigger auto compaction")
	}
	if !strings.Contains(message, "Compacted:") {
		t.Fatalf("compaction message = %q, want Compacted summary", message)
	}
	messages := conv.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("post-compact message count = %d, want 2", len(messages))
	}
	if !strings.Contains(messages[0].Content, "compressed context") {
		t.Fatalf("summary content missing compacted response: %q", messages[0].Content)
	}
}

func TestForceCompactSummaryInputIncludesRuntimeMetadata(t *testing.T) {
	conv := conversation.NewManager()
	conv.AddAssistantFull(
		"I will inspect the file",
		[]conversation.ThinkingBlock{{
			Thinking:         "Need to inspect app.go before editing.",
			Signature:        "sig_123",
			EncryptedContent: "enc_123",
		}},
		[]conversation.ToolUseBlock{{
			ToolUseID: "tool-1",
			ToolName:  "ReadFile",
			Arguments: map[string]any{"file_path": "app.go", "limit": 20},
		}},
	)
	conv.AddToolResultsMessage([]conversation.ToolResultBlock{{
		ToolUseID: "tool-1",
		Content:   "permission denied",
		IsError:   true,
	}})
	client := &fakeSummaryClient{}

	if _, err := ForceCompact(context.Background(), conv, client, 1000, nil, nil); err != nil {
		t.Fatalf("ForceCompact error: %v", err)
	}

	for _, want := range []string{
		"[thinking]: Need to inspect app.go before editing.",
		"[thinking_signature]: sig_123",
		"[thinking_encrypted_content]: 7 bytes",
		"[tool_use ReadFile tool-1 args]:",
		`"file_path":"app.go"`,
		`"limit":20`,
		"[tool_result tool-1 is_error=true]: permission denied",
	} {
		if !strings.Contains(client.summaryInput, want) {
			t.Fatalf("summary input missing %q:\n%s", want, client.summaryInput)
		}
	}
}
