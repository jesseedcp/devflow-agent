import type { ApiClient } from './client';
import type {
  AuditEvent,
  CurrentUser,
  DemandDetail,
  DemandSummary,
  PromoteWikiRequest,
  ReleaseSummary,
  RejectWikiRequest,
  WikiCandidate,
  WikiEntry,
  Workspace,
} from './types';

const BASE = (import.meta.env.VITE_DEVFLOW_API_BASE as string | undefined)?.replace(/\/$/, '') ?? '';

async function parseError(res: Response): Promise<Error> {
  let detail = '';
  try {
    const body = (await res.json()) as { error?: string; detail?: string };
    detail = body.detail ? `${body.error}: ${body.detail}` : body.error ?? '';
  } catch {
    detail = await res.text().catch(() => '');
  }
  return new Error(detail || `request failed: ${res.status} ${res.statusText}`);
}

export class HttpApiClient implements ApiClient {
  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await fetch(`${BASE}${path}`, {
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      ...init,
    });
    if (!res.ok) throw await parseError(res);
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  async getCurrentUser(): Promise<CurrentUser> {
    return this.request<CurrentUser>('/api/me');
  }

  async listWorkspaces(): Promise<Workspace[]> {
    return this.request<Workspace[]>('/api/workspaces');
  }

  async listDemands(workspaceId: string): Promise<DemandSummary[]> {
    return this.request<DemandSummary[]>(`/api/workspaces/${workspaceId}/demands`);
  }

  async getDemand(workspaceId: string, demandKey: string): Promise<DemandDetail> {
    return this.request<DemandDetail>(`/api/workspaces/${workspaceId}/demands/${demandKey}`);
  }

  async getArtifact(workspaceId: string, demandKey: string, artifactName: string): Promise<string> {
    const res = await fetch(
      `${BASE}/api/workspaces/${workspaceId}/demands/${demandKey}/artifacts/${artifactName}`,
      { headers: { Accept: 'text/plain, application/json' } },
    );
    if (!res.ok) throw await parseError(res);
    return res.text();
  }

  async listWikiEntries(workspaceId: string): Promise<WikiEntry[]> {
    return this.request<WikiEntry[]>(`/api/workspaces/${workspaceId}/wiki`);
  }

  async listWikiCandidates(workspaceId: string): Promise<WikiCandidate[]> {
    return this.request<WikiCandidate[]>(`/api/workspaces/${workspaceId}/wiki/candidates`);
  }

  async promoteWikiCandidate(
    workspaceId: string,
    candidateId: string,
    req: PromoteWikiRequest,
  ): Promise<WikiEntry> {
    return this.request<WikiEntry>(
      `/api/workspaces/${workspaceId}/wiki/candidates/${candidateId}/promote`,
      { method: 'POST', body: JSON.stringify(req) },
    );
  }

  async rejectWikiCandidate(
    workspaceId: string,
    candidateId: string,
    req: RejectWikiRequest,
  ): Promise<WikiCandidate> {
    return this.request<WikiCandidate>(
      `/api/workspaces/${workspaceId}/wiki/candidates/${candidateId}/reject`,
      { method: 'POST', body: JSON.stringify(req) },
    );
  }

  async getRelease(workspaceId: string, demandKey: string): Promise<ReleaseSummary> {
    return this.request<ReleaseSummary>(`/api/workspaces/${workspaceId}/release/${demandKey}`);
  }

  async triggerRollback(workspaceId: string, demandKey: string): Promise<ReleaseSummary> {
    return this.request<ReleaseSummary>(
      `/api/workspaces/${workspaceId}/release/${demandKey}/rollback/trigger`,
      { method: 'POST' },
    );
  }

  async refreshObservation(workspaceId: string, demandKey: string): Promise<DemandDetail> {
    return this.request<DemandDetail>(
      `/api/workspaces/${workspaceId}/release/${demandKey}/observe`,
      { method: 'POST' },
    );
  }

  async getAuditEvents(workspaceId: string): Promise<AuditEvent[]> {
    return this.request<AuditEvent[]>(`/api/workspaces/${workspaceId}/audit`);
  }
}
