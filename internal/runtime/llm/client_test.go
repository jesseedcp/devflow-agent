package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
)

func TestNewClientNilConfigReturnsError(t *testing.T) {
	client, err := NewClient(nil, "system")
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil", client)
	}
	if !strings.Contains(err.Error(), "provider config") {
		t.Fatalf("error = %q, want mention of provider config", err.Error())
	}
}

func TestNewClientRejectsUnknownProtocol(t *testing.T) {
	client, err := NewClient(&config.ProviderConfig{
		Protocol: "nope",
		Model:    "test-model",
		BaseURL:  "https://example.invalid",
		APIKey:   "test-key",
	}, "system")
	if err == nil {
		t.Fatal("expected error for unknown protocol")
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil", client)
	}
	if !strings.Contains(err.Error(), "unknown protocol") {
		t.Fatalf("error = %q, want unknown protocol", err.Error())
	}
}

func TestAnthropicStreamRejectsInvalidToolSchema(t *testing.T) {
	client, err := newAnthropicClient(&config.ProviderConfig{
		BaseURL: "https://example.invalid",
		APIKey:  "test-key",
		Model:   "claude-sonnet-4-6",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, []map[string]any{{
		"description": "missing name and input_schema",
	}})

	gotEvents := collectEvents(t, events)
	if len(gotEvents) != 0 {
		t.Fatalf("events = %#v, want none", gotEvents)
	}

	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "input_schema") || !strings.Contains(gotErrs[0].Error(), "name") {
		t.Fatalf("error = %q, want missing name and input_schema", gotErrs[0].Error())
	}
}

func TestOpenAICompatStreamRejectsInvalidToolSchema(t *testing.T) {
	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: "https://example.invalid",
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, []map[string]any{{
		"description": "missing name",
		"parameters":  map[string]any{"type": "object"},
	}})

	gotEvents := collectEvents(t, events)
	if len(gotEvents) != 0 {
		t.Fatalf("events = %#v, want none", gotEvents)
	}

	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "name") {
		t.Fatalf("error = %q, want missing name", gotErrs[0].Error())
	}
}

func TestOpenAIStreamRejectsInvalidToolSchema(t *testing.T) {
	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL: "https://example.invalid",
		APIKey:  "test-key",
		Model:   "o3",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, []map[string]any{{
		"name":        "missing-parameters",
		"description": "missing parameters object",
	}})

	gotEvents := collectEvents(t, events)
	if len(gotEvents) != 0 {
		t.Fatalf("events = %#v, want none", gotEvents)
	}

	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "parameters") {
		t.Fatalf("error = %q, want missing parameters", gotErrs[0].Error())
	}
}

