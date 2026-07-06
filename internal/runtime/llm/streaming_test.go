package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicrespjson "github.com/anthropics/anthropic-sdk-go/packages/respjson"
	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
)

func TestOpenAICompatStreamContextCancellationReturnsNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	ctx, cancel := context.WithCancel(context.Background())
	events, errs := client.Stream(ctx, conv, nil)
	time.AfterFunc(50*time.Millisecond, cancel)

	gotEvents := collectEvents(t, events)
	if len(gotEvents) != 0 {
		t.Fatalf("events = %#v, want none", gotEvents)
	}

	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if _, ok := gotErrs[0].(*NetworkError); !ok {
		t.Fatalf("error type = %T, want *NetworkError", gotErrs[0])
	}
	if !strings.Contains(gotErrs[0].Error(), "context cancelled") {
		t.Fatalf("error = %q, want context cancelled", gotErrs[0].Error())
	}
}

func TestOpenAICompatStreamEmitsSingleStreamEndWithUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{"content":"DEVFLOW_"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{"content":"RUNTIME_OK"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	var text strings.Builder
	var ends []StreamEnd
	for _, event := range gotEvents {
		switch typed := event.(type) {
		case TextDelta:
			text.WriteString(typed.Text)
		case StreamEnd:
			ends = append(ends, typed)
		}
	}

	if got := text.String(); got != "DEVFLOW_RUNTIME_OK" {
		t.Fatalf("text = %q, want DEVFLOW_RUNTIME_OK", got)
	}
	if len(ends) != 1 {
		t.Fatalf("stream end count = %d, want 1", len(ends))
	}
	if ends[0].StopReason != "stop" {
		t.Fatalf("stop reason = %q, want stop", ends[0].StopReason)
	}
	if ends[0].Usage.InputTokens != 7 || ends[0].Usage.OutputTokens != 3 {
		t.Fatalf("usage = %+v, want input 7 output 3", ends[0].Usage)
	}
}

func TestOpenAICompatToolCallsFinishEmitsSingleToolCallCompleteAndStreamEndWithUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1725392480,"model":"ark-code-latest","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}],"usage":null}`,
			`data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1725392480,"model":"ark-code-latest","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Beijing\"}"}}]},"finish_reason":null}],"usage":null}`,
			`data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1725392480,"model":"ark-code-latest","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":null}`,
			`data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1725392480,"model":"ark-code-latest","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	var completes []ToolCallComplete
	var ends []StreamEnd
	for _, event := range gotEvents {
		switch typed := event.(type) {
		case ToolCallComplete:
			completes = append(completes, typed)
		case StreamEnd:
			ends = append(ends, typed)
		}
	}

	if len(completes) != 1 {
		t.Fatalf("tool call complete count = %d, want 1", len(completes))
	}
	if completes[0].ToolID != "call_123" || completes[0].ToolName != "get_weather" {
		t.Fatalf("tool call complete = %+v, want call_123/get_weather", completes[0])
	}
	if got := completes[0].Arguments["city"]; got != "Beijing" {
		t.Fatalf("tool call arguments = %+v, want city=Beijing", completes[0].Arguments)
	}
	if len(ends) != 1 {
		t.Fatalf("stream end count = %d, want 1", len(ends))
	}
	if ends[0].StopReason != "tool_calls" {
		t.Fatalf("stop reason = %q, want tool_calls", ends[0].StopReason)
	}
	if ends[0].Usage.InputTokens != 11 || ends[0].Usage.OutputTokens != 5 {
		t.Fatalf("usage = %+v, want input 11 output 5", ends[0].Usage)
	}
}

func TestOpenAICompatStreamSendsMaxTokensInRequest(t *testing.T) {
	var captured map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-max","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL:         srv.URL,
		APIKey:          "test-key",
		Model:           "ark-code-latest",
		MaxOutputTokens: 13,
	}, "system")
	if err != nil {
		t.Fatal(err)
	}
	client.SetMaxOutputTokens(77)

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	_ = collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	if got := captured["max_tokens"]; got != float64(77) {
		t.Fatalf("max_tokens = %#v, want 77", got)
	}
}

func TestOpenAICompatStreamLengthFinishReasonReturnsErrorWithoutStreamEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-length","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":"length"}]}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)

	assertNoStreamEnd(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "length") {
		t.Fatalf("error = %q, want finish_reason length", gotErrs[0].Error())
	}
}

func TestOpenAICompatStopChunkWithInlineUsageEmitsSingleStreamEndWithUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-inline","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":2,"total_tokens":10}}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	var text strings.Builder
	var ends []StreamEnd
	for _, event := range gotEvents {
		switch typed := event.(type) {
		case TextDelta:
			text.WriteString(typed.Text)
		case StreamEnd:
			ends = append(ends, typed)
		}
	}

	if got := text.String(); got != "ok" {
		t.Fatalf("text = %q, want ok", got)
	}
	if len(ends) != 1 {
		t.Fatalf("stream end count = %d, want 1", len(ends))
	}
	if ends[0].StopReason != "stop" {
		t.Fatalf("stop reason = %q, want stop", ends[0].StopReason)
	}
	if ends[0].Usage.InputTokens != 8 || ends[0].Usage.OutputTokens != 2 {
		t.Fatalf("usage = %+v, want input 8 output 2", ends[0].Usage)
	}
}

func TestOpenAICompatStopWithoutUsageReturnsErrorWithoutStreamEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-nouse","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)

	assertNoStreamEnd(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "without usage") {
		t.Fatalf("error = %q, want without usage", gotErrs[0].Error())
	}
}

func TestOpenAICompatToolCallCompleteOrderFollowsIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-order","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_0","type":"function","function":{"name":"tool_zero","arguments":""}},{"index":1,"id":"call_1","type":"function","function":{"name":"tool_one","arguments":""}}]},"finish_reason":null}],"usage":null}`,
			`data: {"id":"chatcmpl-order","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"first\":0}"}},{"index":1,"function":{"arguments":"{\"second\":1}"}}]},"finish_reason":"tool_calls"}],"usage":null}`,
			`data: {"id":"chatcmpl-order","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[],"usage":{"prompt_tokens":14,"completion_tokens":6,"total_tokens":20}}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	var completes []ToolCallComplete
	var ends []StreamEnd
	for _, event := range gotEvents {
		switch typed := event.(type) {
		case ToolCallComplete:
			completes = append(completes, typed)
		case StreamEnd:
			ends = append(ends, typed)
		}
	}

	if len(completes) != 2 {
		t.Fatalf("tool call complete count = %d, want 2", len(completes))
	}
	assertToolCallComplete(t, completes[0], "call_0", "tool_zero", "first", float64(0))
	assertToolCallComplete(t, completes[1], "call_1", "tool_one", "second", float64(1))
	if len(ends) != 1 {
		t.Fatalf("stream end count = %d, want 1", len(ends))
	}
	if ends[0].StopReason != "tool_calls" {
		t.Fatalf("stop reason = %q, want tool_calls", ends[0].StopReason)
	}
	if ends[0].Usage.InputTokens != 14 || ends[0].Usage.OutputTokens != 6 {
		t.Fatalf("usage = %+v, want input 14 output 6", ends[0].Usage)
	}
}

func TestAnthropicInvalidToolArgumentsJSONReturnsErrorWithoutToolCallComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_bad","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":2,"output_tokens":1}}}`,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"bad_tool","input":{}}}`,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"[]"}}`,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":1}}`,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newAnthropicClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "claude-sonnet-4-6",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	assertNoToolCallComplete(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
}

