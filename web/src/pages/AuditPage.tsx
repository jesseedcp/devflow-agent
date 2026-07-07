import { useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { StatusBadge } from '../components/StatusBadge';
import { formatDateTime } from '../utils/format';

const ACTION_TONE: Record<string, string> = {
  'demand.created': 'tone-info',
  'plan.confirmed': 'tone-good',
  'release.deploy_triggered': 'tone-info',
  'wiki.promoted': 'tone-good',
  'wiki.rejected': 'tone-warn',
  'rollback.pending': 'tone-warn',
  'rollback.triggered': 'tone-bad',
  'observation.refreshed': 'tone-info',
};

export function AuditPage() {
  const { workspaceId = '' } = useParams();
  const { client } = useApp();
  const { data, loading, error, reload } = useAsync(
    () => client.getAuditEvents(workspaceId),
    [client, workspaceId],
  );

  return (
    <div className="page">
      <PageHeader title="Audit" subtitle="High-risk action timeline for this workspace." />
      {loading && <Loading />}
      {error && <ErrorState message={error} onRetry={reload} />}
      {data && (
        <ol className="timeline">
          {data.length === 0 && <li className="muted">No audit events.</li>}
          {data.map((ev) => (
            <li key={ev.id} className="timeline-item">
              <div className="timeline-time">{formatDateTime(ev.createdAt)}</div>
              <div className="timeline-body">
                <StatusBadge label={ev.action} tone={ACTION_TONE[ev.action] ?? 'tone-info'} />
                <span className="mono small muted"> {ev.subjectType}:{ev.subjectId}</span>
                <div className="muted small">by {ev.actorEmail}</div>
                {Object.keys(ev.metadata).length > 0 && (
                  <details className="metadata">
                    <summary className="small muted">metadata</summary>
                    <pre className="mono small">{JSON.stringify(ev.metadata, null, 2)}</pre>
                  </details>
                )}
              </div>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
