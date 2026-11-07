/** Content padding of virtualized lists (FlashList takes no className). Spacing scale 8/16/32. */
export const LIST_CONTENT_STYLE = { paddingHorizontal: 16, paddingTop: 8, paddingBottom: 32 } as const;

/**
 * A row of a virtualized list drawn as part of ONE grouped card (FlashList renders rows as
 * separate cells, so the card is assembled from its edges). Only the first row rounds the
 * top and only the last rounds the bottom; every other row's top border is the divider.
 */
export function groupedRowClass(index: number, count: number): string {
  const first = index === 0;
  const last = index === count - 1;
  return [
    'border-x border-border-subtle bg-bg-surface px-4',
    first ? 'rounded-t-card border-t pt-1' : 'border-t',
    last ? 'rounded-b-card border-b pb-1' : '',
  ]
    .filter(Boolean)
    .join(' ');
}
