import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { useAsync } from '../hooks/useAsync';
import { ErrorState, Loading, PageHeader } from '../components/State';
import { StatusBadge } from '../components/StatusBadge';
import { hasRole, titleCase } from '../utils/format';
import type { WikiCategory, WikiCandidate } from '../api/types';

const CATEGORIES: WikiCategory[] = ['business', 'process', 'archive'];

function slugify(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64);
}

function CandidateCard({
  candidate,
  workspaceId,
  onChanged,
}: {
  candidate: WikiCandidate;
  workspaceId: string;
  onChanged: () => void;
}) {
  const { client, role } = useApp();
  const allowed = hasRole(role, 'Reviewer');
  const resolved = candidate.status !== 'pending';

  const [name, setName] = useState(slugify(candidate.text.slice(0, 24)) || `entry-${candidate.id}`);
  const [category, setCategory] = useState<WikiCategory>(candidate.kind);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function promote() {
    setBusy(true);
    setError('');
    try {
      await client.promoteWikiCandidate(workspaceId, candidate.id, { name: slugify(name), category });
      onChanged();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function reject() {
    setBusy(true);
    setError('');
    try {
      await client.rejectWikiCandidate(workspaceId, candidate.id, { reason: reason || 'no reason given' });
      onChanged();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="card candidate">
      <div className="candidate-head">
        <StatusBadge label={titleCase(candidate.kind)} tone="tone-info" />
        <StatusBadge label={titleCase(candidate.status)} tone={resolved ? 'tone-good' : 'tone-warn'} />
        <span className="mono small muted">{candidate.demandKey} · {candidate.source} · #{candidate.index}</span>
      </div>
      <p className="candidate-text">{candidate.text}</p>
      {resolved && candidate.reason && <p className="muted small">Reject reason: {candidate.reason}</p>}
      {resolved && candidate.wikiPath && <p className="mono small">→ {candidate.wikiPath}</p>}

      {!resolved && (
        <div className="candidate-actions">
          <div className="form-row">
            <label className="field">
              <span className="small muted">wiki name</span>
              <input
                className="input"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={!allowed || busy}
                placeholder="lowercase-slug"
              />
            </label>
            <label className="field">
              <span className="small muted">category</span>
              <select
                className="input"
                value={category}
                onChange={(e) => setCategory(e.target.value as WikiCategory)}
                disabled={!allowed || busy}
              >
                {CATEGORIES.map((c) => (
                  <option key={c} value={c}>{titleCase(c)}</option>
                ))}
              </select>
            </label>
            <button
              type="button"
              className="btn btn-primary"
              onClick={promote}
              disabled={!allowed || busy}
              title={allowed ? 'Promote candidate to wiki entry' : 'Requires Reviewer or Admin'}
            >
              Promote
            </button>
          </div>
          <div className="form-row">
            <label className="field grow">
              <span className="small muted">reject reason</span>
              <input
                className="input"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                disabled={!allowed || busy}
                placeholder="why reject"
              />
            </label>
            <button
              type="button"
              className="btn btn-danger"
              onClick={reject}
              disabled={!allowed || busy}
              title={allowed ? 'Reject candidate' : 'Requires Reviewer or Admin'}
            >
              Reject
            </button>
          </div>
          {!allowed && <p className="muted small">Requires Reviewer or Admin role to promote or reject.</p>}
          {error && <p className="error-text small">{error}</p>}
        </div>
      )}
    </article>
  );
}

export function WikiCandidatesPage() {
  const { workspaceId = '' } = useParams();
  const { client } = useApp();
  const { data, loading, error, reload } = useAsync(
    () => client.listWikiCandidates(workspaceId),
    [client, workspaceId],
  );

  return (
    <div className="page">
      <PageHeader title="Wiki Candidates" subtitle="Review distilled knowledge before promotion." />
      {loading && <Loading />}
      {error && <ErrorState message={error} onRetry={reload} />}
      {data && (
        <div className="candidate-list">
          {data.length === 0 && <p className="muted">No wiki candidates pending.</p>}
          {data.map((c) => (
            <CandidateCard
              key={c.id}
              candidate={c}
              workspaceId={workspaceId}
              onChanged={reload}
            />
          ))}
        </div>
      )}
    </div>
  );
}
