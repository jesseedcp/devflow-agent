import { Link, useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { StatusBadge } from '../components/StatusBadge';
import { formatDateTime, titleCase } from '../utils/format';

export function WikiPage() {
  const { workspaceId = '' } = useParams();
  const { client, isMock } = useApp();
  const { data, loading, error, reload } = useAsync(
    () => client.listWikiEntries(workspaceId),
    [client, workspaceId],
  );

  return (
    <div className="page">
      <PageHeader
        title="Wiki Library"
        subtitle="Promoted internal knowledge entries."
        actions={
          <Link to={`/workspaces/${workspaceId}/wiki/candidates`} className="btn btn-ghost btn-sm">
            Review candidates →
          </Link>
        }
      />
      {!isMock && (
        <p className="muted">当前 HTTP 后端暂未开放 Wiki 接口，请先使用 CLI 或 mock mode 演示。</p>
      )}
      {loading && <Loading />}
      {error && <ErrorState message={error} onRetry={reload} />}
      {data && (
        <table className="table">
          <thead>
            <tr><th>Name</th><th>Category</th><th>Source demand</th><th>Path</th><th>Updated</th></tr>
          </thead>
          <tbody>
            {data.length === 0 && (
              <tr><td colSpan={5} className="muted">No promoted wiki entries yet.</td></tr>
            )}
            {data.map((e) => (
              <tr key={e.id}>
                <td className="mono">{e.name}</td>
                <td><StatusBadge label={titleCase(e.category)} tone="tone-info" /></td>
                <td className="mono">{e.sourceDemandKey}</td>
                <td className="mono small">{e.artifactPath}</td>
                <td>{formatDateTime(e.updatedAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
