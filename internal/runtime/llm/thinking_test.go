package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
)

func fakeAnthropicSSE(w http.ResponseWriter, r *http.Request) []byte {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "event: message_start\n")
	io.WriteString(w, `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":1}}}`+"\n\n")
	io.WriteString(w, "event: content_block_start\n")
	io.WriteString(w, `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`+"\n\n")
	io.WriteString(w, "event: content_block_delta\n")
	io.WriteString(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`+"\n\n")
	io.WriteString(w, "event: content_block_stop\n")
	io.WriteString(w, `data: {"type":"content_block_stop","index":0}`+"\n\n")
	io.WriteString(w, "event: message_delta\n")
	io.WriteString(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`+"\n\n")
	io.WriteString(w, "event: message_stop\n")
	io.WriteString(w, `data: {"type":"message_stop"}`+"\n\n")
	return body
}

func drainStream(client Client, conv *conversation.Manager) {
	events, errs := client.Stream(context.Background(), conv, nil)
	for range events {
	}
	for range errs {
	}
}

func TestSupportsAdaptiveThinking(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-sonnet-4-6", true},
		{"claude-sonnet-4-6-20250514", true},
		{"claude-opus-4-6", true},
		{"claude-opus-4-6-20250514", true},
		{"claude-opus-4-7", true},
		{"claude-sonnet-4-5-20250514", false},
		{"claude-sonnet-4-20250514", false},
		{"claude-3-5-sonnet-20241022", false},
		{"glm-4.7", false},
		{"some-unknown-model", false},
	}
	for _, tt := range tests {
		got := supportsAdaptiveThinking(tt.model)
		if got != tt.want {
			t.Errorf("supportsAdaptiveThinking(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestAnthropicThinkingAdaptive(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := fakeAnthropicSSE(w, r)
		json.Unmarshal(body, &captured)
	}))
	defer srv.Close()

	client, _ := newAnthropicClient(&config.ProviderConfig{
		BaseURL: srv.URL, APIKey: "k", Model: "claude-sonnet-4-6", Thinking: true,
	}, "test system prompt")
	conv := conversation.NewManager()
	conv.AddUserMessage("hello")
	drainStream(client, conv)

	thinking, ok := captured["thinking"].(map[string]any)
	if !ok {
		t.Fatal("thinking field missing from request")
	}
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking.type = %q, want \"adaptive\"", thinking["type"])
	}
	if _, hasBudget := thinking["budget_tokens"]; hasBudget {
		t.Error("adaptive mode should not have budget_tokens")
	}
}

func TestAnthropicThinkingEnabled(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := fakeAnthropicSSE(w, r)
		json.Unmarshal(body, &captured)
	}))
	defer srv.Close()

	client, _ := newAnthropicClient(&config.ProviderConfig{
		BaseURL: srv.URL, APIKey: "k", Model: "glm-4.7", Thinking: true,
	}, "test system prompt")
	conv := conversation.NewManager()
	conv.AddUserMessage("hello")
	drainStream(client, conv)

	thinking, ok := captured["thinking"].(map[string]any)
	if !ok {
		t.Fatal("thinking field missing from request")
	}
	if thinking["type"] != "enabled" {
		t.Errorf("thinking.type = %q, want \"enabled\"", thinking["type"])
	}
	budget, _ := thinking["budget_tokens"].(float64)
	if budget != 63999 {
		t.Errorf("budget_tokens = %v, want 63999", budget)
	}
	maxTokens, _ := captured["max_tokens"].(float64)
	if maxTokens != 64000 {
		t.Errorf("max_tokens = %v, want 64000", maxTokens)
	}
}

func TestAnthropicThinkingDisabled(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := fakeAnthropicSSE(w, r)
		json.Unmarshal(body, &captured)
	}))
	defer srv.Close()

	client, _ := newAnthropicClient(&config.ProviderConfig{
		BaseURL: srv.URL, APIKey: "k", Model: "claude-sonnet-4-6", Thinking: false,
	}, "test system prompt")
	conv := conversation.NewManager()
	conv.AddUserMessage("hello")
	drainStream(client, conv)

	if _, ok := captured["thinking"]; ok {
		t.Error("thinking field should not be in API request when thinking=false")
	}
	maxTokens, _ := captured["max_tokens"].(float64)
	if maxTokens != 8192 {
		t.Errorf("max_tokens = %v, want 8192", maxTokens)
	}
}

