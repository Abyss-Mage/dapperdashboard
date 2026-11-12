'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';
import FormPopup from '../FormPopup';

type Employee = { principalId: string; name: string; email: string; phone: string; active: boolean };
type Invitation = { id: string; email: string; deliveryState: string; expiresAt: string };

async function read<T>(r: Response): Promise<T> {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? 'That could not be done.');
  return b as T;
}
const OUTCOME: Record<string, string> = { sent: 'Invitation emailed.', failed: 'The email could not be sent. Revoke it and try again, or ask the head office to check the email settings.', not_configured: 'Invitation saved, but outgoing email is not set up.', queued: 'Invitation saved. The email goes out within a minute.' };

/* A franchise owner's own team: the Employees of this franchise and the invitations still open. The server fixes the role and franchise. */
export default function FranchiseEmployees({ base, franchiseName }: { base: string; franchiseName: string }) {
  const team = `${base}/team`;
  const [employees, setEmployees] = useState<Employee[] | null>(null);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [email, setEmail] = useState('');
  const [editing, setEditing] = useState<Employee | null>(null), [name, setName] = useState(''), [phone, setPhone] = useState('');
  const [confirm, setConfirm] = useState<Employee | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const b = await fetch(team, { cache: 'no-store' }).then(read<{ employees: Employee[]; invitations: Invitation[] }>);
    setEmployees(b.employees); setInvitations(b.invitations);
  }, [team]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function invite(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      const b = await fetch(`${team}/invitations`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: email.trim() }) }).then(read<{ emailStatus: string }>);
      setNotice(OUTCOME[b.emailStatus] ?? OUTCOME.queued); setEmail('');
      await load();
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  async function act(fn: () => Promise<string>) {
    setBusy(true); setError(''); setNotice('');
    try { const done = await fn(); setEditing(null); setConfirm(null); setNotice(done); await load(); }
    catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  const member = (p: Employee) => `${team}/employees/${p.principalId}`;
  const saveEdit = (e: React.FormEvent) => {
    e.preventDefault();
    if (editing) void act(async () => { await fetch(member(editing), { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, phone }) }).then(read); return 'Details saved.'; });
  };
  const setActive = (p: Employee, action: 'remove' | 'reinstate') => act(async () => { await fetch(`${member(p)}/${action}`, { method: 'POST' }).then(read); return `${p.name || p.email} ${action === 'remove' ? 'was removed and signed out.' : 'can sign in again.'}`; });
  async function revoke(i: Invitation) {
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(`${team}/invitations/${i.id}/revoke`, { method: 'POST' });
      if (!r.ok) await read(r);
      setNotice(`Invitation to ${i.email} withdrawn.`);
      await load();
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }

  return (
    <div className={styles.stack}>
      <div>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Employees at {franchiseName}</h2>
        <p className={styles.personSub}>Invite the people who work at this franchise. They join as an Employee of {franchiseName}.</p>
      </div>
      <form onSubmit={invite} className={styles.inlineForm}>
        <label className="form-field">Email address<input type="email" required maxLength={254} value={email} disabled={busy} onChange={(e) => setEmail(e.target.value)} /></label>
        <button className="primary" disabled={busy || !email.trim()}>{busy ? 'Sending…' : 'Invite employee'}</button>
      </form>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {invitations.length > 0 && <>
        <h3 style={{ margin: 0, fontSize: 16 }}>Waiting to accept</h3>
        <ul className={styles.people}>
          {invitations.map((i) => (
            <li key={i.id} className={styles.person} style={{ gridTemplateColumns: 'minmax(0,1fr) auto', alignItems: 'center' }}>
              <span><strong className={styles.personName}>{i.email}</strong><span className={styles.personSub} style={{ display: 'block' }}>Email {i.deliveryState}</span></span>
              <button type="button" className="secondary" disabled={busy} onClick={() => void revoke(i)}>Revoke</button>
            </li>
          ))}
        </ul>
      </>}
      <h3 style={{ margin: 0, fontSize: 16 }}>Team</h3>
      {!employees && !error && <p className={styles.status}>Loading…</p>}
      {employees?.length === 0 && <p className={styles.status}>No employees yet.</p>}
      <ul className={styles.people}>
        {employees?.map((p) => (
          <li key={p.principalId} className={styles.person} style={{ gridTemplateColumns: 'minmax(0,1fr) auto', alignItems: 'center' }}>
            <span><strong className={styles.personName}>{p.name || p.email}</strong>{p.name && <span className={styles.personSub} style={{ display: 'block' }}>{[p.email, p.phone].filter(Boolean).join(' · ')}</span>}</span>
            <div className={styles.svcActions} style={{ alignItems: 'center' }}>
              <span className={`${styles.badge} ${p.active ? styles.badge_complete : ''}`}>{p.active ? 'Active' : 'Removed'}</span>
              {p.active && <button type="button" className="secondary" disabled={busy} onClick={() => { setError(''); setName(p.name); setPhone(p.phone); setEditing(p); }}>Edit</button>}
              {p.active
                ? <button type="button" className="secondary" disabled={busy} onClick={() => setConfirm(p)}>Remove</button>
                : <button type="button" className="secondary" disabled={busy} onClick={() => void setActive(p, 'reinstate')}>Bring back</button>}
            </div>
          </li>
        ))}
      </ul>
      {editing && (
        <FormPopup label={`Edit ${editing.name || editing.email}`} width={460} onClose={() => { if (!busy) setEditing(null); }}>
          <form onSubmit={saveEdit} style={{ display: 'grid', gap: 14 }}>
            <h3 style={{ margin: 0 }}>Edit {editing.name || editing.email}</h3>
            <label className="form-field" style={{ margin: 0 }}>Full name<input required minLength={2} maxLength={120} value={name} disabled={busy} onChange={(e) => setName(e.target.value)} /></label>
            <label className="form-field" style={{ margin: 0 }}>Phone (optional)<input type="tel" maxLength={30} value={phone} disabled={busy} placeholder="+1 705 555 0100" onChange={(e) => setPhone(e.target.value)} /></label>
            {error && <p role="alert" className="field-error">{error}</p>}
            <div className={styles.rowBetween}>
              <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
              <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
            </div>
          </form>
        </FormPopup>
      )}
      <ConfirmDialog isOpen={confirm !== null} title="Remove employee" entityName={confirm?.name || confirm?.email || ''} entityType="Employee" actionType="archive" isBusy={busy} confirmLabel="Remove"
        explanation="They are signed out and can no longer sign in. Their work stays in your records, and you can bring them back later."
        onConfirm={async () => { if (confirm) await setActive(confirm, 'remove'); }} onCancel={() => { if (!busy) setConfirm(null); }} />
    </div>
  );
}
