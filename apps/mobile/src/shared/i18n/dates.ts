import { LOCALE } from './locale';

const dateTimeFormatter = new Intl.DateTimeFormat(LOCALE, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
const dayFormatter = new Intl.DateTimeFormat(LOCALE, { weekday: 'long', day: 'numeric', month: 'long' });

export function formatDateTime(iso: string): string {
  return dateTimeFormatter.format(new Date(iso));
}

/** `2026-11-05` (a reconciliation day, no time zone) → `jueves, 5 de noviembre`. */
export function formatDay(day: string): string {
  const [year, month, date] = day.split('-').map(Number);
  const text = dayFormatter.format(new Date(year ?? 1970, (month ?? 1) - 1, date ?? 1));
  return text.charAt(0).toLocaleUpperCase(LOCALE) + text.slice(1);
}
