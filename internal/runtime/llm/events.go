// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

package llm

type StreamEvent interface{ streamEvent() }

type TextDelta struct{ Text string }
type ThinkingDelta struct{ Text string }
type ThinkingComplete struct {
	Thinking         string
	Signature        string
	EncryptedContent string
}
type ToolCallStart struct{ ToolName, ToolID string }
type ToolCallDelta struct{ Text string }
type ToolCallComplete struct {
	ToolID    string
	ToolName  string
	Arguments map[string]any
}
type UsageInfo struct {
	InputTokens  int
	OutputTokens int
}

type StreamEnd struct {
	StopReason string
	Usage      UsageInfo
}

func (TextDelta) streamEvent() {}

func (ThinkingDelta) streamEvent() {}

func (ThinkingComplete) streamEvent() {}
func (ToolCallStart) streamEvent()    {}
func (ToolCallDelta) streamEvent()    {}

func (ToolCallComplete) streamEvent() {}

func (StreamEnd) streamEvent() {}
