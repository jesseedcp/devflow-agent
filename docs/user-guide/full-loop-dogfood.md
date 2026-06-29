# Full-Loop Dogfood Guide

Run this before a release branch is merged:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-dev
```

This command rebuilds the CLI, creates a unique isolated dogfood workspace under the system temp directory, runs `devflow dogfood`, and prints the generated `dogfood-report.md` path.

## What It Proves

- A demand can be created from a realistic backend requirement.
- Requirements and plan stages can be drafted.
- Human confirmation gates can be recorded without manually editing state.
- Implementation and verification can run real quality commands against the repository checkout.
- MR review can be represented by an offline no-blocker adapter.
- Closeout and memory candidates can be drafted.
- The final demand state reaches `completed`.
- A report captures state transitions, artifact paths, and quality evidence.

## What It Does Not Prove

- Live model provider quality.
- Real GitLab MR API behavior.
- That generated implementation code is correct for production.

Those remain separate live dogfood or integration checks.
