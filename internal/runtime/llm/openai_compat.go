// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

const openaiCompatStreamIdleTimeout = 5 * time.Minute

type openaiCompatClient struct {
	client          openai.Client
	model           string
	systemPrompt    string
	maxOutputTokens int
}

type openAICompatToolCallAccum struct {
	id       string
	name     string
	argsJSON string
	started  bool
	done     bool
}

func newOpenAICompatClient(cfg *config.ProviderConfig, systemPrompt string) (*openaiCompatClient, error) {
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return nil, &AuthenticationError{
			Message: "OpenAI-compatible API key not found. " + legacyConfigHint("OPENAI_API_KEY"),
		}
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(cfg.BaseURL),
	)

	return &openaiCompatClient{
		client:          client,
		model:           cfg.Model,
		systemPrompt:    systemPrompt,
		maxOutputTokens: cfg.GetMaxOutputTokens(),
	}, nil
}

func (c *openaiCompatClient) SetMaxOutputTokens(tokens int) {
	c.maxOutputTokens = tokens
}

func (c *openaiCompatClient) Stream(ctx context.Context, conv *conversation.Manager, toolSchemas []map[string]any) (<-chan StreamEvent, <-chan error) {
	events := make(chan StreamEvent, 64)
	errs := make(chan error, 1)

	messages := buildChatCompletionMessages(c.systemPrompt, conv.GetMessages())

	tools, err := buildOpenAICompatTools(toolSchemas)
	if err != nil {
		return invalidStream(err)
	}

	go func() {
		defer close(events)
		defer close(errs)

		reqParams := openai.ChatCompletionNewParams{
			Model:    c.model,
			Messages: messages,
			StreamOptions: openai.ChatCompletionStreamOptionsParam{
				IncludeUsage: param.NewOpt(true),
			},
		}
		if c.maxOutputTokens > 0 {
			reqParams.MaxTokens = param.NewOpt(int64(c.maxOutputTokens))
		}
		if len(tools) > 0 {
			reqParams.Tools = tools
		}

		stream := c.client.Chat.Completions.NewStreaming(ctx, reqParams)
		defer stream.Close()

		// Track tool calls being assembled across multiple chunks.
		// The Chat Completions API sends tool call information incrementally:
		// the first chunk for a given index carries the ID and function name,
		// subsequent chunks carry argument fragments.
		toolCalls := make(map[int64]*openAICompatToolCallAccum)
		pendingStreamEnd := false
		pendingStopReason := ""
		streamEnded := false
		hasFinalUsage := false
		finalUsage := UsageInfo{}

		// Read SSE events in a separate goroutine so we can respect ctx cancellation
		// and detect silent connection drops, same pattern as the openai Responses client.
		type sseResult struct {
			hasNext bool
		}
		nextCh := make(chan sseResult, 1)

		readNext := func() {
			nextCh <- sseResult{hasNext: stream.Next()}
		}

		idle := time.NewTimer(openaiCompatStreamIdleTimeout)
		defer idle.Stop()

		go readNext()
		for {
			var res sseResult
			select {
			case <-ctx.Done():
				errs <- &NetworkError{Message: fmt.Sprintf("context cancelled: %v", ctx.Err())}
				return
			case <-idle.C:
				errs <- &NetworkError{Message: fmt.Sprintf("stream idle timeout: no SSE events for %s", openaiCompatStreamIdleTimeout)}
				return
			case res = <-nextCh:
			}

			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(openaiCompatStreamIdleTimeout)

			if !res.hasNext {
				break
			}

			chunk := stream.Current()

			usageInChunk := chunk.JSON.Usage.Valid()
			if usageInChunk {
				hasFinalUsage = true
				finalUsage = UsageInfo{
					InputTokens:  int(chunk.Usage.PromptTokens),
					OutputTokens: int(chunk.Usage.CompletionTokens),
				}
			}

			if len(chunk.Choices) == 0 {
				if pendingStreamEnd && !streamEnded && hasFinalUsage {
					events <- StreamEnd{StopReason: pendingStopReason, Usage: finalUsage}
					streamEnded = true
					pendingStreamEnd = false
					pendingStopReason = ""
				}
				go readNext()
				continue
			}

			choice := chunk.Choices[0]
			delta := choice.Delta

			// Text content delta
			if delta.Content != "" {
				events <- TextDelta{Text: delta.Content}
			}

			// Tool call deltas
			for _, tc := range delta.ToolCalls {
				acc, exists := toolCalls[tc.Index]
				if !exists {
					acc = &openAICompatToolCallAccum{}
					toolCalls[tc.Index] = acc
				}

				// First chunk for this tool call carries the ID and name
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" && !acc.started {
					acc.name = tc.Function.Name
					acc.started = true
					events <- ToolCallStart{ToolName: acc.name, ToolID: acc.id}
				}

				// Accumulate argument fragments
				if tc.Function.Arguments != "" {
					acc.argsJSON += tc.Function.Arguments
					events <- ToolCallDelta{Text: tc.Function.Arguments}
				}
			}

			// When the model signals completion, emit tool-call completions
			// (tool_calls) or finalize the turn (stop). A stop with pending tool
			// calls is an inconsistent provider state and surfaces as an error.
			if choice.FinishReason == "tool_calls" {
				toolCallIndexes := sortedToolCallIndexes(toolCalls)
				for _, index := range toolCallIndexes {
					acc := toolCalls[index]
					if acc.done {
						continue
					}
					args, err := decodeToolArguments("OpenAI-compatible", acc.argsJSON)
					if err != nil {
						errs <- err
						return
					}
					events <- ToolCallComplete{
						ToolID:    acc.id,
						ToolName:  acc.name,
						Arguments: args,
					}
					acc.done = true
				}

				if !streamEnded {
					if hasFinalUsage {
						events <- StreamEnd{StopReason: choice.FinishReason, Usage: finalUsage}
						streamEnded = true
					} else {
						pendingStreamEnd = true
						pendingStopReason = choice.FinishReason
					}
				}
			} else if choice.FinishReason == "stop" {
				for _, index := range sortedToolCallIndexes(toolCalls) {
					if !toolCalls[index].done {
						errs <- &LLMError{Message: "OpenAI-compatible stream finished with stop reason but had pending tool calls"}
						return
					}
				}
				if !streamEnded {
					if hasFinalUsage {
						events <- StreamEnd{StopReason: choice.FinishReason, Usage: finalUsage}
						streamEnded = true
					} else {
						pendingStreamEnd = true
						pendingStopReason = choice.FinishReason
					}
				}
			} else if choice.FinishReason != "" {
				errs <- &LLMError{Message: fmt.Sprintf("OpenAI-compatible stream finished unsuccessfully: %s", choice.FinishReason)}
				return
			}

			go readNext()
		}

		if pendingStreamEnd && !streamEnded {
			errs <- &LLMError{Message: "OpenAI-compatible stream ended without usage after successful finish_reason"}
			return
		}

		if err := stream.Err(); err != nil {
			errs <- classifyOpenAIError(err)
		}
	}()

	return events, errs
}

