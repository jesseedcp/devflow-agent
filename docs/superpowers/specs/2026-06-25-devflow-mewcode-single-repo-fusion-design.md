# Devflow 与 MewCode 单仓库融合设计

## 1. 决策背景

Devflow 已经具备确定性的需求工作流底座，包括状态机、阶段产物、人工确认、验证证据、结项材料和文件型记忆候选。MewCode 已经具备完整 Coding Agent 所需的 LLM provider、Agent loop、工具、权限、Plan、TUI、MCP、Skill、Memory、Subagent、Worktree、Teams、Hooks、Session 和上下文管理能力。

产品目标不是长期维护两个相互调用的程序，而是形成一个统一的后端业务需求专家 Agent。用户确认有权复制、修改并迁移当前本地 MewCode 快照，因此融合采用源码迁移方式推进。

## 2. 已确认的产品决策

### 2.1 唯一产品入口

最终用户只启动：

```text
devflow
```

`devflow-agent` 是唯一主仓库和后续开发仓库。原 `mewcode-golang` 目录只作为迁移参考，不再作为并行演进的产品仓库。

### 2.2 完整融合范围

目标是迁移 MewCode 的完整核心能力，并在迁移过程中同步接入 Devflow 产品语义，而不是先复制出一个换名后的 MewCode。

需要保留的核心能力：

- LLM provider 和流式输出。
- Agent 执行循环和工具调用。
- 权限控制和人工授权。
- Plan 模式和 Plan 子 Agent。
- TUI。
- MCP。
- Skill。
- Memory。
- Subagent。
- Worktree。
- Teams。
- Hooks。
- Session、历史和上下文管理。

允许统一或调整的外部行为：

- 产品命令统一为 `devflow`。
- 项目状态目录统一为 `.devflow`。
- MewCode Plan 输出改为当前需求的阶段产物。
- TUI 改造成 Devflow 双模式工作台。
- 命令名称、配置结构和交互可以为产品一致性调整。

不要求继续维护独立的 `mewcode` 产品入口、旧 TUI 外壳或两套平行工作流。

## 3. 系统职责

### 3.1 Devflow Workflow

Devflow Workflow 是产品流程权威，负责：

- 当前需求和当前阶段。
- 合法状态跳转。
- 人工确认门。
- 阶段产物生命周期。
- 质量门。
- MR 评论触发的阶段回退。
- 证据和事件审计。
- 知识候选的审核状态。

Agent prompt、MewCode Plan Mode、Skill 或 Eino 图都不能绕过 Workflow 直接推进阶段。

### 3.2 Runtime

从 MewCode 迁移而来的 Runtime 负责：

- 调用模型。
- 管理对话和上下文。
- 读取、搜索和修改代码。
- 执行工具和命令。
- 权限检查。
- 调用 Skill、MCP 和子 Agent。
- 创建 Worktree。
- 执行已批准的技术方案。

Runtime 不自行决定需求是否可以进入下一阶段，只向 Workflow 返回产物、工具结果和执行证据。

### 3.3 TUI

TUI 负责展示和交互，不拥有工作流规则。它提供两个模式：

1. Devflow 工作流工作台。
2. 通用编程对话。

两个模式共享项目仓库、工具、Skill、项目记忆和需求产物，但分别保存对话历史。

## 4. 目标目录结构

```text
devflow-agent/
├── cmd/devflow
├── internal/workflow
├── internal/artifacts
├── internal/adapters
├── internal/quality
├── internal/runtime
│   ├── config
│   ├── llm
│   ├── conversation
│   ├── tools
│   ├── permissions
│   ├── agent
│   ├── plan
│   ├── skills
│   ├── memory
│   ├── mcp
│   ├── subagents
│   ├── worktree
│   ├── teams
│   ├── hooks
│   ├── session
│   └── compact
├── internal/tui
│   ├── workflow
│   └── coding
└── docs/migration
    └── mewcode-source-manifest.md
```

