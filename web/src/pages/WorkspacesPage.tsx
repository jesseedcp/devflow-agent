import { Link } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { formatDateTime } from '../utils/format';

export function WorkspacesPage() {
  const { client } = useApp();
  const { data, loading, error, reload } = useAsync(() => client.listWorkspaces(), [client]);

  return (
    <div className="page">
      <PageHeader title="Workspaces" subtitle="Select a workspace to view its demand delivery loop." />
      {loading && <Loading />}
      {error && <ErrorState message={error} onRetry={reload} />}
      {data && (
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Artifact root</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            {data.length === 0 && (
              <tr>
                <td colSpan={3} className="muted">No workspaces.</td>
              </tr>
            )}
            {data.map((ws) => (
              <tr key={ws.id}>
                <td>
                  <Link to={`/workspaces/${ws.id}/demands`} className="link">
                    {ws.name}
                  </Link>
                </td>
                <td className="mono">{ws.artifactRoot}</td>
                <td>{formatDateTime(ws.createdAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
