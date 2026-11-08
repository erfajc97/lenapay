import { describe, expect, it } from 'vitest';

import { localFromServer, serverLanguage } from '../preferences';

// Why: language and theme follow the person across devices (R-ID-5): on sign-in the server value
// wins, but an unset server value must not wipe what this phone already uses.
describe('localFromServer', () => {
  it('takes every value the server has set', () => {
    expect(localFromServer({ locale: 'en-US', theme: 'dark' })).toEqual({ language: 'en-US', theme: 'dark' });
    expect(localFromServer({ locale: 'es-AR', theme: 'system' })).toEqual({ language: 'es-AR', theme: 'system' });
  });

  it('leaves the local value alone for anything the server has not set', () => {
    expect(localFromServer({ locale: null, theme: 'light' })).toEqual({ theme: 'light' });
    expect(localFromServer({ locale: null, theme: null })).toEqual({});
    expect(localFromServer(null)).toEqual({});
  });
});

describe('serverLanguage', () => {
  it('stores a picked language and clears it for "follow the device"', () => {
    expect(serverLanguage('en-US')).toBe('en-US');
    expect(serverLanguage('es-AR')).toBe('es-AR');
    expect(serverLanguage('system')).toBeNull();
  });
});
