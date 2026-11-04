'use client';
import { ArrowUpRight, Coffee, MapPin, Square } from 'lucide-react';
import { useEffect, useState } from 'react';
import styles from './employee.module.css';

export type HomeShift = { id: string; status: 'clocked_in' | 'on_break' | 'clocked_out'; clockInAt: string; clockOutAt?: string; currentBreakStart?: string; totalBreakMinutes?: number; totalWorkMinutes: number };
type Hours = { todayHours: number; weekHours: number; monthHours: number; timezone: string; attribution: string };

const pad = (n: number) => String(n).padStart(2, '0');
function clock(seconds: number) { const s = Math.max(0, Math.floor(seconds)); return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`; }
function workSeconds(shift: HomeShift, now: number) {
  const end = shift.clockOutAt ? new Date(shift.clockOutAt).getTime() : now;
  const onBreak = shift.status === 'on_break' && shift.currentBreakStart ? Math.max(0, now - new Date(shift.currentBreakStart).getTime()) / 1000 : 0;
  return (end - new Date(shift.clockInAt).getTime()) / 1000 - (shift.totalBreakMinutes ?? 0) * 60 - onBreak;
}

/* Live shift card: big work timer, status, and the clock/break controls. Breaks stop the timer, as the server
   also excludes them from recorded work. Location sharing sits underneath because it follows the same state. */
export function ShiftHero({ shift, siteName, canRecord, busy, onPunch }: { shift?: HomeShift; siteName: string; canRecord: boolean; busy: boolean; onPunch: (action: string) => void }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => { if (!shift) return; setNow(Date.now()); const id = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(id); }, [shift?.id, shift?.status]);
  const state = !shift ? 'off' : shift.status === 'on_break' ? 'break' : 'on';
  const label = state === 'off' ? 'Off duty' : state === 'break' ? 'On break' : 'Clocked in';
  return <section className={`${styles.hero} ${state === 'on' ? styles.heroOn : state === 'break' ? styles.heroBreak : ''}`} aria-labelledby="hero-state">
    <p id="hero-state" className={styles.eyebrow}>{label}</p>
    <p className={styles.timer} role="timer" aria-label={shift ? `Recorded work ${clock(workSeconds(shift, now))}` : 'No shift in progress'}>{shift ? clock(workSeconds(shift, now)) : '00:00:00'}</p>
    <p className={styles.heroSite}><MapPin size={15} aria-hidden="true" />{siteName}</p>
    {canRecord ? <div className={styles.heroActions}>
      {!shift ? <button className={styles.heroPrimary} disabled={busy} onClick={() => onPunch('clock-in')}>Clock in<ArrowUpRight size={18} aria-hidden="true" /></button> : <>
        <button className={styles.heroPrimary} disabled={busy} onClick={() => onPunch('clock-out')}><Square size={14} fill="currentColor" aria-hidden="true" />Clock out</button>
        <button className={styles.heroSecondary} disabled={busy} onClick={() => onPunch(shift.status === 'on_break' ? 'end-break' : 'start-break')}><Coffee size={17} aria-hidden="true" />{shift.status === 'on_break' ? 'End break' : 'Take a break'}</button>
      </>}
    </div> : <p className={styles.help}>Clock access has not been granted. Contact your administrator.</p>}
    <div id="gps-slot" />
  </section>;
}

export function StatCards({ refreshKey }: { refreshKey: string }) {
  const [hours, setHours] = useState<Hours | null>(null), [failed, setFailed] = useState(false);
  useEffect(() => {
    let live = true;
    const load = () => fetch('/api/v1/employee/hours', { cache: 'no-store' }).then(r => r.ok ? r.json() : Promise.reject()).then((h: Hours) => { if (live) { setHours(h); setFailed(false); } }).catch(() => { if (live) setFailed(true); });
    load(); const id = setInterval(load, 60_000);
    return () => { live = false; clearInterval(id); };
  }, [refreshKey]);
  const cell = (label: string, value?: number) => <div className={styles.stat}><span className={styles.eyebrow}>{label}</span><strong aria-label={value == null ? `${label}: unavailable` : `${label}: ${value.toFixed(2)} hours`}>{value == null ? '—' : `${value.toFixed(2)}h`}</strong></div>;
  return <div className={styles.stats} aria-label="Hours worked">
    {cell('Today', hours?.todayHours)}{cell('This week', hours?.weekHours)}{cell('This month', hours?.monthHours)}
    <p className={styles.statNote}>{failed ? 'Hours are unavailable right now.' : `Work time without breaks, ${hours?.timezone ?? 'worksite time'}. ${hours?.attribution ?? ''}`}</p>
  </div>;
}

export function RecentPunches({ shifts, timezone }: { shifts: HomeShift[]; timezone: string }) {
  const fmt = (v: string) => new Intl.DateTimeFormat('en-CA', { dateStyle: 'medium', timeStyle: 'short', timeZone: timezone }).format(new Date(v));
  const paid = (m: number) => `${Math.floor(m / 60)}:${pad(m % 60)}`;
  if (!shifts.length) return <section className={styles.punches}><div className={styles.sectionTop}><h2>Recent punches</h2></div><p className={styles.help}>No recorded shifts yet.</p></section>;
  return <section className={styles.punches} aria-labelledby="punches-title"><div className={styles.sectionTop}><h2 id="punches-title">Recent punches</h2><span className={styles.count}>Last {shifts.length}</span></div>
    <div className={styles.tableScroll}><table className={styles.table}><thead><tr><th>In</th><th>Out</th><th>Paid</th><th>Break</th></tr></thead><tbody>
      {shifts.map(s => <tr key={s.id}><td>{fmt(s.clockInAt)}</td><td>{s.clockOutAt ? fmt(s.clockOutAt) : <span className={styles.activePill}>{s.status === 'on_break' ? 'On break' : 'Active'}</span>}</td><td className={styles.mono}>{paid(s.totalWorkMinutes)}</td><td>{s.totalBreakMinutes ? `${s.totalBreakMinutes} min` : '—'}</td></tr>)}
    </tbody></table></div></section>;
}
