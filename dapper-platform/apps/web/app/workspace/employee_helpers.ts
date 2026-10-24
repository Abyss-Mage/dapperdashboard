export type AvailabilityInterval = { day: number; start: string; end: string };
export function calendarRange(day: string, view: 'month' | 'week' | 'day') {
  const d = new Date(`${day}T12:00:00Z`);
  if (view === 'day') return { from: day, to: dateOffset(day, 1), days: 1 };
  const first = view === 'month' ? `${day.slice(0, 7)}-01` : day;
  const start = new Date(`${first}T12:00:00Z`);
  const from = dateOffset(first, -(start.getUTCDay() + 6) % 7);
  const days = view === 'month' ? 42 : 7;
  return { from, to: dateOffset(from, days), days };
}
export function contactFormatError(name: string, email: string, phone: string) {
  if (name.trim().length < 2) return 'Enter the customer’s full name.';
  if (!/^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(email.trim())) return 'Enter a valid email address, for example name@example.ca.';
  if (!/^[+0-9 ().-]+$/.test(phone) || !/^\+[1-9][0-9]{6,14}$/.test(phone.replace(/[ ().-]/g, ''))) return 'Include the country code, for example +1 416 555 0123.';
  return '';
}
export function worksiteDate(value: string | Date, timezone: string) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date(value));
  return ['year', 'month', 'day'].map(k => parts.find(p => p.type === k)?.value).join('-');
}
export function dateOffset(date: string, days: number) {
  const d = new Date(`${date}T12:00:00Z`); d.setUTCDate(d.getUTCDate() + days); return d.toISOString().slice(0, 10);
}
export function readableDay(date: string, options: Intl.DateTimeFormatOptions = { weekday: 'long', month: 'short', day: 'numeric' }) {
  return new Intl.DateTimeFormat('en-CA', { ...options, timeZone: 'UTC' }).format(new Date(`${date}T12:00:00Z`));
}
export function availabilityError(intervals: AvailabilityInterval[], unavailable: boolean) {
  if (!intervals.length && !unavailable) return 'Add at least one time range, or confirm you are unavailable next week.';
  const sorted = [...intervals].sort((a, b) => a.day - b.day || a.start.localeCompare(b.start));
  for (let i = 0; i < sorted.length; i++) {
    const r = sorted[i];
    if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(r.start) || !/^([01]\d|2[0-3]):[0-5]\d$/.test(r.end) || r.start >= r.end) return 'Each time range needs a start before its end on the same day.';
    if (i && sorted[i - 1].day === r.day && sorted[i - 1].end > r.start) return 'Time ranges on the same day cannot overlap.';
  }
  return '';
}
