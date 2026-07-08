import { describe, expect, it, vi } from 'vitest';
import { HttpApiClient } from './httpClient';

function mockResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: '',
    json: async () => body,
    text: async () => JSON.stringify(body),
  };
}

const demandPayload = {
  status: 'created',
  message: 'Demand created',
  demand: {
    demand_key: 'coupon-from-ui',
    title: 'Coupon from UI',
    state: 'requirements_review',
    attention: 'ready to confirm requirements',
    updated_at: '2026-07-08T00:00:00Z',
    description: 'Inactive users must be blocked',
    source: 'web',
    artifacts: [{ name: 'requirements.md', exists: true }],
    evidence: { pass: 0, fail: 0, blocked: 0 },
    release: {
      deployment_status: 'not_started',
      observation_status: '',
      rollback_decision: 'pending',
      run_url: '',
    },
    quality: { stage_summary: { requirements: 'needs_confirmation' }, blockers: 0, warnings: 1 },
    next_actions: [
      { label: 'Confirm requirements', command: 'devflow confirm --stage requirements', reason: 'Needs confirmation.' },
    ],
  },
};

describe('HttpApiClient lifecycle mapping', () => {
  it('creates a demand through the real backend contract', async () => {
    const fetchMock = vi.fn().mockResolvedValue(mockResponse(demandPayload, 201));
    const client = new HttpApiClient('', fetchMock as unknown as typeof fetch);

    const detail = await client.createDemand('ws', {
      key: 'coupon-from-ui',
      title: 'Coupon from UI',
      description: 'Inactive users must be blocked',
      source: 'web',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/workspaces/ws/demands',
      expect.objectContaining({ method: 'POST' }),
    );
    expect(detail.demandKey).toBe('coupon-from-ui');
    expect(detail.state).toBe('requirements_review');
    expect(detail.quality.stageSummary?.requirements).toBe('needs_confirmation');
    expect(detail.nextActions[0].label).toContain('Confirm');
  });

  it('creates a workspace against the real backend contract', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      mockResponse(
        { id: 'ws-2', name: 'Payments', artifact_root: '/tmp/root', created_at: '2026-07-08T00:00:00Z' },
        201,
      ),
    );
    const client = new HttpApiClient('', fetchMock as unknown as typeof fetch);

    const ws = await client.createWorkspace({ id: 'ws-2', name: 'Payments', artifactRoot: '/tmp/root' });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/workspaces',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ id: 'ws-2', name: 'Payments', artifact_root: '/tmp/root' }),
      }),
    );
    expect(ws.id).toBe('ws-2');
    expect(ws.artifactRoot).toBe('/tmp/root');
  });

  it('maps a confirmed demand detail from the action result', async () => {
    const confirmed = JSON.parse(JSON.stringify(demandPayload));
    confirmed.status = 'confirmed';
    confirmed.next_state = 'plan_drafting';
    confirmed.demand.state = 'plan_drafting';
    const fetchMock = vi.fn().mockResolvedValue(mockResponse(confirmed));
    const client = new HttpApiClient('', fetchMock as unknown as typeof fetch);

    const detail = await client.confirmDemand('ws', 'coupon-from-ui', {
      stage: 'requirements',
      summary: 'Reviewed',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/workspaces/ws/demands/coupon-from-ui/confirm',
      expect.objectContaining({ method: 'POST' }),
    );
    expect(detail.state).toBe('plan_drafting');
  });

  it('throws when the backend rejects a lifecycle action', async () => {
    const fetchMock = vi.fn().mockResolvedValue(mockResponse({ error: 'confirmation stage "x" requires current state y' }, 400));
    const client = new HttpApiClient('', fetchMock as unknown as typeof fetch);

    await expect(
      client.confirmDemand('ws', 'coupon-from-ui', { stage: 'x', summary: 's' }),
    ).rejects.toThrow(/confirmation stage/);
  });
});
