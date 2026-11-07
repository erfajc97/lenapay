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