func TestAnthropicThinkingBlocksInConversation(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := fakeAnthropicSSE(w, r)
		json.Unmarshal(body, &captured)
	}))
	defer srv.Close()

	client, _ := newAnthropicClient(&config.ProviderConfig{
		BaseURL: srv.URL, APIKey: "k", Model: "claude-sonnet-4-6", Thinking: true,
	}, "test system prompt")
	conv := conversation.NewManager()
	conv.AddUserMessage("hello")
	conv.AddAssistantFull("hi there", []conversation.ThinkingBlock{
		{Thinking: "let me think about this", Signature: "sig123"},
	}, nil)
	conv.AddUserMessage("thanks")
	drainStream(client, conv)

	messages, _ := captured["messages"].([]any)
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}

	assistantMsg, _ := messages[1].(map[string]any)
	content, _ := assistantMsg["content"].([]any)

	foundThinking := false
	for _, block := range content {
		blockMap, _ := block.(map[string]any)
		if blockMap["type"] == "thinking" {
			foundThinking = true
			if blockMap["thinking"] != "let me think about this" {
				t.Errorf("thinking text = %q, want %q", blockMap["thinking"], "let me think about this")
			}
			if blockMap["signature"] != "sig123" {
				t.Errorf("signature = %q, want %q", blockMap["signature"], "sig123")
			}
		}
	}
	if !foundThinking {
		body, _ := json.MarshalIndent(captured, "", "  ")
		t.Fatalf("no thinking block found in assistant message.\nRequest body:\n%s", string(body))
	}
}

