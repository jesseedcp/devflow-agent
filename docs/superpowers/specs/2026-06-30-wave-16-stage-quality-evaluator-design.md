# Wave 16 Stage Quality Evaluator 设计

## 1. 背景

Wave 15 会让 Devflow 能自动推进到下一个人工门。自动推进之后，新的瓶颈会变成：人工确认时，用户仍然需要自己判断阶段产物质量是否足够。

当前系统知道：

- requirements 是否生成；
- plan 是否生成；
- implementation 是否跑过质量门；
- verification 是否 PASS/FAIL；
- closeout 是否生成 memory candidates。

但系统还没有一个统一的阶段质量评价模型，例如：

```text
requirements 是否有验收标准？
plan 是否覆盖测试策略？
verification 是否映射了验收标准？
closeout 是否总结了 MR 和稳定知识候选？
```

Wave 16 的目标是加一个 deterministic quality evaluator，先用规则化检查给用户“确认前的质量信号”，不引入新 LLM 评审，也不改变人工确认门。

## 2. 产品目标

新增命令：

```powershell
devflow evaluate --demand <id>
devflow evaluate --demand <id> --stage requirements
```

示例输出：

```text
Evaluation: add-coupon-check

requirements   pass      5 checks, 0 blockers
plan           warning   4 checks, 1 warning
verification   fail      3 checks, 1 blocker

Blockers:
  verification: missing acceptance criteria mapping

Next:
  revise verification before confirmation
```

也要把 evaluation 摘要接入 console：

```text
Quality:
  requirements   pass
  plan           warning
  verification   fail - missing acceptance criteria mapping
```

## 3. Evaluation Scope

Wave 16 只做 deterministic file checks，不调用 LLM。

### Requirements Checks

- `requirements.md` exists and is not template-only.
- Has non-empty `## 验收标准` or `## Acceptance Criteria`.
- Has non-empty `## 业务规则` or `## Business Rules`.
- Has non-empty `## 风险与歧义` or `## Risks`.

### Plan Checks

- `plan.md` exists and is not template-only.
- Has non-empty implementation scope section.
- Has non-empty test strategy section.
- Mentions changed files, modules, APIs, or commands in at least one section.
- Has non-empty rollback or risk section.

### Verification Checks

- `verification.md` exists and is not template-only.
- Has latest `verification.recorded` event.
- If latest status is pass, has command evidence.
- Has acceptance mapping section with content.
- Has uncovered risk section with content or explicit none.

### Closeout Checks

- `closeout.md` exists and is not template-only.
- Summarizes demand result.
- References key artifacts.
- Has memory candidates file present.
- Has stable knowledge candidate section, even if empty.

## 4. Result Model

Add in `internal/demandflow`:

```go
type EvaluationStatus string

const (
    EvaluationPass EvaluationStatus = "pass"
    EvaluationWarning EvaluationStatus = "warning"
    EvaluationFail EvaluationStatus = "fail"
    EvaluationNotApplicable EvaluationStatus = "not_applicable"
)

type EvaluationCheck struct {
    ID string
    Label string
    Status EvaluationStatus
    Severity string
    Evidence string
}

type StageEvaluation struct {
    Stage Stage
    Status EvaluationStatus
    Checks []EvaluationCheck
    Blockers int
    Warnings int
}

type DemandEvaluation struct {
    DemandID string
    Stages []StageEvaluation
    Overall EvaluationStatus
}
```

Status aggregation:

- any blocker fail => stage `fail`;
- warnings but no blockers => stage `warning`;
- all required checks pass => stage `pass`;
- missing stage artifact before stage is relevant => `not_applicable`.

## 5. CLI Boundary

Add:

```powershell
devflow evaluate --demand <id> [--stage <stage>] [--strict]
```

Behavior:

- without `--stage`, evaluate all relevant stages;
- with `--stage`, evaluate only that stage;
- `--strict` exits non-zero when any evaluated stage is warning or fail;
- without `--strict`, command prints report and exits zero unless demand cannot be loaded.

## 6. Console Integration

Wave 16 should add evaluation summary to:

```powershell
devflow console --demand <id>
```

Keep console output concise:

```text
Quality:
  requirements   pass
  plan           warning
  verification   fail
```

Do not make console fail because evaluation fails. Console is informational.

## 7. Drive Integration

Wave 16 may add a conservative drive guard:

```powershell
devflow drive --demand <id> --quality-guard
```

If implemented in this wave, `--quality-guard` stops before confirming or advancing from a stage with failed evaluation. Since drive does not auto-confirm, this is mainly a warning gate before continuing automated stages. This flag should default off to avoid surprising users.

If the implementation window is tight, prioritize `evaluate` and console integration. Drive guard can remain documented as a next wave candidate.

## 8. Tests

Demandflow tests:

- requirements pass/fail/warning checks;
- plan test strategy missing warning;
- verification missing recorded event fail;
- closeout memory candidates missing warning;
- aggregate status calculation.

CLI tests:

- `evaluate --demand` prints all relevant stages;
- `evaluate --stage requirements` prints only requirements;
- `--strict` returns error on warning/fail;
- console detail includes Quality section.

## 9. Non-Goals

Wave 16 does not do:

- LLM review;
- semantic correctness scoring;
- auto-rewrite of artifacts;
- automatic confirmation;
- blocking existing `confirm` command;
- changing workflow state machine;
- Eino.

## 10. Completion Criteria

Wave 16 is complete when:

- `devflow evaluate` exists.
- evaluation model is deterministic and tested.
- console shows quality summary.
- strict mode returns non-zero on warning/fail.
- documentation explains checks and limitations.
- full Go verification and CI pass.
