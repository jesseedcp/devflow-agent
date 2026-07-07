import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { StatusBadge } from '../components/StatusBadge';
import { gateTone, hasRole, titleCase } from '../utils/format';

export function ReleasePage() {
  const { workspaceId = '', demandKey = '' } = useParams();
  const { client, role } = useApp();
  const release = useAsync(() => client.getRelease(workspaceId, demandKey), [client, workspaceId, demandKey]);
  const demand = useAsync(() => client.getDemand(workspaceId, demandKey), [client, workspaceId, demandKey]);

  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState('');
  const admin = hasRole(role, 'Admin');
  const developer = hasRole(role, 'Developer');

  async function triggerRollback() {
    setBusy(true);
    setActionError('');
    try {
      await client.triggerRollback(workspaceId, demandKey);
      release.reload();
      demand.reload();
    } catch (err) {
      setActionError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function refreshObservation() {
    setBusy(true);
    setActionError('');
    try {
      await client.refreshObservation(workspaceId, demandKey);
      demand.reload();
    } catch (err) {
      setActionError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (release.loading || demand.loading) return <div className="page"><Loading /></div>;
  if (release.error) return <div className="page"><ErrorState message={release.error} onRetry={release.reload} /></div>;
  if (demand.error) return <div className="page"><ErrorState message={demand.error} onRetry={demand.reload} /></div>;
  if (!release.data || !demand.data) return null;

  const r = release.data;
  const d = demand.data;
  const rl = d.releaseLine;
  const rollbackBlocked = !rl.rollbackNeeded;

  return (
    <div className="page">
      <PageHeader
        title={`Release · ${demandKey}`}
        subtitle={d.title}
        actions={
          <Link to={`/workspaces/${workspaceId}/demands/${demandKey}`} className="btn btn-ghost btn-sm">
            ← Demand
          </Link>
        }
      />

      <section className="card">
        <h2 className="card-title">Release operation</h2>
        <dl className="kv">
          <dt>Kind</dt>
          <dd>{titleCase(r.kind)}</dd>
          <dt>Provider</dt>
          <dd className="mono">{r.provider}</dd>
          <dt>Status</dt>
          <dd><StatusBadge label={titleCase(r.status)} tone={gateTone(r.status)} /></dd>
          <dt>Decision</dt>
          <dd><StatusBadge label={titleCase(r.decision)} tone={rl.rollbackNeeded ? 'tone-warn' : 'tone-info'} /></dd>
          <dt>Run</dt>
          <dd>{r.runUrl ? <a className="link" href={r.runUrl} target="_blank" rel="noreferrer">{r.runUrl}</a> : <span className="muted">—</span>}</dd>
        </dl>
      </section>

      <section className="card">
        <h2 className="card-title">Rollback</h2>
        {rl.rollbackNeeded ? (
          <div className="rollback-warn">
            <p className="attention">⚠ Rollback recommended: observation failed.</p>
            <button
              type="button"
              className="btn btn-danger"
              onClick={triggerRollback}
              disabled={!admin || busy}
              title={admin ? 'Trigger GitHub Actions rollback workflow' : 'Requires Admin role'}
            >
              {busy ? 'Working…' : 'Trigger rollback'}
            </button>
            {!admin && <p className="muted small">Requires Admin role to trigger rollback. Human gate is mandatory.</p>}
          </div>
        ) : (
          <p className="muted">No rollback needed. Deployment status: {titleCase(rl.deploymentStatus)}.</p>
        )}
      </section>

      <section className="card">
        <h2 className="card-title">
          Observation <StatusBadge label={titleCase(d.metrics.status)} tone={gateTone(d.metrics.status)} />
        </h2>
        {d.metrics.metrics.length === 0 ? (
          <p className="muted">{d.metrics.summary}</p>
        ) : (
          <table className="table compact">
            <thead>
              <tr><th>Metric</th><th>Value</th><th>Threshold</th><th>Pass</th></tr>
            </thead>
            <tbody>
              {d.metrics.metrics.map((m) => (
                <tr key={m.name}>
                  <td className="mono">{m.name}</td>
                  <td>{m.value} {m.unit}</td>
                  <td>{m.threshold} {m.unit}</td>
                  <td><StatusBadge label={m.pass ? 'pass' : 'fail'} tone={m.pass ? 'tone-good' : 'tone-bad'} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <p className="muted small">{d.metrics.summary}</p>
        <div className="form-row">
          <button
            type="button"
            className="btn btn-ghost"
            onClick={refreshObservation}
            disabled={!developer || busy}
            title={developer ? 'Refresh observation evidence' : 'Requires Developer role or above'}
          >
            {busy ? 'Working…' : 'Refresh observation'}
          </button>
        </div>
      </section>

      {actionError && <p className="error-text">{actionError}</p>}
      {rollbackBlocked && rl.deploymentStatus === 'not_started' && (
        <p className="muted small">Deployment has not started for this demand.</p>
      )}
    </div>
  );
}
