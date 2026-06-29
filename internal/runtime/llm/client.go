// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

package llm

import (
	"context"
	"fmt"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
)

type Client interface {
	Stream(ctx context.Context, conv *conversation.Manager, tools []map[string]any) (<-chan StreamEvent, <-chan error)
}

type MaxTokensSetter interface {
	SetMaxOutputTokens(tokens int)
}

func NewClient(cfg *config.ProviderConfig, systemPrompt string) (Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("provider config is required")
	}

	switch cfg.Protocol {
	case "anthropic":
		return newAnthropicClient(cfg, systemPrompt)
	case "openai":
		return newOpenAIClient(cfg, systemPrompt)
	case "openai-compat":
		return newOpenAICompatClient(cfg, systemPrompt)
	default:
		return nil, fmt.Errorf("unknown protocol: %s", cfg.Protocol)
	}
}
