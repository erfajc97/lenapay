import { describe, expect, it } from 'vitest';

import { groupedRowClass } from '../listStyle';

// Why: a list drawn as one card breaks visibly when an edge is wrong — a middle row with
// rounded corners leaves a notch, a missing bottom edge leaves the card open.

describe('groupedRowClass', () => {
  it('a single row is the whole card: both edges rounded and closed', () => {
    const classes = groupedRowClass(0, 1);
    expect(classes).toContain('rounded-t-card border-t');
    expect(classes).toContain('rounded-b-card border-b');
  });

  it('the first of several rounds only the top', () => {
    const classes = groupedRowClass(0, 3);
    expect(classes).toContain('rounded-t-card');
    expect(classes).not.toContain('rounded-b-card');
  });

  it('a middle row is square, with its top border as the divider', () => {
    expect(groupedRowClass(1, 3)).toBe('border-x border-border-subtle bg-bg-surface px-4 border-t');
  });

  it('the last rounds only the bottom, and the one before it does not', () => {
    expect(groupedRowClass(2, 3)).toContain('rounded-b-card border-b');
    expect(groupedRowClass(2, 3)).not.toContain('rounded-t-card');
    expect(groupedRowClass(1, 3)).not.toContain('rounded-b-card');
  });
});
