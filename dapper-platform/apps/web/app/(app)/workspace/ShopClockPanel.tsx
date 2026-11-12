'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { ArrowLeft, Coffee, Delete, LayoutDashboard, LogIn, LogOut, Play } from 'lucide-react';
import styles from './shopClock.module.css';
import { askConfirm } from '../confirm';

/* The workspace's clock page on the shop iPad (owner, 2026-11-12): the workspace is signed in with the franchise's shop login, and its
   clock in / out page is the shop's name list. A worker taps their name and types their 6-digit PIN, then clocks in, takes a break or
   clocks out right here, or opens their own dashboard. After a punch, or 30 seconds without a tap, it goes back to the names. */

type Worker = { principalId: string; name: string; status: 'clocked_in' | 'on_break' | 'clocked_out' };
type Shift = { status: 'clocked_in' | 'on_break' | 'clocked_out'; clockInAt: string; currentBreakStart?: string | null };
type Action = 'clock-in' | 'start-break' | 'end-break' | 'clock-out';
const STATUS: Record<Worker['status'], string> = { clocked_in: 'On shift', on_break: 'On break', clocked_out: 'Not clocked in' };
const DONE: Record<Action, string> = { 'clock-in': 'You are clocked in.', 'start-break': 'Enjoy your break.', 'end-break': 'Welcome back. Break ended.', 'clock-out': 'You are clocked out. See you next time.' };
const time = (iso: string) => new Date(iso).toLocaleTimeString('en-CA', { hour: 'numeric', minute: '2-digit' });
const PIN = 6;

// The Monday of this week, the first week a worker can set availability for.
const thisMonday = () => { const t = new Date(); t.setDate(t.getDate() - ((t.getDay() + 6) % 7)); return t.toLocaleDateString('en-CA'); };

/* mode "availability" (owner, 2026-11-12: the same system for availability): after the PIN, the worker's availability form instead of
   the clock buttons. */
