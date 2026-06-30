# Wave 14 Operator Console + Agent Runner Entry 设计

## 1. 背景

Wave 13 已经把分散的需求材料收敛成 `WorkspaceSummary`，并增强了：

```text
devflow status --demand <id>
devflow status --all
devflow next --demand <id>
```

这解决了“看不清当前需求卡在哪里”的问题，但用户仍然要在多个命令之间切换：

```text
status -> next -> 复制命令 -> run -> status -> confirm -> status -> run ...
```

Wave 14 接受 A+B 路线：

- A：Operator Console，让用户一眼看到所有需求和单个需求的操作面板。
- B：Agent Runner Entry，让 console 可以把安全的下一步 agent stage 跑起来。

Wave 14 不做 Web UI，也不做全自动无人值守。它要把 Devflow 从“命令集合”推进到“可操作的后端需求工作台”。

## 2. 产品目标

用户运行：

```powershell
devflow console
```

应该看到本地所有需求的运营列表：

```text
Demand Console

  add-coupon-check       verification        ready to confirm verification
  refund-policy-update   mr_review           needs MR review gate
  order-risk-rule        closeout            memory candidates pending

Next:
  devflow console --demand add-coupon-check
```

用户运行：

```powershell
devflow console --demand add-coupon-check
```

应该看到一个更像操作台的单需求视图：

```text
Demand Console: add-coupon-check
State: verification
Attention: ready to confirm verification

Stages:
  requirements   confirmed
  plan           confirmed
  implementation completed
  mr-review      cleared
  verification   passed
  closeout       pending

Evidence:
  verification   PASS go test ./...
  memory         0 pending, 1 promoted, 0 rejected
  mr             !12 cleared

Recommended:
  Confirm verification
  devflow confirm --demand add-coupon-check --stage verification --by <name> --summary <summary>

Run-ready:
  no safe runner action
```

当下一步是 agent 可执行阶段时，用户可以运行：

```powershell
devflow console --demand add-coupon-check --run-next
```

如果下一步是 requirements / plan / implementation / verification / closeout 这些 `devflow run --stage ...` 阶段，则 console 调用现有 runner 执行。执行后继续打印阶段结果和新的 next action。

如果下一步是人工确认、memory promote/reject、或缺少 GitLab 参数的 MR review，则 `--run-next` 不执行，只解释为什么：

```text
next action is not runner-safe: Confirm verification
manual command:
devflow confirm --demand add-coupon-check --stage verification --by <name> --summary <summary>
```

## 3. Wave 14 做什么

### 3.1 Console Summary

新增命令：

```powershell
devflow console
devflow console --demand <id>
```

`devflow console` 是列表视图，复用 Wave 13 的 `ListWorkspaces(root)`，按 attention priority 展示 demand。

`devflow console --demand <id>` 是单需求视图，复用 `InspectWorkspace(root, id)`，但输出比 `status` 更偏“操作”：

- 当前 demand。
- 当前 state。
- attention reason。
- stages。
- evidence 摘要。
- recommended action。
- run-ready action。

`status` 继续保留为偏材料审计的命令；`console` 偏操作入口。

### 3.2 Typed Console Actions

Wave 13 的 `NextAction` 是面向人看的字符串：

```go
type NextAction struct {
    Label string
    Command string
    Reason string
}
```

Wave 14 不删除它，而是在 `internal/demandflow` 增加一层操作分类：

```go
type ConsoleAction struct {
    Label string
    Command string
    Reason string
    Kind ConsoleActionKind
    Stage Stage
    Runnable bool
    BlockReason string
}
```

建议 action kind：

- `agent_stage`
- `human_confirmation`
- `memory_review`
- `memory_decision`
- `mr_review`
- `manual`
- `none`

分类规则：

| Next command | Kind | Runnable |
| --- | --- | --- |
| `devflow run --stage requirements` | `agent_stage` | yes |
| `devflow run --stage plan` | `agent_stage` | yes |
| `devflow run --stage implementation` | `agent_stage` | yes |
| `devflow run --stage verification` | `agent_stage` | yes |
| `devflow run --stage closeout` | `agent_stage` | yes |
| `devflow confirm ...` | `human_confirmation` | no |
| `devflow memory list ...` | `memory_review` | no |
| `devflow memory promote/reject ...` | `memory_decision` | no |
| `devflow run --stage mr-review` with required GitLab flags present | `mr_review` | yes |
| `devflow run --stage mr-review` with placeholder GitLab flags | `mr_review` | no |
| empty command | `none` | no |

Console action classification must not shell-parse arbitrary commands to execute them. It can inspect known Devflow-generated command shapes, but actual execution should dispatch through typed stage metadata.

### 3.3 Runner Entry

新增：

```powershell
devflow console --demand <id> --run-next
```

Behavior:

1. Load workspace summary.
2. Build console action from first recommended next action.
3. If action is not runnable, print block reason and manual command.
4. If action is runnable, call the same internal execution path as `devflow run`.
5. After execution, print updated console detail or at least updated next action.

Supported runner stages in Wave 14:

- requirements
- plan
- implementation
- verification
- closeout

MR review is allowed only when user provides:

```powershell
--gitlab-project <project>
--gitlab-mr <iid>
```

For implementation and verification, console accepts repeatable:

```powershell
--quality-command "go test ./..."
```

If omitted, use the action’s existing default when it is present in the generated command. Current Wave 13 defaults already include `go test ./...` for implementation and verification.

### 3.4 Human Confirmation Gate

