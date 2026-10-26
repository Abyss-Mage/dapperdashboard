'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import Modal from '../Modal';

/* Staff availability: each person sends the times they can work next week. The list shows one row per person (their name,
   never their email); opening a person shows every week they sent as a small day-by-day table with Approve / Reject.
   A changed week comes back as waiting. */

type Submission = { id: string; principalId: string; locationId: string; locationName: string; weekStart: string; timezone: string; intervals: { day: number; start: string; end: string }[]; notes: string; version: number; submittedAt: string; status: 'submitted' | 'approved' | 'rejected'; reviewedAt: string | null; reviewNote: string };
type Member = { principalId: string; email: string; name?: string };
const DAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const STATUS: Record<Submission['status'], string> = { submitted: 'Waiting', approved: 'Approved', rejected: 'Rejected' };
type Filter = 'submitted' | 'approved' | 'rejected' | 'all';

async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Staff availability is unavailable.'); return b as T; }
const weekLabel = (w: string) => new Date(`${w}T12:00:00`).toLocaleDateString('en-CA', { month: 'short', day: 'numeric', year: 'numeric' });
const badgeClass = (s: Submission['status']) => `${styles.badge} ${s === 'approved' ? styles.badge_complete : s === 'submitted' ? styles.badge_incomplete : ''}`;

