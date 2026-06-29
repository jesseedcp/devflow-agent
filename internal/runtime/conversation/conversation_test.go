package conversation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManagerPreservesToolRoundTrip(t *testing.T) {
	manager := NewManager()
	manager.AddUserMessage("check the repository")
	manager.AddToolUseMessage("I will inspect it", "tool-1", "ReadFile", map[string]any{
		"file_path": "README.md",
	})
	manager.AddToolResultMessage("tool-1", "# Devflow", false)

	messages := manager.GetMessages()
	if len(messages) != 3 {
		t.Fatalf("message count = %d, want 3", len(messages))
	}
	if len(messages[1].ToolUses) != 1 {
		t.Fatalf("tool use count = %d, want 1", len(messages[1].ToolUses))
	}
	if messages[1].ToolUses[0].ToolName != "ReadFile" {
		t.Fatalf("tool name = %q, want ReadFile", messages[1].ToolUses[0].ToolName)
	}
	if len(messages[2].ToolResults) != 1 {
		t.Fatalf("tool result count = %d, want 1", len(messages[2].ToolResults))
	}
	if messages[2].ToolResults[0].Content != "# Devflow" {
		t.Fatalf("tool result = %q, want # Devflow", messages[2].ToolResults[0].Content)
	}
}

func TestInjectLongTermMemoryUsesDevflowIdentityOnce(t *testing.T) {
	manager := NewManager()
	manager.AddUserMessage("implement the demand")
	manager.InjectLongTermMemory("Follow AGENTS.md", "Coupon rules require active members")
	manager.InjectLongTermMemory("duplicate", "duplicate")

	messages := manager.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}

	injected := messages[0].Content
	if !strings.Contains(injected, "# devflowMd") {
		t.Fatalf("injected memory does not use Devflow identity: %q", injected)
	}
	if strings.Contains(injected, "# mewcodeMd") {
		t.Fatalf("injected memory still exposes MewCode identity: %q", injected)
	}
	if strings.Count(injected, "Coupon rules require active members") != 1 {
		t.Fatalf("memory should be injected once: %q", injected)
	}
	if strings.Count(injected, "duplicate") != 0 {
		t.Fatalf("duplicate injection should be ignored: %q", injected)
	}
}

func TestGetMessagesReturnsIndependentSlice(t *testing.T) {
	manager := NewManager()
	manager.AddUserMessage("original")

	copyOfMessages := manager.GetMessages()
	copyOfMessages[0].Content = "changed"

	if got := manager.GetMessages()[0].Content; got != "original" {
		t.Fatalf("manager content = %q, want original", got)
	}
}

func TestGetMessagesReturnsDeepCopiesOfNestedData(t *testing.T) {
	manager := NewManager()
	manager.AddAssistantFull(
		"working",
		[]ThinkingBlock{{Thinking: "plan", Signature: "sig", EncryptedContent: "enc_123"}},
		[]ToolUseBlock{{
			ToolUseID: "tool-1",
			ToolName:  "ReadFile",
			Arguments: map[string]any{
				"nested": map[string]any{"path": "README.md"},
				"items":  []any{"first", map[string]any{"name": "keep"}},
			},
		}},
	)
	manager.AddToolResultsMessage([]ToolResultBlock{{
		ToolUseID: "tool-1",
		Content:   "# Devflow",
		IsError:   false,
	}})

	messages := manager.GetMessages()
	messages[0].ThinkingBlocks[0].Thinking = "rewritten"
	messages[0].ThinkingBlocks[0].EncryptedContent = "mutated"
	messages[0].ToolUses[0].ToolName = "WriteFile"
	messages[0].ToolUses[0].Arguments["nested"].(map[string]any)["path"] = "mutated.md"
	messages[0].ToolUses[0].Arguments["items"].([]any)[1].(map[string]any)["name"] = "changed"
	messages[1].ToolResults[0].Content = "mutated"

	fresh := manager.GetMessages()
	if got := fresh[0].ThinkingBlocks[0].Thinking; got != "plan" {
		t.Fatalf("manager thinking = %q, want plan", got)
	}
	if got := fresh[0].ThinkingBlocks[0].EncryptedContent; got != "enc_123" {
		t.Fatalf("manager encrypted thinking = %q, want enc_123", got)
	}
	if got := fresh[0].ToolUses[0].ToolName; got != "ReadFile" {
		t.Fatalf("manager tool name = %q, want ReadFile", got)
	}
	if got := fresh[0].ToolUses[0].Arguments["nested"].(map[string]any)["path"]; got != "README.md" {
		t.Fatalf("manager nested argument = %#v, want README.md", got)
	}
	if got := fresh[0].ToolUses[0].Arguments["items"].([]any)[1].(map[string]any)["name"]; got != "keep" {
		t.Fatalf("manager list argument = %#v, want keep", got)
	}
	if got := fresh[1].ToolResults[0].Content; got != "# Devflow" {
		t.Fatalf("manager tool result = %q, want # Devflow", got)
	}
}

