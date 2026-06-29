# MewCode Reuse And Eino Integration Decision

> Superseded on 2026-06-25 by
> [Devflow And MewCode Single-Repository Fusion Design](../superpowers/specs/2026-06-25-devflow-mewcode-single-repo-fusion-design.md).
> This document remains as the record of the earlier v0.1 foundation boundary.

## Decision

Devflow owns the product workflow state machine, stage artifacts, and gate semantics.
MewCode is the execution foundation and reference implementation for agent mechanics, not a forked Devflow product workflow.
Eino is a later-stage option for isolated LLM subflows, not the top-level product state machine.

## Why Devflow owns workflow

The workflow rules are product semantics:

- requirements must be confirmed before plan
- plan must be confirmed before implementation
- failed quality gates block verification
- blocking MR comments prevent closeout
- memory candidates require confirmation before reuse as stable knowledge

These rules must stay in Devflow code and tests so the product can be verified without depending on prompts or graph wiring.

## What To Reuse From MewCode First

Review MewCode in this order before creating duplicate primitives:

- `internal/tools`
- `internal/skills`
- `internal/memory` for project-local conventions; MewCode's user-level `~/.mewcode/memory/` scope is outside Devflow v0.1
- `internal/worktree`
- `internal/agent`

Because MewCode uses `internal` packages, Devflow cannot import them across modules. Reuse is still possible through one of these paths:

1. Only after an explicit license and ownership review permits reuse, copy a small package with required attribution and tests.
2. Extract stable packages into a shared module.
3. Keep Devflow independent and use MewCode as the implementation reference.

Before reusing any package or API shape, check license obligations, ownership, attribution requirements, and API stability in the source repo. Attribution alone is not permission to reuse code, and this document makes no conclusion about MewCode's license.

## Current v0.1 Status

v0.1 already ships the deterministic foundation:

- deterministic workflow and artifact creation
- CLI confirmation for stage gates
- local quality evidence recording
- workspace-backed stage files and event logs
- file-backed memory candidate search

Current search indexes demand-local `memory-candidates.md` files. These remain reviewable candidates; promotion to stable knowledge requires explicit human confirmation and is not enforced by the search layer.
This foundation implementation is intentionally narrower than the broader MVP design, which lists historical requirements, plans, verification, and closeout artifacts as future memory sources.
Historical requirements, plan, verification, and closeout artifacts remain future candidate sources, not part of the v0.1 search contract.

What is not connected yet:

- LLM execution
- GitLab API integration
- Eino graphs

## Eino Gate

Introduce Eino only when a stage needs one or more of the following:

- multi-node LLM orchestration
- retries or branching
- observability across substeps
- clearer separation between clarify, planning, review classification, verification summarization, and closeout summarization

If a stage is a single local call or a straightforward file transform, keep it in plain Go code.
Do not use Eino as the top-level state machine. Devflow's workflow state machine remains the product authority.
