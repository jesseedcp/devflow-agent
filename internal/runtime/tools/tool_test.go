package tools

import "testing"

func TestGetAllSchemasConvertsOpenAICompatTools(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ReadFileTool{})

	schemas := reg.GetAllSchemas("openai-compat")
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	schema := schemas[0]
	if _, ok := schema["parameters"]; !ok {
		t.Fatalf("openai-compatible schema missing parameters: %#v", schema)
	}
	if _, ok := schema["input_schema"]; ok {
		t.Fatalf("openai-compatible schema should not expose input_schema: %#v", schema)
	}
}