func TestOpenAIThinkingEnabled(t *testing.T) {
	var captured map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		lines := []string{
			`data: {"type":"response.output_text.delta","delta":"hi"}`,
			`data: {"type":"response.completed","response":{"id":"r1","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":1}}}`,
		}
		io.WriteString(w, strings.Join(lines, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	cfg := &config.ProviderConfig{
		Protocol: "openai",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "o3",
		Thinking: true,
	}
	client, err := newOpenAIClient(cfg, "test system prompt")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	for range events {
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	reasoning, ok := captured["reasoning"]
	if !ok {
		body, _ := json.MarshalIndent(captured, "", "  ")
		t.Fatalf("reasoning field missing from API request.\nFull request body:\n%s", string(body))
	}
	reasoningMap, ok := reasoning.(map[string]any)
	if !ok {
		t.Fatalf("reasoning is not an object: %T", reasoning)
	}
	effort, _ := reasoningMap["effort"].(string)
	if effort != "high" {
		t.Errorf("reasoning.effort = %q, want \"high\"", effort)
	}
	summary, _ := reasoningMap["summary"].(string)
	if summary != "detailed" {
		t.Errorf("reasoning.summary = %q, want \"detailed\"", summary)
	}

	include, _ := captured["include"].([]any)
	foundEncrypted := false
	for _, v := range include {
		if v == "reasoning.encrypted_content" {
			foundEncrypted = true
		}
	}
	if !foundEncrypted {
		t.Errorf("include should contain reasoning.encrypted_content, got: %v", include)
	}
}

func TestOpenAIThinkingDisabled(t *testing.T) {
	var captured map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		lines := []string{
			`data: {"type":"response.output_text.delta","delta":"hi"}`,
			`data: {"type":"response.completed","response":{"id":"r1","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":1}}}`,
		}
		io.WriteString(w, strings.Join(lines, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	cfg := &config.ProviderConfig{
		Protocol: "openai",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "gpt-4o",
		Thinking: false,
	}
	client, err := newOpenAIClient(cfg, "test system prompt")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddUserMessage("hello")

	events, errs := client.Stream(context.Background(), conv, nil)
	for range events {
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if _, ok := captured["reasoning"]; ok {
		t.Error("reasoning field should not be in API request when thinking=false")
	}
}

func TestOpenAIThinkingEncryptedContentIsSentInInput(t *testing.T) {
	var captured map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		lines := []string{
			`data: {"type":"response.completed","response":{"id":"r1","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":1}}}`,
		}
		io.WriteString(w, strings.Join(lines, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		Protocol: "openai",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "o3",
		Thinking: true,
	}, "test system prompt")
	if err != nil {
		t.Fatal(err)
	}

	conv := conversation.NewManager()
	conv.AddAssistantFull("prior answer", []conversation.ThinkingBlock{{
		Thinking:         "consider options",
		Signature:        "rs_1",
		EncryptedContent: "enc_123",
	}}, nil)

	events, errs := client.Stream(context.Background(), conv, nil)
	for range events {
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	input, ok := captured["input"].([]any)
	if !ok || len(input) == 0 {
		t.Fatalf("input = %#v, want non-empty array", captured["input"])
	}

	found := false
	for _, item := range input {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if itemMap["type"] != "reasoning" {
			continue
		}
		found = true
		if got := itemMap["id"]; got != "rs_1" {
			t.Fatalf("reasoning id = %#v, want rs_1", got)
		}
		if got := itemMap["encrypted_content"]; got != "enc_123" {
			t.Fatalf("encrypted_content = %#v, want enc_123", got)
		}
	}
	if !found {
		t.Fatalf("no reasoning input item found in %#v", input)
	}
}

func TestOpenAIThinkingCompleteIncludesEncryptedContentFromCompletedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = body

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		lines := []string{
			`data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
			`data: {"type":"response.reasoning_summary_text.delta","sequence_number":2,"item_id":"rs_1","output_index":0,"summary_index":0,"delta":"thinking"}`,
			`data: {"type":"response.reasoning_summary_text.done","sequence_number":3,"item_id":"rs_1","output_index":0,"summary_index":0,"text":"thinking"}`,
			`data: {"type":"response.completed","sequence_number":4,"response":{"id":"resp_reason","status":"completed","output":[{"id":"rs_1","type":"reasoning","summary":[{"text":"thinking","type":"summary_text"}],"encrypted_content":"enc_123"}],"usage":{"input_tokens":4,"output_tokens":2}}}`,
		}
		io.WriteString(w, strings.Join(lines, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		Protocol: "openai",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "o3",
		Thinking: true,
	}, "test system prompt")
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

	thinkingCompleteCount := 0
	var encrypted string
	for _, event := range gotEvents {
		complete, ok := event.(ThinkingComplete)
		if !ok {
			continue
		}
		thinkingCompleteCount++
		if complete.EncryptedContent != "" {
			encrypted = complete.EncryptedContent
		}
	}
	if thinkingCompleteCount != 1 {
		t.Fatalf("thinking complete count = %d, want 1", thinkingCompleteCount)
	}
	if encrypted != "enc_123" {
		t.Fatalf("encrypted thinking content = %q, want enc_123", encrypted)
	}
}

func TestOpenAIThinkingCompleteTracksMultipleReasoningItemsSeparately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = body

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		lines := []string{
			`data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
			`data: {"type":"response.output_item.added","sequence_number":2,"output_index":1,"item":{"id":"rs_2","type":"reasoning","summary":[]}}`,
			`data: {"type":"response.reasoning_summary_text.delta","sequence_number":3,"item_id":"rs_1","output_index":0,"summary_index":0,"delta":"alpha"}`,
			`data: {"type":"response.reasoning_summary_text.delta","sequence_number":4,"item_id":"rs_2","output_index":1,"summary_index":0,"delta":"beta"}`,
			`data: {"type":"response.reasoning_summary_text.done","sequence_number":5,"item_id":"rs_2","output_index":1,"summary_index":0,"text":"beta"}`,
			`data: {"type":"response.reasoning_summary_text.done","sequence_number":6,"item_id":"rs_1","output_index":0,"summary_index":0,"text":"alpha"}`,
			`data: {"type":"response.completed","sequence_number":7,"response":{"id":"resp_reason_multi","status":"completed","output":[{"id":"rs_1","type":"reasoning","summary":[{"text":"alpha","type":"summary_text"}],"encrypted_content":"enc_a"},{"id":"rs_2","type":"reasoning","summary":[{"text":"beta","type":"summary_text"}],"encrypted_content":"enc_b"}],"usage":{"input_tokens":5,"output_tokens":2}}}`,
		}
		io.WriteString(w, strings.Join(lines, "\n\n")+"\n\n")
	}))
	defer srv.Close()

	client, err := newOpenAIClient(&config.ProviderConfig{
		Protocol: "openai",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "o3",
		Thinking: true,
	}, "test system prompt")
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

	var completes []ThinkingComplete
	for _, event := range gotEvents {
		if complete, ok := event.(ThinkingComplete); ok {
			completes = append(completes, complete)
		}
	}
	if len(completes) != 2 {
		t.Fatalf("thinking complete count = %d, want 2", len(completes))
	}
	if completes[0].Signature != "rs_1" || completes[0].Thinking != "alpha" || completes[0].EncryptedContent != "enc_a" {
		t.Fatalf("first thinking complete = %+v, want rs_1/alpha/enc_a", completes[0])
	}
	if completes[1].Signature != "rs_2" || completes[1].Thinking != "beta" || completes[1].EncryptedContent != "enc_b" {
		t.Fatalf("second thinking complete = %+v, want rs_2/beta/enc_b", completes[1])
	}
}