func TestAddToolUseMessageCopiesArguments(t *testing.T) {
	manager := NewManager()
	arguments := map[string]any{
		"nested": map[string]any{"path": "README.md"},
		"items":  []any{"first", map[string]any{"name": "keep"}},
	}

	manager.AddToolUseMessage("I will inspect it", "tool-1", "ReadFile", arguments)

	arguments["nested"].(map[string]any)["path"] = "mutated.md"
	arguments["items"].([]any)[1].(map[string]any)["name"] = "changed"

	messages := manager.GetMessages()
	if got := messages[0].ToolUses[0].Arguments["nested"].(map[string]any)["path"]; got != "README.md" {
		t.Fatalf("manager nested argument = %#v, want README.md", got)
	}
	if got := messages[0].ToolUses[0].Arguments["items"].([]any)[1].(map[string]any)["name"]; got != "keep" {
		t.Fatalf("manager list argument = %#v, want keep", got)
	}
}

func TestAddAssistantMessageWithToolsCopiesToolUses(t *testing.T) {
	manager := NewManager()
	toolUses := []ToolUseBlock{{
		ToolUseID: "tool-1",
		ToolName:  "ReadFile",
		Arguments: map[string]any{
			"nested": map[string]any{"path": "README.md"},
		},
	}}

	manager.AddAssistantMessageWithTools("I will inspect it", toolUses)

	toolUses[0].ToolName = "WriteFile"
	toolUses[0].Arguments["nested"].(map[string]any)["path"] = "mutated.md"

	messages := manager.GetMessages()
	if got := messages[0].ToolUses[0].ToolName; got != "ReadFile" {
		t.Fatalf("manager tool name = %q, want ReadFile", got)
	}
	if got := messages[0].ToolUses[0].Arguments["nested"].(map[string]any)["path"]; got != "README.md" {
		t.Fatalf("manager nested argument = %#v, want README.md", got)
	}
}

func TestAddAssistantFullCopiesThinkingAndToolUses(t *testing.T) {
	manager := NewManager()
	thinking := []ThinkingBlock{{Thinking: "plan", Signature: "sig"}}
	toolUses := []ToolUseBlock{{
		ToolUseID: "tool-1",
		ToolName:  "ReadFile",
		Arguments: map[string]any{
			"nested": map[string]any{"path": "README.md"},
		},
	}}

	manager.AddAssistantFull("I will inspect it", thinking, toolUses)

	thinking[0].Thinking = "mutated"
	toolUses[0].ToolName = "WriteFile"
	toolUses[0].Arguments["nested"].(map[string]any)["path"] = "mutated.md"

	messages := manager.GetMessages()
	if got := messages[0].ThinkingBlocks[0].Thinking; got != "plan" {
		t.Fatalf("manager thinking = %q, want plan", got)
	}
	if got := messages[0].ToolUses[0].ToolName; got != "ReadFile" {
		t.Fatalf("manager tool name = %q, want ReadFile", got)
	}
	if got := messages[0].ToolUses[0].Arguments["nested"].(map[string]any)["path"]; got != "README.md" {
		t.Fatalf("manager nested argument = %#v, want README.md", got)
	}
}

func TestAddToolResultsMessageCopiesResults(t *testing.T) {
	manager := NewManager()
	results := []ToolResultBlock{{
		ToolUseID: "tool-1",
		Content:   "# Devflow",
		IsError:   false,
	}}

	manager.AddToolResultsMessage(results)

	results[0].Content = "mutated"
	results[0].IsError = true

	messages := manager.GetMessages()
	if got := messages[0].ToolResults[0].Content; got != "# Devflow" {
		t.Fatalf("manager tool result = %q, want # Devflow", got)
	}
	if got := messages[0].ToolResults[0].IsError; got {
		t.Fatalf("manager tool result error = %t, want false", got)
	}
}

func TestAddToolUseMessageCopiesConcreteArgumentTypes(t *testing.T) {
	manager := NewManager()
	paths := []string{"README.md", "AGENTS.md"}
	documents := []map[string]any{{"path": "README.md"}}
	labels := map[string]string{"lang": "go"}

	manager.AddToolUseMessage("I will inspect it", "tool-1", "ReadFile", map[string]any{
		"paths":     paths,
		"documents": documents,
		"labels":    labels,
	})

	paths[0] = "mutated.md"
	documents[0]["path"] = "changed.md"
	labels["lang"] = "rust"

	messages := manager.GetMessages()
	if got := messages[0].ToolUses[0].Arguments["paths"].([]string)[0]; got != "README.md" {
		t.Fatalf("manager paths[0] = %q, want README.md", got)
	}
	if got := messages[0].ToolUses[0].Arguments["documents"].([]map[string]any)[0]["path"]; got != "README.md" {
		t.Fatalf("manager documents[0].path = %#v, want README.md", got)
	}
	if got := messages[0].ToolUses[0].Arguments["labels"].(map[string]string)["lang"]; got != "go" {
		t.Fatalf("manager labels.lang = %q, want go", got)
	}
}