func TestBuildAnthropicToolsPreservesTopLevelInputSchemaConstraints(t *testing.T) {
	tools, err := buildAnthropicTools([]map[string]any{{
		"name": "search_docs",
		"input_schema": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"query": map[string]any{"type": "string"}},
			"required":             []any{"query"},
			"additionalProperties": false,
			"patternProperties": map[string]any{
				"^x-": map[string]any{"type": "string"},
			},
			"oneOf": []any{
				map[string]any{"required": []any{"query"}},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].OfTool == nil {
		t.Fatalf("tools = %#v, want one anthropic tool", tools)
	}

	schema := tools[0].OfTool.InputSchema
	properties, ok := schema.Properties.(map[string]any)
	if !ok {
		t.Fatalf("properties type = %T, want map[string]any", schema.Properties)
	}
	if _, ok := properties["query"]; !ok {
		t.Fatalf("properties = %#v, want query field", properties)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "query" {
		t.Fatalf("required = %#v, want [query]", schema.Required)
	}
	if schema.ExtraFields == nil {
		t.Fatal("extra fields should be preserved")
	}
	if got := schema.ExtraFields["additionalProperties"]; got != false {
		t.Fatalf("additionalProperties = %#v, want false", got)
	}
	if _, ok := schema.ExtraFields["patternProperties"]; !ok {
		t.Fatalf("extra fields = %#v, want patternProperties", schema.ExtraFields)
	}
	if _, ok := schema.ExtraFields["oneOf"]; !ok {
		t.Fatalf("extra fields = %#v, want oneOf", schema.ExtraFields)
	}
	if _, ok := schema.ExtraFields["properties"]; ok {
		t.Fatalf("extra fields should not duplicate properties: %#v", schema.ExtraFields)
	}
	if _, ok := schema.ExtraFields["required"]; ok {
		t.Fatalf("extra fields should not duplicate required: %#v", schema.ExtraFields)
	}
}

func TestBuildAnthropicToolsCopiesExtraFieldsIndependently(t *testing.T) {
	inputSchema := map[string]any{
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
		"required":             []any{"query"},
		"additionalProperties": false,
		"oneOf": []any{
			map[string]any{"required": []any{"query"}},
		},
	}

	tools, err := buildAnthropicTools([]map[string]any{{
		"name":         "search_docs",
		"input_schema": inputSchema,
	}})
	if err != nil {
		t.Fatal(err)
	}

	inputSchema["additionalProperties"] = true
	inputSchema["oneOf"].([]any)[0].(map[string]any)["required"] = []any{"mutated"}
	inputSchema["properties"].(map[string]any)["query"].(map[string]any)["type"] = "number"

	schema := tools[0].OfTool.InputSchema
	if got := schema.ExtraFields["additionalProperties"]; got != false {
		t.Fatalf("additionalProperties = %#v, want false", got)
	}
	oneOfJSON, err := json.Marshal(schema.ExtraFields["oneOf"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(oneOfJSON), "mutated") {
		t.Fatalf("oneOf should be deep copied, got %s", oneOfJSON)
	}
	properties := schema.Properties.(map[string]any)
	if got := properties["query"].(map[string]any)["type"]; got != "string" {
		t.Fatalf("properties should be deep copied, got %#v", got)
	}
}

func TestBuildOpenAIToolsCopiesParametersIndependently(t *testing.T) {
	parameters := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type": "string",
				"enum": []any{"docs", "code"},
			},
		},
		"required":             []any{"query"},
		"additionalProperties": false,
		"allOf": []any{
			map[string]any{"required": []any{"query"}},
		},
	}

	tools, err := buildOpenAITools([]map[string]any{{
		"name":        "search_docs",
		"description": "search",
		"parameters":  parameters,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].OfFunction == nil {
		t.Fatalf("tools = %#v, want one OpenAI function tool", tools)
	}

	// Mutate the caller-owned schema after construction; built tool must be unaffected.
	parameters["additionalProperties"] = true
	parameters["properties"].(map[string]any)["query"].(map[string]any)["type"] = "number"
	parameters["properties"].(map[string]any)["query"].(map[string]any)["enum"].([]any)[0] = "mutated-enum"
	parameters["required"].([]any)[0] = "mutated"
	parameters["allOf"].([]any)[0].(map[string]any)["required"].([]any)[0] = "mutated-allof"

	built := tools[0].OfFunction.Parameters
	if got := built["additionalProperties"]; got != false {
		t.Fatalf("additionalProperties = %#v, want false (deep copy)", got)
	}
	properties := built["properties"].(map[string]any)
	query := properties["query"].(map[string]any)
	if got := query["type"]; got != "string" {
		t.Fatalf("properties should be deep copied, got %#v", got)
	}
	enum := query["enum"].([]any)
	if len(enum) != 2 || enum[0] != "docs" || enum[1] != "code" {
		t.Fatalf("enum = %#v, want [docs code] (deep copy)", enum)
	}
	required := built["required"].([]any)
	if len(required) != 1 || required[0] != "query" {
		t.Fatalf("required = %#v, want [query] (deep copy)", required)
	}
	allOf := built["allOf"].([]any)
	if got := allOf[0].(map[string]any)["required"].([]any)[0]; got != "query" {
		t.Fatalf("allOf = %#v, want nested required to remain query", allOf)
	}
}

func TestBuildOpenAICompatToolsCopiesParametersIndependently(t *testing.T) {
	parameters := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type": "string",
				"enum": []any{"docs", "code"},
			},
		},
		"required":             []any{"query"},
		"additionalProperties": false,
		"allOf": []any{
			map[string]any{"required": []any{"query"}},
		},
	}

	tools, err := buildOpenAICompatTools([]map[string]any{{
		"name":        "search_docs",
		"description": "search",
		"parameters":  parameters,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %#v, want one OpenAI-compatible tool", tools)
	}

	parameters["additionalProperties"] = true
	parameters["properties"].(map[string]any)["query"].(map[string]any)["type"] = "number"
	parameters["properties"].(map[string]any)["query"].(map[string]any)["enum"].([]any)[0] = "mutated-enum"
	parameters["required"].([]any)[0] = "mutated"
	parameters["allOf"].([]any)[0].(map[string]any)["required"].([]any)[0] = "mutated-allof"

	built := tools[0].Function.Parameters
	if got := built["additionalProperties"]; got != false {
		t.Fatalf("additionalProperties = %#v, want false (deep copy)", got)
	}
	properties := built["properties"].(map[string]any)
	query := properties["query"].(map[string]any)
	if got := query["type"]; got != "string" {
		t.Fatalf("properties should be deep copied, got %#v", got)
	}
	enum := query["enum"].([]any)
	if len(enum) != 2 || enum[0] != "docs" || enum[1] != "code" {
		t.Fatalf("enum = %#v, want [docs code] (deep copy)", enum)
	}
	required := built["required"].([]any)
	if len(required) != 1 || required[0] != "query" {
		t.Fatalf("required = %#v, want [query] (deep copy)", required)
	}
	allOf := built["allOf"].([]any)
	if got := allOf[0].(map[string]any)["required"].([]any)[0]; got != "query" {
		t.Fatalf("allOf = %#v, want nested required to remain query", allOf)
	}
}
