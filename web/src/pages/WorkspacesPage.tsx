import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { formatDateTime } from '../utils/format';

export function WorkspacesPage() {
  const { client } = useApp();
  const { data, loading, error, reload } = useAsync(() => client.listWorkspaces(), [client]);

  const [adding, setAdding] = useState(false);
  const [id, setId] = useState('');
  const [name, setName] = useState('');
  const [root, setRoot] = useState('');
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState('');

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!id.trim() || !name.trim() || !root.trim()) return;
    setBusy(true);
    setActionError('');
    try {
      await client.createWorkspace({ id: id.trim(), name: name.trim(), artifactRoot: root.trim() });
      setId('');
      setName('');
      setRoot('');
      setAdding(false);
      reload();
    } catch (err) {
      setActionError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page">
      <PageHeader
        title="Workspaces"
        subtitle="Select a workspace to view its demand delivery loop."
        actions={
          <button
            type="button"
            className="btn btn-primary btn-sm"
            onClick={() => setAdding((v) => !v)}
          >
            新建工作区
          </button>
        }
      />

      {adding && (
        <form className="card" onSubmit={submit}>
          <h2 className="card-title">创建工作区</h2>
          <div className="form-row">
            <label className="field">
              <span className="small muted">工作区 ID</span>
              <input
                className="input"
                value={id}
                onChange={(e) => setId(e.target.value)}
                placeholder="ws-payments"
                disabled={busy}
              />
            </label>
            <label className="field grow">
              <span className="small muted">名称</span>
              <input
                className="input"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Payments Platform"
                disabled={busy}
              />
            </label>
          </div>
          <label className="field">
            <span className="small muted">Artifact Root</span>
            <input
              className="input mono"
              value={root}
              onChange={(e) => setRoot(e.target.value)}
              placeholder="/path/to/artifacts"
              disabled={busy}
            />
          </label>
          <div className="form-row">
            <button type="submit" className="btn btn-primary" disabled={busy || !id.trim() || !name.trim() || !root.trim()}>
              {busy ? '创建中…' : '创建工作区'}
            </button>
            <button type="button" className="btn btn-ghost" onClick={() => setAdding(false)} disabled={busy}>
              取消
            </button>
          </div>
          {actionError && <p className="error-text small">{actionError}</p>}
        </form>
      )}

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
