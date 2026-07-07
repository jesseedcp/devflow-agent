import type {
  ApiClient,
} from './client';
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
  private wikiEntries: WikiEntry[];
  private candidates: WikiCandidate[];
  private releases: Record<string, ReleaseSummary>;
  private audit: AuditEvent[];
  private user: CurrentUser;

  constructor() {
    this.demands = structuredClone(mockDemandDetails);
    this.wikiEntries = structuredClone(mockWikiEntries);
    this.candidates = structuredClone(mockWikiCandidates);
    this.releases = structuredClone(mockReleaseSummaries);
    this.audit = structuredClone(mockAuditEvents);
    this.user = structuredClone(mockCurrentUser);
  }

  setCurrentRole(role: CurrentUser['role']): void {
    this.user = { ...this.user, role };
  }

  async getCurrentUser(): Promise<CurrentUser> {
    await delay(40);
    return structuredClone(this.user);
  }

  async listWorkspaces(): Promise<Workspace[]> {
    await delay(60);
    return structuredClone(mockWorkspaces);
  }

  async listDemands(_workspaceId: string): Promise<DemandSummary[]> {
    await delay(80);
    return structuredClone(mockDemandSummaries);
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
