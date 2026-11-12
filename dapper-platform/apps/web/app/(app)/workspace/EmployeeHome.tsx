'use client';
import { Coffee, LogIn, LogOut, MapPin, Play } from 'lucide-react';
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

/* Live shift card: big work timer and status. Punches happen on the shop iPad (2026-11-11); breaks stop the timer, as the server
   also excludes them from recorded work. On the shop iPad (the worker opened this with their PIN) the card has the clock buttons
   itself; anywhere else it says how to get to them (error404 #28). */
type Action = 'clock-in' | 'start-break' | 'end-break' | 'clock-out';
export function ShiftHero({ shift: given, siteName }: { shift?: HomeShift; siteName: string }) {
  const [now, setNow] = useState(() => Date.now());
  const [onShopIpad, setOnShopIpad] = useState(false);
  const [own, setOwn] = useState<HomeShift | null | undefined>(undefined);
  const [busy, setBusy] = useState(false), [note, setNote] = useState('');
  useEffect(() => { setOnShopIpad(document.cookie.split('; ').includes('dapper_shop_worker=1')); }, []);
  const shift = own === undefined ? given : own ?? undefined;
  useEffect(() => { if (!shift) return; setNow(Date.now()); const id = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(id); }, [shift?.id, shift?.status]);
  async function punch(action: Action) {
    setBusy(true); setNote('');
    try {
      const r = await fetch('/api/v1/shop/me', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action }) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'That did not work. Try again.');
      setOwn(b.shift && b.shift.status !== 'clocked_out' ? b.shift : null);
    } catch (e) { setNote(e instanceof Error ? e.message : 'That did not work. Try again.'); } finally { setBusy(false); }
  }
  const state = !shift ? 'off' : shift.status === 'on_break' ? 'break' : 'on';
  const label = state === 'off' ? 'Off duty' : state === 'break' ? 'On break' : 'Clocked in';
  return <section className={`${styles.hero} ${state === 'on' ? styles.heroOn : state === 'break' ? styles.heroBreak : ''}`} aria-labelledby="hero-state">
    <p id="hero-state" className={styles.eyebrow}>{label}</p>
    <p className={styles.timer} role="timer" aria-label={shift ? `Recorded work ${clock(workSeconds(shift, now))}` : 'No shift in progress'}>{shift ? clock(workSeconds(shift, now)) : '00:00:00'}</p>
    <p className={styles.heroSite}><MapPin size={15} aria-hidden="true" />{siteName}</p>
    {onShopIpad ? <>
      <div className={styles.punchRow}>
        {!shift && <button type="button" className={styles.punchGo} disabled={busy} onClick={() => void punch('clock-in')}><LogIn size={22} aria-hidden="true" />Clock in</button>}
        {shift?.status === 'clocked_in' && <button type="button" className={styles.punchPause} disabled={busy} onClick={() => void punch('start-break')}><Coffee size={22} aria-hidden="true" />Start break</button>}
        {shift?.status === 'on_break' && <button type="button" className={styles.punchGo} disabled={busy} onClick={() => void punch('end-break')}><Play size={22} aria-hidden="true" />End break</button>}
        {shift && <button type="button" className={styles.punchStop} disabled={busy} onClick={() => void punch('clock-out')}><LogOut size={22} aria-hidden="true" />Clock out</button>}
      </div>
      {note && <p role="alert" className={styles.help}>{note}</p>}
    </> : <div className={styles.help}>
      <p style={{ margin: '0 0 6px' }}>Clock in, breaks and clock out are on the shop iPad:</p>
      <ol style={{ margin: 0, paddingLeft: 20 }}>
        <li>On the shop iPad (the workspace signed in with the shop login), find your name on the clock page.</li>
        <li>Tap your name and type your 6-digit PIN. Your manager gives you the PIN (they set it under Staff hours, Shop iPad).</li>
        <li>Your dashboard opens there with Clock in, Start break and Clock out at the top. Tap Done when you finish.</li>
      </ol>
    </div>}
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
