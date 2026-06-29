package llm

import "encoding/json"

func decodeToolArguments(provider, payload string) (map[string]any, error) {
	if payload == "" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(payload), &args); err != nil {
		return nil, &LLMError{Message: provider + " tool call arguments invalid JSON"}
	}
	if args == nil {
		args = map[string]any{}
	}
	return args, nil
}
