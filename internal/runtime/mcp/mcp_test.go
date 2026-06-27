package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTransportKind(t *testing.T) {
	tests := []struct {
		name      string
		transport string
		want      string
	}{
		{name: "empty defaults to streamable http", want: "http"},
		{name: "http uses streamable http", transport: "http", want: "http"},
		{name: "streamable uses streamable http", transport: "streamable", want: "http"},
		{name: "sse selects legacy sse", transport: "sse", want: "sse"},
		{name: "sse is case insensitive", transport: "SSE", want: "sse"},
		{name: "unknown uses streamable http", transport: "websocket", want: "http"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServerConfig{Transport: tt.transport}
			if got := cfg.transportKind(); got != tt.want {
				t.Fatalf("transportKind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewHTTPClientExpandsHeaderEnv(t *testing.T) {
	t.Setenv("DEVFLOW_MCP_TOKEN", "secret-token")

	var gotAuth, gotPlain string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPlain = r.Header.Get("X-Plain")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newHTTPClient(map[string]string{
		"Authorization": "Bearer ${DEVFLOW_MCP_TOKEN}",
		"X-Plain":       "literal",
	})

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer secret-token")
	}
	if gotPlain != "literal" {
		t.Fatalf("X-Plain header = %q, want literal", gotPlain)
	}
}

func TestNewHTTPClientWithoutHeadersUsesDefaultClient(t *testing.T) {
	if got := newHTTPClient(nil); got != http.DefaultClient {
		t.Fatalf("newHTTPClient(nil) = %p, want http.DefaultClient %p", got, http.DefaultClient)
	}
}

func TestSanitizeName(t *testing.T) {
	got := SanitizeName("context-7.io/read docs")
	want := "context_7_io_read_docs"
	if got != want {
		t.Fatalf("SanitizeName() = %q, want %q", got, want)
	}
}

func TestMCPToolWrapperSchemaDefaultsNilInputSchema(t *testing.T) {
	wrapper := &MCPToolWrapper{
		serverName: "context-7",
		toolDef: &sdkmcp.Tool{
			Name:        "read-docs",
			Description: "Read documentation",
		},
	}

	schema := wrapper.Schema()
	if schema["name"] != "mcp__context_7__read_docs" {
		t.Fatalf("schema name = %v", schema["name"])
	}
	if schema["description"] != "Read documentation" {
		t.Fatalf("schema description = %v", schema["description"])
	}
	inputSchema, ok := schema["input_schema"].(map[string]any)
	if !ok {
		t.Fatalf("input_schema type = %T, want map[string]any", schema["input_schema"])
	}
	if inputSchema["type"] != "object" {
		t.Fatalf("input schema type = %v, want object", inputSchema["type"])
	}
	if _, ok := inputSchema["properties"].(map[string]any); !ok {
		t.Fatalf("input schema properties type = %T, want map[string]any", inputSchema["properties"])
	}
}

func TestMCPToolWrapperSchemaPreservesInputSchema(t *testing.T) {
	input := map[string]any{
		"type":       "object",
		"properties": map[string]any{"query": map[string]any{"type": "string"}},
		"required":   []string{"query"},
	}
	wrapper := &MCPToolWrapper{
		serverName: "context7",
		toolDef: &sdkmcp.Tool{
			Name:        "resolve",
			Description: "Resolve library",
			InputSchema: input,
		},
	}

	schema := wrapper.Schema()
	if !reflect.DeepEqual(schema["input_schema"], input) {
		t.Fatalf("input_schema was not preserved")
	}
}

type fakeToolCaller struct {
	gotName string
	gotArgs map[string]any
	text    string
	isError bool
	err     error
}

func (f *fakeToolCaller) CallTool(_ context.Context, name string, args map[string]any) (string, bool, error) {
	f.gotName = name
	f.gotArgs = args
	return f.text, f.isError, f.err
}

func TestMCPToolWrapperExecuteCallsTool(t *testing.T) {
	caller := &fakeToolCaller{text: "result text"}
	args := map[string]any{"query": "bubbles"}
	wrapper := &MCPToolWrapper{
		serverName: "context7",
		toolDef:    &sdkmcp.Tool{Name: "resolve-library-id"},
		client:     caller,
	}

	result := wrapper.Execute(context.Background(), args)
	if result.IsError {
		t.Fatalf("Execute() IsError = true, output %q", result.Output)
	}
	if result.Output != "result text" {
		t.Fatalf("Execute() output = %q, want result text", result.Output)
	}
	if caller.gotName != "resolve-library-id" {
		t.Fatalf("CallTool name = %q", caller.gotName)
	}
	if caller.gotArgs["query"] != "bubbles" {
		t.Fatalf("CallTool args = %#v", caller.gotArgs)
	}
}

func TestMCPToolWrapperExecutePropagatesToolErrorFlag(t *testing.T) {
	wrapper := &MCPToolWrapper{
		serverName: "context7",
		toolDef:    &sdkmcp.Tool{Name: "resolve-library-id"},
		client:     &fakeToolCaller{text: "tool-level error", isError: true},
	}

	result := wrapper.Execute(context.Background(), nil)
	if !result.IsError {
		t.Fatalf("Execute() IsError = false, want true")
	}
	if result.Output != "tool-level error" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

func TestMCPToolWrapperExecuteReportsProtocolError(t *testing.T) {
	wrapper := &MCPToolWrapper{
		serverName: "context7",
		toolDef:    &sdkmcp.Tool{Name: "resolve-library-id"},
		client:     &fakeToolCaller{err: errors.New("boom")},
	}

	result := wrapper.Execute(context.Background(), nil)
	if !result.IsError {
		t.Fatalf("Execute() IsError = false, want true")
	}
	want := "MCP tool call failed: boom"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

func TestClientImplementationNameIsDevflow(t *testing.T) {
	if clientImplementationName != "Devflow" {
		t.Fatalf("clientImplementationName = %q, want Devflow", clientImplementationName)
	}
}
