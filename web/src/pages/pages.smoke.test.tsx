import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AppProvider } from '../context/AppContext';
import { WorkspacesPage } from './WorkspacesPage';
import { DemandsPage } from './DemandsPage';
import { DemandDetailPage } from './DemandDetailPage';

function renderAt(path: string, routes: React.ReactNode) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppProvider>
        <Routes>{routes}</Routes>
      </AppProvider>
    </MemoryRouter>,
  );
}

describe('page smoke (mock client)', () => {
  it('WorkspacesPage renders the mock workspace', async () => {
    renderAt(
      '/workspaces',
      <Route path="/workspaces" element={<WorkspacesPage />} />,
    );
    await waitFor(() => expect(screen.getByText('Payments Platform')).toBeInTheDocument());
  });

  it('DemandsPage lists the three mock demands', async () => {
    renderAt(
      '/workspaces/ws-payments/demands',
      <Route path="/workspaces/:workspaceId/demands" element={<DemandsPage />} />,
    );
    await waitFor(() =>
      expect(screen.getByText('Add retry with exponential backoff')).toBeInTheDocument(),
    );
    expect(screen.getByText('Idempotency keys for payment intents')).toBeInTheDocument();
    expect(screen.getByText('At-least-once webhook delivery')).toBeInTheDocument();
  });

  it('DemandDetailPage renders header, release line, and artifact tabs', async () => {
    renderAt(
      '/workspaces/ws-payments/demands/add-retry-backoff',
      <Route path="/workspaces/:workspaceId/demands/:demandKey" element={<DemandDetailPage />} />,
    );
    await waitFor(() =>
      expect(screen.getByText('Add retry with exponential backoff')).toBeInTheDocument(),
    );
    expect(screen.getByText('Release line')).toBeInTheDocument();
    expect(screen.getByText('Quality gate')).toBeInTheDocument();
    expect(screen.getByText('Artifacts')).toBeInTheDocument();
  });
});