// buildChatCompletionMessages converts conversation history into the Chat Completions
// message format. The system prompt becomes a system message at the start. Thinking
// blocks are skipped because Chat Completions does not support them natively.
func buildChatCompletionMessages(systemPrompt string, messages []conversation.Message) []openai.ChatCompletionMessageParamUnion {
	var result []openai.ChatCompletionMessageParamUnion

	// System prompt as the first message
	if systemPrompt != "" {
		result = append(result, openai.SystemMessage(systemPrompt))
	}

	for _, m := range messages {
		if m.Role == "assistant" {
			// ThinkingBlocks are skipped for Chat Completions

			if len(m.ToolUses) > 0 {
				// Assistant message with tool calls
				assistant := openai.ChatCompletionAssistantMessageParam{}
				if m.Content != "" {
					assistant.Content.OfString = param.NewOpt(m.Content)
				}
				for _, tu := range m.ToolUses {
					argsJSON, _ := json.Marshal(tu.Arguments)
					assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallParam{
						ID: tu.ToolUseID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tu.ToolName,
							Arguments: string(argsJSON),
						},
					})
				}
				result = append(result, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
			} else if m.Content != "" {
				result = append(result, openai.AssistantMessage(m.Content))
			}
		} else if len(m.ToolResults) > 0 {
			// Tool results become individual tool messages
			for _, tr := range m.ToolResults {
				result = append(result, openai.ToolMessage(tr.Content, tr.ToolUseID))
			}
		} else {
			// User messages
			result = append(result, openai.UserMessage(m.Content))
		}
	}

	return result
}

func sortedToolCallIndexes(toolCalls map[int64]*openAICompatToolCallAccum) []int64 {
	indexes := make([]int64, 0, len(toolCalls))
	for index := range toolCalls {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i] < indexes[j]
	})
	return indexes
}
