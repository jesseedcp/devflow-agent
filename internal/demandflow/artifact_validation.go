package demandflow

import (
	"fmt"
	"strings"
)

type artifactContract struct {
	Name     string
	Heading  string
	Sections []string
}

var stageArtifactContracts = map[Stage]artifactContract{
	StageRequirements: {
		Name:    "requirements.md",
		Heading: "# Requirements:",
		Sections: []string{
			"目标行为",
			"非目标范围",
			"业务规则",
			"用户/调用方影响",
			"验收标准",
			"风险与歧义",
			"待确认问题",
			"人工确认记录",
		},
	},
	StagePlan: {
		Name:    "plan.md",
		Heading: "# Technical Plan:",
		Sections: []string{
			"当前实现与代码事实",
			"目标设计",
			"实施步骤",
			"改动范围",
			"数据结构/API/配置变化",
			"测试策略",
			"验收方式",
			"风险与回滚",
			"不做事项",
			"人工确认记录",
		},
	},
	StageImplementation: {
		Name: "progress.md",
		Sections: []string{
			"实现摘要",
			"代码改动",
			"测试与验证",
			"遗留问题",
		},
	},
	StageVerification: {
		Name:    "verification.md",
		Heading: "# Verification:",
		Sections: []string{
			"验收标准映射",
			"自动化测试结果",
			"手动验证记录",
			"接口/日志/监控证据",
			"未覆盖风险",
			"结论",
		},
	},
	StageCloseout: {
		Name:    "closeout.md",
		Heading: "# Closeout:",
		Sections: []string{
			"需求结果",
			"关键产物链接",
			"MR 评论与处理摘要",
			"验收证据摘要",
			"稳定知识候选",
			"流程改进候选",
			"一次性材料归档",
			"人工确认记录",
		},
	},
}

func ValidateStageArtifact(stage Stage, body string) error {
	contract, ok := stageArtifactContracts[stage]
	if !ok {
		return nil
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return fmt.Errorf("%s invalid: empty artifact body", contract.Name)
	}
	if contract.Heading != "" && !strings.Contains(trimmed, contract.Heading) {
		return fmt.Errorf("%s invalid: missing required heading %q", contract.Name, contract.Heading)
	}
	for _, section := range contract.Sections {
		if !containsMarkdownSection(trimmed, section) {
			return fmt.Errorf("%s invalid: missing required section %q", contract.Name, section)
		}
		if strings.TrimSpace(markdownSectionContent(trimmed, section)) == "" {
			return fmt.Errorf("%s invalid: required section %q has no content", contract.Name, section)
		}
	}
	if stage == StageCloseout && !strings.Contains(trimmed, "---DEVFLOW-MEMORY-CANDIDATES---") {
		return fmt.Errorf("%s invalid: missing memory candidates marker", contract.Name)
	}
	return nil
}

func markdownSectionContent(body, section string) string {
	needle := strings.ToLower(section)
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	inSection := false
	targetLevel := 0
	var out strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if level := artifactHeadingLevel(trimmed); level > 0 {
			if inSection && level <= targetLevel {
				break
			}
			if !inSection && strings.Contains(strings.ToLower(trimmed), needle) {
				inSection = true
				targetLevel = level
			}
			continue
		}
		if inSection {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func artifactHeadingLevel(line string) int {
	if line == "" || line[0] != '#' {
		return 0
	}
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == len(line) || line[level] != ' ' {
		return 0
	}
	return level
}
func containsMarkdownSection(body, section string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "##") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if strings.EqualFold(heading, section) || strings.Contains(strings.ToLower(heading), strings.ToLower(section)) {
			return true
		}
	}
	return false
}
