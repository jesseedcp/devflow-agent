# Wave 15 Guided Autopilot Drive 设计

## 1. 背景

Wave 14 已经完成 `devflow console` 和 `devflow console --demand <id> --run-next`。用户现在可以看到需求工作台，也可以安全执行下一步 runner stage。

但 Wave 14 仍然是“一次点火一步”：

```text
console --run-next
-> status/console
-> console --run-next
-> status/console
-> confirm
```

Wave 15 的目标是把这个操作变成“自动开到下一个红灯前停下”：

```text
检查 ConsoleSummary
-> 如果下一步是安全 runner stage，执行
-> 重新检查 ConsoleSummary
-> 继续
-> 遇到人工确认、memory 决策、MR 参数缺失、质量门失败或最大步数，停止并解释
```

这一步让 Devflow 从“能跑下一步”升级成“能连续推进，但不越权”。

## 2. 产品目标

新增命令：

```powershell
devflow drive --demand <id>
```

示例输出：

```text
Drive: add-coupon-check

Step 1
  action: Draft requirements
  command: devflow run --demand add-coupon-check --stage requirements
  result: completed
  state: created -> requirements_review

Stopped
  reason: human_confirmation
  next: Confirm requirements
  command: devflow confirm --demand add-coupon-check --stage requirements --by <name> --summary <summary>
```

如果从 implementation 开始，且质量门通过：

```text
Step 1: Run implementation -> mr_review
Step 2: Check MR review -> verification
Step 3: Draft verification -> verification

Stopped
  reason: human_confirmation
  next: Confirm verification
```

如果质量门失败：

```text
Stopped
  reason: runner_failed
  error: quality gate failed: go test ./...
  next: Retry implementation
```

## 3. Command Shape

```powershell
devflow drive --demand <id> [--root <path>] [--max-steps <n>] [--dry-run]
```

Runner flags mirror `console --run-next`:

```powershell
devflow drive --demand <id> `
  --runner-root <path> `
  --quality-root <path> `
  --config <path> `
  --permission-mode acceptEdits `
  --quality-command "go test ./..." `
  --gitlab-project <project> `
  --gitlab-mr <iid>
```

Defaults:

- `--max-steps` default: `5`.
- `--max-steps` must be between `1` and `20`.
- `--dry-run` shows the planned safe steps without mutating demand files.
- If no explicit quality command is passed, inherit known defaults from `ConsoleAction.Command`, the same behavior fixed after Wave 14.

## 4. Stop Reasons

Drive loop must always stop with one explicit reason:

| Reason | Meaning |
| --- | --- |
| `human_confirmation` | Next action is `devflow confirm ...`; user must confirm manually. |
| `memory_gate` | Next action is memory list/promote/reject; user must decide stable knowledge manually. |
| `mr_flags_required` | MR review is next but GitLab project/MR flags are missing. |
| `runner_failed` | A runner stage returned an error, including quality gate failure. |
| `max_steps_reached` | Loop hit max steps before reaching a manual gate. |
| `complete` | Demand has no remaining command and is completed. |
| `manual_action` | Next action is not recognized as a safe runner or known manual gate. |

## 5. Drive Rules

Drive executes only actions that are safe according to `ConsoleAction`:

- `agent_stage` with requirements, plan, implementation, verification, closeout.
- `mr_review` only when user supplied `--gitlab-project` and `--gitlab-mr`.

Drive never executes:

- human confirmation;
- memory promote/reject;
- memory list as a mutation step;
- arbitrary shell command;
- MR merge.

After every executed step:

1. reload `ConsoleSummary`;
2. append a drive event to `events.jsonl`;
3. append a concise drive section to `progress.md`;
4. evaluate the next action.

## 6. Data Model

Add a deterministic drive model in `internal/demandflow`:

```go
type DriveStopReason string

type DriveStep struct {
    Number int
    Action ConsoleAction
    PreviousState workflow.State
    CurrentState workflow.State
    Message string
}

type DriveReport struct {
    DemandID string
    Steps []DriveStep
    StopReason DriveStopReason
    StopAction ConsoleAction
    Error string
}
```

The actual runner call stays in `internal/cli`, because existing `runDemandStage` is a CLI execution function. Demandflow owns the deterministic decision model; CLI owns side effects.

## 7. CLI Boundary

Add:

```go
func runDrive(args []string, stdout io.Writer, stderr io.Writer) error
```

Responsibilities:

- parse drive flags;
- load `InspectConsole`;
- stop immediately if action is not safe;
- if dry-run, print planned action and stop without mutation;
- execute safe action by calling the same helper used by `console --run-next`;
- loop until stop reason;
- print report.

The loop should reuse existing console runner helpers instead of creating a second execution path.

## 8. Error Handling

- Missing `--demand`: return `--demand is required`.
- Invalid `--max-steps`: return a clear range error.
- Runner error: print the partial drive report and return the runner error.
- MR review without flags: stop with `mr_flags_required`, no mutation.
- Human confirmation: stop with `human_confirmation`, no mutation.
- Memory gate: stop with `memory_gate`, no mutation.
- Completed demand: stop with `complete`.

## 9. Tests

Demandflow tests:

- classify stop reasons from console actions;
- max step decision;
- completed demand stop;
- memory gate stop;
- human confirmation stop.

CLI tests:

- `drive --dry-run` prints planned first step and does not call runner;
- `drive` calls runner repeatedly until human confirmation;
- `drive` stops on runner error and returns error;
- `drive` refuses missing GitLab flags for MR review;
- `drive` enforces max steps;
- help text includes drive.

## 10. Non-Goals

Wave 15 does not do:

- auto-confirm;
- auto-promote or auto-reject memory;
- auto-merge MR;
- Web UI;
- TUI;
- Eino;
- background daemon;
- cross-demand scheduling;
- changing workflow state rules.

## 11. Completion Criteria

Wave 15 is complete when:

- `devflow drive --demand <id>` exists.
- drive can execute multiple runner-safe stages in one command.
- drive stops at the next manual gate with an explicit reason.
- drive has `--dry-run`.
- drive has max-step protection.
- drive records progress evidence.
- all new behavior has tests.
- full Go verification and CI pass.
