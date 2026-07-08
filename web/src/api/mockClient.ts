import type {
  ApiClient,
} from './client';
import type {
  AddEvidenceInput,
  AuditEvent,
  ConfirmDemandInput,
  CreateDemandInput,
  CreateWorkspaceInput,
  CurrentUser,
  DemandDetail,
  DemandState,
  DemandSummary,
  PromoteWikiRequest,
  ReleaseSummary,
  RejectWikiRequest,
  WikiCandidate,
  WikiEntry,
  Workspace,
} from './types';
import {
  mockArtifactContent,
  mockDemandDetails,
  mockDemandSummaries,
  mockReleaseSummaries,
  mockWikiCandidates,
} from '../mocks/demands';
import {
  mockAuditEvents,
  mockCurrentUser,
  mockWikiEntries,
  mockWorkspaces,
} from '../mocks/workspaces';

const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

export class MockApiClient implements ApiClient {
  private demands: Record<string, DemandDetail>;
  private demandSummaries: DemandSummary[];
  private workspaces: Workspace[];
  private wikiEntries: WikiEntry[];
  private candidates: WikiCandidate[];
  private releases: Record<string, ReleaseSummary>;
  private audit: AuditEvent[];
  private user: CurrentUser;

  constructor() {
    this.demands = structuredClone(mockDemandDetails);
    this.demandSummaries = structuredClone(mockDemandSummaries);
    this.workspaces = structuredClone(mockWorkspaces);
    this.wikiEntries = structuredClone(mockWikiEntries);
    this.candidates = structuredClone(mockWikiCandidates);
    this.releases = structuredClone(mockReleaseSummaries);
    this.audit = structuredClone(mockAuditEvents);
    this.user = structuredClone(mockCurrentUser);
  }

  setCurrentRole(role: CurrentUser['role']): void {
    this.user = { ...this.user, role };
  }

  reset(): void {
    this.demands = structuredClone(mockDemandDetails);
    this.demandSummaries = structuredClone(mockDemandSummaries);
    this.workspaces = structuredClone(mockWorkspaces);
    this.wikiEntries = structuredClone(mockWikiEntries);
    this.candidates = structuredClone(mockWikiCandidates);
    this.releases = structuredClone(mockReleaseSummaries);
    this.audit = structuredClone(mockAuditEvents);
    this.user = structuredClone(mockCurrentUser);
  }

  async getCurrentUser(): Promise<CurrentUser> {
    await delay(40);
    return structuredClone(this.user);
  }

  async listWorkspaces(): Promise<Workspace[]> {
    await delay(60);
    return structuredClone(this.workspaces);
  }

  async createWorkspace(input: CreateWorkspaceInput): Promise<Workspace> {
    await delay(90);
    const ws: Workspace = {
      id: input.id,
      name: input.name,
      artifactRoot: input.artifactRoot,
      createdAt: new Date().toISOString(),
    };
    this.workspaces.push(ws);
    return structuredClone(ws);
  }

  async listDemands(_workspaceId: string): Promise<DemandSummary[]> {
    await delay(80);
    return structuredClone(this.demandSummaries);
  }

  async createDemand(workspaceId: string, input: CreateDemandInput): Promise<DemandDetail> {
    await delay(90);
    const now = new Date().toISOString();
    const summary: DemandSummary = {
      id: input.key,
      workspaceId,
      demandKey: input.key,
      title: input.title,
      state: 'requirements_review',
      attention: 'ready to confirm requirements',
      artifactPath: `.devflow/demands/${input.key}`,
      updatedAt: now,
    };
    this.demandSummaries.push(summary);
    const detail: DemandDetail = {
      ...summary,
      description: input.description,
      source: input.source,
      artifacts: [],
      releaseLine: {
        deploymentStatus: 'not_started',
        runUrl: '',
        environment: 'production',
        ref: 'main',
        rollbackDecision: 'pending',
        rollbackNeeded: false,
      },
      quality: { gate: 'blocked', checks: [], stageSummary: { requirements: 'needs_confirmation' }, blockers: 0, warnings: 1 },
      acceptance: [],
      metrics: { adapter: '', status: 'unknown', metrics: [], summary: '' },
      evidence: { pass: 0, fail: 0, blocked: 0 },
      nextActions: [
        { label: 'Confirm requirements', command: `devflow confirm --demand ${input.key} --stage requirements`, reason: 'Requirements need human confirmation before planning.' },
      ],
    };
    this.demands[input.key] = detail;
    this.audit.unshift({
      id: `audit-${Date.now()}`,
      workspaceId,
      actorUserId: this.user.id,
      actorEmail: this.user.email,
      action: 'create_demand',
      subjectType: 'demand',
      subjectId: input.key,
      metadata: { title: input.title, source: input.source },
      createdAt: now,
    });
    return structuredClone(detail);
  }

