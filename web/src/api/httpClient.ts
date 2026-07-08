import type { ApiClient } from './client';
import type {
  AddEvidenceInput,
  ArtifactSummary,
  AuditEvent,
  ConfirmDemandInput,
  CreateDemandInput,
  CreateWorkspaceInput,
  CurrentUser,
  DemandDetail,
  DemandState,
  DemandSummary,
  GateStatus,
  PromoteWikiRequest,
  ReleaseControlStatus,
  ReleaseSummary,
  RejectWikiRequest,
  Role,
  RollbackDecision,
  WikiCandidate,
  WikiEntry,
  Workspace,
} from './types';

function normalizeBase(raw: string | undefined): string {
  let base = (raw ?? '').replace(/\/+$/, '');
  base = base.replace(/\/api$/, '');
  return base;
}

const DEFAULT_BASE = normalizeBase(import.meta.env.VITE_DEVFLOW_API_BASE as string | undefined);

// Raw backend response shapes. The backend is the API source of truth and uses
// snake_case JSON; the frontend types use camelCase. The HttpApiClient maps the
// backend contract into the frontend types so the pages and the mock client
// stay unchanged and mock mode keeps working.
interface RawCurrentUser {
  email: string;
  role: string;
  display_name: string;
}
interface RawWorkspace {
  id: string;
  name: string;
  artifact_root: string;
  created_at: string;
}
interface RawArtifactSummary {
  name: string;
  exists: boolean;
}
interface RawDemandSummary {
  demand_key: string;
  title: string;
  state: string;
  attention: string;
  updated_at: string;
  artifacts?: RawArtifactSummary[];
}
interface RawRelease {
  deployment_status: string;
  observation_status: string;
  rollback_decision: string;
  run_url: string;
}
interface RawQuality {
  stage_summary?: Record<string, string>;
  blockers?: number;
  warnings?: number;
}
interface RawNextAction {
  label: string;
  command?: string;
  disabled?: boolean;
  reason?: string;
}
interface RawDemandDetail extends RawDemandSummary {
  description?: string;
  source?: string;
  evidence?: { pass: number; fail: number; blocked: number };
  release?: RawRelease;
  quality?: RawQuality;
  next_actions?: RawNextAction[];
}
interface RawActionResult {
  status: string;
  message: string;
  demand: RawDemandDetail;
  audit_id?: string;
  next_state?: string;
}
interface RawAuditEvent {
  id: string;
  workspace_id: string;
  actor_user_id: string;
  action: string;
  subject_type: string;
  subject_id: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

function mapArtifact(a: RawArtifactSummary): ArtifactSummary {
  return { name: a.name, path: '', present: !!a.exists, size: 0 };
}

function mapWorkspace(r: RawWorkspace): Workspace {
  return { id: r.id, name: r.name, artifactRoot: r.artifact_root, createdAt: r.created_at };
}

function mapCurrentUser(r: RawCurrentUser): CurrentUser {
  return { id: '', email: r.email, displayName: r.display_name || r.email, role: r.role as Role };
}

function mapDemandSummary(r: RawDemandSummary, workspaceId: string): DemandSummary {
  return {
    id: r.demand_key,
    workspaceId,
    demandKey: r.demand_key,
    title: r.title || '',
    state: (r.state || '') as DemandState,
    attention: r.attention || '',
    artifactPath: '',
    updatedAt: r.updated_at,
  };
}

function mapNextAction(a: RawNextAction) {
  return {
    label: a.label,
    command: a.command,
    reason: a.reason,
    disabled: a.disabled ?? false,
  };
}

function mapDemandDetail(r: RawDemandDetail, workspaceId: string): DemandDetail {
  const release = r.release;
  const quality = r.quality;
  return {
    ...mapDemandSummary(r, workspaceId),
    description: r.description ?? '',
    source: r.source ?? '',
    artifacts: (r.artifacts ?? []).map(mapArtifact),
    releaseLine: {
      deploymentStatus: (release?.deployment_status || 'not_started') as ReleaseControlStatus,
      runUrl: release?.run_url || '',
      environment: 'production',
      ref: 'main',
      rollbackDecision: (release?.rollback_decision || 'pending') as RollbackDecision,
      rollbackNeeded: release?.rollback_decision === 'rollback_confirmed',
    },
    quality: {
      gate: 'unknown' as GateStatus,
      checks: [],
      stageSummary: quality?.stage_summary ?? {},
      blockers: quality?.blockers ?? 0,
      warnings: quality?.warnings ?? 0,
    },
    acceptance: [],
    metrics: { adapter: '', status: 'unknown' as GateStatus, metrics: [], summary: '' },
    evidence: r.evidence ?? { pass: 0, fail: 0, blocked: 0 },
    nextActions: (r.next_actions ?? []).map(mapNextAction),
  };
}

function mapAuditEvent(r: RawAuditEvent): AuditEvent {
  return {
    id: r.id,
    workspaceId: r.workspace_id,
    actorUserId: r.actor_user_id,
    actorEmail: '',
    action: r.action,
    subjectType: r.subject_type,
    subjectId: r.subject_id,
    metadata: r.metadata ?? {},
    createdAt: r.created_at,
  };
}

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
  private readonly base: string;
  private readonly fetchImpl: typeof fetch;

