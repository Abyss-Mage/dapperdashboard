'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import { TimePicker } from '../Pickers';
import NumberField from '../NumberField';
import Select from '../GlassSelect';

/* A franchise is one place with one set of opening hours. Behind the scenes the franchise may book work on more than one
   bay or crew; the hours entered here are saved to all of them, so the owner only ever sets hours once.
   Also here: how many bookings can start at the same time (slots), and a calendar for marking holidays (closed days). */

type Hour = { day: number; start: string; end: string };
type Closure = { start: string; end: string };
type Resource = { id: string; name: string; timezone: string; capacity: number; weeklyHours: Hour[] | null; closed: Closure[] };
type Window = { start: string; end: string };
const DAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The change could not be saved.'));
  return body;
}
const json = { 'Content-Type': 'application/json' };
const ymd = (y: number, m: number, d: number) => `${y}-${String(m + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
/** The calendar day an instant falls on in a time zone. */
const dayIn = (iso: string, tz: string) => new Intl.DateTimeFormat('en-CA', { timeZone: tz, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(iso));

export default function OpeningHours({ base, franchiseName, canManage }: { base: string; franchiseName: string; canManage: boolean }) {
  const [resources, setResources] = useState<Resource[] | null>(null);
  const [days, setDays] = useState<Window[][]>(DAYS.map(() => []));
  const [slots, setSlots] = useState('1');
  const [zone, setZone] = useState(typeof Intl !== 'undefined' ? Intl.DateTimeFormat().resolvedOptions().timeZone : 'America/Toronto');
  const [month, setMonth] = useState(() => { const d = new Date(); return { y: d.getFullYear(), m: d.getMonth() }; });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const zones = useMemo(() => { const z = typeof Intl !== 'undefined' && Intl.supportedValuesOf ? Intl.supportedValuesOf('timeZone') : [zone]; return z.includes(zone) ? z : [zone, ...z]; }, [zone]);

  const load = useCallback(async () => {
    const list: Resource[] = (await readJson(await fetch(`${base}/scheduling/resources`, { cache: 'no-store' }))).items ?? [];
    setResources(list);
    const source = list.find((r) => (r.weeklyHours?.length ?? 0) > 0);
    setDays(DAYS.map((_, d) => (source?.weeklyHours ?? []).filter((h) => h.day === d).map((h) => ({ start: h.start, end: h.end }))));
    if (list[0]) { setSlots(String(list[0].capacity)); setZone(list[0].timezone); }
  }, [base]);
  useEffect(() => { setResources(null); setError(''); setNotice(''); load().catch((e) => setError(e.message)); }, [load]);

  /** Closed whole days, from the first resource (every resource is closed together). */
  const holidays = useMemo(() => {
    const map = new Map<string, Closure>();
    const first = resources?.[0];
    if (!first) return map;
    for (const c of first.closed ?? []) {
      const hours = (Date.parse(c.end) - Date.parse(c.start)) / 3600000;
      if (hours >= 23 && hours <= 25) map.set(dayIn(c.start, first.timezone), c);
    }
    return map;
  }, [resources]);

  const set = (d: number, i: number, key: 'start' | 'end', v: string) => setDays((c) => c.map((w, di) => (di === d ? w.map((x, xi) => (xi === i ? { ...x, [key]: v } : x)) : w)));
  const toggle = (d: number, open: boolean) => setDays((c) => c.map((w, di) => (di === d ? (open ? [{ start: '09:00', end: '17:00' }] : []) : w)));
  const addBreak = (d: number) => setDays((c) => c.map((w, di) => (di === d ? [...w, { start: '13:00', end: '17:00' }] : w)));
  const remove = (d: number, i: number) => setDays((c) => c.map((w, di) => (di === d ? w.filter((_, xi) => xi !== i) : w)));
  const copyMonday = () => setDays((c) => c.map((w, di) => (di === 0 ? w : c[0].map((x) => ({ ...x })))));

  async function saveSlots() {
    setError(''); setNotice('');
    const n = Number(slots);
    if (!Number.isInteger(n) || n < 1 || n > 1000) return setError('Slots must be a whole number from 1 to 1000.');
    setBusy(true);
    try {
      await readJson(await fetch(`${base}/scheduling/slots`, { method: 'PUT', headers: json, body: JSON.stringify({ slots: n, timezone: zone }) }));
      setNotice(resources?.length ? `Saved. Up to ${n} booking${n === 1 ? '' : 's'} can start at the same time. Bookings already made are not moved.` : 'Booking calendar created. Now set the opening hours below.');
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The slots could not be saved.'); }
    finally { setBusy(false); }
  }

  async function save() {
    setError(''); setNotice('');
    for (const [d, ws] of days.entries()) for (const w of ws) if (!w.start || !w.end || w.start >= w.end) return setError(`${DAYS[d]}: closing time must be after opening time.`);
    if (!resources?.length) return setError('Create the booking calendar first (choose the time zone and slots above).');
    setBusy(true);
    try {
      const weekly = days.flatMap((ws, day) => ws.map((w) => ({ day, ...w })));
      for (const r of resources) await readJson(await fetch(`${base}/scheduling/resources/${r.id}/hours`, { method: 'PUT', headers: json, body: JSON.stringify({ weekly }) }));
      setNotice('Opening hours saved. Customers can book inside these hours.');
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The hours could not be saved.'); }
    finally { setBusy(false); }
  }

  async function toggleHoliday(date: string) {
    if (!resources?.length || !canManage) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const closed = holidays.has(date);
      for (const r of resources) {
        if (closed) {
          const c = r.closed.find((x) => dayIn(x.start, r.timezone) === date && (Date.parse(x.end) - Date.parse(x.start)) / 3600000 >= 23);
          if (c) await readJson(await fetch(`${base}/scheduling/resources/${r.id}/closures?start=${encodeURIComponent(c.start)}`, { method: 'DELETE' }));
        } else {
          const res = await fetch(`${base}/scheduling/resources/${r.id}/closures`, { method: 'POST', headers: json, body: JSON.stringify({ date }) });
          if (!res.ok && res.status !== 409) await readJson(res);
        }
      }
      setNotice(closed ? `${date} is open again.` : `${date} is closed for the holiday. Bookings already made on that day are not cancelled.`);
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The holiday could not be saved.'); }
    finally { setBusy(false); }
  }

  const todayKey = dayIn(new Date().toISOString(), resources?.[0]?.timezone ?? zone);
  const first = new Date(month.y, month.m, 1), lead = (first.getDay() + 6) % 7, count = new Date(month.y, month.m + 1, 0).getDate();
  const upcoming = [...holidays.keys()].filter((k) => k >= todayKey).sort();

  return (
    <div className={styles.stack}>
      <div>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Opening hours</h2>
        <p className={styles.personSub}>When {franchiseName} is open for bookings, how many bookings can start at once, and which days it is closed.</p>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {!resources && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading hours" />}

      {resources && (
        <section className={styles.card} style={{ padding: 20 }}>
          <h3 style={{ marginTop: 0 }}>Booking slots</h3>
          <p className={styles.personSub}>A slot is one booking that can run at the same time. With 2 slots, two customers can book the same time; the third is offered another time.</p>
          <div className={styles.inlineForm} style={{ alignItems: 'flex-end' }}>
            <label className="form-field">Slots for each time
              <NumberField inputMode="numeric" min="1" max="1000" step="1" value={slots} onChange={(e) => setSlots(e.target.value)} disabled={busy || !canManage} />
            </label>
            {!resources.length && (
              <label className="form-field" style={{ minWidth: 240 }}>Time zone
                <Select value={zone} onChange={(e) => setZone(e.target.value)} disabled={busy || !canManage}>{zones.map((z) => <option key={z} value={z}>{z}</option>)}</Select>
              </label>
            )}
            {canManage && <button type="button" className="primary" disabled={busy} onClick={() => void saveSlots()}>{busy ? 'Saving…' : resources.length ? 'Save slots' : 'Create booking calendar'}</button>}
          </div>
        </section>
      )}

      {resources && (
        <section className={styles.card} style={{ padding: 20 }}>
          <h3 style={{ marginTop: 0 }}>Weekly hours</h3>
          <ul className={styles.weekGrid}>
            {DAYS.map((name, d) => (
              <li key={name}>
                <span className={styles.weekDay}>{name}</span>
                <div className={styles.weekWindows}>
                  <label style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
                    <input type="checkbox" checked={days[d].length > 0} disabled={busy || !canManage} onChange={(e) => toggle(d, e.target.checked)} />
                    <span>{days[d].length ? 'Open' : 'Closed'}</span>
                  </label>
                  {days[d].map((w, i) => (
                    <div key={i} className={styles.weekWindow}>
                      <TimePicker aria-label={`${name} opens`} value={w.start} onChange={(e) => set(d, i, 'start', e.target.value)} disabled={busy || !canManage} />
                      <span aria-hidden="true">to</span>
                      <TimePicker allowEndOfDay aria-label={`${name} closes`} value={w.end} onChange={(e) => set(d, i, 'end', e.target.value)} disabled={busy || !canManage} />
                      {i > 0 && <button type="button" className="secondary" disabled={busy || !canManage} onClick={() => remove(d, i)} aria-label={`Remove this ${name} time`}>Remove</button>}
                    </div>
                  ))}
                  {days[d].length > 0 && canManage && <button type="button" className="secondary" disabled={busy} onClick={() => addBreak(d)}>Add a second time (after a break)</button>}
                </div>
              </li>
            ))}
          </ul>
          {canManage && (
            <div className={styles.rowBetween} style={{ marginTop: 16 }}>
              <button type="button" className="secondary" disabled={busy} onClick={copyMonday}>Use Monday&rsquo;s hours every day</button>
              <button type="button" className="primary" disabled={busy || !resources.length} onClick={() => void save()}>{busy ? 'Saving…' : 'Save opening hours'}</button>
            </div>
          )}
        </section>
      )}

      {resources && resources.length > 0 && (
        <section className={styles.card} style={{ padding: 20 }}>
          <h3 style={{ marginTop: 0 }}>Holidays</h3>
          <p className={styles.personSub}>Click a day to close it for the whole day (a holiday); click it again to reopen. Customers cannot book a closed day, and bookings already made are left as they are.</p>
          <div className={styles.holidayHead}>
            <button type="button" className="secondary" aria-label="Previous month" onClick={() => setMonth((v) => (v.m === 0 ? { y: v.y - 1, m: 11 } : { y: v.y, m: v.m - 1 }))}>‹</button>
            <h3>{MONTHS[month.m]} {month.y}</h3>
            <button type="button" className="secondary" aria-label="Next month" onClick={() => setMonth((v) => (v.m === 11 ? { y: v.y + 1, m: 0 } : { y: v.y, m: v.m + 1 }))}>›</button>
          </div>
          <div className={styles.holidayGrid} role="group" aria-label={`${MONTHS[month.m]} ${month.y}`}>
            {['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map((d) => <span key={d} className={styles.holidayDow}>{d}</span>)}
            {Array.from({ length: lead }, (_, i) => <span key={`b${i}`} className={styles.holidayBlank} />)}
            {Array.from({ length: count }, (_, i) => {
              const key = ymd(month.y, month.m, i + 1), closed = holidays.has(key);
              return <button key={key} type="button" className={`${styles.holidayDay} ${key === todayKey ? styles.holidayToday : ''}`} aria-pressed={closed} aria-label={`${key}${closed ? ', closed' : ''}`} disabled={busy || !canManage || key < todayKey} onClick={() => void toggleHoliday(key)}>{i + 1}</button>;
            })}
          </div>
          {upcoming.length > 0 && (
            <div style={{ marginTop: 14 }}>
              <strong style={{ fontSize: 13 }}>Closed days coming up</strong>
              <div className={styles.svcTags} style={{ marginTop: 8 }}>
                {upcoming.map((k) => <span key={k} className={styles.chip}>{new Date(`${k}T12:00:00`).toLocaleDateString('en-CA', { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' })}{canManage && <button type="button" onClick={() => void toggleHoliday(k)} disabled={busy} aria-label={`Reopen ${k}`} style={{ all: 'unset', cursor: 'pointer', marginLeft: 8, opacity: .7 }}>×</button>}</span>)}
              </div>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
