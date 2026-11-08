const pad = (value: number) => String(value).padStart(2, '0');

/** Local calendar day as the API's business date (`format: date`, YYYY-MM-DD). */
export function businessDate(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** "DD/MM/AAAA" typed by hand → business date, or null when it is not a real day. */
export function parseDayInput(text: string): string | null {
  const match = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(text.trim());
  if (!match) return null;
  const [day, month, year] = [Number(match[1]), Number(match[2]), Number(match[3])];
  const date = new Date(year, month - 1, day);
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) return null;
  return businessDate(date);
}

/** Business date → "DD/MM/AAAA" (field value and chip label). */
export function formatDayInput(day: string): string {
  const [year, month, date] = day.split('-');
  return `${date}/${month}/${year}`;
}

