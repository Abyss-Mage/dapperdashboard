'use client';

import { useCallback, useEffect, useState } from 'react';
import { Archive, Check, Inbox, Mail, Phone, RefreshCw, RotateCcw, Trash2 } from 'lucide-react';
import { askConfirm } from '../confirm';
import styles from './owner.module.css';

type Enquiry = { id: string; name: string; email: string; phone: string; vehicle: string; service: string; message: string; status: 'new' | 'read' | 'archived'; createdAt: string };
// Each enquiry sits in exactly one folder, so marking it read or archiving it moves it out of the one on screen.
const FILTERS: [string, string][] = [['new', 'New'], ['read', 'Read'], ['archived', 'Archived']];
const when = (iso: string) => new Date(iso).toLocaleString('en-CA', { dateStyle: 'medium', timeStyle: 'short' });

/* Messages sent from the public website's contact form. Staff reply by phone or email and sort them new, read or archived. */
export default function Enquiries({ organizationId, canManage }: { organizationId: string; canManage: boolean }) {
  const base = `/api/v1/organizations/${organizationId}/website-enquiries`;
  const [status, setStatus] = useState('new');
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

  async function remove(id: string, name: string) {
    if (!(await askConfirm({ title: 'Delete this enquiry?', message: `The message from ${name || 'this person'} is erased for good. This cannot be undone.`, confirmLabel: 'Delete', tone: 'danger' }))) return;
    setBusy(true); setError('');
    try {
      const r = await fetch(`${base}/${id}`, { method: 'DELETE' });
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'The enquiry could not be deleted.');
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The enquiry could not be deleted.'); }
    finally { setBusy(false); }
  }
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
        <div className={styles.segments} role="group" aria-label="Enquiry folders">
          {FILTERS.map(([id, label]) => (
            <button key={id} type="button" className={styles.segment} aria-pressed={status === id} onClick={() => setStatus(id)}>
              {label}{id === 'new' && fresh ? <span className={styles.enqCount}>{fresh}</span> : null}
            </button>
          ))}
        </div>
        <button type="button" className="secondary" disabled={busy} onClick={() => void load().catch((e) => setError(e.message))}><RefreshCw size={15} aria-hidden="true" /> Refresh</button>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading enquiries" />}
      {items && items.length === 0 && (
        <div className={styles.enqEmpty}>
          <Inbox size={28} aria-hidden="true" />
          <p>{status === 'new' ? 'You are all caught up. New messages from the website land here.' : status === 'read' ? 'Enquiries you mark as read appear here.' : 'Archived enquiries appear here.'}</p>
        </div>
      )}
      <ul className={styles.enqList}>
        {(items ?? []).map((e) => (
          <li key={e.id} className={styles.enq} data-new={e.status === 'new' || undefined}>
            <span className={styles.enqAvatar} aria-hidden="true">{(e.name.trim()[0] ?? '?').toUpperCase()}</span>
            <div className={styles.enqBody}>
              <div className={styles.enqHead}>
                <strong>{e.name}</strong>
                {e.status === 'new' && <span className={styles.enqNew}>New</span>}
                <time className={styles.enqWhen} dateTime={e.createdAt}>{when(e.createdAt)}</time>
              </div>
              {(e.service || e.vehicle) && <p className={styles.enqAbout}>{[e.service, e.vehicle].filter(Boolean).join(' · ')}</p>}
              {e.message && <p className={styles.enqMsg}>{e.message}</p>}
              <div className={styles.enqFoot}>
                <div className={styles.enqChips}>
                  {e.phone && <a href={`tel:${e.phone}`}><Phone size={14} aria-hidden="true" />{e.phone}</a>}
                  {e.email && <a href={`mailto:${e.email}`}><Mail size={14} aria-hidden="true" />{e.email}</a>}
                </div>
                {canManage && (
                  <div className={styles.enqAct}>
                    {e.status === 'new' && <button type="button" className="secondary" disabled={busy} onClick={() => void mark(e.id, 'read')}><Check size={15} aria-hidden="true" /> Mark as read</button>}
                    {e.status !== 'archived' && <button type="button" className="secondary" disabled={busy} onClick={() => void mark(e.id, 'archived')}><Archive size={15} aria-hidden="true" /> Archive</button>}
                    {e.status !== 'new' && <button type="button" className="secondary" disabled={busy} onClick={() => void mark(e.id, 'new')}><RotateCcw size={15} aria-hidden="true" /> Move to New</button>}
                    {e.status === 'archived' && <button type="button" className="secondary" disabled={busy} onClick={() => void remove(e.id, e.name)}><Trash2 size={15} aria-hidden="true" /> Delete</button>}
                  </div>
                )}
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
