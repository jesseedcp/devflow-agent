# MewCode Source Migration Manifest

The user confirmed authority to copy and modify the local MewCode snapshot.
Snapshot root: `D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang`
Snapshot date: `2026-06-25`

Git commits cannot embed their own final SHA. Each package entry records the
unique Lore intent line of its migration commit; resolve the SHA with
`git log --format="%H %s" --grep="<intent line>"`.

## Packages

### conversation

- Source: `internal/conversation`
- Target: `internal/runtime/conversation`
- Source files:
  - `conversation.go`: `9FC69554F3B713D3B57EB987D768AC229FF1BCFE3C999A04B71A35838BB36958`
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - Replace the injected `mewcodeMd` identity heading with `devflowMd`.
  - Add focused regression tests for deep isolation and protocol behavior across caller-owned inputs, `GetMessages` outputs, Anthropic serialization, and OpenAI serialization.
  - Replace the narrow JSON-like copy path with a reflective deep-copy guard that preserves concrete map/slice types and nil values while keeping external mutations from polluting stored history.
- Windows changes: none required; the package uses platform-neutral Go APIs.
- Verification: `gofmt -w internal/runtime/conversation/conversation.go internal/runtime/conversation/conversation_test.go`; `go test ./internal/runtime/conversation -count=1`; `go test ./... -count=1`; `go vet ./...`; `git diff --check`
- Lore intent: `Preserve conversation semantics inside the Devflow runtime`

### hooks

- Source: `internal/hooks`
- Target: `internal/runtime/hooks`
- Source files:
  - `hooks.go`: `C7127D660853A1C0DC0223FFEB9460FCACDAF684B312AE3A23B8237FF5FA097B`
  - `hooks_test.go`: `CC0436CC9226E175C0A6EEEDD6C72C32788555D8906088884BDB2BFF3B8D461B`
- Fusion changes:
  - Move the package under the Devflow runtime boundary without changing the hook engine API.
  - Split shell process creation into OS-specific `command_windows.go` and `command_unix.go` helpers so tests can verify the selected host shell directly.
  - Export `DEVFLOW_EVENT`, `DEVFLOW_TOOL`, and `DEVFLOW_FILE_PATH` while preserving the `MEWCODE_*` aliases for migration compatibility.
  - Adapt the migrated tests with build-tag helpers so command, timeout, and env assertions are reliable on native Windows instead of depending on Git Bash semantics.
  - Normalize hook glob comparisons to slash-separated paths so the same pattern matches both `src/foo.go` and `src\foo.go`.
  - Clone async hook `HookContext` values, including JSON-like nested `ToolArgs`, so caller mutations after `RunHooks` do not leak into background actions.
- Windows changes:
  - Use `powershell.exe -NoLogo -NoProfile -NonInteractive -Command` for hook commands on supported Windows hosts.
  - Reject any implicit Git Bash dependency; Unix-like targets prefer `bash -c` when Bash is installed and fall back to POSIX `sh -c` when it is not.
- Remaining limitations:
  - Detached descendants are outside the current hook timeout guarantee on both Windows and Unix-like hosts.
- Verification: `gofmt -w internal/runtime/hooks/*.go`; `go test ./internal/runtime/hooks -count=1 -timeout 2m`; `GOOS=linux go test -c ./internal/runtime/hooks -o <temp binary>`; `go test ./... -count=1 -timeout 2m`; `go vet ./...`; `git diff --check`
- Lore intent: `Make runtime hooks executable on the supported Windows host`

### config

- Source: `internal/config`
- Target: `internal/runtime/config`
- Source files:
  - `config.go`: `DCC2376B6382AE0972B7E04B991D0378056361B45368E4816367F0D5DDA12B09`
