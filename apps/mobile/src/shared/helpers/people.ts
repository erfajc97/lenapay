/** How the app names a person (R-ID-5): the name they chose, else their email. */
export function displayName(person: { email: string; fullName: string | null }): string {
  return person.fullName ?? person.email;
}

/** The first word of the chosen name for a greeting; null without a name (never the email). */
export function firstName(fullName: string | null | undefined): string | null {
  const first = fullName?.trim().split(/\s+/)[0];
  return first ? first : null;
}
