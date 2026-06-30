# Wave 13 需求工作台与运营闭环设计

## 1. 背景

Devflow 已经完成从需求输入到 closeout 的后端业务需求闭环，并陆续接入了 GitLab MR 创建、MR review gate、review comment 路由、稳定知识晋升与复用。现在系统已经“能跑流程”，但用户仍然需要记住很多分散命令和隐含判断：

```text
devflow status
devflow next
devflow run
devflow confirm
devflow verify
devflow memory list/promote/reject
GitLab MR 参数
```

当前 `devflow status --demand <id>` 已有雏形：它能展示 demand、state、artifact 文件大小和 next actions。Wave 13 不推倒它，而是在现有 status/next 基础上沉淀一个需求运营摘要，让用户能一眼看到需求当前卡在哪里、缺什么证据、下一步该执行什么。

## 2. 产品目标

Wave 13 的目标是把 Devflow 从“阶段命令集合”升级成“需求运营工作台”。

用户运行：

```powershell
devflow status --demand add-coupon-check
```

应该看到：

- 当前需求状态。
- 阶段产物是否存在。
- 关键阶段是否已人工确认。
- verification 是否有 PASS/FAIL 证据。
- MR 是否已有 evidence，review gate 是否清理。
- memory candidates 是否还有 pending。
- 下一步推荐命令。

用户运行：

```powershell
devflow status --all
```

应该看到所有需求的简表：

```text
add-coupon-check       verification   needs verification evidence
refund-policy-update   mr_review      needs MR review gate
order-risk-rule        closeout       memory candidates pending
```

## 3. Wave 13 做什么

### 3.1 单需求工作台

增强 `devflow status --demand <id>`，输出一个面向操作的 summary。

建议输出结构：

```text
Demand: add-coupon-check
Title: Add coupon eligibility check
State: verification
Directory: .devflow/demands/add-coupon-check

Stage summary:
  requirements   confirmed
  plan           confirmed
  implementation completed
  mr-review      cleared
  verification   needs evidence
  closeout       pending

Artifacts:
  requirements.md       present, confirmed
  plan.md               present, confirmed
  progress.md           present
  verification.md       present, no PASS evidence
  closeout.md           template only
  memory-candidates.md  2 pending candidates
  events.jsonl          18 events

MR:
  !12 open
  review gate cleared

Verification:
  latest: FAIL go test ./...
  failure_kind: exit_nonzero

Memory:
  candidates: 2 pending, 1 promoted, 0 rejected

Next:
  Draft verification
  devflow run --demand add-coupon-check --stage verification --quality-command "go test ./..."
```

输出不需要花哨 UI。重点是可读、稳定、可测试。

### 3.2 多需求列表

新增：

```powershell
devflow status --all
```

行为：

- 扫描 `.devflow/demands/`。
- 只列合法 demand id 目录。
- 对每个 demand 生成轻量 summary。
- 按最需要处理的需求优先排序。

建议优先级：

1. blocked / failed_quality_gate / returned_to_*。
2. mr_review / verification / closeout。
3. requirements / plan / implementation。
4. completed。

### 3.3 Next action 更聪明

现有 `NextActions(state, demandID)` 只看 workflow state。Wave 13 增加 evidence-aware next action：

- closeout 状态下，如果 `memory-candidates.md` 有 pending candidate，提示先处理 memory。
- verification 状态下，如果已有 PASS evidence，提示 confirm verification；如果没有，提示 draft/record verification。
- MR review 状态下，如果 progress 已有 cleared evidence，提示进入 verification；如果没有，提示 run mr-review。
- completed 状态下，如果还有 pending memory candidates，仍提示 memory review，而不是单纯 no action。

保持 `devflow next --demand <id>` 只输出第一条推荐命令，适合脚本使用。

### 3.4 材料索引

增强 status 中的 artifact 描述，不只显示文件大小。

每个 artifact 至少有：

- `missing`
- `template`
- `present`
- `confirmed`
- `needs_review`
- `has_pass_evidence`
- `has_fail_evidence`

这些状态从现有文件和 `events.jsonl` 推导，不新增新的状态文件。

## 4. 数据来源

Wave 13 只从已有材料计算，不创建新的权威数据源。

