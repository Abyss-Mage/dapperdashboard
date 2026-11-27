'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

type Places = { mobile: boolean; valet: boolean };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

/* Which places the website's online booking offers besides the shop (owner, error404 #175). Staff bookings always offer all three. */
export default function BookingPlaces({ base }: { base: string }) {
  const root = `${base}/content`;
  const [form, setForm] = useState<Places | null>(null);
  const [hasSite, setHasSite] = useState<boolean | undefined>(undefined);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const data = await readJson(await fetch(root, { cache: 'no-store' }));
    setHasSite(!!data.site?.slug);
    setForm({ mobile: data.bookingPlaces?.mobile ?? true, valet: data.bookingPlaces?.valet ?? true });
  }, [root]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const saved = (await readJson(await fetch(`${root}/booking-places`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(form) }))) as Places;
      setForm(saved);
      setNotice(!saved.mobile && !saved.valet ? 'Saved. Online bookings are in the shop only, so the "where" question is skipped.' : 'Saved. The online booking offers what is ticked, plus the shop.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The change could not be saved.'); }
    finally { setBusy(false); }
  }

  if (hasSite === false) return null;
  return (
    <form className={styles.resource} onSubmit={save}>
      <h3 style={{ margin: 0 }}>Where customers can book online</h3>
      <p className={styles.personSub}>In the shop is always offered. Switch off a service you are not taking right now; bookings made in the dashboard can still choose it.</p>
      <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}><input type="checkbox" checked={form?.mobile ?? true} disabled={busy || !form} onChange={(e) => { setForm((f) => f && { ...f, mobile: e.target.checked }); setNotice(''); }} />Mobile detailing (&ldquo;We come to you&rdquo;)</label>
      <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}><input type="checkbox" checked={form?.valet ?? true} disabled={busy || !form} onChange={(e) => { setForm((f) => f && { ...f, valet: e.target.checked }); setNotice(''); }} />Pick-up and drop-off</label>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <div className={styles.svcActions}><button className="primary" disabled={busy || !form}>{busy ? 'Saving…' : 'Save'}</button></div>
    </form>
  );
}
