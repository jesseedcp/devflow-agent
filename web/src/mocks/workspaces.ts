import type { AuditEvent, CurrentUser, WikiEntry, Workspace } from '../api/types';

export const mockWorkspaces: Workspace[] = [
  {
    id: 'ws-payments',
    name: 'Payments Platform',
    artifactRoot: 'D:/repos/payments',
    createdAt: '2026-06-01T09:00:00Z',
  },
];

export const mockCurrentUser: CurrentUser = {
  id: 'user-1',
  email: 'admin@example.com',
  displayName: 'Ada Admin',
  role: 'Admin',
};

export const mockWikiEntries: WikiEntry[] = [
  {
    id: 'wiki-retry-backoff-policy',
    workspaceId: 'ws-payments',
    name: 'retry-backoff-policy',
    category: 'business',
    sourceDemandKey: 'add-retry-backoff',
    artifactPath: '.devflow/wiki/retry-backoff-policy.md',
    updatedAt: '2026-07-03T14:22:00Z',
  },
];

export const mockAuditEvents: AuditEvent[] = [
  {
    id: 'audit-1',
    workspaceId: 'ws-payments',
    actorUserId: 'user-1',
    actorEmail: 'admin@example.com',
    action: 'demand.created',
    subjectType: 'demand',
    subjectId: 'add-retry-backoff',
    metadata: { title: 'Add retry with exponential backoff' },
    createdAt: '2026-07-01T10:00:00Z',
  },
  {
    id: 'audit-2',
    workspaceId: 'ws-payments',
    actorUserId: 'user-2',
    actorEmail: 'reviewer@example.com',
    action: 'plan.confirmed',
    subjectType: 'demand',
    subjectId: 'add-retry-backoff',
    metadata: { stage: 'plan_review' },
    createdAt: '2026-07-01T12:30:00Z',
  },
  {
    id: 'audit-3',
    workspaceId: 'ws-payments',
    actorUserId: 'user-1',
    actorEmail: 'admin@example.com',
    action: 'release.deploy_triggered',
    subjectType: 'demand',
    subjectId: 'add-retry-backoff',
    metadata: { environment: 'production', run_url: 'https://github.com/org/payments/actions/runs/101' },
    createdAt: '2026-07-02T18:05:00Z',
  },
  {
    id: 'audit-4',
    workspaceId: 'ws-payments',
    actorUserId: 'user-2',
    actorEmail: 'reviewer@example.com',
    action: 'wiki.promoted',
    subjectType: 'wiki_candidate',
    subjectId: 'add-retry-backoff#0',
    metadata: { name: 'retry-backoff-policy' },
    createdAt: '2026-07-03T14:22:00Z',
  },
  {
    id: 'audit-5',
    workspaceId: 'ws-payments',
    actorUserId: 'user-1',
    actorEmail: 'admin@example.com',
    action: 'rollback.pending',
    subjectType: 'demand',
    subjectId: 'add-retry-backoff',
    metadata: { reason: 'observation failed: error_rate exceeded threshold' },
    createdAt: '2026-07-04T08:11:00Z',
  },
];