实际迁移时允许根据 Go package 依赖关系微调名称，但不能把 Workflow 权限下沉到 Runtime，也不能把通用 Agent 逻辑复制进每个阶段处理器。

## 5. 双模式 TUI

### 5.1 工作流工作台

工作流模式围绕当前需求组织信息，至少展示：

- 当前需求。
- 当前阶段。
- 可执行动作。
- 待确认事项。
- 阶段产物。
- Agent 执行和工具日志。
- 质量门结果。

### 5.2 通用编程模式

通用模式保留 MewCode 的 Coding Agent 能力，可以自由讨论、探索和修改代码。它可以读取当前需求的已确认产物，但不能直接改变需求状态或伪造确认记录。

### 5.3 模式恢复

Devflow 启动时恢复上次使用模式。首次启动默认进入工作流工作台。

工作流对话与通用编程对话分开持久化，避免临时讨论污染正式需求上下文。

## 6. 未关联变更

当用户在没有当前需求的通用编程模式中修改代码时，系统允许执行，但必须记录为未关联变更。

每组未关联变更至少记录：

- 变更 ID。
- 来源 Session。
- 创建时间。
- 仓库和分支。
- 基线 commit。
- 文件 diff 或补丁引用。
- 已执行测试和结果。
- 当前关联状态。

用户可以：

- 关联到已有需求。
- 创建新需求并关联。
- 明确保留为独立维护变更。
- 放弃变更。

只有用户显式关联后，变更来源、diff 和测试证据才进入需求事件流。自动关联和自动创建临时需求不属于第一版行为。

## 7. Plan 融合

MewCode 已有 Plan Mode、Explore Agent、Plan Agent、只读权限和人工批准机制。Devflow 不重新实现 Plan 推理能力，而是改变它的输入、输出和审批归属。

融合后的链路：

```text
requirements 已确认
-> Workflow 进入 plan_drafting
-> Runtime 读取 requirements.md 和代码事实
-> MewCode Plan 能力探索仓库并生成技术方案
-> 输出到 .devflow/demands/<demand-id>/plan.md
-> Workflow 进入 plan_review
-> Devflow 记录人工确认
-> Workflow 进入 implementation
-> Runtime 执行已批准 plan
```

`.mewcode/plans/<random>.md` 不再是 Devflow 工作流 Plan 的权威文件。通用模式仍可保留独立草稿能力，但必须与需求阶段产物明确区分。

## 8. 配置兼容

新配置写入：

```text
.devflow/config.yaml
```

读取顺序：

```text
.devflow/config.yaml
-> 不存在时读取 .mewcode/config.yaml
-> 提示或执行安全迁移
```

API Key 继续优先从环境变量读取，不把明文密钥复制进迁移文件。旧配置兼容是迁移通道，不代表长期维护两套配置语义。

## 9. 源码迁移与提交协议

### 9.1 迁移粒度

每次只迁移一个 Go package。例如：

```text
internal/runtime/config
internal/runtime/llm
internal/runtime/conversation
```

一个 package 完成迁移、融合和验证后，必须立即形成独立提交，才能开始下一个 package。

### 9.2 每个 package 的完成条件

每次迁移必须同时完成：

1. 将目标 MewCode package 迁入 Devflow。
2. 调整 module import 和包边界。
3. 接入 `.devflow` 配置或产品语义。
4. 修复该 package 暴露的 Windows 兼容问题。
5. 保留或补充单元测试。
6. 运行 package 测试。
7. 运行受影响的 Devflow 回归测试。
8. 更新 `docs/migration/mewcode-source-manifest.md`。
9. 创建一笔 Lore commit。

禁止把多个已完成 package 合并进同一提交，也禁止在测试失败时继续迁移后续 package。

### 9.3 来源清单

来源清单为每个 package 记录：