- Fusion changes:
  - Move configuration under the Devflow runtime boundary and retarget hook validation to `internal/runtime/hooks`.
  - Make `.devflow` authoritative for user, project, and project-local scopes, and only fall back to matching legacy `.mewcode` files when the Devflow file is absent in that same scope.
  - Preserve layer order as user -> project -> local override, with providers overriding by slice replacement, MCP servers overriding by name or appending when new, and hooks appending later layers after earlier layers.
  - Preserve explicit API key precedence over environment variables and validate hook definitions during both explicit-path and discovered loads through one shared config-validation path.
  - Validate each discovered layer before merge so duplicate or unnamed MCP servers, duplicate provider names, invalid provider definitions, and invalid hooks fail at the file that introduced them instead of being silently folded during merge.
  - Reject duplicate provider names in the final providers slice, reject duplicate or unnamed MCP servers in a single final layer, and keep cross-layer same-name MCP servers legal as explicit overrides.
  - Allow hook-only or MCP-only override layers during discovered loading, while still requiring at least one provider in the final merged or explicit single-file configuration.
  - Stop silently masking preferred-path stat failures, and deep-clone merged slices and maps so callers cannot mutate loaded layer data through shared backing storage.
- Windows changes: none beyond `filepath`-based discovery and temp-directory coverage for the discovered-load tests.
- Verification: `gofmt -w internal/runtime/config/config.go internal/runtime/config/config_test.go`; `go test ./internal/runtime/config -count=1`; `go test ./... -count=1`; `go vet ./...`; `git diff --check`
- Lore intent: `Make Devflow configuration authoritative without breaking MewCode users`

### llm

- Source: `internal/llm`
- Target: `internal/runtime/llm`
- Source files:
  - `anthropic.go`: `680AD6F8A93FFFD6C80458986DA43701DBC5677998EA397A5E813B20C84FA85F`
  - `client.go`: `2053006E3852C78881F776A059F39B8E96569513FF2F4A1382AE6E411F918813`
  - `errors.go`: `C5207CBF5DA03E4ADD20BE89B85311943D0E351970BE56D0C786040D26D9E58D`
  - `events.go`: `361C8431A9327EF9B72EEAC6DBC2A9B14320B30577D705E7EFB522B8AB3A6C5E`
  - `model_resolver.go`: `E485E5F476C3F1016C8C101F4BAAA0F3BC7E04C51141C7A173FF60BA8063B5FC`
  - `openai_compat.go`: `FCE2001AB8C7A78B31417D78CA629B82A47E2386CB2940853E37C061A22CA330`
  - `openai.go`: `2B0611EC8A76B4C50A113AC89AD819DB450769FDF112664F1D401BB0C0DA2620`
  - `thinking_test.go`: `2FBF296F2ACE17216F5254369D67C67559B771D80EF60E48CD69D4585CF5D0E3`
- Fusion changes:
  - Move the provider clients under the Devflow runtime boundary and retarget configuration and conversation imports to `internal/runtime/*`.
  - Update authentication guidance to prefer `.devflow` while preserving legacy `.mewcode` fallback and environment-variable discovery.
  - Add strict regression coverage for nil provider configs, unknown protocols, malformed Anthropic/OpenAI/OpenAI-compatible tool schemas, and fast context cancellation.
  - Prove that Anthropic, OpenAI Responses, and OpenAI-compatible successful streams each emit exactly one `StreamEnd` and preserve usage in that terminal event.
  - Prove that OpenAI-compatible `finish_reason:"tool_calls"` streams still emit exactly one `ToolCallComplete` and exactly one trailing `StreamEnd`, while preserving any later usage chunk.
  - Add an opt-in real Ark OpenAI-compatible smoke test that first chdirs to the repo root, discovers an Ark `openai-compat` provider through `config.LoadConfig("")` from `.devflow` or legacy `.mewcode`, and relies on provider/env API key resolution without printing the key.
  - Validate tool schemas before opening a provider stream so malformed Anthropic/OpenAI/OpenAI-compatible tool definitions return buffered stream errors instead of panicking on unchecked assertions.