  constructor(base?: string, fetchImpl?: typeof fetch) {
    this.base = base ?? DEFAULT_BASE;
    this.fetchImpl = fetchImpl ?? fetch.bind(globalThis);
  }

  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await this.fetchImpl(`${this.base}${path}`, {
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      ...init,
    });
    if (!res.ok) throw await parseError(res);
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  async getCurrentUser(): Promise<CurrentUser> {
    const r = await this.request<RawCurrentUser>('/api/me');
    return mapCurrentUser(r);
  }

  async listWorkspaces(): Promise<Workspace[]> {
    const r = await this.request<RawWorkspace[]>('/api/workspaces');
    return r.map(mapWorkspace);
  }

  async createWorkspace(input: CreateWorkspaceInput): Promise<Workspace> {
    const r = await this.request<RawWorkspace>('/api/workspaces', {
      method: 'POST',
      body: JSON.stringify({
        id: input.id,
        name: input.name,
        artifact_root: input.artifactRoot,
      }),
    });
    return mapWorkspace(r);
  }

  async listDemands(workspaceId: string): Promise<DemandSummary[]> {
    const r = await this.request<RawDemandSummary[]>(`/api/workspaces/${workspaceId}/demands`);
    return r.map((d) => mapDemandSummary(d, workspaceId));
  }

  async getDemand(workspaceId: string, demandKey: string): Promise<DemandDetail> {
    const r = await this.request<RawDemandDetail>(
      `/api/workspaces/${workspaceId}/demands/${demandKey}`,
    );
    return mapDemandDetail(r, workspaceId);
  }

  async createDemand(workspaceId: string, input: CreateDemandInput): Promise<DemandDetail> {
    const r = await this.request<RawActionResult>(
      `/api/workspaces/${encodeURIComponent(workspaceId)}/demands`,
      {
        method: 'POST',
        body: JSON.stringify({
          key: input.key,
          title: input.title,
          description: input.description,
          source: input.source,
        }),
      },
    );
    return mapDemandDetail(r.demand, workspaceId);
  }

  async confirmDemand(
    workspaceId: string,
    demandKey: string,
    input: ConfirmDemandInput,
  ): Promise<DemandDetail> {
    const r = await this.request<RawActionResult>(
      `/api/workspaces/${encodeURIComponent(workspaceId)}/demands/${encodeURIComponent(demandKey)}/confirm`,
      {
        method: 'POST',
        body: JSON.stringify({ stage: input.stage, summary: input.summary }),
      },
    );
    return mapDemandDetail(r.demand, workspaceId);
  }

  async addEvidence(
    workspaceId: string,
    demandKey: string,
    input: AddEvidenceInput,
  ): Promise<DemandDetail> {
    const r = await this.request<RawActionResult>(
      `/api/workspaces/${encodeURIComponent(workspaceId)}/demands/${encodeURIComponent(demandKey)}/evidence`,
      {
        method: 'POST',
        body: JSON.stringify({
          type: input.type,
          criterion: input.criterion,
          status: input.status,
          summary: input.summary,
          source: input.source,
          link: input.link,
        }),
      },
    );
    return mapDemandDetail(r.demand, workspaceId);
  }

  async getArtifact(workspaceId: string, demandKey: string, artifactName: string): Promise<string> {
    const res = await this.fetchImpl(
      `${this.base}/api/workspaces/${workspaceId}/demands/${demandKey}/artifacts/${artifactName}`,
      { headers: { Accept: 'text/plain, application/json' } },
    );
    if (!res.ok) throw await parseError(res);
    return res.text();
  }

  // The wiki and release workflows are not implemented in the platform backend
  // in v1.7. These pages remain mock-backed; in HTTP mode the client returns
  // empty defaults so the pages render empty states instead of erroring.
  async listWikiEntries(_workspaceId: string): Promise<WikiEntry[]> {
    return [];
  }

  async listWikiCandidates(_workspaceId: string): Promise<WikiCandidate[]> {
    return [];
  }

  async getRelease(_workspaceId: string, _demandKey: string): Promise<ReleaseSummary> {
    return {
      id: '',
      workspaceId: '',
      demandKey: '',
      kind: '',
      provider: '',
      status: 'not_started',
      runUrl: '',
      decision: 'pending',
      updatedAt: '',
    };
  }

  // Write actions for wiki/release are mock-only until the backend implements
  // them; throw a clear message so the page surfaces it instead of a 404.
  async promoteWikiCandidate(
    _workspaceId: string,
    _candidateId: string,
    _req: PromoteWikiRequest,
  ): Promise<WikiEntry> {
    throw new Error('wiki promote is not available against the real API yet; use mock mode');
  }

  async rejectWikiCandidate(
    _workspaceId: string,
    _candidateId: string,
    _req: RejectWikiRequest,
  ): Promise<WikiCandidate> {
    throw new Error('wiki reject is not available against the real API yet; use mock mode');
  }

  async triggerRollback(_workspaceId: string, _demandKey: string): Promise<ReleaseSummary> {
    throw new Error('rollback trigger is not available against the real API yet; use mock mode');
  }

  async refreshObservation(_workspaceId: string, _demandKey: string): Promise<DemandDetail> {
    throw new Error('observation refresh is not available against the real API yet; use mock mode');
  }

  async getAuditEvents(workspaceId: string): Promise<AuditEvent[]> {
    const r = await this.request<RawAuditEvent[]>(`/api/workspaces/${workspaceId}/audit`);
    return r.map(mapAuditEvent);
  }
}