func TestGetMessagesReturnsDeepCopiesOfConcreteArgumentTypes(t *testing.T) {
	manager := NewManager()
	manager.AddToolUseMessage("I will inspect it", "tool-1", "ReadFile", map[string]any{
		"paths":     []string{"README.md", "AGENTS.md"},
		"documents": []map[string]any{{"path": "README.md"}},
		"labels":    map[string]string{"lang": "go"},
	})

	messages := manager.GetMessages()
	messages[0].ToolUses[0].Arguments["paths"].([]string)[0] = "mutated.md"
	messages[0].ToolUses[0].Arguments["documents"].([]map[string]any)[0]["path"] = "changed.md"
	messages[0].ToolUses[0].Arguments["labels"].(map[string]string)["lang"] = "rust"

	fresh := manager.GetMessages()
	if got := fresh[0].ToolUses[0].Arguments["paths"].([]string)[0]; got != "README.md" {
		t.Fatalf("manager paths[0] = %q, want README.md", got)
	}
	if got := fresh[0].ToolUses[0].Arguments["documents"].([]map[string]any)[0]["path"]; got != "README.md" {
		t.Fatalf("manager documents[0].path = %#v, want README.md", got)
	}
	if got := fresh[0].ToolUses[0].Arguments["labels"].(map[string]string)["lang"]; got != "go" {
		t.Fatalf("manager labels.lang = %q, want go", got)
	}
}

func TestDeepCopyPreservesNilArgumentTypes(t *testing.T) {
	manager := NewManager()
	var paths []string
	var documents []map[string]any
	var labels map[string]string
	var nestedItems []any
	var nestedMeta map[string]any

	manager.AddToolUseMessage("I will inspect it", "tool-1", "ReadFile", map[string]any{
		"paths":     paths,
		"documents": documents,
		"labels":    labels,
		"nested": map[string]any{
			"items": nestedItems,
			"meta":  nestedMeta,
		},
	})

	messages := manager.GetMessages()
	if got := messages[0].ToolUses[0].Arguments["paths"].([]string); got != nil {
		t.Fatalf("paths nil = %v, want nil", got)
	}
	if got := messages[0].ToolUses[0].Arguments["documents"].([]map[string]any); got != nil {
		t.Fatalf("documents nil = %v, want nil", got)
	}
	if got := messages[0].ToolUses[0].Arguments["labels"].(map[string]string); got != nil {
		t.Fatalf("labels nil = %v, want nil", got)
	}
	nested := messages[0].ToolUses[0].Arguments["nested"].(map[string]any)
	if got := nested["items"].([]any); got != nil {
		t.Fatalf("nested items nil = %v, want nil", got)
	}
	if got := nested["meta"].(map[string]any); got != nil {
		t.Fatalf("nested meta nil = %v, want nil", got)
	}
}

