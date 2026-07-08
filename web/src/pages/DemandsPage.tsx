import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { StatusBadge } from '../components/StatusBadge';
import { demandStateTone, formatDateTime, titleCase } from '../utils/format';

export function DemandsPage() {
  const { workspaceId = '' } = useParams();
  const { client } = useApp();
  const navigate = useNavigate();
  const { data, loading, error, reload } = useAsync(
    () => client.listDemands(workspaceId),
    [client, workspaceId],
  );

  const [adding, setAdding] = useState(false);
  const [key, setKey] = useState('');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState('');

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!key.trim() || !title.trim()) return;
    setBusy(true);
    setActionError('');
    try {
      await client.createDemand(workspaceId, {
        key: key.trim(),
        title: title.trim(),
        description: description.trim(),
        source: 'web',
      });
      setKey('');
      setTitle('');
      setDescription('');
      setAdding(false);
      reload();
      navigate(`/workspaces/${workspaceId}/demands/${key.trim()}`);
    } catch (err) {
      setActionError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page">
      <PageHeader
        title="Demands"
        subtitle="Delivery demands in this workspace."
        actions={
          <button
            type="button"
            className="btn btn-primary btn-sm"
            onClick={() => setAdding((v) => !v)}
          >
            新建需求
          </button>
        }
      />

      {adding && (
        <form className="card" onSubmit={submit}>
          <h2 className="card-title">创建需求</h2>
          <div className="form-row">
            <label className="field">
              <span className="small muted">需求 Key</span>
              <input
                className="input mono"
                value={key}
                onChange={(e) => setKey(e.target.value)}
                placeholder="coupon-eligibility"
                disabled={busy}
              />
            </label>
            <label className="field grow">
              <span className="small muted">标题</span>
              <input
                className="input"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Block inactive users from claiming coupons"
                disabled={busy}
              />
            </label>
          </div>
          <label className="field">
            <span className="small muted">描述</span>
            <textarea
              className="input"
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Inactive users must be blocked from claiming coupons."
              disabled={busy}
            />
          </label>
          <div className="form-row">
            <button type="submit" className="btn btn-primary" disabled={busy || !key.trim() || !title.trim()}>
              {busy ? '创建中…' : '创建需求'}
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
                <td className={d.attention ? 'attention' : 'muted'}>{d.attention || '-'}</td>
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
