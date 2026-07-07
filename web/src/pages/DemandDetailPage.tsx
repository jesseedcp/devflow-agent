import { Link, useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { StatusBadge } from '../components/StatusBadge';
import { ArtifactPresence, ArtifactTabs } from '../components/ArtifactTabs';
import { demandStateTone, formatDateTime, gateTone, titleCase } from '../utils/format';

export function DemandDetailPage() {
  const { workspaceId = '', demandKey = '' } = useParams();
  const { client } = useApp();
  const { data, loading, error, reload } = useAsync(
    () => client.getDemand(workspaceId, demandKey),
    [client, workspaceId, demandKey],
  );

  if (loading) return <div className="page"><Loading /></div>;
  if (error) return <div className="page"><ErrorState message={error} onRetry={reload} /></div>;
  if (!data) return null;

  const rl = data.releaseLine;
  const rollbackLink = `/workspaces/${workspaceId}/release/${demandKey}`;

  return (
    <div className="page">
      <PageHeader
        title={data.title}
        subtitle={`${data.demandKey} · updated ${formatDateTime(data.updatedAt)}`}
        actions={
          <Link to={`/workspaces/${workspaceId}/demands`} className="btn btn-ghost btn-sm">
            ← Demands
          </Link>
        }
      />

      <div className="detail-head">
        <StatusBadge label={titleCase(data.state)} tone={demandStateTone(data.state)} />
        <ArtifactPresence artifacts={data.artifacts} />
        {data.attention && <span className="attention">⚠ {data.attention}</span>}
        {rl.rollbackNeeded && (
          <Link to={rollbackLink} className="btn btn-danger btn-sm">
            Rollback needed →
          </Link>
        )}
      </div>

      <section className="card">
        <h2 className="card-title">Release line</h2>
        <div className="grid-2">
          <dl className="kv">
            <dt>Deployment</dt>
            <dd><StatusBadge label={titleCase(rl.deploymentStatus)} tone={gateTone(rl.deploymentStatus)} /></dd>
            <dt>Environment</dt>
            <dd className="mono">{rl.environment}</dd>
            <dt>Ref</dt>
            <dd className="mono">{rl.ref}</dd>
            <dt>Rollback decision</dt>
            <dd><StatusBadge label={titleCase(rl.rollbackDecision)} tone={rl.rollbackNeeded ? 'tone-warn' : 'tone-info'} /></dd>
            <dt>Run</dt>
            <dd>
              {rl.runUrl ? (
                <a className="link" href={rl.runUrl} target="_blank" rel="noreferrer">
                  {rl.runUrl}
                </a>
              ) : (
                <span className="muted">—</span>
              )}
            </dd>
          </dl>
          <div>
            <h3 className="subhead">Next actions</h3>
            {data.nextActions.length === 0 ? (
              <p className="muted">No pending actions.</p>
            ) : (
              <ul className="actions">
                {data.nextActions.map((a) => (
                  <li key={a.label}>
                    <div className="action-label">{a.label}</div>
                    <code className="action-cmd mono">{a.command}</code>
                    <div className="muted small">{a.reason}</div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </section>

      <div className="grid-2">
        <section className="card">
          <h2 className="card-title">
            Quality gate <StatusBadge label={titleCase(data.quality.gate)} tone={gateTone(data.quality.gate)} />
          </h2>
          {data.quality.checks.length === 0 ? (
            <p className="muted">No quality checks recorded.</p>
          ) : (
            <table className="table compact">
              <thead>
                <tr><th>Check</th><th>Status</th><th>Summary</th></tr>
              </thead>
              <tbody>
                {data.quality.checks.map((c) => (
                  <tr key={c.name}>
                    <td>{c.name}</td>
                    <td><StatusBadge label={titleCase(c.status)} tone={gateTone(c.status)} /></td>
                    <td className="muted">{c.summary}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className="card">
          <h2 className="card-title">Acceptance evidence</h2>
          <table className="table compact">
            <thead>
              <tr><th>Category</th><th>Provided</th><th>Required</th><th>State</th></tr>
            </thead>
            <tbody>
              {data.acceptance.map((a) => {
                const ok = a.provided >= a.required;
                return (
                  <tr key={a.category}>
                    <td>{titleCase(a.category)}</td>
                    <td>{a.provided}</td>
                    <td>{a.required}</td>
                    <td><StatusBadge label={ok ? 'met' : 'missing'} tone={ok ? 'tone-good' : 'tone-bad'} /></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </section>
      </div>

      <section className="card">
        <h2 className="card-title">
          Metrics <StatusBadge label={titleCase(data.metrics.status)} tone={gateTone(data.metrics.status)} />
        </h2>
        {data.metrics.metrics.length === 0 ? (
          <p className="muted">{data.metrics.summary}</p>
        ) : (
          <table className="table compact">
            <thead>
              <tr><th>Metric</th><th>Value</th><th>Threshold</th><th>Pass</th></tr>
            </thead>
            <tbody>
              {data.metrics.metrics.map((m) => (
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
        <p className="muted small">{data.metrics.summary}</p>
      </section>

      <section className="card">
        <h2 className="card-title">Artifacts</h2>
        <ArtifactTabs workspaceId={workspaceId} demandKey={demandKey} artifacts={data.artifacts} />
      </section>
    </div>
  );
}
