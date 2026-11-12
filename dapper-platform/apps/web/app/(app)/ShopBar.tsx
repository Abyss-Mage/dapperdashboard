'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { usePathname } from 'next/navigation';
import { Coffee, LogIn, LogOut, Play } from 'lucide-react';
import styles from './ShopBar.module.css';

/* The bar across the top of a worker's dashboard while they use the shop iPad (error404 #20): who is signed in, their shift, the clock
   buttons and Done. Done, or 2 minutes without a tap, signs them out and returns the iPad to the name list. It shows only when the iPad
   opened the session (the dapper_shop_worker mark), and the server checks that again for every punch. */

type Shift = { status: 'clocked_in' | 'on_break' | 'clocked_out'; clockInAt: string; currentBreakStart?: string | null };
type Me = { name: string; shop: string; shift: Shift | null };
type Action = 'clock-in' | 'start-break' | 'end-break' | 'clock-out';
const IDLE_MS = 120_000;
const DONE: Record<Action, string> = { 'clock-in': 'Clocked in.', 'start-break': 'Break started.', 'end-break': 'Break ended.', 'clock-out': 'Clocked out.' };
const time = (iso: string) => new Date(iso).toLocaleTimeString('en-CA', { hour: 'numeric', minute: '2-digit' });

export default function ShopBar() {
  const path = usePathname();
  const [active, setActive] = useState(false);
  const [me, setMe] = useState<Me | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState('');
  const idle = useRef<number | undefined>(undefined);
  const bar = useRef<HTMLDivElement>(null);

  const done = useCallback(async () => {
    clearTimeout(idle.current);
    await fetch('/api/v1/shop/done', { method: 'POST' }).catch(() => undefined);
    window.location.assign('/workspace');
  }, []);

  useEffect(() => { setActive(!path?.startsWith('/shop') && document.cookie.split('; ').includes('dapper_shop_worker=1')); }, [path]);

  useEffect(() => {
    if (!active) return;
    fetch('/api/v1/shop/me', { cache: 'no-store' }).then(async (r) => {
      if (r.status === 401) { void done(); return; }
      if (r.ok) setMe(await r.json());
    }).catch(() => undefined);
    const arm = () => { clearTimeout(idle.current); idle.current = window.setTimeout(() => void done(), IDLE_MS); };
    arm();
    const events = ['pointerdown', 'keydown', 'scroll', 'touchstart'] as const;
    events.forEach((e) => addEventListener(e, arm, { passive: true }));
    return () => { clearTimeout(idle.current); events.forEach((e) => removeEventListener(e, arm)); };
  }, [active, done]);

  // Keep the page's own content below the bar.
  useEffect(() => {
    if (!active || !bar.current) return;
    const set = () => document.body.style.setProperty('padding-top', `${bar.current?.offsetHeight ?? 0}px`);
    set();
    const ro = new ResizeObserver(set);
    ro.observe(bar.current);
    return () => { ro.disconnect(); document.body.style.removeProperty('padding-top'); };
  }, [active, me]);

  async function punch(action: Action) {
    setBusy(true); setNote('');
    try {
      const r = await fetch('/api/v1/shop/me', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ action }) });
      const b = await r.json().catch(() => ({}));
      if (r.status === 401) { void done(); return; }
      if (!r.ok) throw new Error(b.error ?? 'That did not work. Try again.');
      setMe(b); setNote(DONE[action]);
    } catch (e) { setNote(e instanceof Error ? e.message : 'That did not work. Try again.'); } finally { setBusy(false); }
  }

  if (!active) return null;
  const shift = me?.shift && me.shift.status !== 'clocked_out' ? me.shift : null;
  return (
    <div ref={bar} className={styles.bar} role="region" aria-label="Shop iPad">
      <div className={styles.who}>
        <strong>{me?.name || 'Signed in on the shop iPad'}</strong>
        <span aria-live="polite">{note || (!me ? 'Loading…' : !shift ? 'Not clocked in' : shift.status === 'on_break' ? `On break since ${time(shift.currentBreakStart ?? shift.clockInAt)}` : `On shift since ${time(shift.clockInAt)}`)}</span>
      </div>
      <div className={styles.actions}>
        {me && !shift && <button type="button" className={styles.go} disabled={busy} onClick={() => void punch('clock-in')}><LogIn size={22} aria-hidden="true" />Clock in</button>}
        {shift?.status === 'clocked_in' && <button type="button" className={styles.pause} disabled={busy} onClick={() => void punch('start-break')}><Coffee size={22} aria-hidden="true" />Start break</button>}
        {shift?.status === 'on_break' && <button type="button" className={styles.go} disabled={busy} onClick={() => void punch('end-break')}><Play size={22} aria-hidden="true" />End break</button>}
        {shift && <button type="button" className={styles.stop} disabled={busy} onClick={() => void punch('clock-out')}><LogOut size={22} aria-hidden="true" />Clock out</button>}
        <button type="button" className={styles.done} onClick={() => void done()}>Done</button>
      </div>
    </div>
  );
}