- Windows changes: verify the streaming clients and the Ark endpoint from the supported Windows host.
- Verification: `gofmt -w internal/runtime/llm`; `go test ./internal/runtime/llm -count=1 -timeout 2m`; live `go test ./internal/runtime/llm -run TestLiveOpenAICompat -count=1 -v -timeout 90s`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Give Devflow a verified streaming model runtime`

### worktree

- Source: `internal/worktree`
- Target: `internal/runtime/worktree`
- Source files:
  - `agent.go`: `BB80B93FD88D89916558BF44761E311C8A92C9906C8830764FC6A3CF2C9E5D18`
  - `changes.go`: `9155E50CD89D6292D269665AADB9E3E9AAF845F1FB58564BF2AC37E25C50C59E`
  - `cleanup.go`: `F1989D4E0832AF0A2A9DC517878B8B5E1020D37D2F8C598F86D45C3734E59D01`
  - `create.go`: `5D2AF4C3619D7C894E8A0F38913160EF95551D097A59F292765F02343D8BA93F`
  - `env.go`: `DB24FEB836DDD5EAE985D794701F544F2886516361FBE54C3B0788E8D9F3DF8C`
  - `filesystem.go`: `A1AEE3D08515B44D1EC5BAAFE6A808E9917D57CFF4617A6C05B43BA9D1BE7918`
  - `notice.go`: `D09D943C50192E3F80D83165BF969F0A310FA9B3AFF0490BD30A56092F5F6FBF`
  - `session.go`: `5850BCE0B5E0AF3F23DC38F72B2F338A8AF2EA1243B9DE04ADAAA62347A73328`
  - `setup.go`: `4CBE0219E2E7CCA9BFC3B8053F86923042AD88F66F66B73037040B14FD7140E8`
  - `validate.go`: `1D61B6C61D67CD5F021672DE9E7737ACE10D565057CD917E1663C873CB66F54F`
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - No import retargeting required; the package only uses the Go standard library.
  - Preserve the filesystem-first git reader and the `env.go` no-prompt subprocess environment.
  - Extend canonical repo-root discovery to walk upward from nested directories before following worktree `commondir`, so Devflow can launch worktree operations from any project subdirectory.
  - Keep directory symlink creation best-effort; the migrated Windows test now skips when the host denies symlink privilege instead of treating that OS policy as a runtime regression.
- Windows changes:
  - No production changes required; `symlinkDirectories` still treats directory symlink creation as best-effort.
  - `TestSymlinkDirectories` now verifies a real symlink when the host allows one and skips on Windows when `os.Symlink` is denied by host privilege or developer-mode policy.
- Verification: `gofmt -w internal/runtime/worktree`; `go test ./internal/runtime/worktree -count=1 -timeout 2m`; `go test ./internal/runtime/worktree -run 'TestGitNoPromptEnv|TestRunGit' -v -count=1`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`; `go test ./... -count=1 -timeout 5m` if feasible
- Lore intent: `Bring git worktree management into the Devflow runtime`

### tools

- Source: `internal/tools`
- Target: `internal/runtime/tools`
- Source files:
  - `ask_user.go`: `9E97806CAE0576E08A15F122AFB738016E7DA9B3C4C63F650EA654B79692B1FA`
  - `bash.go`: `D46A20B2C458A688235202CA590BF57FB4489E1946969AEDBBD0C8E072621A2D`
  - `descriptions.go`: `F9EF0FFC05047F0D70B3F4276A5B180CDFF9FFA064F8DAD4E8FE66F5B93DAD89`
  - `edit_file.go`: `B4E4F5FEC22A87515D9B9A23CC4639D3855F8863EC18FCCFD2598EF389975E65`
  - `enter_worktree.go`: `E002A80B2EB85718C7479B2F11A1B7ADCBF2E3D32926DE498CC10251A510C244`
  - `exit_worktree.go`: `C9A5EF7B035015D815D9977489C5FF3E0289B44C9B44ED88C51F4ED433322D10`
  - `glob.go`: `3AFEB16688344135F5F66BF5009E1041B840026D32F7872113714FB520E6621A`
  - `grep.go`: `F35F7A5A812B594FFBF46AA125226D8F69C3E3C296B6A228C1693B67B0170999`
  - `read_file.go`: `4A61658B1D56987DF17996ECCCA563698DB2ED1B19E1D4F47D6B243CD225C6A8`
  - `tool_search.go`: `DEA7E7756F744B8CFDDDCA065E9DFA3410D41BABE987787C60538D196D8108A0`
  - `tool.go`: `B5AC0C9BAAF4C9BE9571E30F3D751E89DFD57A3CCC14DE503B4BCB4C3B39FEBA`
  - `write_file.go`: `F5C1DB756B2899351A75ED520EE7DB07404B5BAB6DA63D773BDAA1EDE881A8FB`
  - `glob_test.go`: `24DB4468DC28B81B357A27FC15537767728157AD44E040B90077DF646889AC5F`
  - `tool_search_test.go`: `4FEC3449320CB6B27069E05A98CDB37671B6514770E2E3360BE537EA01360B16`
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - Retarget the worktree import in `enter_worktree.go` and `exit_worktree.go` to `internal/runtime/worktree`.
  - Normalize `Glob` and `Grep` relative-path output to slash-separated paths so the migrated package preserves cross-platform tool output on native Windows.
