import type { DemandDetail, DemandSummary, ReleaseSummary, WikiCandidate } from '../api/types';

const WS = 'ws-payments';

export const mockDemandSummaries: DemandSummary[] = [
  {
    id: 'd-1',
    workspaceId: WS,
    demandKey: 'add-retry-backoff',
    title: 'Add retry with exponential backoff',
    state: 'observation',
    attention: 'rollback needed: observation failed',
    artifactPath: '.devflow/demands/add-retry-backoff',
    updatedAt: '2026-07-04T08:11:00Z',
  },
  {
    id: 'd-2',
    workspaceId: WS,
    demandKey: 'idempotency-keys',
    title: 'Idempotency keys for payment intents',
    state: 'mr_review',
    attention: 'ready for MR review',
    artifactPath: '.devflow/demands/idempotency-keys',
    updatedAt: '2026-07-04T09:30:00Z',
  },
  {
    id: 'd-3',
    workspaceId: WS,
    demandKey: 'webhook-delivery',
    title: 'At-least-once webhook delivery',
    state: 'blocked_need_user',
    attention: 'needs requirements decision on ordering',
    artifactPath: '.devflow/demands/webhook-delivery',
    updatedAt: '2026-07-03T16:45:00Z',
  },
];

const baseArtifacts = (key: string, present: string[]) => {
  const all = [
    'requirements.md',
    'plan.md',
    'progress.md',
    'verification.md',
    'deployment.md',
    'observation.md',
    'rollback.md',
    'closeout.md',
    'metrics.md',
    'events.jsonl',
  ];
  return all.map((name) => ({
    name,
    path: `.devflow/demands/${key}/${name}`,
    present: present.includes(name),
    size: present.includes(name) ? 512 + name.length * 7 : 0,
  }));
};

