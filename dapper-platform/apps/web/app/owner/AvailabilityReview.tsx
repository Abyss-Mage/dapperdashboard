'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

/* Staff availability: each person sends the times they can work next week. The franchise owner or the Owner/Super Admin
   approves or rejects each one. A changed submission comes back here as waiting. */

type Submission = { id: string; principalId: string; locationId: string; locationName: string; weekStart: string; timezone: string; intervals: { day: number; start: string; end: string }[]; notes: string; version: number; submittedAt: string; status: 'submitted' | 'approved' | 'rejected'; reviewedAt: string | null; reviewNote: string };
type Member = { principalId: string; email: string };
const DAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const STATUS: Record<Submission['status'], string> = { submitted: 'Waiting', approved: 'Approved', rejected: 'Rejected' };
type Filter = 'submitted' | 'approved' | 'rejected' | 'all';

async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Staff availability is unavailable.'); return b as T; }

export default function AvailabilityReview({ organizationId, locationId }: { organizationId: string; locationId?: string }) {
  const base = `/api/v1/organizations/${organizationId}`;
  const [items, setItems] = useState<Submission[] | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [filter, setFilter] = useState<Filter>('submitted');
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const [w, a] = await Promise.all([fetch(`${base}/employee-worksites`, { cache: 'no-store' }).then(read<{ submissions: Submission[] }>), fetch(`${base}/access`, { cache: 'no-store' }).then(read<{ members: Member[] }>).catch(() => ({ members: [] as Member[] }))]);
    setItems(w.submissions); setMembers(a.members);
  }, [base]);
  useEffect(() => { setItems(null); setError(''); load().catch((e) => setError(e.message)); }, [load]);

  async function decide(s: Submission, decision: 'approved' | 'rejected') {
    setBusy(s.id); setError(''); setNotice('');
    try {
      await read(await fetch(`${base}/employee-availability/${s.id}/review`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ decision, note: notes[s.id] ?? '', version: s.version }) }));
      setNotice(`${who(s)}: availability ${decision === 'approved' ? 'approved' : 'rejected'}.`);
      await load();
    } catch (e) { setError((e as Error).message); } finally { setBusy(''); }
  }
  const who = (s: Submission) => members.find((m) => m.principalId === s.principalId)?.email || 'Team member';
  const shown = (items ?? []).filter((s) => (!locationId || s.locationId === locationId) && (filter === 'all' || s.status === filter));
  const count = (f: Filter) => (items ?? []).filter((s) => (!locationId || s.locationId === locationId) && (f === 'all' || s.status === f)).length;

  return (
    <div className={styles.stack}>
      <div>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Staff availability</h2>
        <p className={styles.personSub}>The times each person says they can work. Approve or reject each week; a changed week comes back as waiting.</p>
      </div>
      <div className={styles.segments} role="group" aria-label="Show">
        {(['submitted', 'approved', 'rejected', 'all'] as Filter[]).map((f) => <button key={f} type="button" className={styles.segment} aria-pressed={filter === f} onClick={() => setFilter(f)}>{f === 'all' ? 'All' : STATUS[f as Submission['status']]} ({count(f)})</button>)}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading availability" />}
      {items && shown.length === 0 && <p className={styles.status}>{filter === 'submitted' ? 'Nothing waiting for a decision.' : 'Nothing here.'}</p>}
      <ul className={styles.people}>
        {shown.map((s) => (
          <li key={s.id} className={styles.person} style={{ gridTemplateColumns: 'minmax(0,1fr)', gap: 10 }}>
            <div className={styles.rowBetween}>
              <div className={styles.personMain}>
                <strong className={styles.personName}>{who(s)}</strong>
                <span className={styles.personSub}>{s.locationName} · week of {new Date(`${s.weekStart}T12:00:00`).toLocaleDateString('en-CA', { month: 'long', day: 'numeric', year: 'numeric' })} · sent {new Date(s.submittedAt).toLocaleString('en-CA', { dateStyle: 'medium', timeStyle: 'short' })}</span>
              </div>
              <span className={`${styles.badge} ${s.status === 'approved' ? styles.badge_complete : s.status === 'submitted' ? styles.badge_incomplete : ''}`}>{STATUS[s.status]}</span>
            </div>
            {s.intervals.length ? <div className={styles.svcTags}>{s.intervals.map((r, n) => <span key={n} className={styles.chip}>{DAYS[r.day]} {r.start}–{r.end}</span>)}</div> : <p className={styles.personSub}>Not available this week.</p>}
            {s.notes && <p className={styles.personSub}>Their note: {s.notes}</p>}
            {s.reviewNote && <p className={styles.personSub}>Your note: {s.reviewNote}</p>}
            {s.status === 'submitted' && (
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'flex-end' }}>
                <label className="form-field" style={{ flex: '1 1 240px', margin: 0 }}>Note (optional)<input maxLength={500} value={notes[s.id] ?? ''} onChange={(e) => setNotes({ ...notes, [s.id]: e.target.value })} disabled={!!busy} /></label>
                <button type="button" className="primary" disabled={!!busy} onClick={() => void decide(s, 'approved')}>{busy === s.id ? 'Saving…' : 'Approve'}</button>
                <button type="button" className="secondary" disabled={!!busy} onClick={() => void decide(s, 'rejected')}>Reject</button>
              </div>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