- Windows changes:
  - Slash-normalize emitted relative paths in `Glob` and `Grep` so Windows hosts return the same `foo/bar.go` shape as other supported platforms.
- Verification: `gofmt -w internal/runtime/tools`; `go test ./internal/runtime/tools -count=1 -timeout 2m`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring the tool registry and file tools into the Devflow runtime`

### permissions

- Source: `internal/permissions`
- Target: `internal/runtime/permissions`
- Source files:
  - `permissions.go`: `FC9D84E43B5B002EE1BABACAD5B57553E496233DB1693C1CFA192CCF7478BB1F`
  - `permissions_test.go`: `8A61B6DF702025F86226AC346234131219859907AE8DC631799C33665E89E5E6`
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - Retarget the tools import in `permissions.go` and `permissions_test.go` to `internal/runtime/tools`.
- Windows changes:
  - Replace separator-unaware sandbox prefix checks with `filepath.Rel` containment checks, preventing sibling directories such as `project-evil` from matching an allowed `project` root on Windows or Unix-like hosts.
- Verification: `gofmt -w internal/runtime/permissions`; `go test ./internal/runtime/permissions -count=1`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring permission checks into the Devflow runtime`

### todo

- Source: `internal/todo`
- Target: `internal/runtime/todo`
- Source files:
  - `store.go`: `D9F5556C32A7A0608A703D9FE6AFC303D03C16E7D7B1C1BD1A5DFA17D06AF490`
  - `todo.go`: `4B618FC97F32F608BBE7E0A54343F0E0EB8B65A46CF658DF4765A8E11B93BB1A`
  - `tools.go`: `13E57FD565402D55DC366508C67E3BCB2DBE8D3E8E87C1E719FBA3E9372CB9C5`
- Added tests:
  - `todo_test.go`: covers corrupt-store fail-closed behavior, update/delete flows, `_internal` filtering, nil dependency guards, and wrapper task creation.
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - Retarget the tools import in `tools.go` to `internal/runtime/tools`.
  - Return load errors from `TaskList.Create` instead of overwriting a corrupt task store with a new list.
  - Make task tool wrappers return normal tool errors when the task list dependency is missing instead of panicking.
- Windows changes: none required.
- Verification: `gofmt -w internal/runtime/todo`; `go test ./internal/runtime/todo -count=1`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring todo tracking into the Devflow runtime`

### planfile

- Source: `internal/planfile`
- Target: `internal/runtime/planfile`
- Source files:
  - `planfile.go`: `3388E47A96F2334196127511A1134DD8BC16955FDBB30211B6EE561A40AB9878`
- Added tests:
  - `planfile_test.go`: covers save/load round-trip, reset rediscovery, newest existing plan discovery, and separate workdir isolation.
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - No import retargeting required; the package uses only the Go standard library.
  - Replace the source package's single process-global plan path with workdir-keyed path caching plus on-disk discovery of existing plan files.
- Windows changes: none required.
- Verification: `gofmt -w internal/runtime/planfile`; `go test ./internal/runtime/planfile -count=1`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring plan file management into the Devflow runtime`

### prompt

- Source: `internal/prompt`
- Target: `internal/runtime/prompt`
- Source files:
  - `builder.go`: `07A619603C62F3878FDBF662FF62B229341FD3C8E5AB58FD9A04175DD596A42E`
  - `plan_mode.go`: `9B821EA76BFE1F5AB6EFCB123D78C4DF54EB86194AF289BA9030AB3BD5899099`
  - `sections.go`: `B34683A865E8786CF31BC274C9792D8574EAF19AFDE943B3BA320A445FE88EAF`
- Added tests:
  - `prompt_test.go`: covers Devflow identity, section ordering, skill/environment inclusion, plan-mode reminder cadence, and OS-aware default shell fallback.
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - No import retargeting required; the package uses only the Go standard library.
  - Retarget the runtime identity section from legacy MewCode branding to Devflow.
