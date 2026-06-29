# Dogfood Report: dogfood-coupon-eligibility

Root: `<temp>\devflow-dogfood-local\artifacts`

QualityRoot: `<repo>\devflow-agent`

FinalState: `completed`

## Steps

- `requirements` -> `requirements_review`: requirements drafted by demand runner
- `confirm requirements` -> `plan_drafting`: deterministic dogfood requirements accepted
- `plan` -> `plan_review`: plan drafted by demand runner
- `confirm plan` -> `implementation`: deterministic dogfood plan accepted
- `implementation` -> `mr_review`: implementation completed and quality gate passed
- `mr-review` -> `verification`: mr review cleared, no blocking unresolved comments
- `verification` -> `verification`: verification drafted by demand runner
- `confirm verification` -> `closeout`: deterministic dogfood verification accepted
- `closeout` -> `closeout`: closeout and memory candidates drafted by demand runner
- `confirm closeout` -> `completed`: deterministic dogfood closeout accepted

## Artifacts

- `requirements.md`
- `plan.md`
- `progress.md`
- `verification.md`
- `closeout.md`
- `memory-candidates.md`
- `events.jsonl`
