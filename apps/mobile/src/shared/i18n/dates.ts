import { activeLocale, type AppLocale } from './locale';

type Formatters = { dateTime: Intl.DateTimeFormat; day: Intl.DateTimeFormat };
const cache = new Map<AppLocale, Formatters>();

function formatters(): Formatters {
  const locale = activeLocale();
  let found = cache.get(locale);
  if (!found) {
    found = {
      dateTime: new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }),
      day: new Intl.DateTimeFormat(locale, { weekday: 'long', day: 'numeric', month: 'long' }),
    };
    cache.set(locale, found);
  }
  return found;
}

export function formatDateTime(iso: string): string {
  return formatters().dateTime.format(new Date(iso));
}

/** `2026-11-05` (a reconciliation day, no time zone) → `Jueves, 5 de noviembre` / `Thursday, November 5`. */
export function formatDay(day: string): string {
  const [year, month, date] = day.split('-').map(Number);
  const text = formatters().day.format(new Date(year ?? 1970, (month ?? 1) - 1, date ?? 1));
  return text.charAt(0).toLocaleUpperCase(activeLocale()) + text.slice(1);
}
