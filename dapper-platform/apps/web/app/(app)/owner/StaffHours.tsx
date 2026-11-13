'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import RangeBar, { resolveRange, type Custom, type RangePresetId } from './DateRange';
import ShopClockSetup from './ShopClockSetup';
import Select from '../GlassSelect';

/* Staff hours for the owner: who is on shift now, the shop iPad (its login, link and each person's PIN), and how long each person worked per
   day, week or month in the chosen dates. Staff punch on the shop iPad; location tracking was retired with the personal clock-in (2026-11-11). */

type Day = { date: string; minutes: number; shifts: number; clockIn?: string | null; clockOut?: string | null; open?: boolean };
type Staff = { principalId: string; name: string; email: string; totalMinutes: number; clockedIn: boolean; days: Day[] };
type Entry = { attendanceId: string; employee: string; shiftStatus: string; clockInAt?: string; breakStartedAt?: string; breakMinutes?: number };
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

export default function StaffHours({ base, timezone, canManage }: { base: string; timezone: string; canManage: boolean }) {
  const [preset, setPreset] = useState<RangePresetId>('last7');
  const [custom, setCustom] = useState<Custom>({ from: '', to: '' });
  const [by, setBy] = useState<By>('day');
  const [who, setWho] = useState('');
  const [staff, setStaff] = useState<Staff[] | null>(null);
  const [live, setLive] = useState<Entry[]>([]);
  const [error, setError] = useState('');
  const range = resolveRange(preset, custom);

  const load = useCallback(async () => {
    const url = `${base}/attendance/totals?from=${range.from}&to=${range.to}&tz=${encodeURIComponent(timezone)}`;
    // One quiet retry: a passing server hiccup should not leave an error on the page (error404 #40).
    let r = await fetch(url, { cache: 'no-store' });
    if (r.status >= 500) r = await fetch(url, { cache: 'no-store' });
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'Staff hours could not be loaded.');
    setStaff(b.staff);
    setError('');
  }, [base, range.from, range.to, timezone]);
  const loadLive = useCallback(async () => {
    const r = await fetch(`${base}/gps/live`, { cache: 'no-store' });
    if (r.ok) { const b = await r.json(); setLive(b.entries ?? []); }
  }, [base]);
  useEffect(() => { setStaff(null); setError(''); load().catch((e) => setError(e.message)); }, [load]);
  useEffect(() => {
    void loadLive();
    const id = window.setInterval(() => { void loadLive(); void load().catch(() => undefined); }, 60_000);
    return () => window.clearInterval(id);
  }, [loadLive, load]);

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
      {/* error404 #97: a clear card per person (who, working or on break, since when, how long) instead of small tags. */}
      <section className={styles.card} style={{ padding: 20 }}>
        <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'baseline', justifyContent: 'space-between', gap: 12 }}>
          <h2 className={styles.setupTitle} style={{ fontSize: 22, margin: 0 }}>On shift now</h2>
          <span className={styles.personSub}>Updates every minute while this page is open</span>
        </div>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', margin: '14px 0 4px' }}>
          {[{ label: 'Working', value: live.filter((e) => e.shiftStatus === 'clocked_in').length, color: '#22c55e' }, { label: 'On break', value: live.filter((e) => e.shiftStatus === 'on_break').length, color: '#f59e0b' }].map((c) => (
            <div key={c.label} style={{ flex: '1 1 160px', border: '1px solid var(--glass-line, rgba(255,255,255,.12))', borderRadius: 16, padding: '14px 18px' }}>
              <div style={{ fontSize: 36, fontWeight: 800, lineHeight: 1, color: c.color }}>{c.value}</div>
              <div style={{ fontSize: 15, fontWeight: 600, marginTop: 6 }}>{c.label}</div>
            </div>
          ))}
        </div>
        {live.length === 0 ? <p className={styles.status} style={{ fontSize: 16 }}>No one is clocked in.</p> : (
          <ul style={{ listStyle: 'none', padding: 0, margin: '14px 0 0', display: 'grid', gap: 12, gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))' }}>
            {live.map((e) => {
              const onBreak = e.shiftStatus === 'on_break';
              const now = Date.now();
              const worked = e.clockInAt ? Math.max(0, Math.round((now - new Date(e.clockInAt).getTime()) / 60000) - (e.breakMinutes ?? 0) - (onBreak && e.breakStartedAt ? Math.round((now - new Date(e.breakStartedAt).getTime()) / 60000) : 0)) : null;
              const breakFor = onBreak && e.breakStartedAt ? Math.max(0, Math.round((now - new Date(e.breakStartedAt).getTime()) / 60000)) : null;
              const initials = e.employee.split(/[\s@.]+/).filter(Boolean).slice(0, 2).map((w) => w[0]?.toUpperCase()).join('');
              return (
                <li key={e.attendanceId} style={{ display: 'flex', gap: 14, alignItems: 'center', border: '1px solid var(--glass-line, rgba(255,255,255,.12))', borderLeft: `5px solid ${onBreak ? '#f59e0b' : '#22c55e'}`, borderRadius: 16, padding: '14px 16px' }}>
                  <span aria-hidden="true" style={{ flex: 'none', width: 48, height: 48, borderRadius: '50%', display: 'grid', placeItems: 'center', fontSize: 18, fontWeight: 800, background: onBreak ? 'rgba(245,158,11,.18)' : 'rgba(34,197,94,.18)', color: onBreak ? '#f59e0b' : '#22c55e' }}>{initials || '?'}</span>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontSize: 18, fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{e.employee}</div>
                    <div style={{ marginTop: 4 }}><span style={{ display: 'inline-block', fontSize: 13, fontWeight: 700, padding: '3px 10px', borderRadius: 999, background: onBreak ? 'rgba(245,158,11,.18)' : 'rgba(34,197,94,.18)', color: onBreak ? '#f59e0b' : '#22c55e' }}>{onBreak ? `On break${breakFor !== null ? ` · ${hours(breakFor)}` : ''}` : 'Working'}</span></div>
                    {e.clockInAt && <div className={styles.personSub} style={{ fontSize: 14, marginTop: 6 }}>Clocked in {clock(e.clockInAt, timezone)}{worked !== null ? ` · ${hours(worked)} worked` : ''}</div>}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <ShopClockSetup base={base} canManage={canManage} />

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
