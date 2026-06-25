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
