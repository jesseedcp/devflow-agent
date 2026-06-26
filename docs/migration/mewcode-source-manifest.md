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
