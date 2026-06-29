package llm

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicparam "github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/openai/openai-go"
	openaiparam "github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

func invalidStream(err error) (<-chan StreamEvent, <-chan error) {
	events := make(chan StreamEvent)
	errs := make(chan error, 1)
	close(events)
	errs <- err
	close(errs)
	return events, errs
}

func legacyConfigHint(envVar string) string {
	return fmt.Sprintf("Set it in .devflow/config.yaml, legacy .mewcode/config.yaml, or via %s.", envVar)
}

func buildAnthropicTools(toolSchemas []map[string]any) ([]anthropic.ToolUnionParam, error) {
	tools := make([]anthropic.ToolUnionParam, 0, len(toolSchemas))
	for i, schema := range toolSchemas {
		if missing := missingSchemaKeys(schema, "name", "input_schema"); len(missing) > 0 {
			return nil, toolSchemaError("Anthropic", i, fmt.Errorf("missing %s", strings.Join(missing, ", ")))
		}
		name, err := requiredSchemaString(schema, "name")
		if err != nil {
			return nil, toolSchemaError("Anthropic", i, err)
		}
		inputSchema, err := requiredSchemaMap(schema, "input_schema")
		if err != nil {
			return nil, toolSchemaError("Anthropic", i, err)
		}
		description, err := optionalSchemaString(schema, "description")
		if err != nil {
			return nil, toolSchemaError("Anthropic", i, err)
		}
		required, err := optionalStringSlice(inputSchema, "required")
		if err != nil {
			return nil, toolSchemaError("Anthropic", i, err)
		}
		extraFields := buildAnthropicInputSchemaExtras(inputSchema)

		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        name,
				Description: anthropicparam.NewOpt(description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties:  cloneSchemaValue(inputSchema["properties"]),
					Required:    required,
					ExtraFields: extraFields,
				},
			},
		})
	}
	return tools, nil
}

func buildOpenAITools(toolSchemas []map[string]any) ([]responses.ToolUnionParam, error) {
	tools := make([]responses.ToolUnionParam, 0, len(toolSchemas))
	for i, schema := range toolSchemas {
		if missing := missingSchemaKeys(schema, "name", "parameters"); len(missing) > 0 {
			return nil, toolSchemaError("OpenAI", i, fmt.Errorf("missing %s", strings.Join(missing, ", ")))
		}
		name, err := requiredSchemaString(schema, "name")
		if err != nil {
			return nil, toolSchemaError("OpenAI", i, err)
		}
		parameters, err := requiredSchemaMap(schema, "parameters")
		if err != nil {
			return nil, toolSchemaError("OpenAI", i, err)
		}
		description, err := optionalSchemaString(schema, "description")
		if err != nil {
			return nil, toolSchemaError("OpenAI", i, err)
		}
		clonedParameters, err := cloneSchemaMap(parameters)
		if err != nil {
			return nil, toolSchemaError("OpenAI", i, err)
		}

		tools = append(tools, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        name,
				Description: openaiparam.NewOpt(description),
				Parameters:  clonedParameters,
				Strict:      openaiparam.NewOpt(false),
			},
		})
	}
	return tools, nil
}

func buildOpenAICompatTools(toolSchemas []map[string]any) ([]openai.ChatCompletionToolParam, error) {
	tools := make([]openai.ChatCompletionToolParam, 0, len(toolSchemas))
	for i, schema := range toolSchemas {
		if missing := missingSchemaKeys(schema, "name", "parameters"); len(missing) > 0 {
			return nil, toolSchemaError("OpenAI-compatible", i, fmt.Errorf("missing %s", strings.Join(missing, ", ")))
		}
		name, err := requiredSchemaString(schema, "name")
		if err != nil {
			return nil, toolSchemaError("OpenAI-compatible", i, err)
		}
		parameters, err := requiredSchemaMap(schema, "parameters")
		if err != nil {
			return nil, toolSchemaError("OpenAI-compatible", i, err)
		}
		description, err := optionalSchemaString(schema, "description")
		if err != nil {
			return nil, toolSchemaError("OpenAI-compatible", i, err)
		}
		clonedParameters, err := cloneSchemaMap(parameters)
		if err != nil {
			return nil, toolSchemaError("OpenAI-compatible", i, err)
		}

		tools = append(tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        name,
				Description: openaiparam.NewOpt(description),
				Parameters:  shared.FunctionParameters(clonedParameters),
				Strict:      openaiparam.NewOpt(false),
			},
		})
	}
	return tools, nil
}

func toolSchemaError(provider string, index int, cause error) error {
	return &LLMError{
		Message: fmt.Sprintf("%s tool schema #%d is invalid: %s", provider, index+1, cause.Error()),
	}
}

func requiredSchemaString(schema map[string]any, key string) (string, error) {
	value, ok := schema[key]
	if !ok {
		return "", fmt.Errorf("missing %s", key)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return text, nil
}

func optionalSchemaString(schema map[string]any, key string) (string, error) {
	value, ok := schema[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return text, nil
}

func requiredSchemaMap(schema map[string]any, key string) (map[string]any, error) {
	value, ok := schema[key]
	if !ok {
		return nil, fmt.Errorf("missing %s", key)
	}
	mapped, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return mapped, nil
}

func optionalStringSlice(schema map[string]any, key string) ([]string, error) {
	value, ok := schema[key]
	if !ok || value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []string:
		out := make([]string, len(typed))
		copy(out, typed)
		return out, nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s entries must be strings", key)
			}
			out = append(out, text)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
}

func missingSchemaKeys(schema map[string]any, keys ...string) []string {
	var missing []string
	for _, key := range keys {
		if _, ok := schema[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func buildAnthropicInputSchemaExtras(inputSchema map[string]any) map[string]any {
	extras := make(map[string]any)
	for key, value := range inputSchema {
		switch key {
		case "properties", "required":
			continue
		case "type":
			text, ok := value.(string)
			if !ok || strings.TrimSpace(text) == "" || text == "object" {
				continue
			}
		}
		extras[key] = cloneSchemaValue(value)
	}
	if len(extras) == 0 {
		return nil
	}
	return extras
}

func cloneSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, inner := range typed {
			cloned[key] = cloneSchemaValue(inner)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for i, inner := range typed {
			cloned[i] = cloneSchemaValue(inner)
		}
		return cloned
	case []string:
		cloned := make([]string, len(typed))
		copy(cloned, typed)
		return cloned
	default:
		return typed
	}
}

func cloneSchemaMap(value map[string]any) (map[string]any, error) {
	cloned, ok := cloneSchemaValue(value).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema object could not be cloned")
	}
	return cloned, nil
}