  async confirmDemand(workspaceId: string, demandKey: string, input: ConfirmDemandInput): Promise<DemandDetail> {
    await delay(90);
    const detail = this.demands[demandKey];
    if (!detail) throw new Error(`demand not found: ${demandKey}`);
    const next = mockNextState(detail.state, input.stage);
    if (!next) throw new Error(`stage ${input.stage} is not confirmable in state ${detail.state}`);
    detail.state = next as DemandState;
    const summary = this.demandSummaries.find((d) => d.demandKey === demandKey);
    if (summary) summary.state = next as DemandState;
    const now = new Date().toISOString();
    this.audit.unshift({
      id: `audit-${Date.now()}`,
      workspaceId,
      actorUserId: this.user.id,
      actorEmail: this.user.email,
      action: 'confirm_gate',
      subjectType: 'demand',
      subjectId: demandKey,
      metadata: { stage: input.stage, summary: input.summary, next_state: next },
      createdAt: now,
    });
    return structuredClone(detail);
  }

  async addEvidence(workspaceId: string, demandKey: string, input: AddEvidenceInput): Promise<DemandDetail> {
    await delay(90);
    const detail = this.demands[demandKey];
    if (!detail) throw new Error(`demand not found: ${demandKey}`);
    const evidence = detail.evidence ?? { pass: 0, fail: 0, blocked: 0 };
    if (input.status === 'pass') evidence.pass += 1;
    if (input.status === 'fail') evidence.fail += 1;
    if (input.status === 'blocked') evidence.blocked += 1;
    detail.evidence = evidence;
    const now = new Date().toISOString();
    this.audit.unshift({
      id: `audit-${Date.now()}`,
      workspaceId,
      actorUserId: this.user.id,
      actorEmail: this.user.email,
      action: 'add_evidence',
      subjectType: 'demand',
      subjectId: demandKey,
      metadata: { type: input.type, criterion: input.criterion, status: input.status },
      createdAt: now,
    });
    return structuredClone(detail);
  }

  async getDemand(_workspaceId: string, demandKey: string): Promise<DemandDetail> {
    await delay(80);
    const detail = this.demands[demandKey];
    if (!detail) throw new Error(`demand not found: ${demandKey}`);
    return structuredClone(detail);
  }

  async getArtifact(_workspaceId: string, demandKey: string, artifactName: string): Promise<string> {
    await delay(60);
    const key = `${demandKey}/${artifactName}`;
    const cached = mockArtifactContent[key];
    if (cached) return cached;
    return `# ${artifactName}\n\nArtifact content for ${demandKey}/${artifactName} is not available in mock mode.`;
  }

  async listWikiEntries(_workspaceId: string): Promise<WikiEntry[]> {
    await delay(60);
    return structuredClone(this.wikiEntries);
  }

  async listWikiCandidates(_workspaceId: string): Promise<WikiCandidate[]> {
    await delay(60);
    return structuredClone(this.candidates);
  }