func TestDecodeToolArgumentsRejectsAnthropicInvalidJSON(t *testing.T) {
	_, err := decodeToolArguments("Anthropic", "[]")
	if err == nil {
		t.Fatal("expected invalid JSON shape error")
	}
	if !strings.Contains(err.Error(), "Anthropic tool call arguments invalid JSON") {
		t.Fatalf("error = %q, want Anthropic invalid JSON", err.Error())
	}
}

func TestOpenAIInvalidToolArgumentsJSONReturnsErrorWithoutToolCallComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"fc_bad","type":"function_call","call_id":"call_bad","name":"bad_tool","arguments":""}}`,
			`data: {"type":"response.function_call_arguments.delta","sequence_number":2,"item_id":"fc_bad","output_index":0,"delta":"{"}`,
			`data: {"type":"response.function_call_arguments.done","sequence_number":3,"item_id":"fc_bad","output_index":0,"arguments":"{"}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "o3",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	assertNoToolCallComplete(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "OpenAI tool call arguments invalid JSON") {
		t.Fatalf("error = %q, want OpenAI invalid JSON", gotErrs[0].Error())
	}
}

func TestOpenAICompatInvalidToolArgumentsJSONReturnsErrorWithoutToolCallComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-bad","object":"chat.completion.chunk","created":1,"model":"ark-code-latest","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_bad","type":"function","function":{"name":"bad_tool","arguments":"{"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":4,"completion_tokens":1,"total_tokens":5}}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "ark-code-latest",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	assertNoToolCallComplete(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "OpenAI-compatible tool call arguments invalid JSON") {
		t.Fatalf("error = %q, want OpenAI-compatible invalid JSON", gotErrs[0].Error())
	}
}

