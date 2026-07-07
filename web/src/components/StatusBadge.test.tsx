import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusBadge } from './StatusBadge';

describe('StatusBadge', () => {
  it('renders the label and tone class', () => {
    render(<StatusBadge label="Failed" tone="tone-bad" />);
    const el = screen.getByText('Failed');
    expect(el).toHaveClass('badge', 'tone-bad');
  });

  it('defaults to info tone', () => {
    render(<StatusBadge label="Pending" />);
    expect(screen.getByText('Pending')).toHaveClass('tone-info');
  });
});
