// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

// Package compact implements Devflow's Layer 2 context management:
// an LLM-driven full-conversation summary, gated by token ratio (default
// >80% of context window). Replaces the entire conversation with a summary
// message + a continuation acknowledgement. Also reachable via ForceCompact
// (the /compact slash command).
//
// Layer 1 (per-result spill + per-message aggregate budget + stale snip)
// lives in package toolresult and is invoked by the Agent loop directly,
// because it needs a ContentReplacementState that crosses turns.
package compact

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
	"github.com/jesseedcp/devflow-agent/internal/runtime/llm"
)

const (
	// autoCompactThreshold gates Layer 2. Below this, Layer 2 stays idle.
	autoCompactThreshold = 0.80
)

// MaxConsecutiveAutoCompactFailures stops auto-compact retries when the context is irrecoverably
// over the limit (e.g., prompt_too_long), so the agent doesn't hammer the API with doomed attempts
// on every iteration.
const MaxConsecutiveAutoCompactFailures = 3

// AutoCompactTrackingState threads circuit-breaker state across agent loop iterations. The caller
// owns the struct; ManageContext mutates it in place.
type AutoCompactTrackingState struct {
	// ConsecutiveFailures counts auto-compact attempts that returned an error since the last success.
	// Reset to zero on success.
	ConsecutiveFailures int
}

// summarySystemPrompt instructs the model to produce a two-phase response: a <analysis> scratchpad
// block followed by a <summary> block. The analysis block is stripped by formatCompactSummary
// before the result is written back into the conversation, leaving only the structured summary.
const summarySystemPrompt = `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.
This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be essential for continuing development work without losing context.

Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all necessary points. In your analysis process:

1. Chronologically analyze each message and section of the conversation. For each section thoroughly identify:
   - The user's explicit requests and intents
   - Your approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like file names, code snippets, function signatures, file edits
   - Errors that you ran into and how you fixed them
   - Specific user feedback received, especially corrections
2. Double-check for technical accuracy and completeness.

After your analysis, output your final summary wrapped in <summary> tags. The summary MUST preserve:
- All file paths that were read, modified, or created
- Key decisions and their rationale
- The current task/goal and overall progress
- Any pending work or next steps
- Error states and their resolution
- Important code snippets or patterns discussed

Output structure:

<analysis>
[Your thought process, ensuring all points are covered thoroughly and accurately]
</analysis>

<summary>
[The final compact summary that retains all actionable context]
</summary>`

// EstimateTokens uses a 3.5 chars/token approximation across content, tool args, tool results, and
// thinking blocks.
func EstimateTokens(messages []conversation.Message) int {
	total := 0
	for _, m := range messages {
		total += int(float64(len(m.Content))/3.5) + 4
		for _, tu := range m.ToolUses {
			argsJSON, _ := json.Marshal(tu.Arguments)
			total += 50 + int(float64(len(argsJSON))/3.5)
		}
		for _, tr := range m.ToolResults {
			total += int(float64(len(tr.Content))/3.5) + 10
		}
		for _, tb := range m.ThinkingBlocks {
			total += int(float64(len(tb.Thinking)) / 3.5)
			total += int(float64(len(tb.EncryptedContent)) / 3.5)
		}
	}
	return total
}

// ManageContext runs Layer 2 (autoCompact) when token ratio exceeds
// autoCompactThreshold. Layer 1 (tool-result budget) is invoked by the
// caller directly via toolresult.Apply before this function — they're
// independent now because Layer 1 needs to thread ContentReplacementState
// across turns, which sits naturally on the Agent rather than buried inside
// a compact helper.
//
// tracking carries the circuit-breaker state across iterations. When nil,
// the circuit breaker is disabled (useful for tests and one-shot callers).
//
// `workDir` is unused at the moment but kept in the signature to avoid
// churn at every existing call site.
func ManageContext(
	ctx context.Context,
	conv *conversation.Manager,
	client llm.Client,
	workDir string,
	contextWindow int,
	tracking *AutoCompactTrackingState,
	recovery *RecoveryState,
	toolSchemas []map[string]any,
) (string, error) {
	_ = workDir // reserved for future use; see func doc

	tokens := EstimateTokens(conv.GetMessages())
	if float64(tokens)/float64(contextWindow) <= autoCompactThreshold {
		return "", nil
	}

	// Circuit breaker: stop retrying after N consecutive failures. Without
	// this, sessions where context is irrecoverably over the limit hammer
	// the API with doomed compaction attempts on every iteration.
	if tracking != nil && tracking.ConsecutiveFailures >= MaxConsecutiveAutoCompactFailures {
		return "", nil
	}

	msg, err := autoCompact(ctx, conv, client, contextWindow, recovery, toolSchemas)
	if err != nil {
		if tracking != nil {
			tracking.ConsecutiveFailures++
		}
		return "", err
	}
	if tracking != nil {
		tracking.ConsecutiveFailures = 0
	}
	return msg, nil
}

