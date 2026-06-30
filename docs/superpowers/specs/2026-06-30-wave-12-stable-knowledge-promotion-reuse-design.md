# Wave 12 稳定知识晋升与复用设计

## 1. 背景

Devflow 目前已经能在需求闭环结束时产出 `closeout.md` 和 `memory-candidates.md`。这些候选知识能帮助后续需求少走弯路，但它们仍然只是候选：没有人工确认、没有稳定存储格式、也没有明确进入下次需求上下文的规则。

MewCode 已经有记忆机制。原始 MewCode 在 `internal/memory` 中实现了文件型记忆，使用 `.mewcode/memory/`、`~/.mewcode/memory/`、`MEMORY.md` 索引、frontmatter、记忆类型和相关记忆选择器。Devflow 已经把这套能力迁入 `internal/runtime/memory`，并改成 `.devflow/memory/`、`~/.devflow/memory/`，同时保留 `MEWCODE_REMOTE_MEMORY_DIR` 作为迁移兼容 fallback。

Wave 12 不重新发明记忆系统。Wave 12 补的是产品闭环：把需求过程里产生的候选知识，经人工确认后晋升为项目级稳定知识，再让后续需求可以稳定复用。

## 2. 产品目标

目标链路：

```text
closeout.md
-> memory-candidates.md
-> devflow memory list
-> devflow memory promote / reject
-> .devflow/memory/<slug>.md + .devflow/memory/MEMORY.md
-> 下一个需求的 requirements / plan 上下文读取稳定知识
```

用户需要能回答三个问题：

1. 这次需求沉淀了哪些可复用知识？
2. 哪些已经被我批准为稳定知识，哪些被拒绝？
3. 下次相似需求开始时，Agent 是否真的看到了这些稳定知识？

## 3. 已确认的 MewCode 复用结论

### 3.1 MewCode 已有能力

MewCode 已经具备：

- 项目级记忆目录：`.mewcode/memory/`。
- 用户级记忆目录：`~/.mewcode/memory/`。
- 入口索引文件：`MEMORY.md`。
- 单条记忆文件 frontmatter：`name`、`description`、`type`。
- 四类记忆：`user`、`feedback`、`project`、`reference`。
- 记忆扫描、frontmatter 解析和相关记忆选择器。
- 记忆提取 prompt，用于把对话信息写成结构化记忆文件。

### 3.2 Devflow 已迁移的能力

Devflow 当前已有：

- `internal/runtime/memory`：从 MewCode 迁来的运行时记忆能力，路径已改为 `.devflow/memory/` 和 `~/.devflow/memory/`。
- `internal/memory`：Devflow 产品层的候选记忆搜索，目前搜索 `.devflow/demands/<demand-id>/memory-candidates.md`。
- `internal/demandflow`：在 requirements / plan prompt 中渲染可复用 memory hits。

因此 Wave 12 应复用 Devflow 已迁入的 runtime memory 格式和路径规则，不再复制原 MewCode `internal/memory`。

## 4. 核心产品规则

### 4.1 候选知识和稳定知识必须分开

`memory-candidates.md` 是候选知识，不能直接等同于稳定知识。它可以被搜索和展示，但必须标记为 candidate。

`.devflow/memory/*.md` 是稳定知识，必须来自人工确认的 promote 动作。只有稳定知识才能在后续需求中以 approved / stable 语义展示。

### 4.2 Wave 12 只做项目级稳定知识

Wave 12 只写项目级记忆：

```text
.devflow/memory/
```

不写用户级记忆：

```text
~/.devflow/memory/
```

原因是 `memory-candidates.md` 来自具体需求过程，默认属于当前项目，不应自动跨项目影响用户。

### 4.3 人工确认是唯一晋升入口

稳定知识不能由 closeout 自动写入。Agent 可以生成候选，用户或流程只能通过显式 promote 才能把候选写入 `.devflow/memory/`。

