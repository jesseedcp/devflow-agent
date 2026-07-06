package demandflow

import (
	"strings"
	"testing"
)

func TestValidateStageArtifactRejectsEmptyOutput(t *testing.T) {
	t.Parallel()

	err := ValidateStageArtifact(StagePlan, "   ")
	if err == nil || !strings.Contains(err.Error(), "empty artifact body") {
		t.Fatalf("err = %v, want empty artifact body", err)
	}
}

func TestValidateStageArtifactRejectsPlanMissingImplementationSteps(t *testing.T) {
	t.Parallel()

	body := "# Technical Plan: Coupon\n\n## 当前实现与代码事实\n\nExisting.\n\n## 目标设计\n\nBuild it.\n\n## 改动范围\n\nscope.\n\n## 数据结构/API/配置变化\n\nnone.\n\n## 测试策略\n\nRun tests.\n\n## 验收方式\n\nverification.\n\n## 风险与回滚\n\nRollback by reverting.\n\n## 不做事项\n\nnone.\n\n## 人工确认记录\n\npending."
	err := ValidateStageArtifact(StagePlan, body)
	if err == nil || !strings.Contains(err.Error(), "missing required section") || !strings.Contains(err.Error(), "实施步骤") {
		t.Fatalf("err = %v, want missing 实施步骤 section", err)
	}
}

func TestValidateStageArtifactAcceptsPlanContract(t *testing.T) {
	t.Parallel()

	body := "# Technical Plan: Coupon\n\n## 当前实现与代码事实\n\nExisting service.\n\n## 目标设计\n\nAdd rule.\n\n## 实施步骤\n\n- Write tests.\n- Implement rule.\n\n## 改动范围\n\nservice and tests.\n\n## 数据结构/API/配置变化\n\nNo schema change.\n\n## 测试策略\n\nRun go test ./...\n\n## 验收方式\n\nAPI evidence.\n\n## 风险与回滚\n\nRevert commit.\n\n## 不做事项\n\nNo external provider.\n\n## 人工确认记录\n\nPending."
	if err := ValidateStageArtifact(StagePlan, body); err != nil {
		t.Fatalf("ValidateStageArtifact returned error: %v", err)
	}
}

func TestValidateStageArtifactRejectsImplementationMissingTests(t *testing.T) {
	t.Parallel()

	body := "## 实现摘要\n\nImplemented.\n\n## 代码改动\n\n- service.go\n\n## 遗留问题\n\nNone."
	err := ValidateStageArtifact(StageImplementation, body)
	if err == nil || !strings.Contains(err.Error(), "测试与验证") {
		t.Fatalf("err = %v, want missing 测试与验证 section", err)
	}
}

func TestValidateStageArtifactAcceptsImplementationContract(t *testing.T) {
	t.Parallel()

	body := "## 实现摘要\n\nImplemented deterministic rule.\n\n## 代码改动\n\n- internal/coupon/service.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nNone."
	if err := ValidateStageArtifact(StageImplementation, body); err != nil {
		t.Fatalf("ValidateStageArtifact returned error: %v", err)
	}
}

func TestValidateStageArtifactAcceptsCloseoutWithMemoryMarker(t *testing.T) {
	t.Parallel()

	body := "# Closeout: Coupon\n\n## 需求结果\n\nDone.\n\n## 关键产物链接\n\n- verification.md\n\n## MR 评论与处理摘要\n\nNone.\n\n## 验收证据摘要\n\nPASS.\n\n## 稳定知识候选\n\n- rule\n\n## 流程改进候选\n\n- none\n\n## 一次性材料归档\n\n- closeout\n\n## 人工确认记录\n\nPending.\n\n---DEVFLOW-MEMORY-CANDIDATES---\n\n# Memory Candidates: Coupon\n\n## 稳定知识候选\n\n- rule\n\n## 流程改进候选\n\n- none\n\n## 不进入长期知识的材料\n\n- none"
	if err := ValidateStageArtifact(StageCloseout, body); err != nil {
		t.Fatalf("ValidateStageArtifact returned error: %v", err)
	}
}
