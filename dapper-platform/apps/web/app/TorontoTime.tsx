'use client';

/* One time zone for everyone (error404 #111, owner 2026-11-14): dates and times show in the business's time, America/Toronto, wherever the
   viewer is. Every date formatting call that does not name a time zone gets this one; calls that already name one are left alone.
   A date at exactly midnight on the device's clock is a calendar day (a calendar grid's day or month, built with new Date(y, m, d)), not a
   moment, so it is named as it is (error404 #136 #140: in India, 1 December at midnight is still 30 November in Toronto, which put
   "November" over December's grid).
   shortcut: only formatting is covered; date arithmetic with getHours()/getDate() still uses the device's zone. Upgrade those call sites if
   staff ever work from another time zone. */
const ZONE = 'America/Toronto';

type Opts = Intl.DateTimeFormatOptions | undefined;
const withZone = (o: Opts): Intl.DateTimeFormatOptions => (o && o.timeZone ? o : { ...(o ?? {}), timeZone: ZONE });
const calendarDay = (d: Date) => d.getHours() === 0 && d.getMinutes() === 0 && d.getSeconds() === 0 && d.getMilliseconds() === 0;
const pick = (d: Date, o: Opts) => (calendarDay(d) ? o : withZone(o));

if (typeof window !== 'undefined' && !(window as unknown as { __dapperZone?: boolean }).__dapperZone) {
  (window as unknown as { __dapperZone?: boolean }).__dapperZone = true;
  const proto = Date.prototype;
  const all = proto.toLocaleString, date = proto.toLocaleDateString, time = proto.toLocaleTimeString;
  proto.toLocaleString = function (this: Date, locales?: Intl.LocalesArgument, options?: Opts) { return all.call(this, locales, pick(this, options)); };
  proto.toLocaleDateString = function (this: Date, locales?: Intl.LocalesArgument, options?: Opts) { return date.call(this, locales, pick(this, options)); };
  proto.toLocaleTimeString = function (this: Date, locales?: Intl.LocalesArgument, options?: Opts) { return time.call(this, locales, withZone(options)); };
  const Original = Intl.DateTimeFormat;
  const Zoned = function (locales?: Intl.LocalesArgument, options?: Opts) {
    const zoned = new Original(locales, withZone(options));
    if (options?.timeZone) return zoned;
    const plain = new Original(locales, options);
    const which = (d?: Date | number) => (d !== undefined && calendarDay(new Date(d)) ? plain : zoned);
    Object.defineProperty(zoned, 'format', { value: (d?: Date | number) => which(d).format(d) });
    Object.defineProperty(zoned, 'formatToParts', { value: (d?: Date | number) => which(d).formatToParts(d) });
    return zoned;
  } as unknown as typeof Intl.DateTimeFormat;
  (Zoned as unknown as { prototype: unknown }).prototype = Original.prototype;
  Zoned.supportedLocalesOf = Original.supportedLocalesOf;
  Intl.DateTimeFormat = Zoned;
}

/* Now as a Date whose year, month, day and hours are Toronto's wall clock, for calendar-day work (today, last 7 days) done with
   getDate()/setDate(). Format such a Date only with timeZone 'UTC' or not at all; it is not the real instant. */
export const torontoNow = () => new Date(new Date().toLocaleString('en-US', { timeZone: ZONE }));

export default function TorontoTime() {
  return null;
}