func TestSerializeAnthropicPreservesBehaviorAndIsolatesToolInput(t *testing.T) {
	manager := NewManager()
	manager.AddUserMessage("first user")
	manager.AddUserMessage("second user")
	manager.AddToolUseMessage("I will inspect it", "tool-1", "ReadFile", map[string]any{
		"nested": map[string]any{"path": "README.md"},
		"paths":  []string{"README.md"},
	})
	manager.AddToolResultsMessage([]ToolResultBlock{{
		ToolUseID: "tool-1",
		Content:   "# Devflow",
		IsError:   false,
	}})

	serialized := manager.Serialize("anthropic")
	if len(serialized) != 3 {
		t.Fatalf("serialized message count = %d, want 3", len(serialized))
	}
	if got := serialized[0]["role"]; got != "user" {
		t.Fatalf("serialized[0] role = %#v, want user", got)
	}
	if got := serialized[0]["content"]; got != "first user\n\nsecond user" {
		t.Fatalf("serialized[0] content = %#v, want merged user text", got)
	}

	assistantContent, ok := serialized[1]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("serialized[1] content type = %T, want []map[string]any", serialized[1]["content"])
	}
	if got := serialized[1]["role"]; got != "assistant" {
		t.Fatalf("serialized[1] role = %#v, want assistant", got)
	}
	if len(assistantContent) != 2 {
		t.Fatalf("assistant content block count = %d, want 2", len(assistantContent))
	}
	if got := assistantContent[0]["type"]; got != "text" {
		t.Fatalf("assistant text block type = %#v, want text", got)
	}
	if got := assistantContent[0]["text"]; got != "I will inspect it" {
		t.Fatalf("assistant text block text = %#v, want I will inspect it", got)
	}
	if got := assistantContent[1]["type"]; got != "tool_use" {
		t.Fatalf("assistant tool block type = %#v, want tool_use", got)
	}
	if got := assistantContent[1]["id"]; got != "tool-1" {
		t.Fatalf("assistant tool block id = %#v, want tool-1", got)
	}
	if got := assistantContent[1]["name"]; got != "ReadFile" {
		t.Fatalf("assistant tool block name = %#v, want ReadFile", got)
	}

	input := assistantContent[1]["input"].(map[string]any)
	input["nested"].(map[string]any)["path"] = "mutated.md"
	input["paths"].([]string)[0] = "changed.md"

	fresh := manager.GetMessages()
	if got := fresh[2].ToolUses[0].Arguments["nested"].(map[string]any)["path"]; got != "README.md" {
		t.Fatalf("manager anthropic nested path = %#v, want README.md", got)
	}
	if got := fresh[2].ToolUses[0].Arguments["paths"].([]string)[0]; got != "README.md" {
		t.Fatalf("manager anthropic paths[0] = %q, want README.md", got)
	}

	resultContent, ok := serialized[2]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("serialized[2] content type = %T, want []map[string]any", serialized[2]["content"])
	}
	if got := serialized[2]["role"]; got != "user" {
		t.Fatalf("serialized[2] role = %#v, want user", got)
	}
	if len(resultContent) != 1 {
		t.Fatalf("tool result block count = %d, want 1", len(resultContent))
	}
	if got := resultContent[0]["type"]; got != "tool_result" {
		t.Fatalf("tool result type = %#v, want tool_result", got)
	}
	if got := resultContent[0]["tool_use_id"]; got != "tool-1" {
		t.Fatalf("tool result id = %#v, want tool-1", got)
	}
	if got := resultContent[0]["content"]; got != "# Devflow" {
		t.Fatalf("tool result content = %#v, want # Devflow", got)
	}
	if got := resultContent[0]["is_error"]; got != false {
		t.Fatalf("tool result is_error = %#v, want false", got)
	}
}

func TestSerializeOpenAIPreservesFunctionCallAndOutput(t *testing.T) {
	manager := NewManager()
	manager.AddToolUseMessage("I will inspect it", "tool-1", "ReadFile", map[string]any{
		"path":  "README.md",
		"lines": []string{"1", "2"},
	})
	manager.AddToolResultMessage("tool-1", "# Devflow", false)

	serialized := manager.Serialize("openai")
	if len(serialized) != 3 {
		t.Fatalf("serialized message count = %d, want 3", len(serialized))
	}
	if got := serialized[0]["role"]; got != "assistant" {
		t.Fatalf("serialized[0] role = %#v, want assistant", got)
	}
	if got := serialized[0]["content"]; got != "I will inspect it" {
		t.Fatalf("serialized[0] content = %#v, want I will inspect it", got)
	}
	if got := serialized[1]["type"]; got != "function_call" {
		t.Fatalf("serialized[1] type = %#v, want function_call", got)
	}
	if got := serialized[1]["name"]; got != "ReadFile" {
		t.Fatalf("serialized[1] name = %#v, want ReadFile", got)
	}
	if got := serialized[1]["call_id"]; got != "tool-1" {
		t.Fatalf("serialized[1] call_id = %#v, want tool-1", got)
	}
	var arguments map[string]any
	if err := json.Unmarshal([]byte(serialized[1]["arguments"].(string)), &arguments); err != nil {
		t.Fatalf("unmarshal arguments: %v", err)
	}
	if got := arguments["path"]; got != "README.md" {
		t.Fatalf("function_call path = %#v, want README.md", got)
	}
	lines := arguments["lines"].([]any)
	if len(lines) != 2 || lines[0] != "1" || lines[1] != "2" {
		t.Fatalf("function_call lines = %#v, want [1 2]", lines)
	}
	if got := serialized[2]["type"]; got != "function_call_output" {
		t.Fatalf("serialized[2] type = %#v, want function_call_output", got)
	}
	if got := serialized[2]["call_id"]; got != "tool-1" {
		t.Fatalf("serialized[2] call_id = %#v, want tool-1", got)
	}
	if got := serialized[2]["output"]; got != "# Devflow" {
		t.Fatalf("serialized[2] output = %#v, want # Devflow", got)
	}
}
