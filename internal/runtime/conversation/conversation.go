package conversation

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

type ToolUseBlock struct {
	ToolUseID string
	ToolName  string
	Arguments map[string]any
}

type ToolResultBlock struct {
	ToolUseID string
	Content   string
	IsError   bool
}

type ThinkingBlock struct {
	Thinking         string
	Signature        string
	EncryptedContent string
}

type Message struct {
	Role           string
	Content        string
	ThinkingBlocks []ThinkingBlock
	ToolUses       []ToolUseBlock
	ToolResults    []ToolResultBlock
}

type Manager struct {
	history     []Message
	ltmInjected bool
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) AddUserMessage(content string) {
	m.history = append(m.history, Message{Role: "user", Content: content})
}

func (m *Manager) AddAssistantMessage(content string) {
	m.history = append(m.history, Message{Role: "assistant", Content: content})
}

func (m *Manager) AddToolUseMessage(text, toolUseID, toolName string, arguments map[string]any) {
	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: text,
		ToolUses: []ToolUseBlock{{
			ToolUseID: toolUseID,
			ToolName:  toolName,
			Arguments: cloneArguments(arguments),
		}},
	})
}

func (m *Manager) AddAssistantMessageWithTools(text string, toolUses []ToolUseBlock) {
	m.history = append(m.history, Message{
		Role:     "assistant",
		Content:  text,
		ToolUses: cloneToolUses(toolUses),
	})
}

func (m *Manager) AddAssistantFull(text string, thinking []ThinkingBlock, toolUses []ToolUseBlock) {
	m.history = append(m.history, Message{
		Role:           "assistant",
		Content:        text,
		ThinkingBlocks: cloneThinkingBlocks(thinking),
		ToolUses:       cloneToolUses(toolUses),
	})
}

func (m *Manager) AddToolResultMessage(toolUseID, content string, isError bool) {
	m.history = append(m.history, Message{
		Role: "user",
		ToolResults: []ToolResultBlock{{
			ToolUseID: toolUseID,
			Content:   content,
			IsError:   isError,
		}},
	})
}

func (m *Manager) AddToolResultsMessage(results []ToolResultBlock) {
	m.history = append(m.history, Message{
		Role:        "user",
		ToolResults: cloneToolResults(results),
	})
}

func (m *Manager) AddSystemReminder(content string) {
	m.history = append(m.history, Message{
		Role:    "user",
		Content: "<system-reminder>\n" + content + "\n</system-reminder>",
	})
}

func (m *Manager) InjectLongTermMemory(instructions, memories string) {
	if m.ltmInjected {
		return
	}
	var sections []string
	if instructions != "" {
		sections = append(sections, "# devflowMd\nCodebase and user instructions are shown below. Be sure to adhere to these instructions. IMPORTANT: These instructions OVERRIDE any default behavior and you MUST follow them exactly as written.\n\n"+instructions)
	}
	if memories != "" {
		sections = append(sections, "# autoMemory\n"+memories)
	}
	if len(sections) == 0 {
		return
	}
	sections = append(sections, "# currentDate\nToday's date is "+time.Now().Format("2006-01-02")+".")
	body := strings.Join(sections, "\n\n")
	wrapped := "<system-reminder>\nAs you answer the user's questions, you can use the following context:\n" +
		body +
		"\n\n      IMPORTANT: this context may or may not be relevant to your tasks. You should not respond to this context unless it is highly relevant to your task.\n</system-reminder>"
	m.history = append([]Message{{Role: "user", Content: wrapped}}, m.history...)
	m.ltmInjected = true
}

func (m *Manager) GetMessages() []Message {
	result := make([]Message, len(m.history))
	for i, msg := range m.history {
		result[i] = cloneMessage(msg)
	}
	return result
}

func cloneMessage(msg Message) Message {
	return deepCopy(msg)
}

func cloneThinkingBlocks(blocks []ThinkingBlock) []ThinkingBlock {
	return deepCopy(blocks)
}

func cloneToolUses(toolUses []ToolUseBlock) []ToolUseBlock {
	return deepCopy(toolUses)
}