- Windows changes:
  - Default shell detection now reports `powershell` on Windows when `SHELL` is unset instead of incorrectly falling back to `bash`.
- Verification: `gofmt -w internal/runtime/prompt`; `go test ./internal/runtime/prompt -count=1`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring prompt building into the Devflow runtime`

### toolresult

- Source: `internal/toolresult`
- Target: `internal/runtime/toolresult`
- Source files:
  - `budget.go`: `3617162B959CA309F60416B2D36C150C3BE3ACAC61E0916EF1D20DFBF7E8EC31`
  - `reconstruct.go`: `8EC245A3FE0B60693F799FE836DC40FDF29DE786E8B86CC67AD26B525FB84CB6`
  - `record.go`: `5305093961E3F8D8135C86A6E530F00A673D1A5F341869323F93ECF4BC0C0607`
  - `state.go`: `09A2513DA1A9C4C42CD1A7867DDCEC74C698896D460C265121DECA8598658D92`
  - `budget_test.go`: `8EF54AEE1B8E57095B4E138D8127495B3F366B536A30196B676762467525C2F8`
  - `record_test.go`: `76EA67BE9D818CB4E340D1B6BB8DF9F5131948A0F43E484A34EE5D0075459C0B`
  - `state_test.go`: `85492F1D226EF29ADF6FCB2BEB4E4DE8D5DB186C225366888145C259C407AE14`
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - Retarget the conversation import in `budget.go`, `budget_test.go`, and `reconstruct.go` to `internal/runtime/conversation`.
- Windows changes: none required.
- Verification: `gofmt -w internal/runtime/toolresult`; `go test ./internal/runtime/toolresult -count=1`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring tool result tracking into the Devflow runtime`

### compact

- Source: `internal/compact`
- Target: `internal/runtime/compact`
- Source files:
  - `compact.go`: `9867CE00E1DAF0D36BDC8D146427E4C3A2B0237116C37FC7EB4A2CA64DC93EE9`
  - `compact_test.go`: `A5C95E382A511D35199D2EFC59A1B714318899484AD396EDDFD1546153C1116F`
  - `recovery.go`: `DD686F65C29DEE5595AB9F2F1491A1A7FDDFCF9ABE137D789F3441C5C38C1A45`
  - `recovery_test.go`: `63460ED0B4F51E2585932E9C77C075F12D935A6A9477E6552C186AE93196E142`
- Fusion changes:
  - Move context compaction under the Devflow runtime boundary.
  - Retarget conversation and llm imports to the migrated runtime packages.
  - Retarget the package comment from MewCode to Devflow identity.
  - Count encrypted reasoning payloads when deciding whether to compact.
  - Include tool arguments, tool-result error state, and thinking metadata in the summary input while avoiding raw encrypted payload replay.
- Windows changes: none required unless package tests expose path or shell behavior.
- Verification: `gofmt -w internal/runtime/compact`; `go test ./internal/runtime/compact -count=1 -timeout 2m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring context compaction into the Devflow runtime`

### mcp

- Source: `internal/mcp`
- Target: `internal/runtime/mcp`
- Source files:
  - `mcp.go`: `87B6063D5F347ECFAAC79AB544BD93FD100E621BEE6A91635CCB1D4197FD1E08`
  - `mcp_test.go`: `B82B35AC1BAE5E798AB638658E1818E7A9E31AEA19D3CBE9877C396FA0CCB21C`
- Fusion changes:
  - Move MCP runtime support under the Devflow runtime boundary.
  - Retarget tool interfaces to `internal/runtime/tools`.
  - Add `github.com/modelcontextprotocol/go-sdk v1.6.0` and align the module Go directive with the SDK's Go 1.25 requirement.
  - Retarget the MCP client implementation name from `mewcode` to `Devflow`.
  - Replace the source live `npx @upstash/context7-mcp` integration test with deterministic unit coverage for transport selection, HTTP header environment expansion, sanitized tool names, nil/default and preserved schemas, and wrapper execution results.
- Windows changes: verify command/process handling through package tests; no package-local Windows production changes required.
- Verification: `gofmt -w internal/runtime/mcp`; `go test ./internal/runtime/mcp -count=1 -timeout 2m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring MCP tool bridging into the Devflow runtime`

