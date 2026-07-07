import type {
  AcceptanceEvidence,
  ArtifactSummary,
  AuditEvent,
  CurrentUser,
  DemandDetail,
  DemandSummary,
  MetricsSummary,
  PromoteWikiRequest,
  QualitySummary,
  ReleaseLine,
  ReleaseSummary,
  RejectWikiRequest,
  WikiCandidate,
  WikiEntry,
  Workspace,
} from './types';

export interface ApiClient {
  getCurrentUser(): Promise<CurrentUser>;
  listWorkspaces(): Promise<Workspace[]>;
  listDemands(workspaceId: string): Promise<DemandSummary[]>;
  getDemand(workspaceId: string, demandKey: string): Promise<DemandDetail>;
  getArtifact(workspaceId: string, demandKey: string, artifactName: string): Promise<string>;
  listWikiEntries(workspaceId: string): Promise<WikiEntry[]>;
  listWikiCandidates(workspaceId: string): Promise<WikiCandidate[]>;
  promoteWikiCandidate(
    workspaceId: string,
    candidateId: string,
    req: PromoteWikiRequest,
  ): Promise<WikiEntry>;
  rejectWikiCandidate(
    workspaceId: string,
    candidateId: string,
    req: RejectWikiRequest,
  ): Promise<WikiCandidate>;
  getRelease(workspaceId: string, demandKey: string): Promise<ReleaseSummary>;
  triggerRollback(workspaceId: string, demandKey: string): Promise<ReleaseSummary>;
  refreshObservation(workspaceId: string, demandKey: string): Promise<DemandDetail>;
  getAuditEvents(workspaceId: string): Promise<AuditEvent[]>;
}

export function createMockArtifact(
  name: string,
  present: boolean,
  size = 0,
): ArtifactSummary {
  return { name, path: `.devflow/demands/<key>/${name}`, present, size };
}

export function createReleaseLine(
  deploymentStatus: ReleaseLine['deploymentStatus'],
  runUrl: string,
  rollbackDecision: ReleaseLine['rollbackDecision'],
  rollbackNeeded: boolean,
  environment = 'production',
  ref = 'main',
): ReleaseLine {
  return { deploymentStatus, runUrl, environment, ref, rollbackDecision, rollbackNeeded };
}

export function createQuality(
  gate: QualitySummary['gate'],
  checks: QualitySummary['checks'],
): QualitySummary {
  return { gate, checks };
}

export function createAcceptance(rows: AcceptanceEvidence[]): AcceptanceEvidence[] {
  return rows;
}

export function createMetrics(summary: MetricsSummary): MetricsSummary {
  return summary;
}