func TestAnthropicSuccessfulStreamEmitsSingleStreamEndWithUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakeAnthropicSSE(w, r)
	}))
	defer srv.Close()

	client, err := newAnthropicClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "claude-sonnet-4-6",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	var text strings.Builder
	var ends []StreamEnd
	for _, event := range gotEvents {
		switch typed := event.(type) {
		case TextDelta:
			text.WriteString(typed.Text)
		case StreamEnd:
			ends = append(ends, typed)
		}
	}

	if got := text.String(); got != "hi" {
		t.Fatalf("text = %q, want hi", got)
	}
	if len(ends) != 1 {
		t.Fatalf("stream end count = %d, want 1", len(ends))
	}
	if ends[0].Usage.InputTokens != 10 || ends[0].Usage.OutputTokens != 1 {
		t.Fatalf("usage = %+v, want input 10 output 1", ends[0].Usage)
	}
}

func TestAnthropicMaxTokensStopReturnsErrorWithoutStreamEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_limit","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":1}}}`,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":1}}`,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newAnthropicClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "claude-sonnet-4-6",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	assertNoStreamEnd(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
}

func TestAnthropicCompletedWithoutUsageReturnsErrorWithoutStreamEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_nouse","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null}}`,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newAnthropicClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "claude-sonnet-4-6",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	assertNoStreamEnd(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
}

func TestValidateAnthropicTerminalMessageRejectsMaxTokens(t *testing.T) {
	message := anthropic.Message{
		StopReason: anthropic.StopReasonMaxTokens,
	}
	message.JSON.Usage = anthropicrespjson.NewField(`{"input_tokens":1,"output_tokens":1}`)

	err := validateAnthropicTerminalMessage(message)
	if err == nil {
		t.Fatal("expected max_tokens error")
	}
	if !strings.Contains(err.Error(), "max_tokens") {
		t.Fatalf("error = %q, want max_tokens", err.Error())
	}
}

func TestValidateAnthropicTerminalMessageRejectsMissingUsage(t *testing.T) {
	message := anthropic.Message{
		StopReason: anthropic.StopReasonEndTurn,
	}

	err := validateAnthropicTerminalMessage(message)
	if err == nil {
		t.Fatal("expected missing usage error")
	}
	if !strings.Contains(err.Error(), "without usage") {
		t.Fatalf("error = %q, want without usage", err.Error())
	}
}

func TestOpenAISuccessfulStreamEmitsSingleStreamEndWithUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"response.output_text.delta","sequence_number":1,"delta":"hi"}`,
			`data: {"type":"response.completed","sequence_number":2,"response":{"id":"resp_1","status":"completed","output":[],"usage":{"input_tokens":9,"output_tokens":4}}}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "o3",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	var text strings.Builder
	var ends []StreamEnd
	for _, event := range gotEvents {
		switch typed := event.(type) {
		case TextDelta:
			text.WriteString(typed.Text)
		case StreamEnd:
			ends = append(ends, typed)
		}
	}

	if got := text.String(); got != "hi" {
		t.Fatalf("text = %q, want hi", got)
	}
	if len(ends) != 1 {
		t.Fatalf("stream end count = %d, want 1", len(ends))
	}
	if ends[0].Usage.InputTokens != 9 || ends[0].Usage.OutputTokens != 4 {
		t.Fatalf("usage = %+v, want input 9 output 4", ends[0].Usage)
	}
}

func TestOpenAICompletedWithoutUsageReturnsErrorWithoutStreamEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"response.completed","sequence_number":1,"response":{"id":"resp_no_usage","status":"completed","output":[]}}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "o3",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)

	assertNoStreamEnd(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "without usage") {
		t.Fatalf("error = %q, want without usage", gotErrs[0].Error())
	}
}

func TestOpenAIStreamErrorEventReturnsErrorWithoutStreamEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"error","sequence_number":1,"code":"server_error","message":"upstream failed","param":""}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "o3",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)

	assertNoStreamEnd(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "upstream failed") {
		t.Fatalf("error = %q, want upstream failed", gotErrs[0].Error())
	}
}

func TestOpenAIStreamFailedEventReturnsErrorWithoutStreamEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"response.failed","sequence_number":1,"response":{"id":"resp_fail","status":"failed","output":[],"error":{"code":"server_error","message":"model crashed"}}}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "o3",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)

	assertNoStreamEnd(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "model crashed") {
		t.Fatalf("error = %q, want model crashed", gotErrs[0].Error())
	}
}

