// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

const openaiStreamIdleTimeout = 5 * time.Minute

type openaiClient struct {
	client          openai.Client
	model           string
	thinking        bool
	systemPrompt    string
	maxOutputTokens int
	contextWindow   int
}

type openAIToolCallState struct {
	toolName    string
	callID      string
	arguments   string
	outputIndex int64
	decodedArgs map[string]any
	done        bool
}

type openAIReasoningState struct {
	itemID      string
	outputIndex int64
	summary     string
}

func newOpenAIClient(cfg *config.ProviderConfig, systemPrompt string) (*openaiClient, error) {
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return nil, &AuthenticationError{
			Message: "OpenAI API key not found. " + legacyConfigHint("OPENAI_API_KEY"),
		}
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(cfg.BaseURL),
	)

	return &openaiClient{
		client:          client,
		model:           cfg.Model,
		thinking:        cfg.Thinking,
		systemPrompt:    systemPrompt,
		maxOutputTokens: cfg.GetMaxOutputTokens(),
		contextWindow:   cfg.GetContextWindow(),
	}, nil
}

func (c *openaiClient) SetMaxOutputTokens(tokens int) {
	c.maxOutputTokens = tokens
}

func (c *openaiClient) Stream(ctx context.Context, conv *conversation.Manager, toolSchemas []map[string]any) (<-chan StreamEvent, <-chan error) {
	events := make(chan StreamEvent, 64)
	errs := make(chan error, 1)

	input := buildOpenAIInput(conv.GetMessages())

	tools, err := buildOpenAITools(toolSchemas)
	if err != nil {
		return invalidStream(err)
	}

	go func() {
		defer close(events)
		defer close(errs)

		reqParams := responses.ResponseNewParams{
			Model:        c.model,
			Instructions: param.NewOpt(c.systemPrompt),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: input,
			},
		}
		if c.maxOutputTokens > 0 {
			reqParams.MaxOutputTokens = param.NewOpt(int64(c.maxOutputTokens))
		}
		if c.thinking {
			reqParams.Reasoning = shared.ReasoningParam{
				Effort:  shared.ReasoningEffortHigh,
				Summary: shared.ReasoningSummaryDetailed,
			}
			reqParams.Include = []responses.ResponseIncludable{
				responses.ResponseIncludableReasoningEncryptedContent,
			}
		}
		if len(tools) > 0 {
			reqParams.Tools = tools
		}

		stream := c.client.Responses.NewStreaming(ctx, reqParams)
		defer stream.Close()

		toolCalls := make(map[string]*openAIToolCallState)
		reasoningStates := make(map[string]*openAIReasoningState)

		// Read SSE events in a separate goroutine so we can respect ctx cancellation
		// and detect silent connection drops. The SDK's stream.Next() may block
		// indefinitely if the underlying connection dies without FIN/RST.
		type sseResult struct {
			hasNext bool
		}
		nextCh := make(chan sseResult, 1)

		readNext := func() {
			nextCh <- sseResult{hasNext: stream.Next()}
		}

		idle := time.NewTimer(openaiStreamIdleTimeout)
		defer idle.Stop()

		go readNext()
		for {
			var res sseResult
			select {
			case <-ctx.Done():
				errs <- &NetworkError{Message: fmt.Sprintf("context cancelled: %v", ctx.Err())}
				return
			case <-idle.C:
				errs <- &NetworkError{Message: fmt.Sprintf("stream idle timeout: no SSE events for %s", openaiStreamIdleTimeout)}
				return
			case res = <-nextCh:
			}

			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(openaiStreamIdleTimeout)

			if !res.hasNext {
				break
			}

			event := stream.Current()
			switch event.Type {
			case "response.output_text.delta":
				events <- TextDelta{Text: event.Delta.OfString}
			case "response.output_item.added":
				added := event.AsResponseOutputItemAdded()
				switch added.Item.Type {
				case "function_call":
					key := openAIItemKey(added.Item.ID, added.OutputIndex)
					toolCalls[key] = &openAIToolCallState{
						toolName:    added.Item.Name,
						callID:      added.Item.CallID,
						outputIndex: added.OutputIndex,
					}
					events <- ToolCallStart{ToolName: added.Item.Name, ToolID: added.Item.CallID}
				case "reasoning":
					key := openAIItemKey(added.Item.ID, added.OutputIndex)
					reasoningStates[key] = &openAIReasoningState{
						itemID:      added.Item.ID,
						outputIndex: added.OutputIndex,
					}
				}
			case "response.reasoning_summary_text.delta":
				delta := event.AsResponseReasoningSummaryTextDelta()
				key := openAIItemKey(delta.ItemID, delta.OutputIndex)
				state, ok := reasoningStates[key]
				if !ok {
					state = &openAIReasoningState{itemID: delta.ItemID, outputIndex: delta.OutputIndex}
					reasoningStates[key] = state
				}
				state.summary += delta.Delta
				events <- ThinkingDelta{Text: delta.Delta}
			case "response.reasoning_summary_text.done":
				done := event.AsResponseReasoningSummaryTextDone()
				key := openAIItemKey(done.ItemID, done.OutputIndex)
				state, ok := reasoningStates[key]
				if !ok {
					state = &openAIReasoningState{itemID: done.ItemID, outputIndex: done.OutputIndex}
					reasoningStates[key] = state
				}
				if done.Text != "" {
					state.summary = done.Text
				}
			case "response.function_call_arguments.delta":
				delta := event.AsResponseFunctionCallArgumentsDelta()
				key := openAIItemKey(delta.ItemID, delta.OutputIndex)
				state, ok := toolCalls[key]
				if !ok {
					state = &openAIToolCallState{}
					toolCalls[key] = state
				}
				state.arguments += delta.Delta
				events <- ToolCallDelta{Text: delta.Delta}
			case "response.function_call_arguments.done":
				done := event.AsResponseFunctionCallArgumentsDone()
				key := openAIItemKey(done.ItemID, done.OutputIndex)
				state, ok := toolCalls[key]
				if !ok {
					state = &openAIToolCallState{outputIndex: done.OutputIndex}
					toolCalls[key] = state
				}
				if done.Arguments != "" {
					state.arguments = done.Arguments
				}
				args, err := decodeToolArguments("OpenAI", state.arguments)
				if err != nil {
					errs <- err
					return
				}
				state.decodedArgs = args
				state.done = true
			case "error":
				errEvent := event.AsError()
				errs <- &LLMError{Message: fmt.Sprintf("OpenAI stream error (%s): %s", errEvent.Code, errEvent.Message)}
				return
			case "response.failed":
				failed := event.AsResponseFailed()
				errs <- classifyOpenAIResponseFailure(failed.Response)
				return
			case "response.incomplete":
				incomplete := event.AsResponseIncomplete()
				errs <- classifyOpenAIResponseIncomplete(incomplete.Response)
				return
			case "response.completed":
				completed := event.AsResponseCompleted()
				toolCallKeys := sortedOpenAIToolCallKeys(toolCalls)
				for _, key := range toolCallKeys {
					state := toolCalls[key]
					if !state.done {
						continue
					}
					events <- ToolCallComplete{
						ToolID:    state.callID,
						ToolName:  state.toolName,
						Arguments: state.decodedArgs,
					}
				}
				for _, item := range completed.Response.Output {
					if item.Type != "reasoning" {
						continue
					}
					reasoning := item.AsReasoning()
					summaryText := reasoningSummaryText(reasoning)
					if summaryText == "" {
						summaryText = reasoningStateSummary(reasoningStates, reasoning.ID)
					}
					if summaryText == "" && reasoning.EncryptedContent == "" {
						continue
					}
					events <- ThinkingComplete{
						Thinking:         summaryText,
						Signature:        reasoning.ID,
						EncryptedContent: reasoning.EncryptedContent,
					}
				}
				if !completed.Response.JSON.Usage.Valid() {
					errs <- &LLMError{Message: "OpenAI response completed without usage"}
					return
				}
				usage := UsageInfo{}
				usage.InputTokens = int(completed.Response.Usage.InputTokens)
				usage.OutputTokens = int(completed.Response.Usage.OutputTokens)
				events <- StreamEnd{StopReason: "end_turn", Usage: usage}
			}

			go readNext()
		}

		if err := stream.Err(); err != nil {
			errs <- classifyOpenAIError(err)
		}
	}()

	return events, errs
}

