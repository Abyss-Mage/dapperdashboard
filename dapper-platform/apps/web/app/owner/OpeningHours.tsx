'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

/* A franchise is one place with one set of opening hours. Behind the scenes the franchise may book work on more than one
   bay or crew; the hours entered here are saved to all of them, so the owner only ever sets hours once. */

type Hour = { day: number; start: string; end: string };
type Resource = { id: string; name: string; weeklyHours: Hour[] | null };
type Window = { start: string; end: string };
const DAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The hours could not be saved.'));
  return body;
}

export default function OpeningHours({ base, franchiseName, canManage }: { base: string; franchiseName: string; canManage: boolean }) {
  const [resources, setResources] = useState<Resource[] | null>(null);
  const [days, setDays] = useState<Window[][]>(DAYS.map(() => []));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const list: Resource[] = (await readJson(await fetch(`${base}/scheduling/resources`, { cache: 'no-store' }))).items ?? [];
    setResources(list);
    const source = list.find((r) => (r.weeklyHours?.length ?? 0) > 0);
    setDays(DAYS.map((_, d) => (source?.weeklyHours ?? []).filter((h) => h.day === d).map((h) => ({ start: h.start, end: h.end }))));
  }, [base]);
  useEffect(() => { setResources(null); setError(''); setNotice(''); load().catch((e) => setError(e.message)); }, [load]);

  const set = (d: number, i: number, key: 'start' | 'end', v: string) => setDays((c) => c.map((w, di) => (di === d ? w.map((x, xi) => (xi === i ? { ...x, [key]: v } : x)) : w)));
  const toggle = (d: number, open: boolean) => setDays((c) => c.map((w, di) => (di === d ? (open ? [{ start: '09:00', end: '17:00' }] : []) : w)));
  const addBreak = (d: number) => setDays((c) => c.map((w, di) => (di === d ? [...w, { start: '13:00', end: '17:00' }] : w)));
  const remove = (d: number, i: number) => setDays((c) => c.map((w, di) => (di === d ? w.filter((_, xi) => xi !== i) : w)));
  const copyMonday = () => setDays((c) => c.map((w, di) => (di === 0 ? w : c[0].map((x) => ({ ...x })))));

  async function save() {
    setError(''); setNotice('');
    for (const [d, ws] of days.entries()) for (const w of ws) if (!w.start || !w.end || w.start >= w.end) return setError(`${DAYS[d]}: closing time must be after opening time.`);
    if (!resources?.length) return setError('This franchise has nothing to take bookings yet. Ask your administrator to finish its setup.');
    setBusy(true);
    try {
      const weekly = days.flatMap((ws, day) => ws.map((w) => ({ day, ...w })));
      for (const r of resources) await readJson(await fetch(`${base}/scheduling/resources/${r.id}/hours`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ weekly }) }));
      setNotice('Opening hours saved. Customers can book inside these hours.');
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The hours could not be saved.'); }
    finally { setBusy(false); }
  }

  return (
    <div className={styles.stack}>
      <div>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Opening hours</h2>
        <p className={styles.personSub}>When {franchiseName} is open for bookings. Set each day once; a day with no hours is closed.</p>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {!resources && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading hours" />}
      {resources && (
        <section className={styles.card} style={{ padding: 20 }}>
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
                      <input type="time" aria-label={`${name} opens`} value={w.start} onChange={(e) => set(d, i, 'start', e.target.value)} disabled={busy || !canManage} />
                      <span aria-hidden="true">to</span>
                      <input type="time" aria-label={`${name} closes`} value={w.end} onChange={(e) => set(d, i, 'end', e.target.value)} disabled={busy || !canManage} />
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
              <button type="button" className="primary" disabled={busy} onClick={() => void save()}>{busy ? 'Saving…' : 'Save opening hours'}</button>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
