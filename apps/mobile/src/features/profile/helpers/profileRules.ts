/** Contract cap of `fullName` after trimming. */
const MAX_FULL_NAME = 120;

/** What PATCH /me gets: the trimmed name, or null to clear it (blank is never sent as ""). */
export function fullNameUpdate(text: string): { value: string | null; error: 'validation.fullNameLength' | null } {
  const trimmed = text.trim();
  if (trimmed.length > MAX_FULL_NAME) return { value: null, error: 'validation.fullNameLength' };
  return { value: trimmed === '' ? null : trimmed, error: null };
}