func TestOpenAIStreamIncompleteEventReturnsErrorWithoutStreamEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"response.incomplete","sequence_number":1,"response":{"id":"resp_incomplete","status":"incomplete","output":[],"incomplete_details":{"reason":"max_output_tokens"}}}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "o3",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)

	assertNoStreamEnd(t, gotEvents)
	if len(gotErrs) != 1 {
		t.Fatalf("error count = %d, want 1", len(gotErrs))
	}
	if !strings.Contains(gotErrs[0].Error(), "max_output_tokens") {
		t.Fatalf("error = %q, want max_output_tokens", gotErrs[0].Error())
	}
}

func TestOpenAIStreamInterleavedToolCallsKeepArgumentsSeparated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discardRequestBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"tool_one","arguments":""}}`,
			`data: {"type":"response.output_item.added","sequence_number":2,"output_index":1,"item":{"id":"fc_2","type":"function_call","call_id":"call_2","name":"tool_two","arguments":""}}`,
			`data: {"type":"response.function_call_arguments.delta","sequence_number":3,"item_id":"fc_1","output_index":0,"delta":"{\"alpha\":"}`,
			`data: {"type":"response.function_call_arguments.delta","sequence_number":4,"item_id":"fc_2","output_index":1,"delta":"{\"beta\":"}`,
			`data: {"type":"response.function_call_arguments.delta","sequence_number":5,"item_id":"fc_1","output_index":0,"delta":"1}"}`,
			`data: {"type":"response.function_call_arguments.delta","sequence_number":6,"item_id":"fc_2","output_index":1,"delta":"2}"}`,
			`data: {"type":"response.function_call_arguments.done","sequence_number":7,"item_id":"fc_2","output_index":1,"arguments":"{\"beta\":2}"}`,
			`data: {"type":"response.function_call_arguments.done","sequence_number":8,"item_id":"fc_1","output_index":0,"arguments":"{\"alpha\":1}"}`,
			`data: {"type":"response.completed","sequence_number":9,"response":{"id":"resp_tools","status":"completed","output":[],"usage":{"input_tokens":12,"output_tokens":6}}}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "o3",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	var deltas []ToolCallDelta
	var completes []ToolCallComplete
	var ends []StreamEnd
	for _, event := range gotEvents {
		switch typed := event.(type) {
		case ToolCallDelta:
			deltas = append(deltas, typed)
		case ToolCallComplete:
			completes = append(completes, typed)
		case StreamEnd:
			ends = append(ends, typed)
		}
	}

	if len(deltas) != 4 {
		t.Fatalf("tool call delta count = %d, want 4", len(deltas))
	}
	if len(completes) != 2 {
		t.Fatalf("tool call complete count = %d, want 2", len(completes))
	}
	assertToolCallComplete(t, completes[0], "call_1", "tool_one", "alpha", float64(1))
	assertToolCallComplete(t, completes[1], "call_2", "tool_two", "beta", float64(2))
	if len(ends) != 1 {
		t.Fatalf("stream end count = %d, want 1", len(ends))
	}
}

