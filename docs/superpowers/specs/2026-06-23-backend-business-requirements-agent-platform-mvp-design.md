# 后端业务需求专家 Agent：平台型 MVP 设计

## 1. 背景与动机

参考文章：[《如何搭建一个端到端业务需求专家 Agent》](https://mp.weixin.qq.com/s/9o_z-POj9r4dbwe3NlC1pw)。

文章的核心判断是：AI 写代码本身已经不是业务研发的最大瓶颈，真正耗时的是需求从进入到上线结项之间的串联成本。一个后端业务需求通常要经历需求澄清、技术方案、实现、代码评审、验收取证、发布观察和结项沉淀。每个环节 AI 都能参与一部分，但如果这些能力散落在不同对话和工具里，最终仍然由人负责组织上下文、切换平台、判断下一步、记录反馈。

本产品要解决的问题是：把后端业务需求交付过程组织成一个可审计、可暂停、可恢复、可复用的 Agent 闭环。第一版不是做一个大而全的企业研发平台，而是用可扩展的研发流程编排底座，先跑通后端业务需求的完整交付路径。

## 2. 产品定位

**一句话定位：**

后端业务需求专家 Agent 是一个面向企业后端研发团队的需求交付编排系统。它围绕一个后端需求持续维护阶段产物、执行质量门、协同 MR 评论、沉淀过程记忆，让需求从输入到结项形成闭环。

**第一版目标：**

用平台型底座跑通一条后端业务需求瘦闭环：

```text
需求文本/PRD 链接
-> requirements.md
-> plan.md
-> 后端实现与测试
-> GitLab MR 协同
-> verification.md
-> closeout.md
-> 稳定知识候选
-> 下次复用
```

**第一版不是：**

- 不是通用聊天机器人。
- 不是单纯代码生成器。
- 不是全自动发布系统。
- 不是一开始就深度接入所有企业平台。
- 不是自动把过程材料写入长期知识库的无人审核系统。

## 3. 目标用户与使用场景

### 3.1 主要用户

第一版主要服务后端研发团队中的一线开发者和技术负责人。

一线开发者使用 Agent 承接具体需求，减少重复的需求整理、方案撰写、测试记录、MR 评论处理和结项总结工作。

技术负责人使用 Agent 形成统一质量门，确保 requirements、plan、verification、closeout 等关键产物可审阅、可追溯。

### 3.2 典型场景

研发同学拿到一个后端业务需求，将需求描述或 PRD 链接交给 Agent。Agent 先整理上下文，生成 requirements。人工确认 requirements 后，Agent 生成技术 plan。人工确认 plan 后，Agent 进入实现阶段，修改代码、补测试、运行质量门，并创建或更新 GitLab MR。后续 Agent 读取 unresolved MR 评论，修复问题、回复评论，并在完成后输出 verification 和 closeout。结项时，Agent 从过程材料中提炼稳定知识候选，供人工审核后进入长期知识库或项目记忆。

## 4. 设计原则

1. **闭环完整，集成克制**
   第一版保留需求交付全流程，但每个集成点只做到验证闭环所需的最小深度。

2. **底座真实，能力可替换**
   workflow 状态机、阶段产物、人工确认门、adapter 接口、project memory、quality gate、skill 编排都按长期平台设计，不写死在单个 demo 流程里。

3. **人工确认是质量门，不是失败**
   Agent 不追求每一步无人值守。需求语义、技术方案、主预发/线上发布、长期知识沉淀都应该保留明确人工确认点。

4. **证据优先于口头结论**
   每个阶段的推进都要留下产物或证据。测试是否通过、评论如何处理、哪些知识进入候选，都要可回读。

5. **项目记忆先于长期知识**
   过程材料先进入项目记忆，不直接污染长期 wiki。长期知识必须经过结项筛选和人工确认。

## 5. MVP 范围

### 5.1 第一版必须支持

- 从需求文本或 PRD 链接启动一个需求工作流。
- 生成并维护 `requirements.md`。
- 生成并维护 `plan.md`。
- 在人工确认后进入实现阶段。
- 在一个后端仓库中执行代码修改与测试。
- 创建或更新 GitLab MR。
- 读取 GitLab MR unresolved 评论。
- 根据评论更新文档或代码。
- 记录测试结果与人工验收证据。
- 生成 `verification.md`。
- 生成 `closeout.md`。
- 生成稳定知识候选与流程改进候选。
- 在下一个需求中读取项目记忆。

### 5.2 第一版轻量支持

- 钉钉或飞书阶段通知。
- 人工确认入口的消息回刷。
- CI 状态读取。
- 手动录入外部验收证据，例如接口调用结果、日志截图或监控链接。

### 5.3 第一版暂不做深

- 不深接需求平台状态流。
- 不自动解析所有 PRD 文档格式。
- 不自动部署主预发或线上环境。
- 不自动执行生产发布。
- 不深接日志平台、监控平台、配置平台。
- 不做跨前端、测试、PD 的多 Agent 团队协同。
- 不自动写长期 wiki。

## 6. 核心流程

### 6.1 需求进入

输入来源：

- 用户手工输入需求文本。
- 用户提供 PRD 链接。
- 用户提供 issue 或需求单链接。

第一版处理策略：

- 需求文本直接进入上下文。
- PRD 链接先作为引用记录；如果无法自动读取，要求用户粘贴关键内容。
- 需求平台链接先作为 metadata，不强依赖平台 API。

输出：

- `demand.json`：需求来源、标题、链接、发起人、时间、当前状态。
- 初始上下文快照：需求材料、相关代码入口、已有项目记忆。

### 6.2 Requirements 阶段

Agent 生成 `requirements.md`，用于回答“到底要做什么”。

建议结构：

```markdown
# Requirements

## 目标行为
## 非目标范围
## 业务规则
## 用户/调用方影响
## 验收标准
## 风险与歧义
## 待确认问题
## 人工确认记录
```

质量门：

- requirements 未确认前，不进入 plan。
- 如果存在待确认问题，Agent 必须停在该阶段。
- 人工确认需要记录确认人、时间和确认摘要。

### 6.3 Plan 阶段

Agent 基于确认后的 requirements 生成 `plan.md`，用于回答“怎么改、怎么验、怎么回滚”。

建议结构：

```markdown
# Technical Plan

## 当前实现与代码事实
## 目标设计
## 改动范围
## 数据结构/API/配置变化
## 测试策略
## 验收方式
## 风险与回滚
## 不做事项
## 人工确认记录
```

质量门：

- plan 未确认前，不进入实现。
- plan 必须包含测试策略和验收方式。
- 涉及高风险改动时，必须显式记录回滚方式。

### 6.4 实现与测试阶段

Agent 在确认后的 plan 范围内修改代码、补测试、运行质量门。

第一版质量门：

- 格式化检查。
- 单元测试，例如 Go 项目中的 `go test ./...`。
- diff-to-test 映射说明：每个关键行为改动对应哪个测试或验证证据。
- Agent 自检 review：检查明显逻辑问题、遗漏测试、越权改动。

输出：

- 代码 diff。
- 测试结果。
- `progress.md` 阶段记录。

### 6.5 GitLab MR 协同阶段

Agent 创建或更新 MR，并围绕 unresolved 评论工作。

第一版能力：

- 创建 MR 或复用已有 MR。
- 读取 MR 评论。
- 区分评论类型：需求语义、技术方案、代码实现、测试缺失、风格问题。
- 对代码/文档评论执行修改。
- 对需求语义或范围变更评论，回退到 requirements 或 plan 阶段。
- 回复评论，说明处理方式和验证结果。

质量门：

- unresolved 阻塞评论未处理前，不进入 verification。
- 如果评论导致需求范围变化，必须重新确认 requirements 或 plan。

### 6.6 Verification 阶段

Agent 生成 `verification.md`，用于证明需求是否完成。

建议结构：

```markdown
# Verification

## 验收标准映射
## 自动化测试结果
## 手动验证记录
## 接口/日志/监控证据
## 未覆盖风险
## 结论
```

第一版支持：

- 自动记录本地测试或 CI 结果。
- 手动粘贴接口调用结果。
- 手动粘贴日志、监控、截图链接。
- 对每条验收标准标记 pass/fail/blocked。

暂不做：

- 自动连接 SLS、Prometheus、链路追踪、发布平台。

### 6.7 Closeout 阶段

Agent 生成 `closeout.md`，用于结项沉淀。

建议结构：

```markdown
# Closeout

## 需求结果
## 关键产物链接
## MR 评论与处理摘要
## 验收证据摘要
## 稳定知识候选
## 流程改进候选
## 一次性材料归档
## 人工确认记录
```

结项输出分三类：

- 稳定知识候选：业务规则、系统边界、排障经验、验收规范。
- 流程改进候选：反复出现的澄清问题、测试遗漏、评论模式、skill/prompt 改进点。
- 一次性归档材料：临时调试记录、一次性上下文、中间状态。

质量门：

- 稳定知识候选必须人工确认后，才允许进入长期知识库。
- 流程改进候选必须人工确认后，才允许改 skill 或 prompt。

## 7. 平台底座设计

### 7.1 Workflow 状态机

核心状态：

```text
created
-> context_loaded
-> requirements_drafting
-> requirements_review
-> plan_drafting
-> plan_review
-> implementation
-> mr_review
-> verification
-> closeout
-> completed
```

异常状态：

```text
blocked_need_user
blocked_need_platform
failed_quality_gate
returned_to_requirements
returned_to_plan
cancelled
```

状态机职责：

- 记录当前阶段。
- 决定下一步可执行动作。
- 阻止跳过人工确认门。
- 支持从 MR 评论回退到 requirements 或 plan。
- 支持失败恢复和重试。

### 7.2 阶段产物标准

每个需求工作区至少包含：

```text
demand.json
requirements.md
plan.md
progress.md
verification.md
closeout.md
memory-candidates.md
events.jsonl
```

产物要求：

- 所有阶段产物可人工审阅。
- 所有重要状态变化写入 `events.jsonl`。
- 所有人工确认记录写入对应阶段文件。
- Agent 的推理摘要和真实证据分开记录，避免把猜测当事实。

### 7.3 人工确认门

第一版确认门：

- requirements 确认。
- plan 确认。
- 高风险 MR 评论处理确认。
- verification 结论确认。
- closeout 知识候选确认。

确认方式：

- CLI 中确认。
- 钉钉/飞书消息中确认。
- MR 评论中确认。

第一版可以先支持 CLI 确认，IM 确认作为轻量增强。

### 7.4 Adapter 接口

Adapter 分层：

- Code adapter：Git 操作、分支、diff、commit。
- Review adapter：GitLab MR、评论、CI 状态。
- IM adapter：钉钉/飞书通知、确认入口。
- Demand adapter：需求平台、PRD、issue。
- Evidence adapter：测试、日志、监控、接口调用。
- Knowledge adapter：项目记忆、长期 wiki。

第一版优先级：

1. Code adapter
2. Review adapter for GitLab
3. Local test evidence adapter
4. IM notification adapter
5. File-based project memory adapter

### 7.5 Project Memory

项目记忆用于保存需求过程材料和可复用上下文。

第一版记忆来源：

- 历史 `requirements.md`
- 历史 `plan.md`
- 历史 `verification.md`
- 历史 `closeout.md`
- 人工确认后的稳定知识候选

第一版记忆读取策略：

- 根据需求关键词、代码路径、业务模块检索相关历史材料。
- 只把摘要和相关片段注入当前上下文。
- 避免把未确认候选直接当成事实。

### 7.6 Quality Gate

质量门分两类。

硬门禁：

- requirements 未确认不能进入 plan。
- plan 未确认不能实现。
- 测试失败不能进入 verification。
- 阻塞 MR 评论未处理不能结项。

软门禁：

- Agent 自检 review。
- diff-to-test 映射完整性。
- 验收证据充分性。
- closeout 知识候选质量。

### 7.7 Skill 编排

每个阶段对应独立 skill 或阶段处理器：

- clarify：生成和修订 requirements。
- plan：生成和修订技术方案。
- execute：按 plan 修改代码和测试。
- review-sync：处理 MR 评论。
- verify：生成验收证据。
- closeout：沉淀过程材料。

编排原则：

- skill 不直接决定跨阶段跳转，跳转由 workflow 状态机控制。
- skill 只能读写自己负责的阶段产物，跨阶段修改必须通过状态机授权。
- 所有工具调用结果都要进入事件日志。

## 8. 系统架构映射

产品架构对应文章里的四层。

### 8.1 上下文输入层

负责组织：

- 当前需求材料。
- PRD 或 issue 链接。
- 代码事实。
- 历史项目记忆。
- 人工补充信息。

### 8.2 业务专家编排层

负责判断：

- 当前需求处于哪个阶段。
- 是否允许进入下一阶段。
- 是否需要人工确认。
- MR 评论是否触发阶段回退。
- 失败后应该重试、回退还是阻塞。

### 8.3 工具执行层

负责执行：

- Git 操作。
- 测试命令。
- GitLab MR 与评论。
- IM 通知。
- 后续日志、监控、发布等平台适配。

### 8.4 反馈学习层

负责沉淀：

- MR 评论模式。
- 验收证据。
- 人工修正。
- 稳定知识候选。
- skill/prompt 改进候选。

## 9. 成功标准

第一版成功不以“完全无人参与”为标准，而以“闭环是否可运行、可审计、可复用”为标准。

核心指标：

- 一个后端需求能完整走完所有阶段。
- requirements 和 plan 都有人工确认记录。
- 每个关键代码改动都有测试或验证证据。
- MR 评论能进入处理闭环。
- verification 能映射验收标准。
- closeout 能产出稳定知识候选。
- 下一个需求能读取上一个需求的确认后记忆。

效率指标：

- 人工整理 requirements 的时间下降。
- 技术方案返工次数下降。
- MR 评论处理遗漏率下降。
- 验收证据补写成本下降。
- 重复业务解释次数下降。

质量指标：

- 测试失败时不会继续推进。
- 未确认 requirements 不会进入 plan。
- 未确认 plan 不会进入实现。
- 未处理阻塞评论不会结项。
- 未审核知识候选不会进入长期知识库。

## 10. 风险与应对

### 10.1 范围过大

风险：第一版同时做深所有平台接入，导致产品价值尚未验证就陷入集成工程。

应对：坚持“底座真实，集成克制”。第一版深做 GitLab 和本地测试，其他平台以 adapter 和手动证据形式占位。

### 10.2 Agent 跳过质量门

风险：只靠提示词约束时，Agent 可能跳过确认、测试或评论处理。

应对：把确认门和质量门写进 workflow 状态机和命令执行规则，而不是只写在 prompt 里。

### 10.3 长期知识污染

风险：过程中的临时信息、错误判断、未确认结论被写入长期知识库。

应对：项目记忆和长期知识分层；closeout 只生成候选，人工确认后才进入长期知识。

### 10.4 平台差异过大

风险：不同公司使用 GitLab、云效、TAPD、禅道、Aone、飞书、钉钉等组合，字段和权限差异大。

应对：第一版只绑定一个 Review adapter，其他系统通过 adapter 接口抽象，后续按客户环境扩展。

### 10.5 验收证据不充分

风险：verification 只记录测试通过，无法证明业务正确。

应对：verification 必须按验收标准映射证据；第一版允许手动粘贴外部证据，后续再接日志和监控平台。

## 11. 版本路线

### v0.1：平台型 MVP

- File-based project memory。
- Workflow 状态机。
- Markdown 阶段产物。
- CLI 人工确认。
- Git/GitLab MR 基础协同。
- 本地测试证据。
- closeout 知识候选。

### v0.2：企业协作增强

- 钉钉/飞书通知与确认。
- CI 状态读取。
- MR 评论自动分类增强。
- 需求平台只读接入。
- 更强的历史记忆检索。

### v0.3：验收取证增强

- 接口调用证据。
- 日志查询 adapter。
- 监控指标 adapter。
- 验收报告自动生成。

### v0.4：长期知识闭环

- 长期 wiki adapter。
- 知识候选审核流。
- skill/prompt 改进候选管理。
- 跨需求效果度量。

### v1.0：多 Agent 团队扩展

- 后端 Agent。
- 前端 Agent。
- 测试 Agent。
- PD Agent。
- 队长/编排 Agent。
- 多产物对齐与跨角色质量门。

## 12. 与 MewCode 的关系

MewCode 已经是一个 Go 实现的 AI Coding Agent 基座，包含 LLM provider、MCP server、tool registry、权限检查、skill 机制、memory、subagent/worktree、TUI 等能力。Devflow Agent 不应该重新发明这些底层 Agent 执行能力。

Devflow Agent 与 MewCode 的推荐关系是：

- `devflow-agent` 是产品仓库，负责后端业务需求交付闭环、workflow 状态机、阶段产物、adapter、quality gate 和 project memory。
- `mewcode` 是可复用/可借鉴的 Agent 执行内核，优先复用它的 LLM、tool、skill、memory、subagent、worktree 等能力。
- v0.1 不直接在 MewCode 仓库里做产品逻辑，避免把业务需求交付平台和通用 Coding Agent 混在一起。
- 如果复用成本低，可以把 MewCode 中稳定的内部包提取为 devflow-agent 的内部模块或 library dependency。
- 如果提取成本高，v0.1 可以先参考 MewCode 的实现方式，在 devflow-agent 中实现最小必要的 workflow、artifact 和 adapter 能力。

因此，MewCode 在本项目中的定位不是被忽略，也不是被整个替换为 Eino，而是作为 Devflow Agent 的底层 Agent 工程参考和潜在执行内核。

## 13. 默认决策

第一版采用如下默认决策，除非后续用户明确覆盖：

- Review adapter 选择 GitLab，不同时适配 GitHub、云效 Codeup 或其他代码平台。
- IM adapter 先抽象为通知接口，默认不绑定钉钉或飞书的复杂审批能力。
- MR 能力优先支持“已有 MR 评论处理”，随后支持由 Agent 创建 MR。
- 第一版落在 `devflow-agent` 仓库中，以独立产品仓库方式推进；MewCode 作为可复用/可借鉴的 Agent 执行内核。
- 项目记忆第一版使用本地文件，不单独建立 memory repo。
- 人工确认第一版使用 CLI，IM 确认放到 v0.2。
- closeout 只生成知识候选，不自动写长期 wiki。

这些决策的目的不是缩小产品愿景，而是让 v0.1 能在本地和单个代码平台上验证完整闭环。后续一旦闭环跑通，再逐步加深企业平台集成。