  async promoteWikiCandidate(
    workspaceId: string,
    candidateId: string,
    req: PromoteWikiRequest,
  ): Promise<WikiEntry> {
    await delay(90);
    const candidate = this.candidates.find((c) => c.id === candidateId);
    if (!candidate) throw new Error(`candidate not found: ${candidateId}`);
    if (candidate.status !== 'pending') {
      throw new Error(`candidate ${candidateId} is already ${candidate.status}`);
    }
    const now = new Date().toISOString();
    const entry: WikiEntry = {
      id: `wiki-${req.name}`,
      workspaceId,
      name: req.name,
      category: req.category,
      sourceDemandKey: candidate.demandKey,
      artifactPath: `.devflow/wiki/${req.name}.md`,
      updatedAt: now,
    };
    this.wikiEntries.push(entry);
    candidate.status = 'promoted';
    candidate.wikiPath = entry.artifactPath;
    this.audit.unshift({
      id: `audit-${Date.now()}`,
      workspaceId,
      actorUserId: this.user.id,
      actorEmail: this.user.email,
      action: 'wiki.promoted',
      subjectType: 'wiki_candidate',
      subjectId: candidateId,
      metadata: { name: req.name, category: req.category },
      createdAt: now,
    });
    return structuredClone(entry);
  }

  async rejectWikiCandidate(
    workspaceId: string,
    candidateId: string,
    req: RejectWikiRequest,
  ): Promise<WikiCandidate> {
    await delay(90);
    const candidate = this.candidates.find((c) => c.id === candidateId);
    if (!candidate) throw new Error(`candidate not found: ${candidateId}`);
    if (candidate.status !== 'pending') {
      throw new Error(`candidate ${candidateId} is already ${candidate.status}`);
    }
    const now = new Date().toISOString();
    candidate.status = 'rejected';
    candidate.reason = req.reason;
    this.audit.unshift({
      id: `audit-${Date.now()}`,
      workspaceId,
      actorUserId: this.user.id,
      actorEmail: this.user.email,
      action: 'wiki.rejected',
      subjectType: 'wiki_candidate',
      subjectId: candidateId,
      metadata: { reason: req.reason },
      createdAt: now,
    });
    return structuredClone(candidate);
  }

  async getRelease(_workspaceId: string, demandKey: string): Promise<ReleaseSummary> {
    await delay(60);
    const release = this.releases[demandKey];
    if (!release) throw new Error(`release not found: ${demandKey}`);
    return structuredClone(release);
  }

  async triggerRollback(workspaceId: string, demandKey: string): Promise<ReleaseSummary> {
    await delay(120);
    const release = this.releases[demandKey];
    if (!release) throw new Error(`release not found: ${demandKey}`);
    if (release.kind !== 'rollback' && !this.demands[demandKey]?.releaseLine.rollbackNeeded) {
      throw new Error(`rollback not needed for ${demandKey}`);
    }
    const now = new Date().toISOString();
    release.status = 'pending';
    release.decision = 'rollback_confirmed';
    release.runUrl = `https://github.com/org/payments/actions/runs/rollback-${Date.now()}`;
    release.updatedAt = now;
    this.audit.unshift({
      id: `audit-${Date.now()}`,
      workspaceId,
      actorUserId: this.user.id,
      actorEmail: this.user.email,
      action: 'rollback.triggered',
      subjectType: 'demand',
      subjectId: demandKey,
      metadata: { run_url: release.runUrl },
      createdAt: now,
    });
    return structuredClone(release);
  }

  async refreshObservation(workspaceId: string, demandKey: string): Promise<DemandDetail> {
    await delay(120);
    const detail = this.demands[demandKey];
    if (!detail) throw new Error(`demand not found: ${demandKey}`);
    const now = new Date().toISOString();
    this.audit.unshift({
      id: `audit-${Date.now()}`,
      workspaceId,
      actorUserId: this.user.id,
      actorEmail: this.user.email,
      action: 'observation.refreshed',
      subjectType: 'demand',
      subjectId: demandKey,
      metadata: { adapter: detail.metrics.adapter },
      createdAt: now,
    });
    return structuredClone(detail);
  }

  async getAuditEvents(_workspaceId: string): Promise<AuditEvent[]> {
    await delay(60);
    return structuredClone(this.audit);
  }
}


function mockNextState(state: string, stage: string): string | null {
  if (state === 'requirements_review' && stage === 'requirements') return 'plan_drafting';
  if (state === 'plan_review' && stage === 'plan') return 'implementation';
  if (state === 'verification' && stage === 'verification') return 'deployment';
  if (state === 'closeout' && stage === 'closeout') return 'completed';
  return null;
}