// ForceCompact is the manual /compact entry. Always runs Layer 2 (full summary) regardless of
// current token ratio. Layer 1 is skipped because a full summary supersedes spill/snip anyway.
func ForceCompact(
	ctx context.Context,
	conv *conversation.Manager,
	client llm.Client,
	contextWindow int,
	recovery *RecoveryState,
	toolSchemas []map[string]any,
) (string, error) {
	return autoCompact(ctx, conv, client, contextWindow, recovery, toolSchemas)
}

// autoCompact is Layer 2: full LLM summary that replaces the conversation with a single summary
// message + a continuation acknowledgement. After the summary lands, a recovery block is appended
// to the same user message so the model still has snapshots of the files it just read, the SOPs
// for any skills it invoked, and the current tool listing.
func autoCompact(
	ctx context.Context,
	conv *conversation.Manager,
	client llm.Client,
	contextWindow int,
	recovery *RecoveryState,
	toolSchemas []map[string]any,
) (string, error) {
	messages := conv.GetMessages()
	beforeTokens := EstimateTokens(messages)

	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
		for _, tb := range m.ThinkingBlocks {
			if tb.Thinking != "" {
				sb.WriteString(fmt.Sprintf("[thinking]: %s\n", tb.Thinking))
			}
			if tb.Signature != "" {
				sb.WriteString(fmt.Sprintf("[thinking_signature]: %s\n", tb.Signature))
			}
			if tb.EncryptedContent != "" {
				sb.WriteString(fmt.Sprintf("[thinking_encrypted_content]: %d bytes\n", len(tb.EncryptedContent)))
			}
		}
		for _, tu := range m.ToolUses {
			argsJSON, err := json.Marshal(tu.Arguments)
			if err != nil {
				argsJSON = []byte(`"<unserializable arguments>"`)
			}
			sb.WriteString(fmt.Sprintf("[tool_use %s %s args]: %s\n", tu.ToolName, tu.ToolUseID, string(argsJSON)))
		}
		for _, tr := range m.ToolResults {
			content := tr.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("[tool_result %s is_error=%t]: %s\n", tr.ToolUseID, tr.IsError, content))
		}
	}

	summaryConv := conversation.NewManager()
	summaryConv.AddUserMessage(summarySystemPrompt + "\n\n" + sb.String())

	events, errs := client.Stream(ctx, summaryConv, nil)
	var summary strings.Builder
	for ev := range events {
		if td, ok := ev.(llm.TextDelta); ok {
			summary.WriteString(td.Text)
		}
	}
	select {
	case err := <-errs:
		if err != nil {
			return "", err
		}
	default:
	}

	finalSummary := formatCompactSummary(summary.String())

	content := fmt.Sprintf("[Compacted conversation summary]\n\n%s", finalSummary)
	if attachment := BuildRecoveryAttachment(recovery, toolSchemas); attachment != "" {
		content += "\n\n---\n\n" + attachment
	}

	compacted := conversation.NewManager()
	compacted.AddUserMessage(content)
	compacted.AddAssistantMessage("Understood. I'll continue based on this context.")

	*conv = *compacted
	afterTokens := EstimateTokens(conv.GetMessages())
	return fmt.Sprintf("Compacted: %d → %d estimated tokens", beforeTokens, afterTokens), nil
}

// formatCompactSummary strips the <analysis> scratchpad block from the model's two-phase response,
// returning only the contents of the <summary> block. Falls back to the raw text when neither tag
// is present (model disobeyed the prompt structure) so we never lose the summary altogether.
func formatCompactSummary(raw string) string {
	if start := strings.Index(raw, "<summary>"); start >= 0 {
		body := raw[start+len("<summary>"):]
		if end := strings.Index(body, "</summary>"); end >= 0 {
			return strings.TrimSpace(body[:end])
		}
		return strings.TrimSpace(body)
	}
	// No <summary> block — drop any <analysis>.</analysis> block if present and return what's left.
	if start := strings.Index(raw, "<analysis>"); start >= 0 {
		if end := strings.Index(raw, "</analysis>"); end > start {
			return strings.TrimSpace(raw[:start] + raw[end+len("</analysis>"):])
		}
	}
	return strings.TrimSpace(raw)
}
