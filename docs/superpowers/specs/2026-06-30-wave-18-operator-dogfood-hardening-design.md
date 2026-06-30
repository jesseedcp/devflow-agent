# Wave 18 Operator Dogfood Hardening 设计

## 1. 背景

Wave 15 到 Wave 17 已经补上了三个面向操作者的能力：

- `devflow drive`：自动执行 runner-safe 阶段，直到人工门、MR 参数缺失、runner 失败或 max steps。
- `devflow evaluate`：用确定性规则检查 requirements、plan、verification、closeout 的结构质量。
- `devflow workbench`：用 TUI 列出需求、查看详情、触发 run-next / drive / evaluate。

这些能力单独有测试，也已经合并进 `main`。但现有 deterministic dogfood 主要走 `internal/dogfood.Run -> demandflow.Engine`，它能证明底层 workflow 能跑通，却不能证明操作者实际会使用的入口也顺滑。

Wave 18 不扩展新 Agent，也不做 GitLab 写回。它专门做一件事：用一个真实的 operator dogfood loop 去压刚落地的操作面，发现并固化使用体验。

## 2. 产品目标

新增一个操作者验收模式：

```powershell
devflow dogfood --operator-loop
```

这个模式应该在本地、无外部 API、无真实 LLM 的情况下，完整验证：

1. console 能识别当前需求状态和下一步；
2. drive 决策能识别 runner-safe 阶段和人工门；
3. evaluate 能在确认前给出质量信号；
4. workbench 能用非交互 snapshot 输出当前需求视图；
5. 完整流程能从 created 走到 completed；
6. 最终报告能告诉用户每一步发生了什么、在哪里卡住、哪些证据可复查。

推荐同时新增：

```powershell
devflow workbench --snapshot
devflow workbench --snapshot --demand <id>
```

`workbench --snapshot` 是非交互模式，用来让 CI、dogfood、用户截图前快速确认当前 workbench 视图。它不取代 TUI，只是让 TUI 的数据视图可验证。

## 3. 用户视角

### 3.1 Operator Dogfood

用户执行：

```powershell
devflow dogfood --operator-loop --quality-command "go test ./..."
```

输出示例：

```text
operator dogfood completed for dogfood-coupon-eligibility
state: completed
root: C:\Users\dd\AppData\Local\Temp\devflow-operator-dogfood-...
report: ...\.devflow\demands\dogfood-coupon-eligibility\operator-dogfood-report.md
```

报告需要包含：

- final state；
- 每个 operator step；
- console attention；
- drive stop reason 或 runnable stage；
- evaluate overall / stage status；
- workbench snapshot excerpt；
- artifact paths；
- release-readiness 是否包含 operator loop。

### 3.2 Workbench Snapshot

用户执行：

```powershell
devflow workbench --snapshot
```

输出示例：

```text
Devflow Workbench Snapshot

> dogfood-coupon-eligibility completed complete

Summary
State: completed
Attention: complete
Next:
  No action
Run-ready:
  no runner-safe action available
```

如果传入 `--demand <id>`，snapshot 应选中并展示该需求详情。找不到需求时返回非零，并说明 demand 不存在。

## 4. 范围

Wave 18 做：

- `devflow workbench --snapshot`；
- `devflow workbench --snapshot --demand <id>`；
- `internal/dogfood.RunOperator`，用 deterministic runner 压 console / drive decision / evaluate / workbench snapshot；
- `devflow dogfood --operator-loop`；
- operator dogfood report；
- release-readiness script 默认跑 operator dogfood；
- 用户文档和 release note。

Wave 18 不做：

- GitLab 评论回复、resolve、approval；
- 真正打开 TUI 并模拟键盘；
- 引入浏览器 UI；
- 引入 Eino 编排；
- 自动确认人工门以外的危险操作；
- 新的 PD / frontend / testing Agent。

## 5. 架构

### 5.1 Workbench Snapshot

在 `internal/cli/workbench.go` 增加 snapshot flag：

```text
workbenchOptions
  root
  configPath
  noAltScreen
  snapshot
  demandID
  qualityCommand
```

