// API types mirror the Devflow platform backend contract defined in
// docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
// Keep these in sync with internal/platform/api/types.go.

export type Role = 'Viewer' | 'Developer' | 'Reviewer' | 'Admin';

export interface CurrentUser {
  id: string;
  email: string;
  displayName: string;
  role: Role;
}

export interface Workspace {
  id: string;
  name: string;
  artifactRoot: string;
  createdAt: string;
}

export type DemandState =
  | 'created'
  | 'context_loaded'
  | 'requirements_drafting'
  | 'requirements_review'
  | 'plan_drafting'
  | 'plan_review'
  | 'implementation'
  | 'mr_review'
  | 'verification'
  | 'deployment'
  | 'observation'
  | 'closeout'
  | 'completed'
  | 'blocked_need_user'
  | 'blocked_need_release_decision'
  | 'blocked_need_platform'
  | 'failed_quality_gate'
  | 'returned_to_requirements'
  | 'returned_to_plan'
  | 'cancelled';

export interface DemandSummary {
  id: string;
  workspaceId: string;
  demandKey: string;
  title: string;
  state: DemandState;
  attention: string;
  artifactPath: string;
  updatedAt: string;
}

export interface ArtifactSummary {
  name: string;
  path: string;
  present: boolean;
  size: number;
}

export type GateStatus = 'pass' | 'fail' | 'blocked' | 'unknown';

export interface QualityCheck {
  name: string;
  status: GateStatus;
  summary: string;
}

export interface QualitySummary {
  gate: GateStatus;
  checks: QualityCheck[];
  stageSummary?: Record<string, string>;
  blockers?: number;
  warnings?: number;
}

export interface AcceptanceEvidence {
  category: string;
  required: number;
  provided: number;
}

export interface MetricValue {
  name: string;
  value: number;
  unit: string;
  threshold: number;
  pass: boolean;
}

export interface MetricsSummary {
  adapter: string;
  status: GateStatus;
  metrics: MetricValue[];
  summary: string;
}

export type ReleaseControlStatus =
  | 'not_started'
  | 'pending'
  | 'passed'
  | 'failed'
  | 'blocked'
  | 'unknown';

export type RollbackDecision =
  | 'pending'
  | 'rollback_confirmed'
  | 'risk_accepted'
  | 'redeploy_required';

export interface ReleaseLine {
  deploymentStatus: ReleaseControlStatus;
  runUrl: string;
  environment: string;
  ref: string;
  rollbackDecision: RollbackDecision;
  rollbackNeeded: boolean;
}

export interface NextAction {
  label: string;
  command?: string;
  reason?: string;
  disabled?: boolean;
}

export interface EvidenceCounts {
  pass: number;
  fail: number;
  blocked: number;
}

export interface DemandDetail extends DemandSummary {
  description?: string;
  source?: string;
  artifacts: ArtifactSummary[];
  releaseLine: ReleaseLine;
  quality: QualitySummary;
  acceptance: AcceptanceEvidence[];
  metrics: MetricsSummary;
  evidence?: EvidenceCounts;
  nextActions: NextAction[];
}

export type WikiCategory = 'business' | 'process' | 'archive';

export type WikiCandidateStatus = 'pending' | 'promoted' | 'rejected';

export interface WikiEntry {
  id: string;
  workspaceId: string;
  name: string;
  category: WikiCategory;
  sourceDemandKey: string;
  artifactPath: string;
  updatedAt: string;
}

export interface WikiCandidate {
  id: string;
  workspaceId: string;
  demandKey: string;
  index: number;
  kind: WikiCategory;
  text: string;
  source: string;
  status: WikiCandidateStatus;
  wikiPath: string;
  reason: string;
}

export interface ReleaseSummary {
  id: string;
  workspaceId: string;
  demandKey: string;
  kind: string;
  provider: string;
  status: ReleaseControlStatus;
  runUrl: string;
  decision: RollbackDecision;
  updatedAt: string;
}

export interface AuditEvent {
  id: string;
  workspaceId: string;
  actorUserId: string;
  actorEmail: string;
  action: string;
  subjectType: string;
  subjectId: string;
  metadata: Record<string, unknown>;
  createdAt: string;
}

export interface PromoteWikiRequest {
  name: string;
  category: WikiCategory;
}

export interface RejectWikiRequest {
  reason: string;
}

export interface CreateWorkspaceInput {
  id: string;
  name: string;
  artifactRoot: string;
}

export interface CreateDemandInput {
  key: string;
  title: string;
  description: string;
  source: string;
}

export interface ConfirmDemandInput {
  stage: string;
  summary: string;
}

export interface AddEvidenceInput {
  type: 'manual' | 'api' | 'link';
  criterion: string;
  status: 'pass' | 'fail' | 'blocked';
  summary: string;
  source?: string;
  link?: string;
}

export interface ApiError {
  error: string;
  detail?: string;
}
