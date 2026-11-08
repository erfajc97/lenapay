/**
 * The ONE reporting module (M12). No vendor is wired yet, so every call is a no-op; when a
 * vendor arrives it is imported only here, and only ids and enums leave the device.
 */
const ALLOWED_KEYS = new Set(['code', 'status', 'screen', 'orgId', 'payoutId', 'name']);

/** Keeps only non-personal keys and drops anything that looks like a token or a URL query. */
export function scrub(context: Record<string, unknown>): Record<string, string | number> {
  const clean: Record<string, string | number> = {};
  for (const [key, value] of Object.entries(context)) {
    if (!ALLOWED_KEYS.has(key)) continue;
    if (typeof value === 'number') clean[key] = value;
    if (typeof value === 'string' && !/[?&=]|bearer|token/i.test(value)) clean[key] = value;
  }
  return clean;
}

export function captureException(error: unknown, context: Record<string, unknown> = {}): void {
  // No vendor configured: intentionally a no-op (fails closed, sends nothing).
  void error;
  void scrub(context);
}

/** True in a development bundle only (Metro defines `__DEV__`; node tests do not). */
const IS_DEV = typeof __DEV__ !== 'undefined' && __DEV__;

/**
 * i18next `missingKeyHandler` (M8): in development a key with no copy is logged loudly so it is
 * fixed before it ships; release builds stay silent (the locales spec is the gate there).
 */
export function missingKeyReporter(options: { dev?: boolean; warn?: (message: string) => void } = {}) {
  const dev = options.dev ?? IS_DEV;
  // Dev-only output: the reporting module is the one logger (M12).
  const warn = options.warn ?? ((message: string) => console.warn(message));
  return (locales: readonly string[], _namespace: string, key: string): void => {
    if (dev) warn(`[i18n] missing key "${key}" (${locales.join(', ')})`);
  };
}