func buildOpenAIInput(messages []conversation.Message) responses.ResponseInputParam {
	var input responses.ResponseInputParam
	for _, m := range messages {
		if m.Role == "assistant" {
			for _, tb := range m.ThinkingBlocks {
				if tb.EncryptedContent != "" {
					reasoning := responses.ResponseReasoningItemParam{
						ID:               tb.Signature,
						Summary:          []responses.ResponseReasoningItemSummaryParam{{Text: tb.Thinking}},
						EncryptedContent: param.NewOpt(tb.EncryptedContent),
					}
					input = append(input, responses.ResponseInputItemUnionParam{
						OfReasoning: &reasoning,
					})
					continue
				}
				input = append(input, responses.ResponseInputItemParamOfReasoning(
					tb.Signature,
					[]responses.ResponseReasoningItemSummaryParam{{Text: tb.Thinking}},
				))
			}
			if m.Content != "" {
				input = append(input, responses.ResponseInputItemUnionParam{
					OfMessage: &responses.EasyInputMessageParam{
						Role: responses.EasyInputMessageRoleAssistant,
						Content: responses.EasyInputMessageContentUnionParam{
							OfString: param.NewOpt(m.Content),
						},
					},
				})
			}
			for _, tu := range m.ToolUses {
				argsJSON, _ := json.Marshal(tu.Arguments)
				input = append(input, responses.ResponseInputItemUnionParam{
					OfFunctionCall: &responses.ResponseFunctionToolCallParam{
						Name:      tu.ToolName,
						CallID:    tu.ToolUseID,
						Arguments: string(argsJSON),
					},
				})
			}
		} else if len(m.ToolResults) > 0 {
			for _, tr := range m.ToolResults {
				input = append(input, responses.ResponseInputItemUnionParam{
					OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
						CallID: tr.ToolUseID,
						Output: tr.Content,
					},
				})
			}
		} else {
			input = append(input, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Role: responses.EasyInputMessageRoleUser,
					Content: responses.EasyInputMessageContentUnionParam{
						OfString: param.NewOpt(m.Content),
					},
				},
			})
		}
	}
	return input
}