export const mockDemandDetails: Record<string, DemandDetail> = {
  'add-retry-backoff': {
    ...mockDemandSummaries[0],
    artifacts: baseArtifacts('add-retry-backoff', [
      'requirements.md',
      'plan.md',
      'progress.md',
      'verification.md',
      'deployment.md',
      'observation.md',
      'rollback.md',
      'metrics.md',
      'events.jsonl',
    ]),
    releaseLine: {
      deploymentStatus: 'failed',
      runUrl: 'https://github.com/org/payments/actions/runs/101',
      environment: 'production',
      ref: 'main',
      rollbackDecision: 'pending',
      rollbackNeeded: true,
    },
    quality: {
      gate: 'pass',
      checks: [
        { name: 'unit tests', status: 'pass', summary: '184/184 passed' },
        { name: 'integration', status: 'pass', summary: '12/12 passed' },
        { name: 'ci gate', status: 'pass', summary: 'GitHub Actions green' },
      ],
    },
    acceptance: [
      { category: 'unit', required: 1, provided: 1 },
      { category: 'integration', required: 1, provided: 1 },
      { category: 'manual', required: 1, provided: 0 },
    ],
    metrics: {
      adapter: 'generic-json',
      status: 'fail',
      metrics: [
        { name: 'error_rate', value: 0.04, unit: 'ratio', threshold: 0.01, pass: false },
        { name: 'p95_latency_ms', value: 120, unit: 'ms', threshold: 300, pass: true },
        { name: 'active_alerts', value: 1, unit: 'count', threshold: 0, pass: false },
      ],
      summary: 'error_rate 0.04 > 0.01; active_alerts 1 > 0',
    },
    nextActions: [
      {
        label: 'Trigger rollback',
        command: 'devflow rollback trigger --demand add-retry-backoff',
        reason: 'Observation failed; Admin must confirm rollback.',
      },
      {
        label: 'Refresh observation',
        command: 'devflow observe refresh --demand add-retry-backoff',
        reason: 'Re-check metrics after mitigation.',
      },
    ],
  },
  'idempotency-keys': {
    ...mockDemandSummaries[1],
    artifacts: baseArtifacts('idempotency-keys', [
      'requirements.md',
      'plan.md',
      'progress.md',
      'verification.md',
    ]),
    releaseLine: {
      deploymentStatus: 'not_started',
      runUrl: '',
      environment: 'production',
      ref: 'main',
      rollbackDecision: 'pending',
      rollbackNeeded: false,
    },
    quality: {
      gate: 'blocked',
      checks: [
        { name: 'unit tests', status: 'pass', summary: '96/96 passed' },
        { name: 'integration', status: 'blocked', summary: 'awaiting MR review' },
        { name: 'ci gate', status: 'unknown', summary: 'not run yet' },
      ],
    },
    acceptance: [
      { category: 'unit', required: 1, provided: 1 },
      { category: 'integration', required: 1, provided: 0 },
      { category: 'manual', required: 1, provided: 0 },
    ],
    metrics: {
      adapter: 'generic-json',
      status: 'unknown',
      metrics: [],
      summary: 'No observation recorded; deployment not started.',
    },
    nextActions: [
      {
        label: 'Review MR',
        command: 'devflow review --demand idempotency-keys',
        reason: 'Implementation ready for Reviewer MR review.',
      },
    ],
  },
  'webhook-delivery': {
    ...mockDemandSummaries[2],
    artifacts: baseArtifacts('webhook-delivery', ['requirements.md', 'plan.md']),
    releaseLine: {
      deploymentStatus: 'not_started',
      runUrl: '',
      environment: 'production',
      ref: 'main',
      rollbackDecision: 'pending',
      rollbackNeeded: false,
    },
    quality: {
      gate: 'blocked',
      checks: [
        { name: 'unit tests', status: 'blocked', summary: 'no implementation yet' },
        { name: 'integration', status: 'blocked', summary: 'no implementation yet' },
        { name: 'ci gate', status: 'unknown', summary: 'not run yet' },
      ],
    },
    acceptance: [
      { category: 'unit', required: 1, provided: 0 },
      { category: 'integration', required: 1, provided: 0 },
      { category: 'manual', required: 1, provided: 0 },
    ],
    metrics: {
      adapter: 'generic-json',
      status: 'unknown',
      metrics: [],
      summary: 'Blocked on requirements decision.',
    },
    nextActions: [
      {
        label: 'Resolve requirements',
        command: 'devflow run --demand webhook-delivery --stage requirements',
        reason: 'Ordering semantics need a user decision.',
      },
    ],
  },
  'coupon-eligibility': {
    ...mockDemandSummaries[0],
    id: 'coupon-eligibility',
    workspaceId: 'ws-demo',
    demandKey: 'coupon-eligibility',
    title: 'Coupon eligibility',
    state: 'requirements_review',
    attention: 'ready to confirm requirements',
    artifactPath: '.devflow/demands/coupon-eligibility',
    updatedAt: '2026-07-08T08:00:00Z',
    description: 'Inactive users must be blocked from claiming coupons.',
    source: 'web',
    artifacts: baseArtifacts('coupon-eligibility', ['requirements.md']),
    releaseLine: {
      deploymentStatus: 'not_started',
      runUrl: '',
      environment: 'production',
      ref: 'main',
      rollbackDecision: 'pending',
      rollbackNeeded: false,
    },
    quality: {
      gate: 'blocked',
      checks: [{ name: 'requirements', status: 'blocked', summary: 'needs confirmation' }],
      stageSummary: { requirements: 'needs_confirmation' },
      blockers: 0,
      warnings: 1,
    },
    acceptance: [],
    metrics: { adapter: '', status: 'unknown', metrics: [], summary: '' },
    evidence: { pass: 0, fail: 0, blocked: 0 },
    nextActions: [
      {
        label: 'Confirm requirements',
        command: 'devflow confirm --demand coupon-eligibility --stage requirements',
        reason: 'Requirements need human confirmation before planning.',
      },
    ],
  },
  'verification-ready': {
    ...mockDemandSummaries[0],
    id: 'verification-ready',
    workspaceId: 'ws-demo',
    demandKey: 'verification-ready',
    title: 'Verification ready',
    state: 'verification',
    attention: 'needs verification evidence',
    artifactPath: '.devflow/demands/verification-ready',
    updatedAt: '2026-07-08T09:00:00Z',
    description: 'Acceptance evidence must be recorded before closeout.',
    source: 'web',
    artifacts: baseArtifacts('verification-ready', ['requirements.md', 'plan.md', 'progress.md', 'verification.md']),
    releaseLine: {
      deploymentStatus: 'not_started',
      runUrl: '',
      environment: 'production',
      ref: 'main',
      rollbackDecision: 'pending',
      rollbackNeeded: false,
    },
    quality: {
      gate: 'blocked',
      checks: [{ name: 'verification', status: 'blocked', summary: 'needs evidence' }],
      stageSummary: { verification: 'needs_evidence' },
      blockers: 0,
      warnings: 1,
    },
    acceptance: [],
    metrics: { adapter: '', status: 'unknown', metrics: [], summary: '' },
    evidence: { pass: 0, fail: 0, blocked: 0 },
    nextActions: [
      {
        label: 'Add acceptance evidence',
        command: 'devflow evidence add --demand verification-ready --type manual',
        reason: 'Record manual acceptance evidence before confirming verification.',
      },
    ],
  },
};

