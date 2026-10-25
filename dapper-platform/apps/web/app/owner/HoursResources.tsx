'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import { OwnerIcon } from './OwnerIcons';
import { DatePicker, TimePicker } from '../Pickers';

type Window = { start: string; end: string };
type Hour = { day: number; start: string; end: string };
type Resource = {
  id: string;
  name: string;
  timezone: string;
  capacity: number;
  open: { start: string; end: string }[];
  closed: { start: string; end: string }[];
  weeklyHours: Hour[] | null;
};

const DAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const ZONES = ['America/Toronto', 'America/Vancouver', 'America/Edmonton', 'America/Winnipeg', 'America/Halifax', 'America/St_Johns', 'America/Regina'];

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'The change could not be saved.');
  return body;
}
const json = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

function dayText(iso: string, tz: string) {
  return new Date(iso).toLocaleDateString('en-CA', { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric', timeZone: tz });
}
export function summarize(r: Resource) {
  const now = Date.now();
  const future = r.open.filter((w) => new Date(w.end).getTime() > now);
  const through = future.length ? future.reduce((a, w) => (w.end > a ? w.end : a), future[0].end) : '';
  return { openWindows: future.length, through };
}

function WeeklyEditor({ resource, base, onSaved }: { resource: Resource; base: string; onSaved: () => void }) {
  const initial = (): Window[][] => DAYS.map((_, d) => (resource.weeklyHours ?? []).filter((h) => h.day === d).map((h) => ({ start: h.start, end: h.end })));
  const [days, setDays] = useState<Window[][]>(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  useEffect(() => setDays(initial()), [resource.id, resource.weeklyHours]); // eslint-disable-line react-hooks/exhaustive-deps

  const set = (d: number, i: number, key: 'start' | 'end', v: string) => setDays((cur) => cur.map((w, di) => (di === d ? w.map((x, xi) => (xi === i ? { ...x, [key]: v } : x)) : w)));
  const add = (d: number) => setDays((cur) => cur.map((w, di) => (di === d ? [...w, w.length ? { start: '13:00', end: '17:00' } : { start: '09:00', end: '17:00' }] : w)));
  const remove = (d: number, i: number) => setDays((cur) => cur.map((w, di) => (di === d ? w.filter((_, xi) => xi !== i) : w)));

  async function save() {
    setError('');
    setNotice('');
    for (const [d, ws] of days.entries()) for (const w of ws) if (!w.start || !w.end || (w.end !== '24:00' && w.start >= w.end)) return setError(`${DAYS[d]}: each window must end after it starts.`);
    setBusy(true);
    try {
      const weekly = days.flatMap((ws, day) => ws.map((w) => ({ day, ...w })));
      await readJson(await fetch(`${base}/scheduling/resources/${resource.id}/hours`, json('PUT', { weekly })));
      setNotice('Weekly hours saved. Opening times are generated 12 weeks ahead and extended automatically.');
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'The hours could not be saved.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={styles.stack}>
      <ul className={styles.weekGrid}>
        {DAYS.map((name, d) => (
          <li key={name}>
            <span className={styles.weekDay}>{name}</span>
            <div className={styles.weekWindows}>
              {days[d].length === 0 && <span className={styles.personSub}>Closed</span>}
              {days[d].map((w, i) => (
                <div key={i} className={styles.weekWindow}>
                  <TimePicker aria-label={`${name} opens`} value={w.start} onChange={(e) => set(d, i, 'start', e.target.value)} disabled={busy} />
                  <span aria-hidden="true">to</span>
                  <TimePicker aria-label={`${name} closes`} value={w.end === '24:00' ? '23:59' : w.end} onChange={(e) => set(d, i, 'end', e.target.value)} disabled={busy} />
                  <button type="button" className={styles.linkBtn} onClick={() => remove(d, i)} disabled={busy} aria-label={`Remove ${name} window`}>
                    <OwnerIcon name="close" size={14} />
                  </button>
                </div>
              ))}
              {days[d].length < 4 && (
                <button type="button" className={styles.linkBtn} onClick={() => add(d)} disabled={busy}>
                  {days[d].length ? 'Add another window' : 'Open this day'}
                </button>
              )}
            </div>
          </li>
        ))}
      </ul>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <div>
        <button type="button" className="primary" disabled={busy} onClick={save}>{busy ? 'Saving…' : 'Save weekly hours'}</button>
      </div>
      <p className={styles.footnote}>Saving replaces opening times from now on. Existing bookings and reservations are not moved or cancelled. Times are in {resource.timezone}; daylight-saving changes keep the same local opening time.</p>
    </div>
  );
}

function Closures({ resource, base, onSaved }: { resource: Resource; base: string; onSaved: () => void }) {
  const [date, setDate] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const now = Date.now();
  const upcoming = resource.closed.filter((c) => new Date(c.end).getTime() > now).sort((a, b) => a.start.localeCompare(b.start));
  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setError('');
    try {
      await fn();
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'The change could not be saved.');
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className={styles.stack}>
      <form className={styles.inlineForm} onSubmit={(e) => { e.preventDefault(); if (date) void run(async () => { await readJson(await fetch(`${base}/scheduling/resources/${resource.id}/closures`, json('POST', { date }))); setDate(''); }); }}>
        <label className="form-field">Close for a whole day (holiday, maintenance)<DatePicker required value={date} onChange={(e) => setDate(e.target.value)} disabled={busy} /></label>
        <button className="secondary" disabled={busy || !date}>Add closure</button>
      </form>
      {error && <p role="alert" className="field-error">{error}</p>}
      {upcoming.length === 0 ? <p className={styles.personSub}>No upcoming closures.</p> : (
        <ul className={styles.closures}>
          {upcoming.map((c) => (
            <li key={c.start}>
              <span>{dayText(c.start, resource.timezone)}</span>
              <button type="button" className={styles.linkBtn} disabled={busy} onClick={() => void run(async () => readJson(await fetch(`${base}/scheduling/resources/${resource.id}/closures?start=${encodeURIComponent(c.start)}`, json('DELETE'))))}>Remove</button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export default function HoursResources({ base, canManage, onChanged }: { base: string; canManage: boolean; onChanged?: () => void }) {
  const [items, setItems] = useState<Resource[] | null>(null);
  const [error, setError] = useState('');
  const [name, setName] = useState('');
  const [zone, setZone] = useState('');
  const [capacity, setCapacity] = useState('1');
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState('');

  const load = useCallback(async () => {
    try {
      const body = await readJson(await fetch(`${base}/scheduling/resources`, { cache: 'no-store' }));
      setItems(body.items);
      setError('');
      onChanged?.();
    } catch (e) {
      setItems(null);
      setError(e instanceof Error ? e.message : 'Resources could not be loaded.');
    }
  }, [base, onChanged]);
  useEffect(() => {
    void load();
  }, [load]);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const made = await readJson(await fetch(`${base}/scheduling/resources`, json('POST', { name: name.trim(), timezone: zone.trim(), capacity: Number(capacity), open: [], closed: [] })));
      setName('');
      setCapacity('1');
      await load();
      setOpen(made.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'The resource could not be created.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={styles.stack}>
      {error && <p role="alert" className="field-error">{error}</p>}
      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading resources" />}
      {items && items.length === 0 && <p className={styles.status}>No resources yet. A resource is a bay, room or crew that can take bookings at the same time; add one below, then set its weekly hours.</p>}
      {items?.map((r) => {
        const s = summarize(r);
        const hasHours = (r.weeklyHours?.length ?? 0) > 0;
        const expanded = open === r.id;
        return (
          <section key={r.id} className={styles.resource}>
            <div className={styles.resourceHead}>
              <div>
                <h3>{r.name}</h3>
                <p className={styles.personSub}>{r.timezone} · capacity {r.capacity} booking{r.capacity === 1 ? '' : 's'} at a time</p>
              </div>
              <span className={`${styles.badge} ${hasHours ? styles.badge_complete : styles.badge_incomplete}`}>
                {hasHours ? `Open through ${s.through ? dayText(s.through, r.timezone) : 'n/a'}` : 'No weekly hours'}
              </span>
            </div>
            {hasHours && (
              <p className={styles.personSub}>
                {DAYS.map((d, i) => {
                  const w = (r.weeklyHours ?? []).filter((h) => h.day === i);
                  return w.length ? `${d.slice(0, 3)} ${w.map((x) => `${x.start}–${x.end}`).join(', ')}` : null;
                }).filter(Boolean).join(' · ')}
              </p>
            )}
            {!hasHours && s.openWindows > 0 && <p className={styles.personSub}>This resource has {s.openWindows} hand-entered dated opening windows. Saving weekly hours replaces the ones that have not ended yet.</p>}
            {canManage && (
              <>
                <button type="button" className="secondary" aria-expanded={expanded} onClick={() => setOpen(expanded ? '' : r.id)}>{expanded ? 'Hide hours editor' : 'Edit hours and closures'}</button>
                {expanded && (
                  <div className={styles.stack}>
                    <h4 className={styles.subHead}>Weekly hours</h4>
                    <WeeklyEditor resource={r} base={base} onSaved={() => void load()} />
                    <h4 className={styles.subHead}>Closures</h4>
                    <Closures resource={r} base={base} onSaved={() => void load()} />
                  </div>
                )}
              </>
            )}
          </section>
        );
      })}

      {canManage && (
        <form onSubmit={create} className={styles.resource}>
          <h3>Add a resource</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Name<input required minLength={2} maxLength={120} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} placeholder="e.g. Main bay" /></label>
            <label className="form-field">Timezone<input required list="owner-zones" value={zone} onChange={(e) => setZone(e.target.value)} disabled={busy} placeholder="Where the franchise is, e.g. America/Toronto" /></label>
            <label className="form-field">Bookings at once<input required type="number" min={1} max={1000} value={capacity} onChange={(e) => setCapacity(e.target.value)} disabled={busy} /></label>
            <button className="primary" disabled={busy || name.trim().length < 2 || !zone.trim()}>{busy ? 'Adding…' : 'Add resource'}</button>
          </div>
          <datalist id="owner-zones">{ZONES.map((z) => <option key={z} value={z} />)}</datalist>
          <p className={styles.footnote}>Names and numbers come from you; nothing is pre-filled from policy. Customers can book a resource only after it has hours and a scheduling rule.</p>
        </form>
      )}
    </div>
  );
}