func classifyOpenAIError(err error) error {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == 413 || (apiErr.StatusCode == 400 && containsContextLengthError(apiErr.Error())) {
			return &ContextTooLongError{Message: fmt.Sprintf("Context too long: %s", apiErr.Error())}
		}
		switch apiErr.StatusCode {
		case 401:
			return &AuthenticationError{Message: fmt.Sprintf("Invalid API key: %s", apiErr.Error())}
		case 429:
			retry := ""
			if apiErr.Response != nil {
				retry = apiErr.Response.Header.Get("Retry-After")
			}
			msg := "Rate limited."
			if retry != "" {
				msg += fmt.Sprintf(" Retry after %ss.", retry)
			} else {
				msg += " Please wait."
			}
			return &RateLimitError{Message: msg, RetryAfter: retry}
		default:
			return &LLMError{Message: fmt.Sprintf("API error (%d): %s", apiErr.StatusCode, apiErr.Error())}
		}
	}
	return &NetworkError{Message: fmt.Sprintf("Network error: %s", err.Error())}
}

func containsContextLengthError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "context_length_exceeded") ||
		strings.Contains(lower, "maximum context length") ||
		strings.Contains(lower, "prompt is too long")
}

func openAIToolCallKey(itemID string, outputIndex int64) string {
	return openAIItemKey(itemID, outputIndex)
}

func openAIItemKey(itemID string, outputIndex int64) string {
	if itemID != "" {
		return itemID
	}
	return "output_index:" + strconv.FormatInt(outputIndex, 10)
}

func classifyOpenAIResponseFailure(response responses.Response) error {
	if response.Error.Message != "" {
		return &LLMError{Message: fmt.Sprintf("OpenAI response failed: %s", response.Error.Message)}
	}
	return &LLMError{Message: fmt.Sprintf("OpenAI response failed with status %s", response.Status)}
}

func classifyOpenAIResponseIncomplete(response responses.Response) error {
	if response.IncompleteDetails.Reason != "" {
		return &LLMError{Message: fmt.Sprintf("OpenAI response incomplete: %s", response.IncompleteDetails.Reason)}
	}
	return &LLMError{Message: fmt.Sprintf("OpenAI response incomplete with status %s", response.Status)}
}

func reasoningSummaryText(reasoning responses.ResponseReasoningItem) string {
	if len(reasoning.Summary) == 0 {
		return ""
	}
	parts := make([]string, 0, len(reasoning.Summary))
	for _, part := range reasoning.Summary {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "")
}

func reasoningStateSummary(states map[string]*openAIReasoningState, itemID string) string {
	if state, ok := states[itemID]; ok && state.summary != "" {
		return state.summary
	}
	for _, state := range states {
		if state.itemID == itemID && state.summary != "" {
			return state.summary
		}
	}
	return ""
}

func sortedOpenAIToolCallKeys(toolCalls map[string]*openAIToolCallState) []string {
	keys := make([]string, 0, len(toolCalls))
	for key := range toolCalls {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := toolCalls[keys[i]]
		right := toolCalls[keys[j]]
		if left.outputIndex != right.outputIndex {
			return left.outputIndex < right.outputIndex
		}
		return keys[i] < keys[j]
	})
	return keys
}