func cloneToolResults(results []ToolResultBlock) []ToolResultBlock {
	return deepCopy(results)
}

func cloneArguments(arguments map[string]any) map[string]any {
	return deepCopy(arguments)
}

func deepCopy[T any](value T) T {
	copied := deepCopyValue(reflect.ValueOf(value))
	if !copied.IsValid() {
		var zero T
		return zero
	}
	return copied.Interface().(T)
}

func deepCopyValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Type().Elem())
		cloned.Elem().Set(deepCopyValue(value.Elem()))
		return cloned
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Type()).Elem()
		cloned.Set(deepCopyValue(value.Elem()))
		return cloned
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			cloned.SetMapIndex(iter.Key(), deepCopyValue(iter.Value()))
		}
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(deepCopyValue(value.Index(i)))
		}
		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(deepCopyValue(value.Index(i)))
		}
		return cloned
	case reflect.Struct:
		cloned := reflect.New(value.Type()).Elem()
		for i := 0; i < value.NumField(); i++ {
			field := cloned.Field(i)
			if !field.CanSet() {
				continue
			}
			source := value.Field(i)
			if source.CanInterface() {
				field.Set(deepCopyValue(source))
				continue
			}
			field.Set(source)
		}
		return cloned
	default:
		return value
	}
}

func (m *Manager) Serialize(protocol string) []map[string]any {
	if protocol == "openai" {
		return m.serializeOpenAI()
	}
	return m.serializeAnthropic()
}

func (m *Manager) serializeAnthropic() []map[string]any {
	var result []map[string]any
	for _, msg := range m.history {
		if len(msg.ToolUses) > 0 {
			var content []map[string]any
			if msg.Content != "" {
				content = append(content, map[string]any{"type": "text", "text": msg.Content})
			}
			for _, tu := range msg.ToolUses {
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    tu.ToolUseID,
					"name":  tu.ToolName,
					"input": cloneArguments(tu.Arguments),
				})
			}
			result = append(result, map[string]any{"role": "assistant", "content": content})
		} else if len(msg.ToolResults) > 0 {
			var content []map[string]any
			for _, tr := range msg.ToolResults {
				content = append(content, map[string]any{
					"type":        "tool_result",
					"tool_use_id": tr.ToolUseID,
					"content":     tr.Content,
					"is_error":    tr.IsError,
				})
			}
			result = append(result, map[string]any{"role": "user", "content": content})
		} else {
			if len(result) > 0 {
				prev := result[len(result)-1]
				prevRole, _ := prev["role"].(string)
				if prevRole == msg.Role {
					prevContent, isString := prev["content"].(string)
					if isString {
						result[len(result)-1]["content"] = prevContent + "\n\n" + msg.Content
						continue
					}
					if blocks, ok := prev["content"].([]map[string]any); ok {
						blocks = append(blocks, map[string]any{"type": "text", "text": msg.Content})
						result[len(result)-1]["content"] = blocks
						continue
					}
				}
			}
			result = append(result, map[string]any{"role": msg.Role, "content": msg.Content})
		}
	}
	return result
}

func (m *Manager) serializeOpenAI() []map[string]any {
	var result []map[string]any
	for _, msg := range m.history {
		if len(msg.ToolUses) > 0 {
			if msg.Content != "" {
				result = append(result, map[string]any{"role": "assistant", "content": msg.Content})
			}
			for _, tu := range msg.ToolUses {
				argsJSON, _ := json.Marshal(tu.Arguments)
				result = append(result, map[string]any{
					"type":      "function_call",
					"name":      tu.ToolName,
					"call_id":   tu.ToolUseID,
					"arguments": string(argsJSON),
				})
			}
		} else if len(msg.ToolResults) > 0 {
			for _, tr := range msg.ToolResults {
				result = append(result, map[string]any{
					"type":    "function_call_output",
					"call_id": tr.ToolUseID,
					"output":  tr.Content,
				})
			}
		} else {
			result = append(result, map[string]any{"role": msg.Role, "content": msg.Content})
		}
	}
	return result
}
