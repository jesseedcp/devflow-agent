import { describe, expect, it } from 'vitest';
import { hasRole, titleCase } from './format';

describe('format utils', () => {
  it('title-cases snake_case', () => {
    expect(titleCase('blocked_need_user')).toBe('Blocked Need User');
  });

  it('enforces role rank', () => {
    expect(hasRole('Reviewer', 'Reviewer')).toBe(true);
    expect(hasRole('Reviewer', 'Admin')).toBe(false);
    expect(hasRole('Admin', 'Viewer')).toBe(true);
    expect(hasRole('Viewer', 'Developer')).toBe(false);
  });
});