### skills

- Source: `internal/skills`
- Target: `internal/runtime/skills`
- Source files:
  - `builtins.go`: `5D2795491E892BFF226B0D32B62C283B8873AAEE0866C81DFFC545FC18DAFF8D`
  - `catalog.go`: `D885C94BC0C6269DC055C3841D2CA238554D672409114A2FBFE82AC416BAE0D3`
  - `catalog_test.go`: `A637432FBF45BEBEAA8D612D24E8403C6D3D7809D2AE5334BD4A297D1944A1B0`
  - `directory.go`: `264E9CF8F086F198A031987CC834D521E5C65A7B2BBE5E2AB6ECDE07B7BE8965`
  - `executor.go`: `9F60C2EC418E11389CB9A2408295E9F5A5CF7465A1A26A53AB29C39D2D02E583`
  - `executor_test.go`: `B615B52E0CBD16A75CC0F6FCEB8BBA01F286C2D18AAA1C265BE3F9EB7BCF5E86`
  - `install.go`: `8A25837BE08B94A56364594E953420159A19F4315410D4C2AFCE02684ACA38B1`
  - `install_test.go`: `78272D5C0AAC1C897DD487C3E81FC814B217AFAB0167716E744F6DDC469BEF59`
  - `install_tool.go`: `AE6C7B8B60AF30236DA0EC5ADF93D43672BC8155A4B9546CFAAB66CB335FBAAB`
  - `load_skill_tool.go`: `DBC44ADCFA374FAEE864DC11E6813810EE45C919A184CA67CE7FC2F008B471CB`
  - `parse_resume.go`: `43E47C661832E4CE98EE262B4A1FFEBD024507FB342E1FE87AB7FAE5F2A02EEF`
  - `parser.go`: `55D8D15A63FE92E74B7813EBCA308D1134755374AAF46F11CC0C3FC9A4918D33`
  - `skills.go`: `965FE1BAAEE5D00F1F931AA9EA490335D71BEA9A90057B5A09C3708B10053ADB`
  - `skills_test.go`: `E67661627E671375B5ADEA2435BACF3A00D27E53BEC757B3084AB02F39A6C8F4`
- Bundled skills:
  - `builtins/commit/SKILL.md`
  - `builtins/test/SKILL.md`
  - `builtins/backend-interview/SKILL.md`
  - `builtins/backend-interview/tool.json`
- Fusion changes:
  - Move skill catalog, parser, executor, installation tools, and bundled skills under the Devflow runtime boundary.
  - Retarget conversation and tools imports to runtime packages.
  - Use `.devflow/skills` as the new user and project skill location while preserving `.mewcode/skills` as a lower-priority legacy fallback.
  - Retarget skill install user-agent and tool descriptions from MewCode to Devflow.
  - Replace the source real-project `.mewcode/skills` discovery test with deterministic temp-directory coverage.
- Windows changes:
  - Use slash-based embedded filesystem paths for bundled skills so `embed.FS` works on Windows.
- Verification: `gofmt -w internal/runtime/skills`; `go test ./internal/runtime/skills -count=1 -timeout 2m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring skill loading and execution into the Devflow runtime`

### memory

- Source: `internal/memory`
- Target: `internal/runtime/memory`
- Source files:
  - `find_relevant_memories.go`: `5D67908640CE24FC78B55152BCD80469BF27DC50DEA780344D944F58646CF389`
  - `find_relevant_memories_test.go`: `09EA599B83878151C48E67AF3E35AB05FCD02B43A06CA37CE57DC6EE04B252FE`
  - `instructions.go`: `58320DEAB80889340CDEC2664CC935DBE4B03805AB24BF23AF7F636FB4F3B9A4`
  - `instructions_test.go`: `A09A71C2B49CEB65E42B654EB54C4E94AC9FD16D2B760DC4CA5B68FC94C11687`
  - `memdir.go`: `8D71BA3798E9A0BB88436BA71CB25496775708D07AB176176CB6C9FA9D42F23C`
  - `memory.go`: `DCDEFFB77D8F68D6F0FE423325B5258085C7FF6AC577F65AA55A4A3CE6E25CD8`
  - `memory_age.go`: `AE2B527D545062F0CDCDA78F903AE0316C1165A124F2C4B83DC9E5EA07269FEF`
  - `memory_age_test.go`: `5E62CE6D833384501E30EAAE788A60CD256B4F449351F2A94129F833CDBBED5A`
  - `memory_scan.go`: `310CCDFFDF4A18B20EADEE00B563A9A57BF05C7105926F87E87C8A8ACF98C346`
  - `memory_scan_test.go`: `F60673292CB55B83BFCB870CC30E7AC3F142F5B6DBAA569538E69FE5EF831841`
  - `memory_test.go`: `36EEF1591E66050564099F343DC8CFF9807E3D6562E0E531331F39ADCAD29FDD`
  - `memory_types.go`: `DCEB7405259E43749117CB978491A240132E03E0E5AEAE896C81747EB0FE5D82`
  - `paths.go`: `5D2E10EE5A1497465BE09CECC077929CBEED3AAE7AE5B0CF78B976EA6717520A`