export default function AvailabilityReview({ organizationId, locationId }: { organizationId: string; locationId?: string }) {
  const base = `/api/v1/organizations/${organizationId}`;
  const [items, setItems] = useState<Submission[] | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [filter, setFilter] = useState<Filter>('submitted');
  const [openId, setOpenId] = useState('');
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const [w, a] = await Promise.all([fetch(`${base}/employee-worksites`, { cache: 'no-store' }).then(read<{ submissions: Submission[] }>), fetch(`${base}/access`, { cache: 'no-store' }).then(read<{ members: Member[] }>)]);
    setItems(w.submissions); setMembers(a.members);
  }, [base]);
  useEffect(() => { setItems(null); setError(''); load().catch((e) => setError(e.message)); }, [load]);

  const nameOf = useCallback((id: string) => { const m = members.find((x) => x.principalId === id); return m?.name?.trim() || (m?.email ? m.email.split('@')[0] : '') || 'Team member'; }, [members]);
  const here = useMemo(() => (items ?? []).filter((s) => !locationId || s.locationId === locationId), [items, locationId]);
  const people = useMemo(() => {
    const byPerson = new Map<string, Submission[]>();
    for (const s of here) byPerson.set(s.principalId, [...(byPerson.get(s.principalId) ?? []), s]);
    return [...byPerson.entries()]
      .map(([id, subs]) => ({ id, name: nameOf(id), subs: subs.sort((a, b) => b.weekStart.localeCompare(a.weekStart)) }))
      .filter((p) => filter === 'all' || p.subs.some((s) => s.status === filter))
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [here, filter, nameOf]);
  const count = (f: Filter) => new Set(here.filter((s) => f === 'all' || s.status === f).map((s) => s.principalId)).size;
  const open = people.find((p) => p.id === openId) ?? (openId ? { id: openId, name: nameOf(openId), subs: here.filter((s) => s.principalId === openId).sort((a, b) => b.weekStart.localeCompare(a.weekStart)) } : null);

  async function decide(s: Submission, decision: 'approved' | 'rejected') {
    setBusy(s.id); setError(''); setNotice('');
    try {
      await read(await fetch(`${base}/employee-availability/${s.id}/review`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ decision, note: notes[s.id] ?? '', version: s.version }) }));
      setNotice(`${nameOf(s.principalId)}: availability ${decision === 'approved' ? 'approved' : 'rejected'}.`);
      await load();
    } catch (e) { setError((e as Error).message); } finally { setBusy(''); }
  }

  return (
    <div className={styles.stack}>
      <div>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Staff availability</h2>
        <p className={styles.personSub}>The times each person says they can work. Open a person to see their weeks and approve or reject them.</p>
      </div>
      <div className={styles.segments} role="group" aria-label="Show">
        {(['submitted', 'approved', 'rejected', 'all'] as Filter[]).map((f) => <button key={f} type="button" className={styles.segment} aria-pressed={filter === f} onClick={() => setFilter(f)}>{f === 'all' ? 'Everyone' : STATUS[f]} ({count(f)})</button>)}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading availability" />}
      {items && people.length === 0 && <p className={styles.status}>{filter === 'submitted' ? 'Nothing waiting for a decision.' : 'Nothing here.'}</p>}
      <ul className={styles.people}>
        {people.map((p) => {
          const waiting = p.subs.filter((s) => s.status === 'submitted').length, latest = p.subs[0];
          return (
            <li key={p.id} className={styles.person} style={{ gridTemplateColumns: 'minmax(0,1fr) auto', alignItems: 'center' }}>
              <button type="button" onClick={() => setOpenId(p.id)} style={{ all: 'unset', cursor: 'pointer', display: 'flex', flexDirection: 'column', gap: 3, minWidth: 0 }} aria-label={`Open ${p.name}'s availability`}>
                <strong className={styles.personName}>{p.name}</strong>
                <span className={styles.personSub}>{p.subs.length} week{p.subs.length === 1 ? '' : 's'} sent · latest: week of {weekLabel(latest.weekStart)}</span>
              </button>
              {waiting > 0 ? <span className={`${styles.badge} ${styles.badge_incomplete}`}>{waiting} waiting</span> : <span className={`${styles.badge} ${styles.badge_complete}`}>Up to date</span>}
            </li>
          );
        })}
      </ul>

      <Modal open={!!open} title={open ? `${open.name}'s availability` : ''} subtitle="The times they can work, week by week" onClose={() => setOpenId('')} wide>
        {open && open.subs.length === 0 && <p className={styles.status}>No weeks sent yet.</p>}
        {open?.subs.map((s) => (
          <section key={s.id} style={{ border: '1px solid var(--border)', borderRadius: 16, padding: 14, marginBottom: 14 }}>
            <div className={styles.rowBetween}>
              <div><strong>Week of {weekLabel(s.weekStart)}</strong><div className={styles.personSub}>{s.locationName} · sent {new Date(s.submittedAt).toLocaleString('en-CA', { dateStyle: 'medium', timeStyle: 'short' })}</div></div>
              <span className={badgeClass(s.status)}>{STATUS[s.status]}</span>
            </div>
            <table className={styles.table} style={{ marginTop: 10 }}>
              <thead><tr><th scope="col">Day</th><th scope="col">Can work</th></tr></thead>
              <tbody>
                {DAYS.map((d, i) => { const r = s.intervals.filter((x) => x.day === i); return <tr key={d}><th scope="row" style={{ fontWeight: 600 }}>{d}</th><td>{r.length ? r.map((x) => `${x.start}–${x.end}`).join(', ') : <span className={styles.personSub}>Not available</span>}</td></tr>; })}
              </tbody>
            </table>
            {s.notes && <p className={styles.personSub}>Their note: {s.notes}</p>}
            {s.reviewNote && <p className={styles.personSub}>Your note: {s.reviewNote}</p>}
            {s.status === 'submitted' && (
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'flex-end', marginTop: 10 }}>
                <label className="form-field" style={{ flex: '1 1 220px', margin: 0 }}>Note (optional)<input maxLength={500} value={notes[s.id] ?? ''} onChange={(e) => setNotes({ ...notes, [s.id]: e.target.value })} disabled={!!busy} /></label>
                <button type="button" className="primary" disabled={!!busy} onClick={() => void decide(s, 'approved')}>{busy === s.id ? 'Saving…' : 'Approve'}</button>
                <button type="button" className="secondary" disabled={!!busy} onClick={() => void decide(s, 'rejected')}>Reject</button>
              </div>
            )}
          </section>
        ))}
      </Modal>
    </div>
  );
}