拒绝同样需要记录。被 reject 的候选不会写入稳定记忆，但审计中应能看到它为什么没有被采用。

## 5. 用户命令

Wave 12 增加 `devflow memory` 子命令。

### 5.1 列出候选

```text
devflow memory list --demand <demand-id>
```

行为：

- 读取 `.devflow/demands/<demand-id>/memory-candidates.md`。
- 将候选解析成编号列表。
- 显示候选内容、来源文件、当前决策状态。
- 如果候选文件不存在，返回清晰错误，不创建空文件。

### 5.2 晋升候选

```text
devflow memory promote --demand <demand-id> --candidate <n> --by <name>
```

可选参数：

```text
--name <stable-name>
--description <one-line-description>
```

行为：

- 按编号读取候选。
- 生成项目级稳定记忆文件。
- 更新 `.devflow/memory/MEMORY.md` 索引。
- 写入需求事件：`memory.promoted`。
- 输出稳定记忆文件路径。

### 5.3 拒绝候选

```text
devflow memory reject --demand <demand-id> --candidate <n> --by <name> --reason <text>
```

行为：

- 记录需求事件：`memory.rejected`。
- 不写入 `.devflow/memory/`。
- 后续 `memory list` 能显示该候选已被拒绝。

## 6. 稳定记忆文件格式

稳定记忆复用 `internal/runtime/memory` 的 frontmatter 格式，并允许增加审计字段。

```markdown
---
name: coupon-eligibility-policy
description: Coupon eligibility must check active membership before applying discount rules.
type: project
source_demand: add-coupon-check
promoted_at: 2026-06-30T10:30:00+08:00
promoted_by: dd
---

# Coupon eligibility policy

Coupon eligibility must check active membership before applying discount rules.

Why: This rule was confirmed during demand add-coupon-check.

How to apply: Reuse this rule when generating requirements or plans for coupon, discount, membership, or order pricing work.
```

`internal/runtime/memory` 目前只需要 `name`、`description`、`type`，额外字段应保持兼容。

`MEMORY.md` 索引保持一行一个入口：

```markdown
- [Coupon eligibility policy](coupon-eligibility-policy.md) - membership gate for coupon eligibility.
```

## 7. 数据流

### 7.1 当前需求结束

```text
closeout stage
-> 生成 memory-candidates.md
-> 用户运行 devflow memory list
-> 用户 promote / reject
-> 事件流记录决策
```

### 7.2 下一个需求开始

```text
requirements / plan context loader
-> 搜索 .devflow/memory/*.md 稳定知识
-> 搜索历史需求 memory-candidates.md 候选知识
-> prompt 中先渲染 stable，再渲染 candidate
```

稳定知识必须优先于候选知识展示。候选知识需要明确标注为未确认，避免 Agent 把未批准内容当成事实。

## 8. 代码边界

### 8.1 `internal/runtime/memory`

继续负责通用运行时记忆能力：

- `.devflow/memory/` 路径规则。
- `MEMORY.md` 入口。
- frontmatter 格式。
- 记忆扫描和相关记忆选择。
- 兼容旧 MewCode 环境变量 fallback。

Wave 12 可以复用这里的类型、路径函数和扫描能力，但不让 runtime memory 决定需求阶段状态。

### 8.2 `internal/memory`

作为 Devflow 产品层 memory store，负责：

- 解析需求级 `memory-candidates.md`。
- 读取候选的 promote / reject 决策。
- 写入项目级稳定记忆文件。
- 更新 `.devflow/memory/MEMORY.md`。
- 搜索稳定知识和候选知识。
- 保持路径安全，禁止越过 `.devflow`。

### 8.3 `internal/demandflow`

负责在需求上下文中使用 memory：

- 扩展 `MemoryHit`，区分 `stable` 和 `candidate`。
- requirements / plan prompt 中明确分区渲染。
- 当前需求自己的候选不应作为可复用知识注入当前需求。

### 8.4 `internal/cli`

