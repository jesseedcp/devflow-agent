package demandflow

import (
	"fmt"
	"strings"
)

func BuildPrompt(stage Stage, ctx ContextSnapshot) (string, ToolMode, error) {
	switch stage {
	case StageRequirements:
		return requirementsPrompt(ctx), ToolModeReadOnly, nil
	case StagePlan:
		return planPrompt(ctx), ToolModeReadOnly, nil
	case StageImplementation:
		return implementationPrompt(ctx), ToolModeEditAndShell, nil
	case StageVerification:
		return verificationPrompt(ctx), ToolModeReadOnly, nil
	case StageCloseout:
		return closeoutPrompt(ctx), ToolModeReadOnly, nil
	default:
		return "", "", fmt.Errorf("stage %s does not have an agent prompt", stage)
	}
}

func requirementsPrompt(ctx ContextSnapshot) string {
	return strings.TrimSpace(fmt.Sprintf(`# Role
You are the backend business requirements expert for Devflow.

# Demand
Title: %s
Description:
%s

# Reusable memory
%s

# Output contract
Return the complete requirements.md body only.
Use these headings exactly:
- # Requirements: %s
- ## 目标行为
- ## 非目标范围
- ## 业务规则
- ## 用户/调用方影响
- ## 验收标准
- ## 风险与歧义
- ## 待确认问题
- ## 人工确认记录
Do not include chat commentary around the artifact body.
`, ctx.Demand.Title, ctx.Demand.Description, renderMemoryHits(ctx.Memories), ctx.Demand.Title))
}

func planPrompt(ctx ContextSnapshot) string {
	return strings.TrimSpace(fmt.Sprintf(`# Role
You are the backend technical planning expert for Devflow.

# Demand
Title: %s
Description:
%s

# Current requirements
%s

# Reusable memory
%s

# Output contract
Return the complete plan.md body only.
Use these headings exactly:
- # Technical Plan: %s
- ## 当前实现与代码事实
- ## 目标设计
- ## 改动范围
- ## 数据结构/API/配置变化
- ## 测试策略
- ## 验收方式
- ## 风险与回滚
- ## 不做事项
- ## 人工确认记录
Do not include chat commentary around the artifact body.
`, ctx.Demand.Title, ctx.Demand.Description, ctx.Artifacts.Requirements, renderMemoryHits(ctx.Memories), ctx.Demand.Title))
}

func implementationPrompt(ctx ContextSnapshot) string {
	return strings.TrimSpace(fmt.Sprintf(`# Role
You are the backend implementation engineer for Devflow.

# Demand
Title: %s
Description:
%s

# Current requirements
%s

# Current plan
%s

# Output contract
Implement the agreed changes in the repository using the available tools.
After implementation, return the progress.md body to append.
Use these headings exactly:
- ## 实现摘要
- ## 代码改动
- ## 测试与验证
- ## 遗留问题
Do not include chat commentary around the artifact body.
`, ctx.Demand.Title, ctx.Demand.Description, ctx.Artifacts.Requirements, ctx.Artifacts.Plan))
}

func verificationPrompt(ctx ContextSnapshot) string {
	return strings.TrimSpace(fmt.Sprintf(`# Role
You are the backend verification engineer for Devflow.

# Demand
Title: %s
Description:
%s

# Current requirements
%s

# Current plan
%s

# Implementation progress
%s

# Output contract
Return the complete verification.md body only.
Use these headings exactly:
- # Verification: %s
- ## 验收标准映射
- ## 自动化测试结果
- ## 手动验证记录
- ## 接口/日志/监控证据
- ## 未覆盖风险
- ## 结论
Do not include chat commentary around the artifact body.
`, ctx.Demand.Title, ctx.Demand.Description, ctx.Artifacts.Requirements, ctx.Artifacts.Plan, ctx.Artifacts.Progress, ctx.Demand.Title))
}

func closeoutPrompt(ctx ContextSnapshot) string {
	return strings.TrimSpace(fmt.Sprintf(`# Role
You are the backend closeout engineer for Devflow.

# Demand
Title: %s
Description:
%s

# Current requirements
%s

# Current plan
%s

# Implementation progress
%s

# Verification
%s

# Output contract
Return two sections separated by this exact marker on its own line:

---DEVFLOW-MEMORY-CANDIDATES---

The first section is the complete closeout.md body. Use these headings exactly:
- # Closeout: %s
- ## 需求结果
- ## 关键产物链接
- ## MR 评论与处理摘要
- ## 验收证据摘要
- ## 稳定知识候选
- ## 流程改进候选
- ## 一次性材料归档
- ## 人工确认记录

The second section after the marker is the complete memory-candidates.md body. Use these headings exactly:
- # Memory Candidates: %s
- ## 稳定知识候选
- ## 流程改进候选
- ## 不进入长期知识的材料
Do not include chat commentary around either artifact body.
`, ctx.Demand.Title, ctx.Demand.Description, ctx.Artifacts.Requirements, ctx.Artifacts.Plan, ctx.Artifacts.Progress, ctx.Artifacts.Verification, ctx.Demand.Title, ctx.Demand.Title))
}

func renderMemoryHits(hits []MemoryHit) string {
	if len(hits) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, hit := range hits {
		b.WriteString("- ")
		b.WriteString(hit.DemandID)
		if strings.TrimSpace(hit.Snippet) != "" {
			b.WriteString(": ")
			b.WriteString(strings.TrimSpace(hit.Snippet))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