- Fusion changes:
  - Move project memory scanning, typed memory prompts, instruction discovery, and memory manager code under the Devflow runtime boundary.
  - Make `.devflow/memory` and `~/.devflow/memory` the new project-level and user-level write targets.
  - Keep `MEWCODE_REMOTE_MEMORY_DIR`, `MEWCODE.md`, `.mewcode/INSTRUCTIONS.md`, and `MEWCODE.local.md` as legacy read/migration fallbacks.
  - Prefer `DEVFLOW_REMOTE_MEMORY_DIR` over the legacy memory override when both environment variables are present.
  - Retarget memory selector prompts, behavioral instructions, and examples from MewCode to Devflow.
  - Add Windows-stable tests for path discovery, environment overrides, and user-home isolation.
- Windows changes:
  - Replace hard-coded slash-path expectations with `filepath.Join` and temp directories.
  - Set both `HOME` and `USERPROFILE` in tests that depend on `os.UserHomeDir`.
- Verification: `gofmt -w internal/runtime/memory`; `go test ./internal/runtime/memory -count=1 -timeout 2m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring project memory into the Devflow runtime`

### agent

- Source: `internal/agent`
- Target: `internal/runtime/agent`
- Source files:
  - `agent.go`: `213F8A091B5BBE4F17725522D912732659B26B36E4AF93BB1A280AAC1956D69F`
  - `agent_live_test.go`: `E3A456E25A7028E78CBE80D71864C47070E01D9719CF815EF7BCAF4EF96C0676`
  - `agent_test.go`: `8C00E83DEA66E0AD2DC624A8D84078CA7C0EF75115DC5ED1E099F1A308CAFF57`
  - `events.go`: `1140F44FBAA608BB6FA4F34AE8FA9AD3BF3D94845F06D81B6A3FBA4BDABFDAAE`
  - `skills_test.go`: `D5223562CA3A86FA19D4B8EA061F3AEDC7E9C60F9965D81AC4F2BE4A51A70E22`
  - `streaming_executor.go`: `1E707421E819E52E9530D4632B14A6B62017169EE4F05CB71C8E92A2E64CB320`
- Fusion changes:
  - Move the agent loop under the Devflow runtime boundary.
  - Retarget compact, conversation, hooks, llm, permissions, planfile, prompt, skills, toolresult, and tools imports to runtime packages.
  - Retarget skill-creation tests from `.mewcode/skills` to `.devflow/skills`.
  - Gate live agent tests behind `DEVFLOW_LIVE_AGENT=1`, load provider config through Devflow discovery, and keep API keys out of test output.
  - Preserve hook, permission, active-skill, compaction, tool-result replacement, and recovery-state behavior from the source package.
- Windows changes:
  - Use `filepath` paths in skill tests and slash-normalize path checks where test output paths are compared as strings.
  - Live agent smoke was verified on the supported Windows host through the Ark/OpenAI-compatible provider configured in Devflow.
- Verification: `gofmt -w internal/runtime/agent`; `go test ./internal/runtime/agent -count=1 -timeout 3m`; `DEVFLOW_LIVE_AGENT=1 go test ./internal/runtime/agent -run TestLiveSimpleChat -count=1 -v -timeout 120s`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go test ./... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring the agent loop into the Devflow runtime`
