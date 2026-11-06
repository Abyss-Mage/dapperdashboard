'use client';

import p from './profile.module.css';

type B = { id: string; serviceName: string; status: string; bookingTime: string };
const when = (iso: string) => new Intl.DateTimeFormat('en-CA', { dateStyle: 'full', timeStyle: 'short' }).format(new Date(iso));

/* Coming up (soonest first) and previous bookings (latest first), with Book again on the ones that are over. */
export default function BookingHistory({ bookings }: { bookings: B[] }) {
  const now = Date.now();
  const live = (b: B) => new Date(b.bookingTime).getTime() > now && !/cancel|complete|no_show/i.test(b.status);
  const upcoming = bookings.filter(live).sort((a, b) => +new Date(a.bookingTime) - +new Date(b.bookingTime));
  const past = bookings.filter((b) => !live(b)).sort((a, b) => +new Date(b.bookingTime) - +new Date(a.bookingTime));
  const row = (b: B, again: boolean) => (
    <li key={b.id} className={p.item}>
      <div><strong>{b.serviceName}</strong><p>{when(b.bookingTime)}</p></div>
      <div className={p.row}>
        <span className={p.tag + ' ' + p.plain}>{b.status.replaceAll('_', ' ')}</span>
        {again && <a href="/booking" className={p.ghost + ' ' + p.link}>Book again</a>}
      </div>
    </li>
  );
  return (
    <section className={p.card + ' ' + p.col} aria-labelledby="bh-title">
      <h3 id="bh-title" className={p.h}>Your bookings</h3>
      {bookings.length === 0 && <p className={p.empty}>No bookings yet. <a href="/booking" className={p.inline}>Book your first detail</a>.</p>}
      {upcoming.length > 0 && <><h4 className={p.h4}>Coming up</h4><ul className={p.list}>{upcoming.map((b) => row(b, false))}</ul></>}
      {past.length > 0 && <><h4 className={p.h4}>Previous bookings</h4><ul className={p.list}>{past.map((b) => row(b, true))}</ul></>}
    </section>
  );
}
