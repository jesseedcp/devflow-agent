import { describe, expect, it } from 'vitest';
import { MockApiClient } from './mockClient';

const WS = 'ws-payments';

describe('MockApiClient contract', () => {
  const client = new MockApiClient();

  it('lists one workspace', async () => {
    const ws = await client.listWorkspaces();
    expect(ws).toHaveLength(1);
    expect(ws[0].id).toBe(WS);
  });

  it('lists three demands in different states', async () => {
    const demands = await client.listDemands(WS);
    expect(demands).toHaveLength(3);
    const states = demands.map((d) => d.state);
    expect(new Set(states).size).toBe(3);
  });

  it('returns demand detail with artifacts, quality, metrics, and release line', async () => {
    const detail = await client.getDemand(WS, 'add-retry-backoff');
    expect(detail.artifacts.length).toBeGreaterThan(0);
    expect(detail.releaseLine.rollbackNeeded).toBe(true);
    expect(detail.quality.checks.length).toBeGreaterThan(0);
    expect(detail.metrics.status).toBe('fail');
    expect(detail.acceptance.length).toBeGreaterThan(0);
  });

  it('serves artifact content', async () => {
    const content = await client.getArtifact(WS, 'add-retry-backoff', 'observation.md');
    expect(content).toContain('Observation');
  });

  it('lists one promoted wiki entry', async () => {
    const entries = await client.listWikiEntries(WS);
    expect(entries).toHaveLength(1);
    expect(entries[0].name).toBe('retry-backoff-policy');
  });

  it('lists two pending wiki candidates', async () => {
    const candidates = await client.listWikiCandidates(WS);
    expect(candidates).toHaveLength(2);
    expect(candidates.every((c) => c.status === 'pending')).toBe(true);
  });

  it('returns five audit events', async () => {
    const events = await client.getAuditEvents(WS);
    expect(events).toHaveLength(5);
  });

  it('returns a rollback-needed release summary', async () => {
    const release = await client.getRelease(WS, 'add-retry-backoff');
    expect(release.kind).toBe('rollback');
    expect(release.decision).toBe('pending');
  });
});

describe('MockApiClient mutations', () => {
  it('promotes a candidate into a wiki entry and records audit', async () => {
    const client = new MockApiClient();
    const before = await client.listWikiEntries(WS);
    const entry = await client.promoteWikiCandidate(WS, 'idempotency-keys#0', {
      name: 'idempotency-window',
      category: 'process',
    });
    expect(entry.name).toBe('idempotency-window');
    const after = await client.listWikiEntries(WS);
    expect(after).toHaveLength(before.length + 1);
    const candidates = await client.listWikiCandidates(WS);
    const promoted = candidates.find((c) => c.id === 'idempotency-keys#0');
    expect(promoted?.status).toBe('promoted');
    const audit = await client.getAuditEvents(WS);
    expect(audit.some((e) => e.action === 'wiki.promoted')).toBe(true);
  });

  it('rejects a candidate with a reason and records audit', async () => {
    const client = new MockApiClient();
    const rejected = await client.rejectWikiCandidate(WS, 'webhook-delivery#0', {
      reason: 'duplicate of existing entry',
    });
    expect(rejected.status).toBe('rejected');
    expect(rejected.reason).toBe('duplicate of existing entry');
    const audit = await client.getAuditEvents(WS);
    expect(audit.some((e) => e.action === 'wiki.rejected')).toBe(true);
  });

  it('rejects promotion of an already-resolved candidate', async () => {
    const client = new MockApiClient();
    await client.promoteWikiCandidate(WS, 'idempotency-keys#0', { name: 'x', category: 'process' });
    await expect(
      client.promoteWikiCandidate(WS, 'idempotency-keys#0', { name: 'y', category: 'process' }),
    ).rejects.toThrow(/already/);
  });

  it('triggers rollback for a rollback-needed demand and records audit', async () => {
    const client = new MockApiClient();
    const release = await client.triggerRollback(WS, 'add-retry-backoff');
    expect(release.decision).toBe('rollback_confirmed');
    expect(release.runUrl).toContain('rollback');
    const audit = await client.getAuditEvents(WS);
    expect(audit.some((e) => e.action === 'rollback.triggered')).toBe(true);
  });

  it('refuses rollback when not needed', async () => {
    const client = new MockApiClient();
    await expect(client.triggerRollback(WS, 'idempotency-keys')).rejects.toThrow(/not needed/);
  });

  it('supports role switching for audit attribution', async () => {
    const client = new MockApiClient();
    client.setCurrentRole('Reviewer');
    const user = await client.getCurrentUser();
    expect(user.role).toBe('Reviewer');
    await client.promoteWikiCandidate(WS, 'idempotency-keys#0', { name: 'r-entry', category: 'process' });
    const audit = await client.getAuditEvents(WS);
    const promoted = audit.find((e) => e.action === 'wiki.promoted');
    expect(promoted?.actorEmail).toBe(user.email);
  });
});
