'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

type Enquiry = { id: string; name: string; email: string; phone: string; vehicle: string; service: string; message: string; status: 'new' | 'read' | 'archived'; createdAt: string };
const FILTERS: [string, string][] = [['', 'Open'], ['new', 'New'], ['read', 'Read'], ['archived', 'Archived']];
const when = (iso: string) => new Date(iso).toLocaleString('en-CA', { dateStyle: 'medium', timeStyle: 'short' });

/* Messages sent from the public website's contact form. Staff reply by phone or email and sort them new, read or archived. */
export default function Enquiries({ organizationId, canManage }: { organizationId: string; canManage: boolean }) {
  const base = `/api/v1/organizations/${organizationId}/website-enquiries`;
  const [status, setStatus] = useState('');
  const [items, setItems] = useState<Enquiry[] | null>(null);
  const [fresh, setFresh] = useState(0);
  const [busy, setBusy] = useState(false), [error, setError] = useState('');

  const load = useCallback(async () => {
    const r = await fetch(`${base}${status ? `?status=${status}` : ''}`, { cache: 'no-store' });
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'Enquiries are unavailable or access has changed.');
    setItems(b.items ?? []); setFresh(b.new ?? 0); setError('');
  }, [base, status]);
  useEffect(() => { setItems(null); load().catch((e) => setError(e.message)); }, [load]);

  async function mark(id: string, next: Enquiry['status']) {
    setBusy(true); setError('');
    try {
      const r = await fetch(`${base}/${id}/status`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status: next }) });
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'The change could not be saved.');
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); }
    finally { setBusy(false); }
  }

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Enquiries{fresh ? ` · ${fresh} new` : ''}</h2>
        </div>
        <button type="button" className="secondary" disabled={busy} onClick={() => void load().catch((e) => setError(e.message))}>Refresh</button>
      </div>
      <div className={styles.segments} role="group" aria-label="Filter enquiries">
        {FILTERS.map(([id, label]) => <button key={id} type="button" className={styles.segment} aria-pressed={status === id} onClick={() => setStatus(id)}>{label}</button>)}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading enquiries" />}
      {items && items.length === 0 && <p className={styles.status}>No enquiries here.</p>}
      <ul className={styles.enqList}>
        {(items ?? []).map((e) => (
          <li key={e.id} className={styles.enq} data-new={e.status === 'new' || undefined}>
            <div className={styles.enqHead}>
              <strong>{e.name}</strong>
              {e.status === 'new' && <span className={styles.enqNew}>New</span>}
              {e.status === 'archived' && <span className={styles.enqOld}>Archived</span>}
              <time className={styles.enqWhen}>{when(e.createdAt)}</time>
            </div>
            <div className={styles.enqChips}>
              {e.phone && <a href={`tel:${e.phone}`}>{e.phone}</a>}
              {e.email && <a href={`mailto:${e.email}`}>{e.email}</a>}
              {(e.service || e.vehicle) && <span>{[e.service, e.vehicle].filter(Boolean).join(' · ')}</span>}
            </div>
            {e.message && <p className={styles.enqMsg}>{e.message}</p>}
            {canManage && (
              <div className={styles.enqAct}>
                {e.status !== 'read' && e.status !== 'archived' && <button type="button" className="secondary" disabled={busy} onClick={() => void mark(e.id, 'read')}>Mark read</button>}
                {e.status !== 'archived' && <button type="button" className="secondary" disabled={busy} onClick={() => void mark(e.id, 'archived')}>Archive</button>}
                {e.status !== 'new' && <button type="button" className="secondary" disabled={busy} onClick={() => void mark(e.id, 'new')}>Mark new</button>}
              </div>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