负责 `devflow memory` 命令：

- 参数解析。
- 人类可读输出。
- 错误码。
- 与 store / demand event log 的连接。

## 9. 候选解析规则

Wave 12 采用确定性解析，不调用 LLM。

默认规则：

- 优先读取标题包含 `稳定知识候选`、`Stable Knowledge Candidates` 或 `Memory Candidates` 的段落。
- 段落下每个非空 bullet 是一个候选。
- 如果没有匹配标题，则读取整个文件中的顶层 bullet。
- 候选编号从 1 开始。
- 空文件返回 `no memory candidates found`。

这保持 MVP 可测、可解释，也避免 closeout 文案变化导致 Agent 自动误读。

## 10. 错误处理

- demand 不存在：返回 `demand not found`。
- `memory-candidates.md` 不存在：返回 `memory candidates not found`。
- candidate 编号越界：返回 `candidate index out of range`。
- promote 目标文件名冲突：使用确定性后缀 `<slug>-<demand-id>.md`，避免覆盖旧知识。
- `.devflow/memory/` 下存在 symlink 或 reparse point 风险：拒绝写入并返回路径安全错误。
- `MEMORY.md` 更新失败：不留下半写文件；promote 整体失败。
- reject 缺少 reason：返回参数错误。

## 11. 测试策略

### 11.1 单元测试

- candidate parser 能解析中文标题、英文标题和 fallback bullets。
- promote 会生成稳定记忆文件和 `MEMORY.md` 索引。
- promote 文件名冲突时不会覆盖已有文件。
- reject 会记录事件，不会写稳定记忆。
- stable search 能读取 `.devflow/memory/*.md`。
- context loader 会把 stable 排在 candidate 前面。
- 当前 demand 的候选不会回灌到当前 demand。

### 11.2 CLI 测试

- `devflow memory list --demand <id>` 输出编号候选。
- `devflow memory promote --candidate 1` 输出稳定文件路径。
- `devflow memory reject --reason ...` 后 list 能看到 rejected 状态。
- 缺失文件和越界编号给出稳定错误。

### 11.3 Dogfood 测试

构造两个需求：

1. 第一个需求生成 `memory-candidates.md` 并 promote 一条稳定知识。
2. 第二个相似需求启动 requirements / plan。
3. 断言 prompt / context 中出现 promoted stable memory。
4. 断言未 promote 的候选只以 candidate 语义出现，或在没有候选搜索时不出现。

## 12. 非目标

Wave 12 不做：

- 向量数据库。
- 自动 LLM 晋升稳定知识。
- 用户级 `~/.devflow/memory/` 写入。
- 跨项目同步。
- 记忆编辑和删除 UI。
- GitLab 评论自动沉淀为稳定知识。
- 用 Eino 重写 workflow。
- 替换 `internal/runtime/memory` 的 MewCode 记忆格式。

## 13. 完成标准

Wave 12 完成时需要满足：

- 用户能列出某个 demand 的 memory candidates。
- 用户能把候选 promote 成 `.devflow/memory/*.md` 稳定知识。
- 用户能 reject 候选并保留审计记录。
- `MEMORY.md` 索引被正确维护。
- 下一个需求的 requirements / plan 上下文能读取 stable memory。
- prompt 明确区分 stable memory 和 candidate memory。
- 所有行为有自动化测试覆盖。
- `go test ./...`、`go vet ./...`、`go build ./cmd/devflow` 通过。

## 14. 实施原则

Wave 12 是对 MewCode memory 的产品化接线，不是新的 memory 平台。

实现时应遵守：

- 优先复用 `internal/runtime/memory` 的路径和文件格式。
- 保持候选和稳定知识的语义边界。
- 每个 promote / reject 都进入需求审计。
- 不让未确认候选伪装成稳定知识。
- 不把用户级长期偏好和项目级需求知识混在一起。
- 先做确定性 MVP，再考虑 LLM 辅助提取或相关性排序。