export const mockWikiCandidates: WikiCandidate[] = [
  {
    id: 'idempotency-keys#0',
    workspaceId: WS,
    demandKey: 'idempotency-keys',
    index: 0,
    kind: 'process',
    text: 'Store the idempotency key hash alongside the intent so retries within the key window return the original response instead of creating a duplicate charge.',
    source: 'plan.md',
    status: 'pending',
    wikiPath: '',
    reason: '',
  },
  {
    id: 'webhook-delivery#0',
    workspaceId: WS,
    demandKey: 'webhook-delivery',
    index: 0,
    kind: 'business',
    text: 'Webhook delivery is at-least-once; consumers must dedupe by event id. Exactly-once was rejected because it requires distributed locking that is not justified for the current throughput.',
    source: 'requirements.md',
    status: 'pending',
    wikiPath: '',
    reason: '',
  },
];

export const mockReleaseSummaries: Record<string, ReleaseSummary> = {
  'add-retry-backoff': {
    id: 'rel-1',
    workspaceId: WS,
    demandKey: 'add-retry-backoff',
    kind: 'rollback',
    provider: 'github_actions',
    status: 'pending',
    runUrl: 'https://github.com/org/payments/actions/runs/101',
    decision: 'pending',
    updatedAt: '2026-07-04T08:11:00Z',
  },
  'idempotency-keys': {
    id: 'rel-2',
    workspaceId: WS,
    demandKey: 'idempotency-keys',
    kind: 'deploy',
    provider: 'github_actions',
    status: 'not_started',
    runUrl: '',
    decision: 'pending',
    updatedAt: '2026-07-04T09:30:00Z',
  },
  'webhook-delivery': {
    id: 'rel-3',
    workspaceId: WS,
    demandKey: 'webhook-delivery',
    kind: 'deploy',
    provider: 'github_actions',
    status: 'not_started',
    runUrl: '',
    decision: 'pending',
    updatedAt: '2026-07-03T16:45:00Z',
  },
};

export const mockArtifactContent: Record<string, string> = {
  'add-retry-backoff/observation.md': [
    '# Observation',
    '',
    'Adapter: generic-json',
    'Endpoint: http://payments-demo/metrics.json',
    '',
    '- error_rate: 0.04 (threshold 0.01) FAIL',
    '- p95_latency_ms: 120 (threshold 300) pass',
    '- active_alerts: 1 (threshold 0) FAIL',
    '',
    'Verdict: FAIL — rollback recommended.',
  ].join('\n'),
  'add-retry-backoff/rollback.md': [
    '# Rollback',
    '',
    'Trigger: observation failed',
    'Recommended: revert to previous green deployment',
    'Decision: pending (awaiting Admin confirmation)',
  ].join('\n'),
  'add-retry-backoff/metrics.md': [
    '# Metrics',
    '',
    'error_rate=0.04 threshold=0.01 pass=false',
    'p95_latency_ms=120 threshold=300 pass=true',
    'active_alerts=1 threshold=0 pass=false',
  ].join('\n'),
  'idempotency-keys/plan.md': [
    '# Plan',
    '',
    '1. Add idempotency_key column to payment_intents',
    '2. Hash key on write; return stored response on duplicate key',
    '3. Add unit + integration tests for the key window',
  ].join('\n'),
  'webhook-delivery/requirements.md': [
    '# Requirements',
    '',
    '- Delivery is at-least-once',
    '- Consumers dedupe by event id',
    '- OPEN: ordering guarantees across events (needs decision)',
  ].join('\n'),
};
