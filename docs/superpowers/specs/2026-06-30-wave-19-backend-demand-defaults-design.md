# Wave 19 Backend Demand Defaults 设计

## 1. 背景

Wave 15 到 Wave 18 让 Devflow 的操作者闭环更完整：

- `run` 执行单阶段；
- `console --run-next` 执行下一个安全阶段；
- `drive` 自动推进到人工门；
- `evaluate` 检查产物质量；
- `workbench` 提供 TUI 和 snapshot；
- `dogfood --operator-loop` 验证 operator surface。

但真实使用时，用户还需要反复输入这些参数：

```powershell
--permission-mode acceptEdits
--quality-command "go test ./... -count=1 -timeout 5m"
--gitlab-project "group/project"
--gitlab-base-url "https://gitlab.example"
--create-mr-target-branch main
```

这会让 `drive` 和 `workbench` 的价值打折。Wave 19 要把这些“项目级稳定默认值”沉到 `.devflow/config.yaml`，让日常命令更短，同时保留 CLI flags 的显式覆盖能力。

## 2. 产品目标

在配置文件中增加 backend demand 默认值：

```yaml
backend_demand:
  runner_root: .
  quality_root: .
  quality_commands:
    - go test ./... -count=1 -timeout 5m
  permission_mode: acceptEdits
  gitlab:
    project: group/project
    base_url: https://gitlab.example
    default_target_branch: main
```

用户之后可以运行：

```powershell
devflow run --demand add-coupon --stage implementation
devflow drive --demand add-coupon
devflow console --demand add-coupon --run-next
devflow workbench --quality-command "go test ./internal/dogfood"
```

预期行为：

- 如果 CLI 没传 `--quality-command`，使用配置默认值；
- 如果 CLI 没传 `--permission-mode`，使用配置默认值；
- 如果 CLI 没传 `--quality-root` 或 `--runner-root`，使用配置默认值；
- 如果 CLI 没传 `--gitlab-project` 或 `--gitlab-base-url`，使用配置默认值；
- 如果 create MR 没传 target branch，使用配置默认 target branch；
- CLI 显式 flag 永远优先于配置；
- 没有配置时保持现有行为。

## 3. 范围

Wave 19 做：

- 扩展 runtime config schema；
- 增加 backend demand defaults resolver；
- `devflow run` 读取默认值；
- `devflow console --run-next` 读取默认值；
- `devflow drive` 读取默认值；
- `devflow workbench` action shortcuts 读取默认值；
- `devflow doctor` 显示 backend demand defaults 是否可用；
- `devflow init` 生成带注释或默认段的配置；
- 文档、例子、release notes。

Wave 19 不做：

- 多 profile 切换，例如 `--profile prod`；
- team/user 级权限策略；
- 自动发现 GitLab MR iid；
- GitLab comment writeback；
- 修改 provider/model 配置语义；
- Eino 编排。

## 4. 配置模型

新增配置结构：

```go
type BackendDemandConfig struct {
	RunnerRoot          string       `yaml:"runner_root"`
	QualityRoot         string       `yaml:"quality_root"`
	QualityCommands     []string     `yaml:"quality_commands"`
	PermissionMode      string       `yaml:"permission_mode"`
	GitLab              GitLabConfig `yaml:"gitlab"`
	CreateMRTargetBranch string      `yaml:"create_mr_target_branch"`
}

type GitLabConfig struct {
	Project             string `yaml:"project"`
	BaseURL             string `yaml:"base_url"`
	DefaultTargetBranch string `yaml:"default_target_branch"`
}
```

Recommended field name is `backend_demand`, not `demandflow`, because the user-facing product is “后端业务需求专家 Agent” and future frontend/test/PD agents may have separate defaults.

## 5. Resolver

Create `internal/cli/demand_defaults.go`.

The resolver takes:

- config path;
- existing CLI option values;
- command context such as root and stage.

It returns a normalized struct used by CLI commands:

```go
type demandCommandDefaults struct {
	RunnerRoot          string
	QualityRoot         string
	QualityCommands     []string
	PermissionMode      string
	GitLabProject       string
	GitLabBaseURL       string
	CreateMRTargetBranch string
}
```

Rules:

1. Load config from `--config` if passed.
2. If config is missing, return empty defaults without failing. This preserves current CLI behavior for demand commands.
3. If config exists but is invalid, return the config error.
4. CLI flags override config defaults.
5. Repeated `--quality-command` replaces config quality commands, rather than appending.
6. Empty strings are ignored.

## 6. Command Integration

### 6.1 `devflow run`

Apply defaults before building `demandflow.Options`:

- `RunnerRoot`;
- `QualityRoot`;
- `PermissionMode`;
- `QualityCommands`;
- GitLab review fields;
- create MR target branch.

Implementation and verification stages should get default quality commands. Requirements/plan/closeout can carry them harmlessly but do not need to run them.

### 6.2 `devflow console --run-next`

Console currently parses run-ready command strings and preserves default `--permission-mode acceptEdits` / `--quality-command "go test ./..."` from `NextAction`. Wave 19 should layer config defaults before falling back to generated command defaults.

Order:

1. CLI flags;
2. config backend_demand defaults;
3. generated action command flags.

### 6.3 `devflow drive`

Drive constructs `consoleArgs`. It should resolve defaults once at command start and pass them into each stage run. Explicit drive flags override config.

### 6.4 `devflow workbench`

Workbench action shortcuts call `runConsoleNext`, `runDrive`, and `runEvaluate`. Workbench should apply config defaults when it builds those calls, so pressing `r` or `d` behaves like the CLI commands.

`workbench --snapshot` stays read-only. It can load config only for consistency but should not require config.

## 7. Doctor

`devflow doctor` should include a backend demand defaults check:

```text
[OK] backend-demand: quality command defaults configured
```

If no defaults exist:

```text
[OK] backend-demand: no defaults configured; CLI flags remain required
```

If defaults are invalid, doctor should fail with a clear message, for example:

```text
[FAIL] backend-demand: quality_commands[0] must contain a program
```

## 8. Backward Compatibility

- Existing `.devflow/config.yaml` files without `backend_demand` remain valid.
- Existing CLI commands with explicit flags keep the same behavior.
- Existing tests that expect missing GitLab flags for MR review should still pass when no config default exists.
- Provider config validation remains unchanged.
- Secret values are not printed.

## 9. Testing

Required tests:

- config loads `backend_demand`;
- config merge overrides backend demand fields from local config;
- resolver returns empty defaults when no config exists;
- resolver returns parsed quality commands when config exists;
- CLI flags override config defaults;
- `run --stage implementation` uses config quality command and permission mode;
- `run --stage mr-review` uses config GitLab project/base URL;
- `console --run-next` uses config defaults;
- `drive` uses config defaults;
- `workbench` shortcuts pass config defaults;
- `doctor` prints backend-demand status;
- `init` output includes backend_demand section.

## 10. Completion Criteria

Wave 19 is complete when:

- `.devflow/config.yaml` supports `backend_demand`;
- demand CLI commands can run with fewer repeated flags;
- explicit flags override config;
- no-config behavior remains compatible;
- docs explain the config section and precedence;
- `go test ./... -count=1 -timeout 5m` passes;
- `go vet ./...` passes;
- `go build ./cmd/devflow` passes;
- `git diff --check` passes;
- PR CI passes on Ubuntu and Windows.
