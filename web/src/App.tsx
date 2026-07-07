import { Navigate, Route, Routes } from 'react-router-dom';
import { AppProvider } from './context/AppContext';
import { AppShell } from './components/AppShell';
import { WorkspacesPage } from './pages/WorkspacesPage';
import { DemandsPage } from './pages/DemandsPage';
import { DemandDetailPage } from './pages/DemandDetailPage';
import { WikiPage } from './pages/WikiPage';
import { WikiCandidatesPage } from './pages/WikiCandidatesPage';
import { ReleasePage } from './pages/ReleasePage';
import { AuditPage } from './pages/AuditPage';

export function App() {
  return (
    <AppProvider>
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/" element={<Navigate to="/workspaces" replace />} />
          <Route path="/workspaces" element={<WorkspacesPage />} />
          <Route path="/workspaces/:workspaceId/demands" element={<DemandsPage />} />
          <Route path="/workspaces/:workspaceId/demands/:demandKey" element={<DemandDetailPage />} />
          <Route path="/workspaces/:workspaceId/wiki" element={<WikiPage />} />
          <Route path="/workspaces/:workspaceId/wiki/candidates" element={<WikiCandidatesPage />} />
          <Route path="/workspaces/:workspaceId/release/:demandKey" element={<ReleasePage />} />
          <Route path="/workspaces/:workspaceId/audit" element={<AuditPage />} />
          <Route path="*" element={<Navigate to="/workspaces" replace />} />
        </Route>
      </Routes>
    </AppProvider>
  );
}
