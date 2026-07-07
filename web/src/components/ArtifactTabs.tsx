import { useEffect, useState } from 'react';
import { useApp } from '../context/AppContext';
import type { ArtifactSummary } from '../api/types';
import { StatusBadge } from './StatusBadge';

interface ArtifactTabsProps {
  workspaceId: string;
  demandKey: string;
  artifacts: ArtifactSummary[];
}

export function ArtifactTabs({ workspaceId, demandKey, artifacts }: ArtifactTabsProps) {
  const { client } = useApp();
  const present = artifacts.filter((a) => a.present);
  const [active, setActive] = useState<string>(present[0]?.name ?? artifacts[0]?.name ?? '');
  const [content, setContent] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');

  useEffect(() => {
    if (!active) return;
    let alive = true;
    setLoading(true);
    setError('');
    client
      .getArtifact(workspaceId, demandKey, active)
      .then((text) => {
        if (alive) setContent(text);
      })
      .catch((err: Error) => {
        if (alive) setError(err.message);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [client, workspaceId, demandKey, active]);

  return (
    <div className="artifact-tabs">
      <div className="tab-list" role="tablist" aria-label="Artifacts">
        {artifacts.map((a) => {
          const isActive = a.name === active;
          return (
            <button
              key={a.name}
              role="tab"
              type="button"
              aria-selected={isActive}
              className={`tab ${isActive ? 'tab-active' : ''} ${a.present ? '' : 'tab-missing'}`}
              onClick={() => setActive(a.name)}
              title={a.present ? a.path : 'not present'}
            >
              {a.present ? '●' : '○'} {a.name}
            </button>
          );
        })}
      </div>
      <div className="tab-panel" role="tabpanel">
        {loading && <p className="muted">Loading {active}…</p>}
        {error && <p className="error-text">{error}</p>}
        {!loading && !error && (
          <pre className="artifact-content">{content || `Artifact ${active} has no content.`}</pre>
        )}
      </div>
    </div>
  );
}

export function ArtifactPresence({ artifacts }: { artifacts: ArtifactSummary[] }) {
  const present = artifacts.filter((a) => a.present).length;
  const tone = present === artifacts.length ? 'tone-good' : present === 0 ? 'tone-bad' : 'tone-warn';
  return (
    <StatusBadge label={`${present}/${artifacts.length} artifacts`} tone={tone} />
  );
}
