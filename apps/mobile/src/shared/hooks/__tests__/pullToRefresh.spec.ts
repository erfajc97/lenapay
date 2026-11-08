import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it, vi } from 'vitest';

import { createPull } from '../usePullToRefresh';

// Why: the iPhone "stuck loading". A RefreshControl whose `refreshing` follows a query's
// isRefetching/isFetching starts refreshing on its own (mount, focus, background refetch); iOS
// then insets the scroll view, shows the spinner above the content and does not give the inset
// back until the person scrolls. The spinner may only reflect a pull the person made.
function deferred() {
  let resolve!: () => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe('createPull', () => {
  it('shows the spinner only from the pull until its refetch settles', async () => {
    const states: boolean[] = [];
    const pull = createPull((value) => states.push(value));
    const refetch = deferred();
    const done = pull(() => refetch.promise);
    expect(states).toEqual([true]);
    refetch.resolve();
    await done;
    expect(states).toEqual([true, false]);
  });

  it('hides the spinner when the refetch fails too', async () => {
    const states: boolean[] = [];
    const pull = createPull((value) => states.push(value));
    await pull(() => Promise.reject(new Error('network.offline')));
    expect(states).toEqual([true, false]);
  });

  it('ignores a second pull while the first one is still running', async () => {
    const states: boolean[] = [];
    const pull = createPull((value) => states.push(value));
    const refetch = deferred();
    const second = vi.fn(async () => undefined);
    const first = pull(() => refetch.promise);
    await pull(second);
    expect(second).not.toHaveBeenCalled();
    refetch.resolve();
    await first;
    await pull(second);
    expect(second).toHaveBeenCalledTimes(1);
    expect(states).toEqual([true, false, true, false]);
  });
});

const SRC = fileURLToPath(new URL('../../..', import.meta.url));
const QUERY_DRIVEN = /refreshing[=:]\s*\{?[^}\n]*\b(isRefetching|isFetching|isLoading|isPending)\b/;

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === '__tests__' ? [] : sourceFiles(path);
    return /\.tsx?$/.test(name) ? [path] : [];
  });
}

describe('refresh controls', () => {
  it('no screen binds refreshing to a query flag', () => {
    const offenders = sourceFiles(SRC).filter((path) => QUERY_DRIVEN.test(readFileSync(path, 'utf8')));
    expect(offenders).toEqual([]);
  });

  it('the census bites on the shapes it must catch', () => {
    expect(QUERY_DRIVEN.test('refreshing={screen.payers.isRefetching && !screen.searching}')).toBe(true);
    expect(QUERY_DRIVEN.test('refreshing: balances.isRefetching || movements.isRefetching,')).toBe(true);
    expect(QUERY_DRIVEN.test('refreshing={pull.refreshing}')).toBe(false);
  });
});
