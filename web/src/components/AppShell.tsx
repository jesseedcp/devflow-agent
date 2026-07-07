import { NavLink, Outlet, useParams } from 'react-router-dom';
import { useApp } from '../context/AppContext';
import { StatusBadge } from './StatusBadge';
import type { Role } from '../api/types';

const ROLES: Role[] = ['Viewer', 'Developer', 'Reviewer', 'Admin'];

export function AppShell() {
  const { workspaceId } = useParams();
  const { user, role, isMock, setRole, loading } = useApp();

  const base = workspaceId ? `/workspaces/${workspaceId}` : '';
  const navItems = [
    { to: base ? `${base}/demands` : '/workspaces', label: 'Demands', end: !workspaceId },
    ...(workspaceId
      ? [
          { to: `${base}/wiki`, label: 'Wiki Library' },
          { to: `${base}/wiki/candidates`, label: 'Wiki Candidates' },
          { to: `${base}/audit`, label: 'Audit' },
        ]
      : []),
  ];

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">DF</span>
          <span className="brand-name">Devflow</span>
          {isMock && <span className="badge tone-warn mock-tag">mock</span>}
        </div>
        <nav className="nav">
          <NavLink to="/workspaces" className="nav-link" end>
            Workspaces
          </NavLink>
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className="nav-link"
              end={'end' in item ? item.end : false}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">
          <p className="muted small">Operational console</p>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <div className="topbar-left">
            <span className="topbar-label">Workspace</span>
            <span className="topbar-value">{workspaceId ?? 'none selected'}</span>
          </div>
          <div className="topbar-right">
            {loading ? (
              <span className="muted">loading user…</span>
            ) : (
              <>
                <span className="user-name">{user?.displayName ?? 'unknown'}</span>
                <span className="user-email">{user?.email ?? ''}</span>
                <StatusBadge label={role} tone="tone-info" />
                {isMock && (
                  <label className="role-switch">
                    <span className="muted small">role</span>
                    <select
                      value={role}
                      onChange={(e) => void setRole(e.target.value as Role)}
                      aria-label="Switch demo role"
                    >
                      {ROLES.map((r) => (
                        <option key={r} value={r}>
                          {r}
                        </option>
                      ))}
                    </select>
                  </label>
                )}
              </>
            )}
          </div>
        </header>
        <main className="content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