func TestOpenAIStreamSendsMaxOutputTokensInRequest(t *testing.T) {
	var captured map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"response.completed","sequence_number":1,"response":{"id":"resp_max","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":1}}}`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		BaseURL:         srv.URL,
		APIKey:          "test-key",
		Model:           "o3",
		MaxOutputTokens: 77,
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	_ = collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	if got := captured["max_output_tokens"]; got != float64(77) {
		t.Fatalf("max_output_tokens = %#v, want 77", got)
	}
}

func collectEvents(t *testing.T, events <-chan StreamEvent) []StreamEvent {
	t.Helper()

	done := make(chan []StreamEvent, 1)
	go func() {
		var out []StreamEvent
		for event := range events {
			out = append(out, event)
		}
		done <- out
	}()

	select {
	case out := <-done:
		return out
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for events channel to close")
		return nil
	}
}

func collectErrors(t *testing.T, errs <-chan error) []error {
	t.Helper()

	done := make(chan []error, 1)
	go func() {
		var out []error
		for err := range errs {
			if err != nil {
				out = append(out, err)
			}
		}
		done <- out
	}()

	select {
	case out := <-done:
		return out
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error channel to close")
		return nil
	}
}

func assertNoStreamEnd(t *testing.T, events []StreamEvent) {
	t.Helper()
	for _, event := range events {
		if _, ok := event.(StreamEnd); ok {
			t.Fatalf("unexpected StreamEnd in events: %#v", events)
		}
	}
}

func assertNoToolCallComplete(t *testing.T, events []StreamEvent) {
	t.Helper()
	for _, event := range events {
		if _, ok := event.(ToolCallComplete); ok {
			t.Fatalf("unexpected ToolCallComplete in events: %#v", events)
		}
	}
}

func assertToolCallComplete(t *testing.T, event ToolCallComplete, wantID, wantName, wantArgKey string, wantArgValue any) {
	t.Helper()
	if event.ToolID != wantID || event.ToolName != wantName {
		t.Fatalf("tool call complete = %+v, want id=%s name=%s", event, wantID, wantName)
	}
	if got := event.Arguments[wantArgKey]; got != wantArgValue {
		t.Fatalf("tool call arguments = %+v, want %s=%v", event.Arguments, wantArgKey, wantArgValue)
	}
}

func discardRequestBody(r *http.Request) {
	io.Copy(io.Discard, r.Body)
	r.Body.Close()
}

func TestOpenAICompatArkStyleFragmentedToolCallAccumulatesByIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-ark","object":"chat.completion.chunk","created":1,"model":"glm-5.2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_read_1","type":"function","function":{"name":"ReadFile","arguments":""}}]},"finish_reason":null}],"usage":null}`,
			`data: {"id":"chatcmpl-ark","object":"chat.completion.chunk","created":1,"model":"glm-5.2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"file_"}}]},"finish_reason":null}],"usage":null}`,
			`data: {"id":"chatcmpl-ark","object":"chat.completion.chunk","created":1,"model":"glm-5.2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"path\":\"internal/weather/service.go\"}"}}]},"finish_reason":null}],"usage":null}`,
			`data: {"id":"chatcmpl-ark","object":"chat.completion.chunk","created":1,"model":"glm-5.2","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":null}`,
			`data: {"id":"chatcmpl-ark","object":"chat.completion.chunk","created":1,"model":"glm-5.2","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":8,"total_tokens":18}}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "glm-5.2",
	}, "system")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("read the file")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}

	var starts []ToolCallStart
	var deltas []ToolCallDelta
	var completes []ToolCallComplete
	var ends []StreamEnd
	for _, event := range gotEvents {
		switch typed := event.(type) {
		case ToolCallStart:
			starts = append(starts, typed)
		case ToolCallDelta:
			deltas = append(deltas, typed)
		case ToolCallComplete:
			completes = append(completes, typed)
		case StreamEnd:
			ends = append(ends, typed)
		}
	}
	if len(starts) != 1 {
		t.Fatalf("starts = %d, want 1; events=%#v", len(starts), gotEvents)
	}
	if len(deltas) != 2 {
		t.Fatalf("deltas = %d, want 2; events=%#v", len(deltas), gotEvents)
	}
	if len(completes) != 1 {
		t.Fatalf("completes = %d, want 1; events=%#v", len(completes), gotEvents)
	}
	if completes[0].ToolID != "call_read_1" || completes[0].ToolName != "ReadFile" {
		t.Fatalf("complete = %+v, want call_read_1/ReadFile", completes[0])
	}
	if got := completes[0].Arguments["file_path"]; got != "internal/weather/service.go" {
		t.Fatalf("arguments = %+v, want file_path", completes[0].Arguments)
	}
	if len(ends) != 1 || ends[0].StopReason != "tool_calls" {
		t.Fatalf("stream ends = %+v, want one tool_calls end", ends)
	}
}

func TestOpenAICompatDoesNotEmitDuplicateToolCompleteAcrossUsageChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join([]string{
			`data: {"id":"chatcmpl-dup","object":"chat.completion.chunk","created":1,"model":"glm-5.2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"Glob","arguments":"{\"pattern\":\"*.go\"}"}}]},"finish_reason":"tool_calls"}],"usage":null}`,
			`data: {"id":"chatcmpl-dup","object":"chat.completion.chunk","created":1,"model":"glm-5.2","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":4,"total_tokens":9}}`,
			`data: [DONE]`,
		}, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAICompatClient(&config.ProviderConfig{BaseURL: srv.URL, APIKey: "test-key", Model: "glm-5.2"}, "system")
	if err != nil {
		t.Fatal(err)
	}
	conv := conversation.NewManager()
	conv.AddUserMessage("glob")

	events, errs := client.Stream(context.Background(), conv, nil)
	gotEvents := collectEvents(t, events)
	gotErrs := collectErrors(t, errs)
	if len(gotErrs) != 0 {
		t.Fatalf("errors = %v, want none", gotErrs)
	}
	var completes []ToolCallComplete
	for _, event := range gotEvents {
		if complete, ok := event.(ToolCallComplete); ok {
			completes = append(completes, complete)
		}
	}
	if len(completes) != 1 {
		t.Fatalf("complete count = %d, want 1; events=%#v", len(completes), gotEvents)
	}
}
