'use client';
import { useEffect, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, RefreshCw, X } from 'lucide-react';
import { calendarRange, dateOffset, readableDay, worksiteDate } from '../workspace/employee_helpers';
import styles from '../workspace/employee.module.css';

/* Master's bookings calendar: the same month / week / day calendar as the staff workspace, for the selected franchise. */

type Booking = { id: string; customerName: string; customerEmail: string; customerPhone: string; serviceName: string; bookingTime: string; status: string; channel: string; addons: { name: string }[] };

export default function BookingCalendar({ base, timezone, calendarLink }: { base: string; timezone: string; calendarLink: string }) {
  const today = worksiteDate(new Date().toISOString(), timezone);
  const [day, setDay] = useState(today), [view, setView] = useState<'month' | 'week' | 'day'>('month');
  const [items, setItems] = useState<Booking[]>([]), [selected, setSelected] = useState<Booking | null>(null);
  const [loading, setLoading] = useState(true), [error, setError] = useState(''), [revision, setRevision] = useState(0), [copied, setCopied] = useState(false);
  const generation = useRef(0), detail = useRef<HTMLHeadingElement>(null);
  const { from, to, days } = calendarRange(day, view);

  useEffect(() => {
    const current = ++generation.current;
    setLoading(true); setError(''); setItems([]); setSelected(null);
    // A day either side so bookings near midnight in the franchise time zone are not missed.
    const start = `${dateOffset(from, -1)}T00:00:00Z`, end = `${dateOffset(to, 2)}T00:00:00Z`;
    fetch(`${base}/bookings?from=${encodeURIComponent(start)}&to=${encodeURIComponent(end)}&limit=1000`, { cache: 'no-store' })
      .then(async (r) => { const b = await r.json(); if (!r.ok) throw new Error(b.error ?? 'The calendar could not load.'); return b; })
      .then((b) => { if (current === generation.current) setItems(b.items ?? []); })
      .catch((e) => { if (current === generation.current) setError(e.message); })
      .finally(() => { if (current === generation.current) setLoading(false); });
    return () => { generation.current++; };
  }, [base, from, to, revision]);
  useEffect(() => { if (selected) detail.current?.focus(); }, [selected]);

  const time = (value: string) => new Intl.DateTimeFormat('en-CA', { timeZone: timezone, hour: 'numeric', minute: '2-digit' }).format(new Date(value));
  function step(direction: number) {
    if (view !== 'month') { setDay(dateOffset(day, direction * (view === 'week' ? 7 : 1))); return; }
    const d = new Date(`${day.slice(0, 7)}-01T12:00:00Z`); d.setUTCMonth(d.getUTCMonth() + direction); setDay(d.toISOString().slice(0, 10));
  }
  const link = typeof window === 'undefined' ? calendarLink : new URL(calendarLink, window.location.origin).toString();

  return (
    <section className={styles.calendar} aria-label="Bookings calendar">
      <div className={styles.calendarToolbar}>
        <div className={styles.calendarPeriod}>
          <button className={styles.iconButton} aria-label="Previous" onClick={() => step(-1)}><ChevronLeft size={18} /></button>
          <h2>{readableDay(day, view === 'month' ? { month: 'long', year: 'numeric' } : { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' })}</h2>
          <button className={styles.iconButton} aria-label="Next" onClick={() => step(1)}><ChevronRight size={18} /></button>
        </div>
        <div className={styles.calendarViews} aria-label="Calendar view">{(['month', 'week', 'day'] as const).map((v) => <button key={v} className={styles.secondary} aria-pressed={view === v} onClick={() => setView(v)}>{v}</button>)}</div>
        <button className={styles.secondary} onClick={() => setDay(today)}>Today</button>
        <label>Go to date<input type="date" value={day} onChange={(e) => e.target.value && setDay(e.target.value)} /></label>
        <button className={styles.iconButton} aria-label="Reload calendar" onClick={() => setRevision((v) => v + 1)}><RefreshCw size={18} /></button>
        <button className={styles.secondary} onClick={() => { void navigator.clipboard?.writeText(link).then(() => { setCopied(true); setTimeout(() => setCopied(false), 2500); }); }} title="Add this link to Google Calendar or Apple Calendar to see bookings on your phone">{copied ? 'Link copied' : 'Link Calendar'}</button>
      </div>
      <p className={styles.help}>{timezone} · start times · Link Calendar copies a link you can add to Google or Apple Calendar.</p>
      {loading ? <p role="status" className={styles.empty}>Loading calendar…</p> : error ? <p role="alert" className={styles.error}>{error}</p> : (
        <div className={styles.calendarScroll} tabIndex={0} aria-label="Calendar dates; scroll sideways on small screens">
          <div className={view === 'month' ? styles.monthGrid : styles.calendarAgenda}>
            {view === 'month' && ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map((d) => <span key={d} className={styles.weekday}>{d}</span>)}
            {Array.from({ length: days }, (_, i) => {
              const date = dateOffset(from, i), bookings = items.filter((b) => worksiteDate(b.bookingTime, timezone) === date).sort((a, b) => a.bookingTime.localeCompare(b.bookingTime));
              return (
                <section key={date} className={`${styles.calendarDay} ${date === today ? styles.calendarToday : ''}`} aria-label={readableDay(date)}>
                  <button className={styles.calendarDate} aria-label={`Open ${readableDay(date)}`} onClick={() => { setDay(date); setView('day'); }}><time dateTime={date}>{view === 'month' ? Number(date.slice(-2)) : readableDay(date)}</time></button>
                  {bookings.map((b) => <button className={styles.calendarEvent} key={b.id} onClick={() => setSelected(b)}><time dateTime={b.bookingTime}>{time(b.bookingTime)}</time><strong>{b.customerName}</strong><span>{b.serviceName}</span><span>{b.status.replaceAll('_', ' ')}</span></button>)}
                  {!bookings.length && view !== 'month' && <p className={styles.help}>No bookings.</p>}
                </section>
              );
            })}
          </div>
        </div>
      )}
      {selected && (
        <section className={styles.calendarDetails} aria-label="Selected booking">
          <div className={styles.sectionTop}><h2 ref={detail} tabIndex={-1}>{selected.customerName}</h2><button className={styles.iconButton} aria-label="Close booking details" onClick={() => setSelected(null)}><X size={18} /></button></div>
          <p>{readableDay(worksiteDate(selected.bookingTime, timezone))} · {time(selected.bookingTime)} · {selected.status.replaceAll('_', ' ')} · {selected.channel === 'online' ? 'Online' : 'Walk-in'}</p>
          <p>{selected.serviceName}{selected.addons?.length ? ` · ${selected.addons.map((a) => a.name).join(', ')}` : ''}</p>
          <div className={styles.contacts}>{selected.customerEmail && <a href={`mailto:${selected.customerEmail}`}>{selected.customerEmail}</a>}{selected.customerPhone && <a href={`tel:${selected.customerPhone}`}>{selected.customerPhone}</a>}</div>
        </section>
      )}
    </section>
  );
}
