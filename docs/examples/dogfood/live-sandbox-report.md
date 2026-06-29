# Live Dogfood Report: live-dogfood-coupon-eligibility

Root: `<temp>\devflow-live-dogfood-<id>`

RepoRoot: `<temp>\devflow-live-dogfood-<id>\repo`

DemandRoot: `<temp>\devflow-live-dogfood-<id>\artifacts`

FinalState: `completed`

## Steps

- `requirements` -> `requirements_review`: requirements drafted by demand runner
- `confirm requirements` -> `plan_drafting`: live requirements accepted for sandbox dogfood
- `plan` -> `plan_review`: plan drafted by demand runner
- `confirm plan` -> `implementation`: live plan accepted for sandbox dogfood
- `implementation` -> `mr_review`: implementation completed and quality gate passed
- `mr-review` -> `verification`: mr review cleared, no blocking unresolved comments
- `verification` -> `verification`: verification drafted by demand runner
- `confirm verification` -> `closeout`: live sandbox verification passed
- `closeout` -> `closeout`: closeout and memory candidates drafted by demand runner
- `confirm closeout` -> `completed`: live sandbox closeout accepted
