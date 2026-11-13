'use client';

/* One time zone for everyone (error404 #111, owner 2026-11-14): dates and times show in the business's time, America/Toronto, wherever the
   viewer is. Every date formatting call that does not name a time zone gets this one; calls that already name one are left alone.
   shortcut: only formatting is covered; date arithmetic with getHours()/getDate() still uses the device's zone. Upgrade those call sites if
   staff ever work from another time zone. */
const ZONE = 'America/Toronto';

type Opts = Intl.DateTimeFormatOptions | undefined;
const withZone = (o: Opts): Intl.DateTimeFormatOptions => (o && o.timeZone ? o : { ...(o ?? {}), timeZone: ZONE });

if (typeof window !== 'undefined' && !(window as unknown as { __dapperZone?: boolean }).__dapperZone) {
  (window as unknown as { __dapperZone?: boolean }).__dapperZone = true;
  const proto = Date.prototype;
  const all = proto.toLocaleString, date = proto.toLocaleDateString, time = proto.toLocaleTimeString;
  proto.toLocaleString = function (this: Date, locales?: Intl.LocalesArgument, options?: Opts) { return all.call(this, locales, withZone(options)); };
  proto.toLocaleDateString = function (this: Date, locales?: Intl.LocalesArgument, options?: Opts) { return date.call(this, locales, withZone(options)); };
  proto.toLocaleTimeString = function (this: Date, locales?: Intl.LocalesArgument, options?: Opts) { return time.call(this, locales, withZone(options)); };
  const Original = Intl.DateTimeFormat;
  const Zoned = function (locales?: Intl.LocalesArgument, options?: Opts) { return new Original(locales, withZone(options)); } as unknown as typeof Intl.DateTimeFormat;
  (Zoned as unknown as { prototype: unknown }).prototype = Original.prototype;
  Zoned.supportedLocalesOf = Original.supportedLocalesOf;
  Intl.DateTimeFormat = Zoned;
}

export default function TorontoTime() {
  return null;
}