- MewCode 原始路径。
- Devflow 目标路径。
- 本地快照日期。
- 原始文件列表或校验信息。
- 融合调整。
- Windows 修复。
- 测试证据。
- 对应 Devflow commit。
- 授权前提。

用户已确认有权复制和修改当前本地 MewCode 快照。该确认是本项目迁移工作的授权前提；来源清单仍需保留原始来源说明。

### 9.4 Commit 规则

每笔迁移提交遵循仓库 Lore Commit Protocol，例如：

```text
Bring provider configuration under the Devflow product boundary

Migrate MewCode's configuration package so the fused runtime can load
providers through Devflow while preserving legacy .mewcode fallback.

Constraint: One migrated Go package per commit
Constraint: User confirmed authority to copy and modify the local MewCode snapshot
Rejected: Keep two permanent configuration authorities | would preserve product ambiguity
Confidence: high
Scope-risk: narrow
Directive: New configuration writes belong under .devflow
Tested: go test ./internal/runtime/config
Not-tested: End-to-end provider request until the llm package is migrated
```

## 10. Windows 兼容策略

Windows 是当前开发和验收环境，不能作为后续补做项。

采用随迁随修策略：

- 只修当前 package 相关问题。
- 使用 `filepath` 和平台适配文件处理路径差异。
- 对符号链接、reparse point、命令执行和终端行为分别测试。
- 不把 MewCode 当前全量测试失败当作迁移后可接受状态。
- 每个迁移提交必须声明已测和未测范围。

## 11. 错误处理与恢复

- Runtime 错误必须返回 Workflow，不得静默推进状态。
- LLM 或平台暂时不可用时，需求进入可恢复的阻塞状态并记录原因。
- 阶段产物写入必须使用安全写入策略，避免半文件覆盖。
- 工具执行结果、失败原因和重试结果写入事件日志。
- 通用模式失败不得污染当前需求状态。
- 未关联变更关联失败时保留原记录，不自动丢弃 diff。

## 12. 测试策略

### 12.1 Package 级

每个迁移 package 保留原有效测试，并增加 Devflow 路径、配置和 Windows 行为测试。

### 12.2 Runtime 集成

逐步验证：

- 配置可以加载真实 provider。
- LLM 可以完成最小流式请求。
- Agent 可以调用只读工具。
- 权限系统能阻止越权写入。
- Plan Mode 只能写目标 Plan 文件。
- 批准后的 Plan 可以交给执行 Agent。

### 12.3 产品闭环

最终必须自动化或可重复地证明：

```text
创建需求
-> 生成 requirements
-> 人工确认
-> 生成 plan
-> 人工确认
-> 修改后端代码
-> 执行测试
-> 记录 verification
-> 生成 closeout 和知识候选
```

双模式还必须验证：

- 对话记录隔离。
- 共享仓库和项目记忆。
- 通用模式不能推进需求状态。
- 未关联变更可以显式关联需求。

## 13. 实施边界

本设计确定完整融合的目标形态，但实施仍按 package 顺序推进。不得用“大提交复制全部源码”代替逐包迁移，也不得为了短期演示绕开 Workflow、人工确认或事件审计。

Eino 仍不是顶层工作流引擎。只有当某个阶段内部确实需要多节点分支、重试或观测时，才评估在该阶段内部引入 Eino。

## 14. 完成标准

融合完成需要同时满足：

- `devflow` 是唯一产品入口。
- MewCode 核心能力已经进入 Devflow 主仓库。
- 工作流模式和通用编程模式均可使用。
- 两种模式共享项目能力但隔离对话和状态权限。
- `.devflow` 是新配置和产品状态权威。
- 旧 `.mewcode` 配置可以兼容读取和迁移。
- Requirements、Plan、实现、验证和结项形成真实闭环。
- 每个迁移 package 都有独立来源记录、测试证据和 Lore commit。
- 当前支持范围内的 Windows 测试通过。
- 原 MewCode 仓库不再是并行开发依赖。
