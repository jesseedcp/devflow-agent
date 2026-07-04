package templates

import "fmt"

func Intake(title, source string) string {
	return fmt.Sprintf("# Intake: %s\n\nSource: `%s`\n\n## 原始需求材料\n", title, source)
}
func Context(title string) string {
	return fmt.Sprintf(`# Context: %s

## Reusable Memory

No reusable memory recalled yet.

## Historical Demand Candidates

No historical candidate memory recalled yet.
`, title)
}
func Requirements(title, description string) string {
	return fmt.Sprintf(`# Requirements: %s

## 目标行为

%s

## 非目标范围

## 业务规则

## 用户/调用方影响

## 验收标准

## 风险与歧义

## 待确认问题

## 人工确认记录
`, title, description)
}

func Plan(title string) string {
	return fmt.Sprintf(`# Technical Plan: %s

## 当前实现与代码事实

## 目标设计

## 改动范围

## 数据结构/API/配置变化

## 测试策略

## 验收方式

## 风险与回滚

## 不做事项

## 人工确认记录
`, title)
}

func Verification(title string) string {
	return fmt.Sprintf(`# Verification: %s

## 验收标准映射

## 自动化测试结果

## 手动验证记录

## 接口/日志/监控证据

## 未覆盖风险

## 结论
`, title)
}

func Closeout(title string) string {
	return fmt.Sprintf(`# Closeout: %s

## 需求结果

## 关键产物链接

## MR 评论与处理摘要

## 验收证据摘要

## 稳定知识候选

## 流程改进候选

## 一次性材料归档

## 人工确认记录
`, title)
}

func MemoryCandidates(title string) string {
	return fmt.Sprintf(`# Memory Candidates: %s

## 稳定知识候选

## 流程改进候选

## 不进入长期知识的材料
`, title)
}
func CloseoutRawLog(title string) string {
	return fmt.Sprintf(`# Closeout Raw Log: %s

## Closeout

No closeout material captured yet.

## Memory Candidates

No memory candidate material captured yet.

## Implementation Review

No implementation review captured yet.

## Review And Verification Events

No events captured yet.
`, title)
}

func WikiCandidates(title string) string {
	return fmt.Sprintf(`# Wiki Candidates: %s

## Stable Business Knowledge

No stable business knowledge candidates distilled yet.

## Process Improvement Candidates

No process improvement candidates distilled yet.

## Archive Only

No archive-only material distilled yet.
`, title)
}