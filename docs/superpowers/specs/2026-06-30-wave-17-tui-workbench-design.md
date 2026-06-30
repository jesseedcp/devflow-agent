# Wave 17 TUI Workbench 设计

## 1. 背景

Wave 13 建立 `WorkspaceSummary`。
Wave 14 建立 `ConsoleSummary` 和 `devflow console`。
Wave 15 计划建立 `devflow drive`，让需求自动推进到人工门。
Wave 16 计划建立 `devflow evaluate`，给阶段产物质量信号。

到 Wave 17，数据模型和操作语义已经稳定，可以开始做一个真正的交互式 TUI 工作台。

当前 repo 已有 MewCode 风格的 Bubble Tea runtime TUI：

```text
devflow chat
devflow tui
```

但这个 TUI 是通用 Agent chat surface，不是 Devflow 产品 workflow workbench。Wave 17 不重写已有 chat TUI，也不把 workflow 规则塞进 runtime TUI。Wave 17 做一个独立的 Devflow demand workbench TUI，复用 console/evaluate/drive 模型。

## 2. 产品目标

新增命令：

```powershell
devflow workbench
```

或：

```powershell
devflow console --tui
```

推荐实现为 `devflow workbench`，避免让 `console` 命令承担太多模式。

第一版 TUI 只需要三栏：

```text
┌ Demands ──────────────┐ ┌ Summary ───────────────────────┐
│ add-coupon-check      │ │ State: verification             │
│ refund-policy-update  │ │ Attention: ready to confirm     │
│ order-risk-rule       │ │ Quality: pass / warning / fail  │
└───────────────────────┘ │ Next: Confirm verification      │
                          └─────────────────────────────────┘

Keys: ↑/↓ select  Enter detail  r run-next  d drive  e evaluate  q quit
```

## 3. User Actions

TUI v1 supports:

- list demands;
- select demand;
- view summary;
- view stages;
- view next action;
- run next safe action;
- drive to next manual gate;
- evaluate selected demand;
- refresh.

TUI v1 does not need:

- full text artifact editor;
- embedded chat;
- automatic confirmation;
- memory promote/reject forms;
- MR merge controls.

Manual gates should show copyable commands rather than executing them.

## 4. Architecture

Create a new product TUI package:

```text
internal/cli/workbench.go
internal/demandflow/workbench.go
internal/runtime/tui remains unchanged
```

Recommended package split:

```text
internal/demandflow/workbench.go
  WorkbenchSnapshot
  LoadWorkbench
  SelectDemand

internal/cli/workbench.go
  command parsing and Bubble Tea launch

internal/cli/workbench_model.go
  Bubble Tea model, update, view
```

The TUI must consume existing models:

```text
ListConsole
InspectConsole
DemandEvaluation
DriveReport
```

It must not compute workflow rules itself.

## 5. Command Shape

```powershell
devflow workbench [--root <path>] [--config <path>] [--quality-command <command>]
```

Flags:

- `--root`: demand root.
- `--config`: passed to runner execution.
- `--quality-command`: used by run-next/drive actions.
- `--no-alt-screen`: useful for tests and terminals where alt screen is undesirable.

## 6. Interaction Contract

Keys:

| Key | Action |
| --- | --- |
| `up/k` | previous demand |
| `down/j` | next demand |
| `enter` | toggle detail view |
| `r` | run next safe action |
| `d` | drive to next manual gate |
| `e` | evaluate demand |
| `R` | refresh snapshot |
| `q` | quit |

After every action, refresh the selected demand summary.

If action is blocked:

```text
Blocked: human confirmation is required
Manual command:
devflow confirm ...
```

## 7. Testing Strategy

Use Bubble Tea model tests without launching a real terminal:

- initial model renders demand list;
- selection moves with up/down;
- detail view shows stages and next action;
- blocked run-next displays block message;
- successful run-next calls stubbed runner and refreshes;
- evaluate action displays quality summary;
- drive action calls stubbed drive runner and displays stop reason.

CLI tests:

- `workbench` dispatches to a stubbed program runner;
- help includes command;
- `--no-alt-screen` is parsed.

## 8. Error Handling

- No demands: show `No demands found` and keep TUI usable.
- Load error: show error panel.
- Runner error: show error panel and keep selected demand.
- Evaluate fail: show evaluation fail as quality signal, not TUI crash.
- Terminal too narrow: render a compact single-column view.

## 9. Non-Goals

Wave 17 does not do:

- Web UI;
- artifact editor;
- embedded LLM chat;
- automatic human confirmation;
- automatic memory promote/reject;
- custom mouse UI;
- Eino;
- replacing existing `devflow chat` runtime TUI.

## 10. Completion Criteria

Wave 17 is complete when:

- `devflow workbench` opens a product demand TUI.
- TUI lists demands and selected summary.
- TUI can run next safe action via existing console runner path.
- TUI can call drive/evaluate helpers when those exist.
- TUI handles blocked manual gates gracefully.
- TUI has model tests and CLI dispatch tests.
- full Go verification and CI pass.
