import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { StatusBadge } from '../components/StatusBadge';
import { ArtifactPresence, ArtifactTabs } from '../components/ArtifactTabs';
import { demandStateTone, formatDateTime, gateTone, titleCase } from '../utils/format';
import type { AddEvidenceInput, GateStatus } from '../api/types';

function confirmStageForState(state: string): string | null {
  if (state === 'requirements_review') return 'requirements';
  if (state === 'plan_review') return 'plan';
  if (state === 'verification') return 'verification';
  if (state === 'closeout') return 'closeout';
  return null;
}

export function DemandDetailPage() {
  const { workspaceId = '', demandKey = '' } = useParams();
  const { client } = useApp();
  const { data, loading, error, reload, setData } = useAsync(
    () => client.getDemand(workspaceId, demandKey),
    [client, workspaceId, demandKey],
  );

  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const [actionError, setActionError] = useState('');

  const [showEvidence, setShowEvidence] = useState(false);
  const [evType, setEvType] = useState<AddEvidenceInput['type']>('manual');
  const [evStatus, setEvStatus] = useState<AddEvidenceInput['status']>('pass');
  const [criterion, setCriterion] = useState('');
  const [summary, setSummary] = useState('');
  const [link, setLink] = useState('');

  if (loading) return <div className="page"><Loading /></div>;
  if (error) return <div className="page"><ErrorState message={error} onRetry={reload} /></div>;
  if (!data) return null;

  const rl = data.releaseLine;
  const rollbackLink = `/workspaces/${workspaceId}/release/${demandKey}`;
  const stage = confirmStageForState(data.state);
  const canConfirm = stage !== null;
  const canEvidence = data.state === 'verification';
  const evidence = data.evidence ?? { pass: 0, fail: 0, blocked: 0 };
  const stages = data.quality.stageSummary ?? {};
  const hasStages = Object.keys(stages).length > 0;

  async function onConfirm() {
    if (!canConfirm) return;
    setBusy(true);
    setActionError('');
    setNotice('');
    try {
      const updated = await client.confirmDemand(workspaceId, demandKey, {
        stage: stage as string,
        summary: '通过 Web UI 确认推进',
      });
      setNotice(`已确认 · 状态推进到 ${updated.state}`);
      setData(updated);
    } catch (err) {
      setActionError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function onAddEvidence(e: React.FormEvent) {
    e.preventDefault();
    if (!criterion.trim() || !summary.trim()) return;
    setBusy(true);
    setActionError('');
    setNotice('');
    try {
      const updated = await client.addEvidence(workspaceId, demandKey, {
        type: evType,
        status: evStatus,
        criterion: criterion.trim(),
        summary: summary.trim(),
        source: 'web',
        link: link.trim() || undefined,
      });
      const ev = updated.evidence ?? { pass: 0, fail: 0, blocked: 0 };
      setNotice(`证据已记录 · pass=${ev.pass} fail=${ev.fail} blocked=${ev.blocked}`);
      setCriterion('');
      setSummary('');
      setLink('');
      setShowEvidence(false);
      setData(updated);
    } catch (err) {
      setActionError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

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
            Rollback needed -&gt;
          </Link>
        )}
      </div>

      {data.description && <p className="muted">{data.description}</p>}

      <section className="card">
        <h2 className="card-title">Lifecycle actions</h2>
        <div className="form-row">
          <button
            type="button"
            className="btn btn-primary"
            onClick={onConfirm}
            disabled={!canConfirm || busy}
            title={canConfirm ? 'Confirm the current manual gate' : '当前阶段需要 CLI/Agent 产物，暂不支持页面直接推进'}
          >
            {busy ? '处理中…' : '通过下一步'}
          </button>
          <button
            type="button"
            className="btn btn-ghost"
            onClick={() => setShowEvidence((v) => !v)}
            disabled={!canEvidence || busy}
            title={canEvidence ? '记录验收证据' : '只有 verification 阶段可以记录验收证据'}
          >
            添加证据
          </button>
        </div>
        {!canConfirm && <p className="muted small">当前阶段需要 CLI/Agent 产物，暂不支持页面直接推进。</p>}
        {!canEvidence && <p className="muted small">只有 verification 阶段可以记录验收证据。</p>}
        <p className="muted small">验收证据 pass={evidence.pass} fail={evidence.fail} blocked={evidence.blocked}</p>

        {showEvidence && canEvidence && (
          <form className="card" onSubmit={onAddEvidence}>
            <h3 className="subhead">记录验收证据</h3>
            <div className="form-row">
              <label className="field">
                <span className="small muted">类型</span>
                <select className="input" value={evType} onChange={(e) => setEvType(e.target.value as AddEvidenceInput['type'])} disabled={busy}>
                  <option value="manual">manual</option>
                  <option value="api">api</option>
                  <option value="link">link</option>
                </select>
              </label>
              <label className="field">
                <span className="small muted">状态</span>
                <select className="input" value={evStatus} onChange={(e) => setEvStatus(e.target.value as AddEvidenceInput['status'])} disabled={busy}>
                  <option value="pass">pass</option>
                  <option value="fail">fail</option>
                  <option value="blocked">blocked</option>
                </select>
              </label>
            </div>
            <label className="field">
              <span className="small muted">验收标准</span>
              <input className="input" value={criterion} onChange={(e) => setCriterion(e.target.value)} placeholder="Inactive users are blocked" disabled={busy} />
            </label>
            <label className="field">
              <span className="small muted">证据摘要</span>
              <input className="input" value={summary} onChange={(e) => setSummary(e.target.value)} placeholder="POST /coupon/claim returned 403" disabled={busy} />
            </label>
            <label className="field">
              <span className="small muted">链接 (可选)</span>
              <input className="input mono" value={link} onChange={(e) => setLink(e.target.value)} placeholder="https://..." disabled={busy} />
            </label>
            <div className="form-row">
              <button type="submit" className="btn btn-primary" disabled={busy || !criterion.trim() || !summary.trim()}>
                {busy ? '保存中…' : '保存证据'}
              </button>
              <button type="button" className="btn btn-ghost" onClick={() => setShowEvidence(false)} disabled={busy}>
                取消
              </button>
            </div>
          </form>
        )}

        {notice && <p className="muted small">✓ {notice}</p>}
        {actionError && <p className="error-text small">{actionError}</p>}
      </section>

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
                <span className="muted">-</span>
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
                    {a.command && <code className="action-cmd mono">{a.command}</code>}
                    {a.reason && <div className="muted small">{a.reason}</div>}
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
          {data.quality.checks.length > 0 ? (
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
          ) : hasStages ? (
            <table className="table compact">
              <thead>
                <tr><th>Stage</th><th>Status</th></tr>
              </thead>
              <tbody>
                {Object.entries(stages).map(([name, status]) => (
                  <tr key={name}>
                    <td>{name}</td>
                    <td><StatusBadge label={titleCase(status)} tone={gateTone(status as GateStatus)} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p className="muted">No quality checks recorded.</p>
          )}
          {(data.quality.blockers ?? 0) > 0 || (data.quality.warnings ?? 0) > 0 ? (
            <p className="muted small">blockers={data.quality.blockers ?? 0} warnings={data.quality.warnings ?? 0}</p>
          ) : null}
        </section>

        <section className="card">
          <h2 className="card-title">Acceptance evidence</h2>
          <table className="table compact">
            <thead>
              <tr><th>Category</th><th>Provided</th><th>Required</th><th>State</th></tr>
            </thead>
            <tbody>
              {data.acceptance.length === 0 && (
                <tr><td colSpan={4} className="muted">No acceptance evidence recorded.</td></tr>
              )}
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