当 `snapshot == true` 时，不启动 Bubble Tea program，而是调用一个纯文本 renderer：

```text
renderWorkbenchSnapshot(opts) (string, error)
```

Renderer 使用现有 demandflow API：

- `demandflow.ListConsole(root)`；
- `demandflow.InspectConsole(root, demandID)`；
- `demandflow.EvaluateDemand(root, demandID)`。

它不能执行 run-next、drive、evaluate action。snapshot 是只读的。

### 5.2 Operator Dogfood Runner

新增 `internal/dogfood/operator.go`。

它不 import `internal/cli`，避免循环依赖。它直接复用底层 domain API：

- `artifacts.Store` 创建 demand；
- `demandflow.NewEngine` + `StaticRunner` 生成阶段产物；
- `demandflow.InspectConsole` 获取 operator 状态；
- `demandflow.DecideDriveStop` 验证 drive 决策；
- `demandflow.EvaluateDemand` 获取 quality signal；
- `demandflow.Confirm` 表达人工确认门；
- offline MR / review adapter 复用现有 dogfood adapter。

这样 dogfood 能验证 operator semantics，又不需要真实 LLM 或 GitLab。

### 5.3 CLI Dogfood

修改 `internal/cli/dogfood.go`：

```powershell
devflow dogfood --operator-loop
```

默认 `devflow dogfood` 仍跑原 deterministic loop，保持兼容。

### 5.4 Release Readiness

修改 `scripts/release-readiness.ps1`，在 deterministic dogfood 后新增：

```powershell
devflow dogfood --operator-loop
```

或调用构建出的 `dist/devflow-windows-amd64.exe dogfood --operator-loop`。

这一步应该默认开启，因为它不访问外部 API。

## 6. 状态与报告

新增报告文件：

```text
.devflow/demands/<id>/operator-dogfood-report.md
```

报告结构：

````markdown
# Operator Dogfood Report: <demand>

Root: `<root>`
QualityRoot: `<qualityRoot>`
FinalState: `<state>`

## Operator Steps

| Step | State | Attention | Drive | Evaluation |
| --- | --- | --- | --- | --- |
| initial console | created | ready to draft requirements | requirements runnable | requirements fail |

## Workbench Snapshot

```text
...
```

## Artifacts

- `requirements.md`
- `plan.md`
- `progress.md`
- `verification.md`
- `closeout.md`
- `memory-candidates.md`
- `events.jsonl`
````

## 7. Quality Expectations

Operator dogfood is successful when:

- final state is `completed`;
- report exists;
- every major workflow stage appears in report;
- report includes at least one drive runnable decision;
- report includes at least one human confirmation stop;
- report includes evaluate results;
- report includes workbench snapshot;
- `devflow workbench --snapshot --demand <id>` works for the generated demand.

## 8. Error Handling

- Unsupported scenario returns the same style of error as existing dogfood.
- If drive decision says the next stage is not runnable when it should be runnable, operator dogfood fails with the current state and action label.
- If evaluation fails at a point that should pass after artifact generation, operator dogfood fails with stage and check ids.
- If workbench snapshot cannot render the selected demand, operator dogfood fails and points to the demand id.
- `workbench --snapshot --demand missing` returns non-zero and prints a clear error.

## 9. Testing

Required tests:

- workbench snapshot list renders demand id, state, attention;
- workbench snapshot selected demand renders Summary / Quality / Next / Run-ready;
- workbench snapshot missing demand returns error;
- operator dogfood completes to `completed`;
- operator report includes drive / evaluate / workbench evidence;
- `devflow dogfood --operator-loop` dispatches operator runner;
- release-readiness script includes operator dogfood.

## 10. Completion Criteria

Wave 18 is complete when:

- `devflow workbench --snapshot` exists and is tested;
- `devflow dogfood --operator-loop` exists and is tested;
- operator dogfood report is written;
- release readiness runs operator dogfood by default;
- docs explain when to use operator dogfood;
- `go test ./... -count=1 -timeout 5m` passes;
- `go vet ./...` passes;
- `go build ./cmd/devflow` passes;
- `git diff --check` passes;
- PR CI passes on Ubuntu and Windows.