Wave 14 must not auto-confirm:

```powershell
devflow confirm ...
```

Even if verification is PASS, console should stop and tell the user that confirmation is manual. This is important because the product promise is not “autonomous merge machine”; it is a back-end demand expert with explicit human gates.

### 3.5 Memory Gate

Wave 14 must not auto-promote or auto-reject memory.

If pending memory exists, console should show:

```text
Run-ready:
  no safe runner action

Manual:
  devflow memory list --demand <id>
```

The human still decides which candidate becomes stable knowledge.

## 4. Command Shape

### 4.1 Read-Only Console

```powershell
devflow console [--root <path>]
devflow console --demand <id> [--root <path>]
```

### 4.2 Execute Next Runner Stage

```powershell
devflow console --demand <id> --run-next [--root <path>] [--runner-root <path>] [--quality-root <path>] [--config <path>] [--permission-mode acceptEdits|bypassPermissions] [--quality-command <command>]
```

### 4.3 MR Review Runner Stage

```powershell
devflow console --demand <id> --run-next --gitlab-project <project> --gitlab-mr <iid>
```

Only needed when the next action is MR review.

## 5. Code Boundary

### 5.1 `internal/demandflow`

Add console model:

```go
type ConsoleSummary struct {
    Workspace WorkspaceSummary
    PrimaryAction ConsoleAction
    RunReadyAction ConsoleAction
}

func InspectConsole(root, demandID string) (ConsoleSummary, error)
func ListConsole(root string) ([]ConsoleSummary, error)
func BuildConsoleAction(summary WorkspaceSummary, action NextAction) ConsoleAction
```

This layer should be read-only and deterministic.

### 5.2 `internal/cli`

Add:

```go
func runConsole(args []string, stdout io.Writer, stderr io.Writer) error
```

Responsibilities:

- parse flags;
- render list or detail;
- execute `--run-next` only when `ConsoleAction.Runnable == true`;
- reuse existing `runDemandStage` behavior instead of duplicating stage execution.

Use test stubs around execution so CLI tests do not call real LLM providers.

### 5.3 No MewCode Runtime Migration

The repo already contains runtime/TUI code inherited from MewCode-style foundations. Wave 14 should not migrate console into that TUI. The point is to keep the product surface slim:

```text
WorkspaceSummary -> ConsoleSummary -> CLI console -> future TUI can reuse ConsoleSummary
```

Bubble Tea or richer TUI can be Wave 15+ after the console semantics are stable.

### 5.4 No Eino

Do not introduce Eino in Wave 14. The runner entry is still a single stage dispatch into existing Devflow engine/runtime runner. Eino remains a later option for multi-node LLM subflows, not the top-level product state machine.

## 6. Error Handling

- `devflow console` with no demands prints `No demands found`.
- `devflow console --demand <missing>` returns the existing demand load error.
- `--run-next` without `--demand` returns `--demand is required for --run-next`.
- `--run-next` on a human confirmation action exits with a clear message and no mutation.
- `--run-next` on memory review or memory decision exits with a clear message and no mutation.
- `--run-next` on MR review without GitLab flags exits with required flag guidance.
- If runner execution fails, print the partial run result just like `devflow run` and return the error.
- Console output must not print API keys or secret config values.

## 7. Testing Strategy

### 7.1 Demandflow Unit Tests

Cover:

- list console summaries preserve Wave 13 attention ordering;
- verification PASS action is `human_confirmation`, not runnable;
- requirements/plan/implementation/verification/closeout run actions are `agent_stage`, runnable;
- memory pending action is not runnable;
- MR review action is runnable only with real flags supplied later by CLI;
- empty completed action is `none`.

### 7.2 CLI Tests

Cover:

- `devflow console` renders all demands;
- `devflow console --demand <id>` renders stages, evidence, recommended action, run-ready action;
- `devflow console --demand <id> --run-next` calls the stage runner when next is runnable;
- `--run-next` refuses human confirmation;
- `--run-next` refuses memory decisions;
- `--run-next` refuses MR review when required flags are missing;
- help text includes console command.

### 7.3 Regression Verification

Run:

```powershell
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git diff --check
```

## 8. Non-Goals

Wave 14 does not do:

- Web UI.
- Full Bubble Tea TUI.
- automatic human confirmation.
- automatic memory promote/reject.
- automatic MR merge.
- new database or daemon.
- Eino orchestration.
- changing workflow state machine rules.
- replacing `devflow status`.
- replacing `devflow run`.

## 9. Completion Criteria

Wave 14 is complete when:

- `devflow console` lists all demands with attention reasons.
- `devflow console --demand <id>` shows a concise operator view.
- console identifies whether the next action is runner-safe.
- `--run-next` executes requirements/plan/implementation/verification/closeout runner stages through existing execution code.
- `--run-next` refuses human confirmation and memory decisions.
- MR review requires explicit GitLab flags.
- all new behavior has tests.
- full Go verification and CI pass.

## 10. Why This Is The Right Wave 14

Wave 13 created the data model:

```text
Demand files + events + memory decisions
-> WorkspaceSummary
-> status / next
```

Wave 14 should turn that into a product surface:

```text
WorkspaceSummary
-> ConsoleSummary
-> devflow console
-> safe runner entry
-> human gates remain explicit
```

This gives the user a real operating loop without committing to a heavy UI too early. It also keeps the long-term architecture clean: future PD Agent, test Agent, frontend Agent, or TUI can reuse the same console/action model instead of reverse-engineering CLI strings.
