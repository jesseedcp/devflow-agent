import { beforeEach, describe, expect, it } from 'vitest';
import { mockClient } from '../api';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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
  beforeEach(() => mockClient.reset());
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


  it('creates a workspace from the Workspaces page', async () => {
    renderAt('/workspaces', <Route path="/workspaces" element={<WorkspacesPage />} />);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /新建工作区/ })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole('button', { name: /新建工作区/ }));
    fireEvent.change(screen.getByLabelText(/工作区 ID/), { target: { value: 'ws-new' } });
    fireEvent.change(screen.getByLabelText(/名称/), { target: { value: 'New Workspace' } });
    fireEvent.change(screen.getByLabelText(/Artifact Root/i), { target: { value: '/tmp/root' } });
    fireEvent.click(screen.getByRole('button', { name: /^创建工作区$/ }));
    expect(await screen.findByText('New Workspace')).toBeInTheDocument();
  });

  it('creates a demand from the Demands page and opens its detail', async () => {
    renderAt(
      '/workspaces/ws-demo/demands',
      <>
        <Route path="/workspaces/:workspaceId/demands" element={<DemandsPage />} />
        <Route path="/workspaces/:workspaceId/demands/:demandKey" element={<DemandDetailPage />} />
      </>,
    );
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /新建需求/ })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole('button', { name: /新建需求/ }));
    fireEvent.change(screen.getByLabelText(/需求 Key/), { target: { value: 'coupon-from-page' } });
    fireEvent.change(screen.getByLabelText(/标题/), { target: { value: 'Coupon from page' } });
    fireEvent.change(screen.getByLabelText(/描述/), { target: { value: 'Inactive users must be blocked' } });
    fireEvent.click(screen.getByRole('button', { name: /^创建需求$/ }));
    expect(await screen.findByText('Coupon from page')).toBeInTheDocument();
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

  it('confirms the current manual gate from demand detail', async () => {
    renderAt(
      '/workspaces/ws-demo/demands/coupon-eligibility',
      <Route path="/workspaces/:workspaceId/demands/:demandKey" element={<DemandDetailPage />} />,
    );
    const btn = await screen.findByRole('button', { name: /通过下一步/ });
    fireEvent.click(btn);
    expect(await screen.findByText(/已确认/)).toBeInTheDocument();
  });

  it('records acceptance evidence from demand detail', async () => {
    renderAt(
      '/workspaces/ws-demo/demands/verification-ready',
      <Route path="/workspaces/:workspaceId/demands/:demandKey" element={<DemandDetailPage />} />,
    );
    fireEvent.click(await screen.findByRole('button', { name: /添加证据/ }));
    fireEvent.change(screen.getByLabelText(/验收标准/), { target: { value: 'Inactive users are blocked' } });
    fireEvent.change(screen.getByLabelText(/证据摘要/), { target: { value: 'POST /coupon/claim returned 403' } });
    fireEvent.click(screen.getByRole('button', { name: /保存证据/ }));
    expect(await screen.findByText(/证据已记录/)).toBeInTheDocument();
  });
});
