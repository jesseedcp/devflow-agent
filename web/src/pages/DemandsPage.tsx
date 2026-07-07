import { Link, useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { StatusBadge } from '../components/StatusBadge';
import { demandStateTone, formatDateTime, titleCase } from '../utils/format';

export function DemandsPage() {
  const { workspaceId = '' } = useParams();
  const { client } = useApp();
  const { data, loading, error, reload } = useAsync(
    () => client.listDemands(workspaceId),
    [client, workspaceId],
  );

  return (
    <div className="page">
      <PageHeader title="Demands" subtitle="Delivery demands in this workspace." />
      {loading && <Loading />}
      {error && <ErrorState message={error} onRetry={reload} />}
      {data && (
        <table className="table">
          <thead>
            <tr>
              <th>Demand</th>
              <th>State</th>
              <th>Attention</th>
              <th>Updated</th>
              <th>Release</th>
            </tr>
          </thead>
          <tbody>
            {data.length === 0 && (
              <tr>
                <td colSpan={5} className="muted">No demands.</td>
              </tr>
            )}
            {data.map((d) => (
              <tr key={d.id}>
                <td>
                  <Link to={`/workspaces/${workspaceId}/demands/${d.demandKey}`} className="link">
                    {d.title}
                  </Link>
                  <div className="mono small muted">{d.demandKey}</div>
                </td>
                <td>
                  <StatusBadge label={titleCase(d.state)} tone={demandStateTone(d.state)} />
                </td>
                <td className={d.attention ? 'attention' : 'muted'}>{d.attention || '—'}</td>
                <td>{formatDateTime(d.updatedAt)}</td>
                <td>
                  <Link to={`/workspaces/${workspaceId}/release/${d.demandKey}`} className="btn btn-ghost btn-sm">
                    Release
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