| 信息 | 来源 |
| --- | --- |
| demand 元数据 | `.devflow/demands/<id>/demand.json` |
| workflow state | `demand.json.state` |
| artifact 存在性 | 标准 artifact 文件 |
| confirmation | `events.jsonl` 中 `stage.confirmed` / confirmation data |
| verification | `verification.recorded` event 和 `verification.md` |
| MR evidence | `progress.md` 与 `implementation.completed` / `mr_review.*` events |
| memory candidate 状态 | `memory-candidates.md` + `memory.promoted` / `memory.rejected` events |
| next action | workflow state + evidence summary |

## 5. 代码边界

### 5.1 `internal/demandflow`

新增或扩展 status 模型：

```go
type WorkspaceSummary struct {
    Demand artifacts.Demand
    State workflow.State
    DemandDir string
    Stages []StageSummary
    Artifacts []ArtifactSummary
    Verification VerificationSummary
    MergeRequest MergeRequestSummary
    Memory MemorySummary
    Actions []NextAction
    Attention string
}
```

`InspectStatus` 可以保留，但内部应委托到新的 summary 构建器，避免 `status` 和未来 TUI 各自重算。

建议新增：

```go
func InspectWorkspace(root, demandID string) (WorkspaceSummary, error)
func ListWorkspaces(root string) ([]WorkspaceSummary, error)
```

### 5.2 `internal/cli`

增强：

```powershell
devflow status --demand <id>
devflow status --all
devflow next --demand <id>
```

不新增单独 `inspect` 命令，避免命令面膨胀。`status` 默认是单需求 detail，`--all` 是列表。

### 5.3 `internal/memory`

复用 Wave 12 的 candidate parser 和 decision loader 能力。必要时把“统计 pending/promoted/rejected”封装成可导出函数，供 demandflow status 使用。

### 5.4 `internal/artifacts`

不改变 artifact 写入规则。只在需要时新增只读 event 读取 helper。不要让 status 直接写 events。

## 6. 错误处理

- `.devflow/demands` 不存在时，`status --all` 输出空列表说明，而不是报 panic。
- 单个 demand 的 `events.jsonl` 尾部损坏时，可以复用现有尾部修复逻辑；中间损坏时报告该 demand 的 status 为 `events_error`，但 `status --all` 仍继续列其他 demand。
- 某个 artifact 不可读时，标记该 artifact 为 `read_error`，不要让整个 `status --all` 失败。
- demand id 不合法或不存在时，单需求 status 保持明确错误。
- 不访问 GitLab API；MR 状态只读本地已有 evidence。

## 7. 测试策略

### 7.1 单元测试

覆盖：

- 事件解析出 confirmed stages。
- verification PASS/FAIL/latest command 推导。
- memory pending/promoted/rejected 统计。
- MR evidence 从 progress/events 推导。
- evidence-aware next action 优先级。
- template-only artifact 和 present artifact 区分。

### 7.2 CLI 测试

覆盖：

- `devflow status --demand <id>` 输出 Stage summary、Artifacts、Verification、Memory、Next。
- `devflow status --all` 输出多个需求简表。
- `devflow next --demand <id>` 在有 PASS verification 时优先输出 confirm verification。
- completed 但有 pending memory candidate 时，输出 memory review action。

### 7.3 回归验证

每次实现完成后运行：

```powershell
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

## 8. 非目标

Wave 13 不做：

- TUI 工作流界面。
- Web UI。
- 调用 GitLab API 拉最新 MR 状态。
- 自动执行 next action。
- 自动 promote/reject memory。
- 新增数据库。
- 改 workflow 状态机。
- 改 Agent prompt 生成逻辑。

## 9. 完成标准

Wave 13 完成时，需要满足：

- 单需求 status 能展示运营摘要，而不只是文件大小。
- `status --all` 能列出所有需求和各自 attention reason。
- `next` 能根据 evidence 选择更合适的第一命令。
- memory pending/promoted/rejected 能在 status 中展示。
- verification PASS/FAIL evidence 能在 status 中展示。
- MR 本地 evidence 能在 status 中展示。
- 所有新增行为有测试。
- 全量 Go 验证通过。

## 10. 为什么先做 A 再做 TUI

TUI 需要一个稳定的数据模型。如果直接做 TUI，界面会到处解析 artifacts、events、memory 和 progress，逻辑会散在 UI 层。

Wave 13 先做 CLI 版需求工作台，实际沉淀的是：

```text
Demand files + events + memory decisions
-> WorkspaceSummary
-> CLI status / next
-> future TUI
```

这样 Wave 14 如果做 TUI，可以直接复用 `WorkspaceSummary`，而不是重新拼业务逻辑。
