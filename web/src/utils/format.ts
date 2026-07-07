import type { DemandState, GateStatus, ReleaseControlStatus, Role } from '../api/types';

export function formatDateTime(iso: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function titleCase(input: string): string {
  return input
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export const ROLE_RANK: Record<Role, number> = {
  Viewer: 0,
  Developer: 1,
  Reviewer: 2,
  Admin: 3,
};

export function hasRole(actual: Role, required: Role): boolean {
  return ROLE_RANK[actual] >= ROLE_RANK[required];
}

export function demandStateTone(state: DemandState): string {
  if (state === 'completed') return 'tone-good';
  if (state.startsWith('blocked') || state === 'failed_quality_gate') return 'tone-bad';
  if (state.endsWith('_review') || state === 'observation' || state === 'deployment') return 'tone-warn';
  return 'tone-info';
}

export function gateTone(status: GateStatus | ReleaseControlStatus): string {
  switch (status) {
    case 'pass':
    case 'passed':
      return 'tone-good';
    case 'fail':
    case 'failed':
      return 'tone-bad';
    case 'blocked':
    case 'pending':
      return 'tone-warn';
    default:
      return 'tone-info';
  }
}