export default function ShopClockPanel({ onSignedOut, mode = 'clock', renderAvailability }: { onSignedOut: () => void; mode?: 'clock' | 'availability'; renderAvailability?: (worker: { principalId: string; name: string }, pin: string, data: unknown, back: () => void) => ReactNode }) {
  const [shopLogin, setShopLogin] = useState(false);
  const [avail, setAvail] = useState<unknown>(null);
  const [workers, setWorkers] = useState<Worker[] | null>(null);
  const [who, setWho] = useState<Worker | null>(null);
  const [pin, setPin] = useState('');
  const [shift, setShift] = useState<Shift | null | undefined>(undefined); // undefined: PIN not checked yet
  const [done, setDone] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState('');

  const load = useCallback(async () => {
    const r = await fetch('/api/v1/shop/workers', { cache: 'no-store' });
    if (r.status === 401) { onSignedOut(); return; }
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'The names could not be loaded.');
    setWorkers(b.workers ?? []); setShopLogin(!!b.shopLogin);
  }, [onSignedOut]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);
  const reset = useCallback(() => { setWho(null); setPin(''); setShift(undefined); setAvail(null); setDone(''); setError(''); void load().catch(() => undefined); }, [load]);
  // Back to the names after a punch (4 seconds) or 30 seconds without a tap.
  useEffect(() => { if (!who || avail) return; const t = setTimeout(reset, done ? 4000 : 30000); return () => clearTimeout(t); }, [who, pin, shift, done, reset, avail]);

  async function call(action: 'status' | Action, code = pin) {
    if (!who) return;
    setBusy(true); setError('');
    if (mode === 'availability') {
      try {
        const r = await fetch(`/api/v1/shop/workers/${who.principalId}/availability`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pin: code, week: thisMonday() }) });
        const b = await r.json().catch(() => ({}));
        if (r.status === 401 && b.code === 'shop_signed_out') { onSignedOut(); return; }
        if (!r.ok) throw new Error(b.error ?? 'That did not work. Try again.');
        setAvail(b); setShift(null);
      } catch (e) { setError(e instanceof Error ? e.message : 'That did not work. Try again.'); setPin(''); } finally { setBusy(false); }
      return;
    }
    try {
      const r = await fetch(`/api/v1/shop/workers/${who.principalId}/punch`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pin: code, action }) });
      const b = await r.json().catch(() => ({}));
      if (r.status === 401 && b.code === 'shop_signed_out') { onSignedOut(); return; }
      if (!r.ok) throw new Error(b.error ?? 'That did not work. Try again.');
      setShift(b.shift && b.shift.status !== 'clocked_out' ? b.shift : null);
      if (action !== 'status') setDone(DONE[action]);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'That did not work. Try again.');
      if (action === 'status') setPin('');
    } finally { setBusy(false); }
  }
  async function openDashboard() {
    if (!who) return;
    setBusy(true); setError('');
    try {
      const r = await fetch(`/api/v1/shop/workers/${who.principalId}/unlock`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pin }) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'That did not work. Try again.');
      window.location.assign('/workspace');
    } catch (e) { setError(e instanceof Error ? e.message : 'That did not work. Try again.'); setBusy(false); }
  }
  const press = (d: string) => {
    if (busy || !who || shift !== undefined) return;
    setError('');
    const next = (pin + d).slice(0, PIN);
    setPin(next);
    if (next.length === PIN) void call('status', next);
  };
  useEffect(() => {
    if (!who || shift !== undefined) return;
    const key = (e: KeyboardEvent) => { if (/^[0-9]$/.test(e.key)) press(e.key); else if (e.key === 'Backspace') setPin((p) => p.slice(0, -1)); else if (e.key === 'Escape') reset(); };
    addEventListener('keydown', key);
    return () => removeEventListener('keydown', key);
  });
  async function signOutShop() {
    if (!(await askConfirm({ title: 'Sign this iPad out of the shop?', message: 'Workers cannot clock in here until someone signs in again with the shop username and password.', confirmLabel: 'Sign out' }))) return;
    await fetch('/api/v1/shop/logout', { method: 'POST' }).catch(() => undefined);
    onSignedOut();
  }

  if (!who) return (
    <section className={styles.panel} aria-labelledby="shop-names" data-no-motion="">
      <h2 id="shop-names">Tap your name</h2>
      {error && <p role="alert" className={styles.error}>{error}</p>}
      {!workers && !error && <p className={styles.help}>Loading…</p>}
      {workers?.length === 0 && <p className={styles.help}>No one has a PIN yet. The owner sets PINs under Staff hours, Shop iPad.</p>}
      {!!workers?.length && <ul className={styles.grid}>{workers.map((w) => (
        <li key={w.principalId}><button type="button" onClick={() => { setWho(w); setPin(''); setShift(undefined); setDone(''); setError(''); }}><strong>{w.name}</strong><span data-s={w.status}><i aria-hidden="true" />{STATUS[w.status]}</span></button></li>
      ))}</ul>}
      {shopLogin && <button type="button" className={styles.link} onClick={() => void signOutShop()}>Sign this iPad out of the shop</button>}
    </section>
  );

  if (shift === undefined) return (
    <section className={`${styles.panel} ${styles.center}`} aria-live="polite" data-no-motion="">
      <h2>Hi, {who.name.split(' ')[0]}</h2>
      <div className={styles.dots} aria-label={`${pin.length} of ${PIN} digits entered`}>{Array.from({ length: PIN }, (_, i) => <i key={i} data-on={i < pin.length || undefined} />)}</div>
      {error ? <p role="alert" className={styles.error}>{error}</p> : <p className={styles.help}>{busy ? 'Checking…' : mode === 'availability' ? 'Type your 6-digit PIN to set your availability.' : 'Type your 6-digit PIN.'}</p>}
      <div className={styles.pad}>
        {['1', '2', '3', '4', '5', '6', '7', '8', '9'].map((d) => <button key={d} type="button" onClick={() => press(d)} disabled={busy}>{d}</button>)}
        <button type="button" className={styles.soft} onClick={() => { setPin(''); setError(''); }} disabled={busy || !pin}>Clear</button>
        <button type="button" onClick={() => press('0')} disabled={busy}>0</button>
        <button type="button" className={styles.soft} onClick={() => setPin((p) => p.slice(0, -1))} disabled={busy || !pin} aria-label="Delete last digit"><Delete size={26} aria-hidden="true" /></button>
      </div>
      <button type="button" className={`${styles.soft} ${styles.wide}`} onClick={reset} disabled={busy}><ArrowLeft size={20} aria-hidden="true" /> Not you? Back to names</button>
    </section>
  );

  if (mode === 'availability' && avail && renderAvailability) return (
    <section className={styles.panel} data-no-motion="">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <h2>{who.name}</h2>
        <button type="button" className={styles.soft} onClick={reset}><ArrowLeft size={20} aria-hidden="true" /> Done, back to names</button>
      </div>
      {renderAvailability(who, pin, avail, reset)}
    </section>
  );

  return (
    <section className={`${styles.panel} ${styles.center}`} aria-live="polite" data-no-motion="">
      <h2>{done ? `Thanks, ${who.name.split(' ')[0]}.` : `Hi, ${who.name.split(' ')[0]}`}</h2>
      <p className={done ? styles.doneLine : styles.state}>{done || (!shift ? 'You are not clocked in.' : shift.status === 'on_break' ? `On break since ${time(shift.currentBreakStart ?? shift.clockInAt)}` : `Clocked in since ${time(shift.clockInAt)}`)}</p>
      {error && <p role="alert" className={styles.error}>{error}</p>}
      {!done && <div className={styles.actions}>
        {!shift && <button type="button" className={styles.go} disabled={busy} onClick={() => void call('clock-in')}><LogIn size={28} aria-hidden="true" />Clock in</button>}
        {shift?.status === 'clocked_in' && <button type="button" className={styles.pause} disabled={busy} onClick={() => void call('start-break')}><Coffee size={28} aria-hidden="true" />Start break</button>}
        {shift?.status === 'on_break' && <button type="button" className={styles.go} disabled={busy} onClick={() => void call('end-break')}><Play size={28} aria-hidden="true" />End break</button>}
        {shift && <button type="button" className={styles.stop} disabled={busy} onClick={() => void call('clock-out')}><LogOut size={28} aria-hidden="true" />Clock out</button>}
        {shopLogin && <button type="button" className={styles.soft} disabled={busy} onClick={() => void openDashboard()}><LayoutDashboard size={24} aria-hidden="true" />Open my dashboard</button>}
      </div>}
      <button type="button" className={`${styles.soft} ${styles.wide}`} onClick={reset} disabled={busy}>{done ? 'Done' : 'Not you? Back to names'}</button>
    </section>
  );
}
