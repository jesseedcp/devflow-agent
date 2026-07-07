import type { ApiClient } from './client';
import { HttpApiClient } from './httpClient';
import { MockApiClient } from './mockClient';

export const mockClient = new MockApiClient();

export function createApiClient(): ApiClient {
  const mode = (import.meta.env.VITE_DEVFLOW_API_MODE as string | undefined)?.toLowerCase();
  if (mode === 'http') return new HttpApiClient();
  return mockClient;
}

export { MockApiClient, HttpApiClient };
