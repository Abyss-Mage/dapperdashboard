'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import RangeBar, { resolveRange, type Custom, type RangePresetId } from './DateRange';
import StaffMap, { type MapPoint } from './StaffMap';
import Select from '../GlassSelect';

/* Staff hours for the owner: where clocked-in staff are right now, and how long each person worked per day, week or month
   in the chosen dates. Clocking in and out happens in the staff app, not here. */

type Day = { date: string; minutes: number; shifts: number; clockIn?: string | null; clockOut?: string | null; open?: boolean };
type Staff = { principalId: string; name: string; email: string; totalMinutes: number; clockedIn: boolean; days: Day[] };
type Entry = { attendanceId: string; employee: string; shiftStatus: string; stale: boolean; latitude?: number | null; longitude?: number | null };
type By = 'day' | 'week' | 'month';

const hours = (m: number) => (m < 60 ? `${m} min` : `${Math.floor(m / 60)} h${m % 60 ? ` ${m % 60} min` : ''}`);
const weekOf = (date: string) => { const d = new Date(`${date}T12:00:00`); d.setDate(d.getDate() - ((d.getDay() + 6) % 7)); return d.toISOString().slice(0, 10); };
const label = (key: string, by: By) => {
  const d = new Date(`${key}T12:00:00`);
  if (by === 'month') return d.toLocaleDateString('en-CA', { month: 'long', year: 'numeric' });
  if (by === 'week') return `Week of ${d.toLocaleDateString('en-CA', { month: 'short', day: 'numeric', year: 'numeric' })}`;
  return d.toLocaleDateString('en-CA', { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' });
};

const clock = (iso: string, zone: string) => new Date(iso).toLocaleTimeString('en-CA', { hour: 'numeric', minute: '2-digit', timeZone: zone });

export default function StaffHours({ base, timezone }: { base: string; timezone: string }) {
  const [preset, setPreset] = useState<RangePresetId>('last7');
  const [custom, setCustom] = useState<Custom>({ from: '', to: '' });
  const [by, setBy] = useState<By>('day');
  const [who, setWho] = useState('');
  const [staff, setStaff] = useState<Staff[] | null>(null);
  const [live, setLive] = useState<Entry[]>([]);
  const [error, setError] = useState('');
  const range = resolveRange(preset, custom);

  const load = useCallback(async () => {
    const r = await fetch(`${base}/attendance/totals?from=${range.from}&to=${range.to}&tz=${encodeURIComponent(timezone)}`, { cache: 'no-store' });
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'Staff hours could not be loaded.');
    setStaff(b.staff);
  }, [base, range.from, range.to, timezone]);
  const loadLive = useCallback(async () => {
    const r = await fetch(`${base}/gps/live`, { cache: 'no-store' });
    if (r.ok) setLive((await r.json()).entries ?? []);
  }, [base]);
  useEffect(() => { setStaff(null); setError(''); load().catch((e) => setError(e.message)); }, [load]);
  useEffect(() => {
    void loadLive();
    const id = window.setInterval(() => { void loadLive(); void load().catch(() => undefined); }, 60_000);
    return () => window.clearInterval(id);
  }, [loadLive, load]);

  const points: MapPoint[] = live.filter((e) => e.latitude != null && e.longitude != null).map((e) => ({ id: e.attendanceId, label: e.employee, lat: e.latitude as number, lon: e.longitude as number, stale: e.stale }));
  const shown = (staff ?? []).filter((s) => !who || s.principalId === who);
  const periods = useMemo(() => {
    const keys = new Set<string>();
    for (const s of shown) for (const d of s.days) keys.add(by === 'day' ? d.date : by === 'week' ? weekOf(d.date) : d.date.slice(0, 7) + '-01');
    return [...keys].sort().reverse();
  }, [shown, by]);
  const cell = (s: Staff, key: string) => s.days.filter((d) => (by === 'day' ? d.date : by === 'week' ? weekOf(d.date) : d.date.slice(0, 7) + '-01') === key).reduce((n, d) => n + d.minutes, 0);
  const total = shown.reduce((n, s) => n + s.totalMinutes, 0);

  return (
    <div className={styles.stack}>
      <section className={styles.card} style={{ padding: 20 }}>
        <div className={styles.rowBetween}>
          <div>
            <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Where your team is now</h2>
            <p className={styles.personSub}>{live.filter((e) => e.shiftStatus === 'clocked_in').length} on shift · updates every minute while this page is open</p>
          </div>
        </div>
        <StaffMap points={points} />
        {live.length > 0 && <ul className={styles.svcTags} style={{ marginTop: 10 }}>{live.map((e) => <li key={e.attendanceId} className={styles.chip}>{e.employee} · {e.shiftStatus === 'on_break' ? 'On break' : e.latitude == null ? 'No location shared' : e.stale ? 'Location not updated recently' : 'Sharing location'}</li>)}</ul>}
      </section>

      <section className={styles.card} style={{ padding: 20 }}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: '0 0 12px' }}>Time worked</h2>
        <div className={styles.stack} style={{ gap: 10 }}>
          <RangeBar presets={['last7', 'last30', 'month', 'year']} preset={preset} custom={custom} onPreset={setPreset} onCustom={setCustom} />
          <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
            <div className={styles.segments} role="group" aria-label="Show totals by">
              {(['day', 'week', 'month'] as By[]).map((b) => <button key={b} type="button" className={styles.segment} aria-pressed={by === b} onClick={() => setBy(b)}>{b === 'day' ? 'By day' : b === 'week' ? 'By week' : 'By month'}</button>)}
            </div>
            <label className={styles.filterSelect}><span>Staff</span>
              <Select value={who} onChange={(e) => setWho(e.target.value)}><option value="">Everyone</option>{(staff ?? []).map((s) => <option key={s.principalId} value={s.principalId}>{s.name}</option>)}</Select>
            </label>
          </div>
        </div>
        {error && <p role="alert" className="field-error">{error}</p>}
        {!staff && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading staff hours" />}
        {staff && shown.length === 0 && <p className={styles.status}>No time worked in these dates.</p>}
        {staff && shown.length > 0 && (
          <div style={{ overflowX: 'auto', marginTop: 14 }}>
            <table className={styles.table}>
              <caption className={styles.personSub} style={{ textAlign: 'left', marginBottom: 6 }}>Shift time minus breaks, counted on the day the shift started · {hours(total)} in total</caption>
              <thead><tr><th scope="col">{by === 'day' ? 'Day' : by === 'week' ? 'Week' : 'Month'}</th>{shown.map((s) => <th key={s.principalId} scope="col">{s.name}{s.clockedIn ? ' ●' : ''}</th>)}</tr></thead>
              <tbody>
                {periods.map((k) => <tr key={k}><th scope="row">{label(k, by)}</th>{shown.map((s) => { const m = cell(s, k); const d = by === 'day' ? s.days.find((x) => x.date === k) : undefined; return <td key={s.principalId}>{m ? <>{hours(m)}{d?.clockIn && <small className={styles.personSub} style={{ display: 'block' }}>In {clock(d.clockIn, timezone)} · {d.clockOut ? `Out ${clock(d.clockOut, timezone)}` : 'still on shift'}{d.shifts > 1 ? ` · ${d.shifts} shifts` : ''}</small>}</> : '—'}</td>; })}</tr>)}
              </tbody>
              <tfoot><tr><th scope="row">Total</th>{shown.map((s) => <td key={s.principalId}><strong>{hours(s.totalMinutes)}</strong></td>)}</tr></tfoot>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
